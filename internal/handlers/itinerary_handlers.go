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
    SELECT id, name, start_date, travelers, budget,
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
		if err := rows.Scan(&i.ID, &i.Name, &i.StartDate, &i.Travelers, &i.Budget, &i.Season, &i.SeasonNote); err != nil {
			log.Printf("❌ GetItineraries scan error: %v", err)
			continue
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
		Name      string  `json:"name" binding:"required"`
		StartDate string  `json:"startDate"`
		Travelers int     `json:"travelers"`
		Budget    float64 `json:"budget"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if req.Travelers < 1 {
		req.Travelers = 1
	}

	var newItinerary models.Itinerary
	err := database.DB.QueryRow(`
		INSERT INTO itineraries (user_id, name, start_date, travelers, budget)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, start_date, travelers, budget, created_at
	`, userID, req.Name, req.StartDate, req.Travelers, req.Budget).Scan(
		&newItinerary.ID, &newItinerary.Name, &newItinerary.StartDate,
		&newItinerary.Travelers, &newItinerary.Budget, &newItinerary.CreatedAt,
	)

	if err != nil {
		log.Printf("❌ CreateItinerary insert error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create itinerary"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"itinerary": newItinerary})
}

// ============================================
// GET /api/v1/itineraries/:id — fetch full trip (with days & stops)
// ============================================
func GetItineraryByID(c *gin.Context) {
	userID, _ := c.Get("user_id")
	itineraryID := c.Param("id")

	var trip models.Itinerary
	err := database.DB.QueryRow(`
		SELECT id, name, start_date, travelers, budget,
       COALESCE(season, ''), COALESCE(season_note, '')
FROM itineraries
WHERE id = $1 AND user_id = $2
	`, itineraryID, userID).Scan(
		&trip.ID, &trip.Name, &trip.StartDate, &trip.Travelers, &trip.Budget, &trip.Season, &trip.SeasonNote,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Itinerary not found"})
		return
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