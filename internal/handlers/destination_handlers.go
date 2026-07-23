package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"karibu-api/internal/database"
	"karibu-api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

// ============================================
// GET /api/v1/destinations — browse the full catalog
// ============================================
func GetDestinations(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user context"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT
			d.id, d.slug, d.name, d.region, d.short_description, d.long_description,
			d.cover_image_url, d.color, d.created_at,
			(sd.id IS NOT NULL) AS is_saved
		FROM destinations d
		LEFT JOIN saved_destinations sd
			ON sd.destination_id = d.id AND sd.user_id = $1
		WHERE d.is_active = true
		ORDER BY d.name ASC
	`, userID)
	if err != nil {
		log.Printf("❌ GetDestinations query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load destinations"})
		return
	}
	defer rows.Close()

	destinations := []models.DestinationWithSaved{}
	for rows.Next() {
		var d models.DestinationWithSaved
		if err := rows.Scan(
			&d.ID, &d.Slug, &d.Name, &d.Region, &d.ShortDescription, &d.LongDescription,
			&d.CoverImageURL, &d.Color, &d.CreatedAt, &d.IsSaved,
		); err != nil {
			log.Printf("❌ GetDestinations scan error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read destinations"})
			return
		}
		destinations = append(destinations, d)
	}

	c.JSON(http.StatusOK, gin.H{"destinations": destinations})
}

// ============================================
// GET /api/v1/destinations/:slug — one destination's detail page
// ============================================
func GetDestinationBySlug(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user context"})
		return
	}

	slug := c.Param("slug")

	var d models.DestinationWithSaved
	err := database.DB.QueryRow(`
		SELECT
			d.id, d.slug, d.name, d.region, d.short_description, d.long_description,
			d.cover_image_url, d.color, d.created_at,
			(sd.id IS NOT NULL) AS is_saved
		FROM destinations d
		LEFT JOIN saved_destinations sd
			ON sd.destination_id = d.id AND sd.user_id = $1
		WHERE d.slug = $2 AND d.is_active = true
	`, userID, slug).Scan(
		&d.ID, &d.Slug, &d.Name, &d.Region, &d.ShortDescription, &d.LongDescription,
		&d.CoverImageURL, &d.Color, &d.CreatedAt, &d.IsSaved,
	)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Destination not found"})
		return
	}
	if err != nil {
		log.Printf("❌ GetDestinationBySlug query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load destination"})
		return
	}

	// The detail page is the only place that needs the full photo gallery,
	// so it's a second, separate query rather than joining it into every
	// browse-list row (which only ever shows the cover image).
	imageRows, err := database.DB.Query(`
		SELECT image_url FROM destination_images
		WHERE destination_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`, d.ID)
	if err != nil {
		log.Printf("❌ GetDestinationBySlug gallery query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load destination images"})
		return
	}
	defer imageRows.Close()

	d.GalleryImageURLs = []string{}
	for imageRows.Next() {
		var url string
		if err := imageRows.Scan(&url); err != nil {
			log.Printf("❌ GetDestinationBySlug gallery scan error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read destination images"})
			return
		}
		d.GalleryImageURLs = append(d.GalleryImageURLs, url)
	}

	c.JSON(http.StatusOK, gin.H{"destination": d})
}

// ============================================
// GET /api/v1/places — the logged-in user's saved destinations
// ============================================
func GetSavedPlaces(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user context"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT
			d.id, d.slug, d.name, d.region, d.short_description, d.long_description,
			d.cover_image_url, d.color, d.created_at
		FROM saved_destinations sd
		JOIN destinations d ON d.id = sd.destination_id
		WHERE sd.user_id = $1 AND d.is_active = true
		ORDER BY sd.created_at DESC
	`, userID)
	if err != nil {
		log.Printf("❌ GetSavedPlaces query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load saved places"})
		return
	}
	defer rows.Close()

	destinations := []models.Destination{}
	for rows.Next() {
		var d models.Destination
		if err := rows.Scan(
			&d.ID, &d.Slug, &d.Name, &d.Region, &d.ShortDescription, &d.LongDescription,
			&d.CoverImageURL, &d.Color, &d.CreatedAt,
		); err != nil {
			log.Printf("❌ GetSavedPlaces scan error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read saved places"})
			return
		}
		destinations = append(destinations, d)
	}

	c.JSON(http.StatusOK, gin.H{"places": destinations})
}

// ============================================
// POST /api/v1/destinations/:id/save — bookmark a destination
// ============================================
func SaveDestination(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user context"})
		return
	}

	destinationID := c.Param("id")

	_, err := database.DB.Exec(`
		INSERT INTO saved_destinations (user_id, destination_id)
		VALUES ($1, $2)
	`, userID, destinationID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				c.JSON(http.StatusOK, gin.H{"message": "Already saved"})
				return
			case "23503":
				c.JSON(http.StatusBadRequest, gin.H{"error": "Destination not found"})
				return
			}
		}
		log.Printf("❌ SaveDestination insert error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save destination"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Saved"})
}

// ============================================
// DELETE /api/v1/destinations/:id/save — remove a bookmark
// ============================================
func UnsaveDestination(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user context"})
		return
	}

	destinationID := c.Param("id")

	result, err := database.DB.Exec(`
		DELETE FROM saved_destinations WHERE destination_id = $1 AND user_id = $2
	`, destinationID, userID)
	if err != nil {
		log.Printf("❌ UnsaveDestination error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not remove bookmark"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bookmark not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Removed"})
}