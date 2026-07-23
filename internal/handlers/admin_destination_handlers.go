package handlers

import (
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"karibu-api/internal/database"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

type AdminDestinationView struct {
	ID               string    `json:"id"`
	Slug             string    `json:"slug"`
	Name             string    `json:"name"`
	Region           string    `json:"region"`
	ShortDescription string    `json:"shortDescription"`
	LongDescription  string    `json:"longDescription"`
	CoverImageURL    string    `json:"coverImageUrl"`
	GalleryImageURLs []string  `json:"galleryImageUrls"`
	Color            string    `json:"color"`
	IsActive         bool      `json:"isActive"`
	CreatedAt        time.Time `json:"createdAt"`
}

type AdminDestinationInput struct {
	Name             string   `json:"name" binding:"required,min=1,max=255"`
	Region           string   `json:"region" binding:"required,min=1,max=100"`
	ShortDescription string   `json:"shortDescription" binding:"required"`
	LongDescription  string   `json:"longDescription" binding:"required"`
	CoverImageURL    string   `json:"coverImageUrl" binding:"required"`
	GalleryImageURLs []string `json:"galleryImageUrls"`
	Color            string   `json:"color" binding:"required"`
}

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonSlugChars.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func fetchGalleryImages(destinationID string) ([]string, error) {
	rows, err := database.DB.Query(`
		SELECT image_url FROM destination_images
		WHERE destination_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`, destinationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	urls := []string{}
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, err
		}
		urls = append(urls, url)
	}
	return urls, nil
}

func replaceGalleryImages(destinationID string, urls []string) error {
	_, err := database.DB.Exec(`DELETE FROM destination_images WHERE destination_id = $1`, destinationID)
	if err != nil {
		return err
	}
	for i, url := range urls {
		if strings.TrimSpace(url) == "" {
			continue
		}
		_, err := database.DB.Exec(`
			INSERT INTO destination_images (destination_id, image_url, sort_order)
			VALUES ($1, $2, $3)
		`, destinationID, url, i)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetAllDestinationsAdmin(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT id, slug, name, region, short_description, long_description,
			cover_image_url, color, is_active, created_at
		FROM destinations
		ORDER BY created_at DESC
	`)
	if err != nil {
		log.Printf("❌ GetAllDestinationsAdmin query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load destinations"})
		return
	}
	defer rows.Close()

	destinations := []AdminDestinationView{}
	for rows.Next() {
		var d AdminDestinationView
		if err := rows.Scan(
			&d.ID, &d.Slug, &d.Name, &d.Region, &d.ShortDescription, &d.LongDescription,
			&d.CoverImageURL, &d.Color, &d.IsActive, &d.CreatedAt,
		); err != nil {
			log.Printf("❌ GetAllDestinationsAdmin scan error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read destinations"})
			return
		}
		destinations = append(destinations, d)
	}

	c.JSON(http.StatusOK, gin.H{"destinations": destinations})
}

func GetDestinationAdmin(c *gin.Context) {
	id := c.Param("id")

	var d AdminDestinationView
	err := database.DB.QueryRow(`
		SELECT id, slug, name, region, short_description, long_description,
			cover_image_url, color, is_active, created_at
		FROM destinations
		WHERE id = $1
	`, id).Scan(
		&d.ID, &d.Slug, &d.Name, &d.Region, &d.ShortDescription, &d.LongDescription,
		&d.CoverImageURL, &d.Color, &d.IsActive, &d.CreatedAt,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Destination not found"})
		return
	}

	gallery, err := fetchGalleryImages(d.ID)
	if err != nil {
		log.Printf("❌ GetDestinationAdmin gallery error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load gallery"})
		return
	}
	d.GalleryImageURLs = gallery

	c.JSON(http.StatusOK, gin.H{"destination": d})
}

func CreateDestinationAdmin(c *gin.Context) {
	var input AdminDestinationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid fields: " + err.Error()})
		return
	}

	slug := slugify(input.Name)
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name must contain at least one letter or number"})
		return
	}

	var d AdminDestinationView
	err := database.DB.QueryRow(`
		INSERT INTO destinations
			(slug, name, region, short_description, long_description, cover_image_url, color)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, slug, name, region, short_description, long_description,
			cover_image_url, color, is_active, created_at
	`, slug, input.Name, input.Region, input.ShortDescription, input.LongDescription,
		input.CoverImageURL, input.Color,
	).Scan(
		&d.ID, &d.Slug, &d.Name, &d.Region, &d.ShortDescription, &d.LongDescription,
		&d.CoverImageURL, &d.Color, &d.IsActive, &d.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "A destination with a matching name/slug already exists"})
			return
		}
		log.Printf("❌ CreateDestinationAdmin insert error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create destination"})
		return
	}

	if err := replaceGalleryImages(d.ID, input.GalleryImageURLs); err != nil {
		log.Printf("❌ CreateDestinationAdmin gallery error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Destination created, but gallery images failed to save"})
		return
	}
	d.GalleryImageURLs = input.GalleryImageURLs

	c.JSON(http.StatusCreated, gin.H{"destination": d})
}

func UpdateDestinationAdmin(c *gin.Context) {
	id := c.Param("id")

	var input AdminDestinationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid fields: " + err.Error()})
		return
	}

	var d AdminDestinationView
	err := database.DB.QueryRow(`
		UPDATE destinations
		SET name = $1, region = $2, short_description = $3, long_description = $4,
			cover_image_url = $5, color = $6
		WHERE id = $7
		RETURNING id, slug, name, region, short_description, long_description,
			cover_image_url, color, is_active, created_at
	`, input.Name, input.Region, input.ShortDescription, input.LongDescription,
		input.CoverImageURL, input.Color, id,
	).Scan(
		&d.ID, &d.Slug, &d.Name, &d.Region, &d.ShortDescription, &d.LongDescription,
		&d.CoverImageURL, &d.Color, &d.IsActive, &d.CreatedAt,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Destination not found"})
		return
	}

	if err := replaceGalleryImages(d.ID, input.GalleryImageURLs); err != nil {
		log.Printf("❌ UpdateDestinationAdmin gallery error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Destination updated, but gallery images failed to save"})
		return
	}
	d.GalleryImageURLs = input.GalleryImageURLs

	c.JSON(http.StatusOK, gin.H{"destination": d})
}

func ToggleDestinationActive(c *gin.Context) {
	id := c.Param("id")

	var isActive bool
	err := database.DB.QueryRow(`
		UPDATE destinations SET is_active = NOT is_active
		WHERE id = $1
		RETURNING is_active
	`, id).Scan(&isActive)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Destination not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"isActive": isActive})
}