package models

import (
	"time"
)

// ItineraryStop represents a single activity or location visit
type ItineraryStop struct {
	ID        string  `json:"id"`
	DayID     string  `json:"dayId"`
	Name      string  `json:"name"`
	TimeLabel string  `json:"timeLabel"`
	Cost      float64 `json:"cost"`
	Category  string  `json:"category"`
	SortOrder int     `json:"sortOrder"`
}

// ItineraryDay represents a single day's stub on the trail
type ItineraryDay struct {
	ID          string          `json:"id"`
	ItineraryID string          `json:"itineraryId"`
	Date        time.Time       `json:"date"`
	Region      string          `json:"region"`
	Place       string          `json:"place"`
	Weather     string          `json:"weather"`
	SortOrder   int             `json:"sortOrder"`
	Stops       []ItineraryStop `json:"stops"` // Nested stops for easy frontend mapping
}

// TripCollaborator represents a user who has been granted access to a trip
type TripCollaborator struct {
	ItineraryID string    `json:"itineraryId"`
	UserID      string    `json:"userId"`
	Role        string    `json:"role"`
	AddedAt     time.Time `json:"addedAt"`
}

// Itinerary represents the top-level permit card and contains all days
type Itinerary struct {
	ID            string         `json:"id"`
	UserID        string         `json:"userId"`
	Name          string         `json:"name"`
	StartDate     time.Time      `json:"startDate"`
	EndDate       *time.Time     `json:"endDate,omitempty"` // nullable — not every trip has a fixed end date
	CoverImageURL string         `json:"coverImageUrl,omitempty"`
	Travelers     int            `json:"travelers"`
	Budget        float64        `json:"budget"`
	Season        string         `json:"season"`
	SeasonNote    string         `json:"seasonNote"`
	Days          []ItineraryDay `json:"days"` // Nested days to construct the trail map
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}