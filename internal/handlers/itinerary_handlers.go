package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"karibu-api/internal/database"
	"karibu-api/internal/models"

	"github.com/gin-gonic/gin"
)

// ============================================
// GET /api/v1/itineraries — fetch the user's trips
// ============================================
func GetItineraries(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user context"})
		return
	}

	rows, err := database.DB.Query(`
    SELECT id, name, start_date, end_date, COALESCE(cover_image_url, ''), travelers, budget,
           COALESCE(season, ''), COALESCE(season_note, '')
    FROM itineraries
    WHERE user_id = $1
    ORDER BY start_date ASC
`, userID)
	if err != nil {
		log.Printf("❌ GetItineraries query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load itineraries"})
		return
	}
	defer rows.Close()

	var itineraries []models.Itinerary
	var itineraryIDs []string
	for rows.Next() {
		var i models.Itinerary
		var endDate sql.NullTime
		if err := rows.Scan(&i.ID, &i.Name, &i.StartDate, &endDate, &i.CoverImageURL, &i.Travelers, &i.Budget, &i.Season, &i.SeasonNote); err != nil {
			log.Printf("❌ GetItineraries scan error: %v", err)
			continue
		}
		if endDate.Valid {
			i.EndDate = &endDate.Time
		}
		i.Days = []models.ItineraryDay{}
		itineraries = append(itineraries, i)
		itineraryIDs = append(itineraryIDs, i.ID)
	}

	if len(itineraryIDs) > 0 {
		// ORDER BY was missing here — Postgres doesn't guarantee row order
		// without one, so days could render out of sequence after a few
		// inserts/edits. sort_order first (what the trail map draws by),
		// date as a tiebreaker.
		dayRows, err := database.DB.Query(`
			SELECT itinerary_id, id, date, region, place, COALESCE(weather, ''), sort_order
FROM itinerary_days
WHERE itinerary_id::text = ANY($1)
ORDER BY sort_order ASC, date ASC
		`, itineraryIDs)
		if err != nil {
			log.Printf("❌ GetItineraries days query error: %v", err)
		} else {
			defer dayRows.Close()
			daysByItinerary := make(map[string][]models.ItineraryDay)
			for dayRows.Next() {
				var itineraryID string
				var d models.ItineraryDay
				if err := dayRows.Scan(&itineraryID, &d.ID, &d.Date, &d.Region, &d.Place, &d.Weather, &d.SortOrder); err == nil {
					d.Stops = []models.ItineraryStop{}
					daysByItinerary[itineraryID] = append(daysByItinerary[itineraryID], d)
				}
			}
			for i := range itineraries {
				if days, ok := daysByItinerary[itineraries[i].ID]; ok {
					itineraries[i].Days = days
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"itineraries": itineraries})
}

// ============================================
// POST /api/v1/itineraries — create a new trip permit
// ============================================
func CreateItinerary(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user context"})
		return
	}

	var req struct {
		Name          string  `json:"name" binding:"required"`
		StartDate     string  `json:"startDate"`
		EndDate       string  `json:"endDate"`
		CoverImageURL string  `json:"coverImageUrl"`
		Travelers     int     `json:"travelers"`
		Budget        float64 `json:"budget"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if req.Travelers < 1 {
		req.Travelers = 1
	}

	// EndDate is optional — NULLIF turns an empty string into a real SQL NULL
	// instead of failing the DATE cast.
	var newItinerary models.Itinerary
	var endDate sql.NullTime
	var coverImageURL sql.NullString
	err := database.DB.QueryRow(`
		INSERT INTO itineraries (user_id, name, start_date, end_date, cover_image_url, travelers, budget)
		VALUES ($1, $2, $3, NULLIF($4, '')::date, NULLIF($5, ''), $6, $7)
		RETURNING id, name, start_date, end_date, COALESCE(cover_image_url, ''), travelers, budget, created_at
	`, userID, req.Name, req.StartDate, req.EndDate, req.CoverImageURL, req.Travelers, req.Budget).Scan(
		&newItinerary.ID, &newItinerary.Name, &newItinerary.StartDate, &endDate, &coverImageURL,
		&newItinerary.Travelers, &newItinerary.Budget, &newItinerary.CreatedAt,
	)

	if err != nil {
		log.Printf("❌ CreateItinerary insert error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create itinerary"})
		return
	}

	if endDate.Valid {
		newItinerary.EndDate = &endDate.Time
	}
	newItinerary.CoverImageURL = coverImageURL.String

	c.JSON(http.StatusCreated, gin.H{"itinerary": newItinerary})
}

// ============================================
// GET /api/v1/itineraries/:id — fetch full trip (with days & stops)
// ============================================
func GetItineraryByID(c *gin.Context) {
	userID, _ := c.Get("user_id")
	itineraryID := c.Param("id")

	var trip models.Itinerary
	var endDate sql.NullTime
	err := database.DB.QueryRow(`
		SELECT id, name, start_date, end_date, COALESCE(cover_image_url, ''), travelers, budget,
       COALESCE(season, ''), COALESCE(season_note, '')
FROM itineraries
WHERE id = $1 AND user_id = $2
	`, itineraryID, userID).Scan(
		&trip.ID, &trip.Name, &trip.StartDate, &endDate, &trip.CoverImageURL, &trip.Travelers, &trip.Budget, &trip.Season, &trip.SeasonNote,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Itinerary not found"})
		return
	}
	if endDate.Valid {
		trip.EndDate = &endDate.Time
	}

	// Same missing-ORDER-BY issue as GetItineraries — fixed here too.
	dayRows, err := database.DB.Query(`
		SELECT id, date, region, place, COALESCE(weather, ''), sort_order
FROM itinerary_days
WHERE itinerary_id = $1
ORDER BY sort_order ASC, date ASC
	`, trip.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load days"})
		return
	}
	defer dayRows.Close()

	var days []models.ItineraryDay
	var dayIDs []string

	for dayRows.Next() {
		var d models.ItineraryDay
		if err := dayRows.Scan(&d.ID, &d.Date, &d.Region, &d.Place, &d.Weather, &d.SortOrder); err == nil {
			d.Stops = []models.ItineraryStop{}
			days = append(days, d)
			dayIDs = append(dayIDs, d.ID)
		}
	}

	if len(dayIDs) > 0 {
		stopRows, err := database.DB.Query(`
			SELECT id, day_id, name, time_label, cost, category, sort_order
			FROM itinerary_stops
			WHERE day_id::text = ANY($1)
			ORDER BY sort_order ASC
		`, dayIDs)

		if err == nil {
			defer stopRows.Close()
			stopsByDay := make(map[string][]models.ItineraryStop)

			for stopRows.Next() {
				var s models.ItineraryStop
				if err := stopRows.Scan(&s.ID, &s.DayID, &s.Name, &s.TimeLabel, &s.Cost, &s.Category, &s.SortOrder); err == nil {
					stopsByDay[s.DayID] = append(stopsByDay[s.DayID], s)
				}
			}

			for i := range days {
				if stops, exists := stopsByDay[days[i].ID]; exists {
					days[i].Stops = stops
				}
			}
		}
	}

	trip.Days = days
	c.JSON(http.StatusOK, gin.H{"itinerary": trip})
}

// ============================================
// POST /api/v1/itineraries/:id/days — add a day to the trail
// ============================================
func AddItineraryDay(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user context"})
		return
	}
	itineraryID := c.Param("id")

	var req struct {
		Date      string `json:"date"`
		Region    string `json:"region"`
		Place     string `json:"place" binding:"required"`
		SortOrder int    `json:"sortOrder"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	var newDay models.ItineraryDay
	err := database.DB.QueryRow(`
		INSERT INTO itinerary_days (itinerary_id, date, region, place, sort_order)
		SELECT $1, $2, $3, $4, $5
		WHERE EXISTS (SELECT 1 FROM itineraries WHERE id = $1 AND user_id = $6)
		RETURNING id, date, region, place, sort_order
	`, itineraryID, req.Date, req.Region, req.Place, req.SortOrder, userID).Scan(
		&newDay.ID, &newDay.Date, &newDay.Region, &newDay.Place, &newDay.SortOrder,
	)

	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this itinerary"})
		return
	}
	if err != nil {
		log.Printf("❌ AddItineraryDay insert error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not add day"})
		return
	}

	newDay.Stops = []models.ItineraryStop{}
	c.JSON(http.StatusCreated, gin.H{"day": newDay})
}

// ============================================
// POST /api/v1/itineraries/days/:dayId/stops — add a stop to a day
// ============================================
func AddItineraryStop(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user context"})
		return
	}
	dayID := c.Param("dayId")

	var req struct {
		Name      string  `json:"name" binding:"required"`
		TimeLabel string  `json:"timeLabel"`
		Cost      float64 `json:"cost"`
		Category  string  `json:"category"`
		SortOrder int     `json:"sortOrder"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	var newStop models.ItineraryStop
	err := database.DB.QueryRow(`
		INSERT INTO itinerary_stops (day_id, name, time_label, cost, category, sort_order)
		SELECT $1, $2, $3, $4, $5, $6
		WHERE EXISTS (
			SELECT 1 FROM itinerary_days d
			JOIN itineraries i ON i.id = d.itinerary_id
			WHERE d.id = $1 AND i.user_id = $7
		)
		RETURNING id, day_id, name, time_label, cost, category, sort_order
	`, dayID, req.Name, req.TimeLabel, req.Cost, req.Category, req.SortOrder, userID).Scan(
		&newStop.ID, &newStop.DayID, &newStop.Name, &newStop.TimeLabel, &newStop.Cost, &newStop.Category, &newStop.SortOrder,
	)

	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this trip"})
		return
	}
	if err != nil {
		log.Printf("❌ AddItineraryStop insert error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not add stop"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"stop": newStop})
}

// ============================================
// PATCH /api/v1/itineraries/days/:dayId — edit a day on the trail
// ============================================
func UpdateItineraryDay(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user context"})
		return
	}
	dayID := c.Param("dayId")

	var req struct {
		Date      string `json:"date"`
		Region    string `json:"region"`
		Place     string `json:"place" binding:"required"`
		SortOrder *int   `json:"sortOrder"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// COALESCE-style pattern isn't used here on purpose: this is a full-field
	// edit (matches what the "Edit day" form sends), not a partial patch.
	// sortOrder is the one field that's genuinely optional, since drag-reorder
	// isn't built yet — when omitted we keep whatever order it already has.
	var updatedDay models.ItineraryDay
	err := database.DB.QueryRow(`
		UPDATE itinerary_days d
		SET date = $1, region = $2, place = $3,
		    sort_order = COALESCE($4, d.sort_order)
		FROM itineraries i
		WHERE d.id = $5 AND d.itinerary_id = i.id AND i.user_id = $6
		RETURNING d.id, d.date, d.region, d.place, COALESCE(d.weather, ''), d.sort_order
	`, req.Date, req.Region, req.Place, req.SortOrder, dayID, userID).Scan(
		&updatedDay.ID, &updatedDay.Date, &updatedDay.Region, &updatedDay.Place, &updatedDay.Weather, &updatedDay.SortOrder,
	)

	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this day"})
		return
	}
	if err != nil {
		log.Printf("❌ UpdateItineraryDay error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not update day"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"day": updatedDay})
}

// ============================================
// DELETE /api/v1/itineraries/days/:dayId — remove a day (and its stops)
// ============================================
func DeleteItineraryDay(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user context"})
		return
	}
	dayID := c.Param("dayId")

	// ON DELETE CASCADE on itinerary_stops.day_id (see schema_itinerary.sql)
	// means the day's stops are removed automatically.
	res, err := database.DB.Exec(`
		DELETE FROM itinerary_days d
		USING itineraries i
		WHERE d.id = $1 AND d.itinerary_id = i.id AND i.user_id = $2
	`, dayID, userID)

	if err != nil {
		log.Printf("❌ DeleteItineraryDay error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not delete day"})
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this day, or it no longer exists"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Day deleted"})
}

// ============================================
// PATCH /api/v1/itineraries/stops/:stopId — edit a stop
// ============================================
func UpdateItineraryStop(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user context"})
		return
	}
	stopID := c.Param("stopId")

	var req struct {
		Name      string  `json:"name" binding:"required"`
		TimeLabel string  `json:"timeLabel"`
		Cost      float64 `json:"cost"`
		Category  string  `json:"category"`
		SortOrder *int    `json:"sortOrder"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	var updatedStop models.ItineraryStop
	err := database.DB.QueryRow(`
		UPDATE itinerary_stops s
		SET name = $1, time_label = $2, cost = $3, category = $4,
		    sort_order = COALESCE($5, s.sort_order)
		FROM itinerary_days d
		JOIN itineraries i ON i.id = d.itinerary_id
		WHERE s.id = $6 AND s.day_id = d.id AND i.user_id = $7
		RETURNING s.id, s.day_id, s.name, s.time_label, s.cost, s.category, s.sort_order
	`, req.Name, req.TimeLabel, req.Cost, req.Category, req.SortOrder, stopID, userID).Scan(
		&updatedStop.ID, &updatedStop.DayID, &updatedStop.Name, &updatedStop.TimeLabel, &updatedStop.Cost, &updatedStop.Category, &updatedStop.SortOrder,
	)

	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this stop"})
		return
	}
	if err != nil {
		log.Printf("❌ UpdateItineraryStop error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not update stop"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stop": updatedStop})
}

// ============================================
// DELETE /api/v1/itineraries/stops/:stopId — remove a single stop
// ============================================
func DeleteItineraryStop(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user context"})
		return
	}
	stopID := c.Param("stopId")

	res, err := database.DB.Exec(`
		DELETE FROM itinerary_stops s
		USING itinerary_days d, itineraries i
		WHERE s.id = $1 AND s.day_id = d.id AND d.itinerary_id = i.id AND i.user_id = $2
	`, stopID, userID)

	if err != nil {
		log.Printf("❌ DeleteItineraryStop error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not delete stop"})
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this stop, or it no longer exists"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Stop deleted"})
}