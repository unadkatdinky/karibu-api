package models

import "time"

// Destination is one row from the shared catalog — the same destination
// looks identical to every user who views or saves it.
type Destination struct {
	ID                string    `json:"id"`
	Slug              string    `json:"slug"`
	Name              string    `json:"name"`
	Region            string    `json:"region"`
	ShortDescription  string    `json:"shortDescription"`
	LongDescription   string    `json:"longDescription"`
	CoverImageURL     string    `json:"coverImageUrl"`
	GalleryImageURLs  []string  `json:"galleryImageUrls"`
	Color             string    `json:"color"`
	IsActive          bool      `json:"-"` // internal only — inactive destinations are never sent to the frontend
	CreatedAt         time.Time `json:"createdAt"`
}

// DestinationWithSaved is a Destination plus whether the CURRENT logged-in
// user has bookmarked it — used on the browse page so the UI can show a
// filled vs. outline bookmark icon without a second round trip.
type DestinationWithSaved struct {
	Destination
	IsSaved bool `json:"isSaved"`
}
