package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"   // added

	"karibu-api/internal/database"
	"karibu-api/internal/models"

	"github.com/gin-gonic/gin"
	"google.golang.org/genai"
)

type AIGeneratedTrip struct {
	Season     string `json:"season"`
	SeasonNote string `json:"seasonNote"`
	Days       []struct {
		Date    string `json:"date"`
		Region  string `json:"region"`
		Place   string `json:"place"`
		Weather string `json:"weather"`
		Stops   []struct {
			Name      string  `json:"name"`
			TimeLabel string  `json:"timeLabel"`
			Cost      float64 `json:"cost"`
			Category  string  `json:"category"`
		} `json:"stops"`
	} `json:"days"`
}

const geminiModel = "gemini-3.5-flash"

func GenerateTripSuggestions(c *gin.Context) {
	itineraryID := c.Param("id")
	userID, _ := c.Get("user_id")

	var trip models.Itinerary
	err := database.DB.QueryRow(`
		SELECT name, start_date, travelers, budget 
		FROM itineraries WHERE id = $1 AND user_id = $2
	`, itineraryID, userID).Scan(&trip.Name, &trip.StartDate, &trip.Travelers, &trip.Budget)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trip not found"})
		return
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Println("❌ GenerateTripSuggestions called but GEMINI_API_KEY is not set")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI suggestions aren't configured on this server yet"})
		return
	}

	prompt := fmt.Sprintf(`
		You are an expert East African travel guide. Create a custom day-by-day itinerary.
		Trip Name: "%s"
		Start Date: %s
		Travelers: %d
		Total Budget: $%f
		
		Return ONLY a JSON object with the following structure:
		- "season" (e.g., "dry season", "rainy season")
		- "seasonNote" (brief description of weather/conditions for this season)
		- "days" array where each day has:
		  - "date" (YYYY-MM-DD starting from Start Date)
		  - "region" ("mainland" or "coast")
		  - "place" (e.g., "Mwanza", "Zanzibar")
		  - "weather" (e.g., "☀ 27° · dry season")
		  - "stops": array of 2-3 stops with "name", "timeLabel" (e.g. "9am"), "cost" (numeric), and "category" ("Food", "Stays", "Transport", "Activity").
		Keep the total cost of all stops under the Total Budget.
	`, trip.Name, trip.StartDate.Format("2006-01-02"), trip.Travelers, trip.Budget)

	ctx := context.Background()

	// google.golang.org/genai is the current SDK (the old
	// github.com/google/generative-ai-go/genai is deprecated/EOL as of Aug 2025).
	// Client construction here is stateless over HTTP — there's no client.Close()
	// to defer, unlike the old SDK.
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("❌ Failed to initialize Gemini client: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize AI"})
		return
	}

	resp, err := client.Models.GenerateContent(
		ctx,
		geminiModel,
		genai.Text(prompt),
		&genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
		},
	)
	if err != nil {
		log.Printf("❌ Gemini GenerateContent error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI generation failed"})
		return
	}

	rawJSON := resp.Text()
	if rawJSON == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI returned an empty response"})
		return
	}

	var aiTrip AIGeneratedTrip
	decoder := json.NewDecoder(strings.NewReader(rawJSON))
	if err := decoder.Decode(&aiTrip); err != nil {
		log.Printf("❌ JSON parse error: %v — raw response: %s", err, rawJSON)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse AI response"})
		return
	}

	if _, err := database.DB.Exec(`
    UPDATE itineraries SET season = $1, season_note = $2 WHERE id = $3
`, aiTrip.Season, aiTrip.SeasonNote, itineraryID); err != nil {
		log.Printf("⚠️ Failed to save season info for itinerary %s: %v", itineraryID, err)
	}

	for dayIndex, day := range aiTrip.Days {
		var dayID string
		err := database.DB.QueryRow(`
			INSERT INTO itinerary_days (itinerary_id, date, region, place, weather, sort_order)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id
		`, itineraryID, day.Date, day.Region, day.Place, day.Weather, dayIndex).Scan(&dayID)

		if err == nil {
			for stopIndex, stop := range day.Stops {
				database.DB.Exec(`
					INSERT INTO itinerary_stops (day_id, name, time_label, cost, category, sort_order)
					VALUES ($1, $2, $3, $4, $5, $6)
				`, dayID, stop.Name, stop.TimeLabel, stop.Cost, stop.Category, stopIndex)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Trip populated successfully!"})
}