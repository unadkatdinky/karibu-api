
package handlers

import (
	"database/sql"
	"net/http"

	"karibu-api/internal/database"
	"karibu-api/internal/models"

	"github.com/gin-gonic/gin"
)

// GetCurrentUser returns the profile of the currently authenticated user
func GetCurrentUser(c *gin.Context) {
	// 1. Get the user ID from the Gin context (injected by AuthRequired middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify user context"})
		return
	}

	// 2. Query the database for this specific user
	var user models.User
	query := `
		SELECT id, full_name, email, role, created_at, updated_at 
		FROM users 
		WHERE id = $1
	`

	err := database.DB.QueryRow(query, userID).Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	// 3. Handle database errors
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "User profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error while fetching profile"})
		return
	}

	// 4. Return the user profile
	// Note: Because your models.User struct uses `json:"-"` on PasswordHash, 
	// it is physically impossible for the password to leak in this response!
	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}