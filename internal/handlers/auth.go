package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"crypto/rand"
    "encoding/hex"

	"karibu-api/internal/database"
	"karibu-api/internal/models"
	"karibu-api/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ============================================
// INPUT VALIDATION STRUCTS
// ============================================
// These structs define what JSON data we expect from the frontend
// The `binding:"required"` means the field MUST be present
// The `binding:"email"` means it must be a valid email format

// RegisterInput matches the JSON from your React Register form
type RegisterInput struct {
	FullName string `json:"fullName" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Role     string `json:"role" binding:"required,oneof=Traveler LocalGuide"`
}

// LoginInput matches the JSON from your React Login form
type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RefreshInput for refresh token requests
type RefreshInput struct {
	RefreshToken string `json:"refreshToken"`
}

type ForgotPasswordInput struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordInput struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}
// ============================================
// REGISTER HANDLER
// ============================================
// This function creates a new user account
// Called when user submits the registration form
func Register(c *gin.Context) {
	// ---- STEP 1: Parse incoming JSON ----
	var input RegisterInput

	// c.ShouldBindJSON validates the JSON against our struct tags
	// If validation fails (missing fields, wrong type, etc), this returns error
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Validation failed",
			"details": err.Error(),
		})
		return
	}

	// ---- STEP 2: Validate password strength ----
	// Just requiring min=8 is not enough
	// We need uppercase, lowercase, numbers, special chars
	passwordErrors := utils.ValidatePassword(input.Password)
	if len(passwordErrors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Password is too weak",
			"details": passwordErrors,
		})
		return
	}

	// ---- STEP 3: Hash the password ----
	// WHY WE HASH:
	// - We NEVER store plain passwords in database
	// - If database is breached, attacker can't use the passwords
	// - We can't reverse hashes, so we can't recover passwords
	// - Users should ALWAYS have password reset, never "recover password"
	//
	// BCRYPT COST (12):
	// - Cost = 2^cost iterations
	// - Cost 12 = 4096 iterations (takes ~100ms to hash)
	// - This makes brute force attacks SLOW
	// - Higher cost = slower (more secure, but slower)
	// - Use 12 for web, 14-15 for passwords you want really secure
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		// This rarely happens (system issues)
		log.Printf("Bcrypt error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Server error processing password",
		})
		return
	}

	// ---- STEP 4: Insert user into database ----
	// We use parameterized queries to prevent SQL injection
	// $1, $2, etc are placeholders - the actual values are passed separately
	// This means even if user enters SQL code, it won't be executed
	var newUserID string
	query := `
		INSERT INTO users (full_name, email, password_hash, role) 
		VALUES ($1, $2, $3, $4) 
		RETURNING id
	`
	
	err = database.DB.QueryRow(
		query,
		input.FullName,
		input.Email,
		string(hashedPassword), // Convert byte slice to string for storage
		input.Role,
	).Scan(&newUserID)
	
	if err != nil {
		// Check if it's a duplicate email error
		if err.Error() == "pq: duplicate key value violates unique constraint \"users_email_key\"" {
			c.JSON(http.StatusConflict, gin.H{
				"error": "An account with this email already exists",
			})
		} else {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Server error creating account",
			})
		}
		return
	}

	// ---- STEP 5: Generate JWT tokens ----
	// After successful registration, we log them in automatically
	// This avoids the annoying "please login" step
	generateAndSetTokens(c, newUserID, input.Role)

	// ---- STEP 6: Log the event ----
	// Always log security events for debugging and auditing
	log.Printf("✅ New user registered: ID=%s, Email=%s, Role=%s", newUserID, input.Email, input.Role)

	// ---- STEP 7: Return success response ----
	// Notice: We don't return tokens in the response body
	// They're in HTTP-only cookies (automatically sent by browser)
	c.JSON(http.StatusCreated, gin.H{
		"message": "Account created successfully",
		"user": gin.H{
			"id":       newUserID,
			"fullName": input.FullName,
			"email":    input.Email,
			"role":     input.Role,
		},
	})
}

// ============================================
// LOGIN HANDLER
// ============================================
// This function authenticates a user
// Called when user submits the login form
func Login(c *gin.Context) {
	// ---- STEP 1: Parse incoming JSON ----
	var input LoginInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid email or password format",
		})
		return
	}

	// ---- STEP 2: Query database for user ----
	var user models.User
	query := `
		SELECT id, full_name, email, password_hash, role 
		FROM users 
		WHERE email = $1
	`
	
	err := database.DB.QueryRow(query, input.Email).Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
	)
	
	if err != nil {
		// User not found
		// IMPORTANT: We return generic error "Invalid credentials"
		// We DON'T say "email not found"
		// WHY? To prevent attackers from enumerating valid emails
		// If we said "email not found", attacker would know which emails are registered
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid credentials",
		})
		log.Printf("⚠️ Login attempt with non-existent email: %s", input.Email)
		return
	}

	// ---- STEP 3: Compare passwords ----
	// bcrypt.CompareHashAndPassword:
	// 1. Hashes the provided password with the same method
	// 2. Compares the hash with the one in the database
	// 3. Returns nil if they match, error if they don't
	//
	// WHY THIS WORKS:
	// - bcrypt hashing is deterministic
	// - "Password123" always hashes to same hash
	// - Even a one-character difference produces completely different hash
	// - Hash can't be reversed, so we compare hashes
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password))
	if err != nil {
		// Password doesn't match
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid credentials",
		})
		log.Printf("⚠️ Failed login attempt for: %s", input.Email)
		return
	}

	// ---- STEP 4: Generate JWT tokens ----
	// Password was correct, user is authenticated
	generateAndSetTokens(c, user.ID, string(user.Role))

	// ---- STEP 5: Log the event ----
	log.Printf("✅ User logged in: ID=%s, Email=%s", user.ID, user.Email)

	// ---- STEP 6: Return success response ----
	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"user": gin.H{
			"id":       user.ID,
			"fullName": user.FullName,
			"email":    user.Email,
			"role":     user.Role,
		},
	})
}

// ============================================
// REFRESH TOKEN HANDLER
// ============================================
// This function issues a new access token using the refresh token
// Called every 15 minutes (when access token expires but refresh token is still valid)
func RefreshToken(c *gin.Context) {
	// ---- STEP 1: Get refresh token from cookie ----
	// The browser automatically includes cookies in requests
	// We need to extract it and verify it's valid
	refreshTokenString, err := c.Cookie("karibu_refresh")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "No refresh token found. Please login again.",
		})
		return
	}

	// ---- STEP 2: Verify refresh token ----
	// Parse the JWT token using the refresh secret
	token, err := jwt.Parse(refreshTokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method is what we expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// Return the secret key for verification
		return []byte(os.Getenv("REFRESH_SECRET")), nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid or expired refresh token. Please login again.",
		})
		return
	}

	// ---- STEP 3: Extract user ID from token claims ----
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid token claims",
		})
		return
	}

	userID, ok := claims["sub"].(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid user ID in token",
		})
		return
	}

	// ---- STEP 4: Get user's current role from database ----
	// WHY? The role might have changed since token was issued
	// Example: User was "Traveler", admin upgraded them to "Admin"
	// With refresh, they should get new token with new role
	var role string
	query := `SELECT role FROM users WHERE id = $1`
	err = database.DB.QueryRow(query, userID).Scan(&role)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not found",
		})
		return
	}

	// ---- STEP 5: Generate new access token ----
	// The refresh token is still valid, so we issue a new access token
	generateAndSetTokens(c, userID, role)

	log.Printf("✅ Token refreshed for user: %s", userID)

	// ---- STEP 6: Return success ----
	c.JSON(http.StatusOK, gin.H{
		"message": "Token refreshed successfully",
	})
}

// ============================================
// LOGOUT HANDLER
// ============================================
// This function clears the user's tokens
// Called when user clicks "logout" button
func Logout(c *gin.Context) {
	// ---- STEP 1: Get user ID from context ----
	// The AuthRequired middleware already verified the token
	// and put the user ID in the context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Not authenticated",
		})
		return
	}

	// ---- STEP 2: Clear cookies ----
	// Setting MaxAge to -1 tells browser to delete the cookie
	// The cookie will still be in the request (for this response)
	// but will be deleted from the browser after response is sent
	c.SetCookie(
		"karibu_access",           // Cookie name
		"",                         // Empty value
		-1,                         // MaxAge -1 = delete cookie
		"/",                        // Path (all paths)
		"localhost",                // Domain
		false,                      // Secure (false for dev, true for prod)
		true,                       // HttpOnly (JavaScript can't access)
	)

	c.SetCookie(
		"karibu_refresh",
		"",
		-1,
		"/",
		"localhost",
		false,
		true,
	)

	log.Printf("✅ User logged out: ID=%s", userID)

	// ---- STEP 3: Return success ----
	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// ============================================
// HELPER FUNCTION: Generate and Set Tokens
// ============================================
// This function:
// 1. Creates access token (15 minute expiry)
// 2. Creates refresh token (7 day expiry)
// 3. Sets them as HTTP-only cookies
func generateAndSetTokens(c *gin.Context, userID string, role string) {
	// ---- Get secrets from environment ----
	// These should be in your .env file
	// In production, use strong random strings
	accessSecret := os.Getenv("ACCESS_SECRET")
	if accessSecret == "" {
		accessSecret = "local_access_secret_123"
	}

	refreshSecret := os.Getenv("REFRESH_SECRET")
	if refreshSecret == "" {
		refreshSecret = "local_refresh_secret_123"
	}

	// ---- Create Access Token (SHORT-LIVED) ----
	// Duration: 15 minutes
	// Used for: Every API request
	// If stolen: Only 15 minutes of damage
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,                             // Subject (who it's for)
		"role": role,                               // User's role (for authorization)
		"exp":  time.Now().Add(15 * time.Minute).Unix(), // Expiration time
		"type": "access",                           // Type of token
	})
	
	accessTokenString, err := accessToken.SignedString([]byte(accessSecret))
	if err != nil {
		log.Printf("Error signing access token: %v", err)
		return
	}

	// ---- Create Refresh Token (LONG-LIVED) ----
	// Duration: 7 days
	// Used for: Getting new access tokens only
	// If stolen: Limited damage (requires secure HTTP-only cookie)
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,                                  // Subject
		"exp":  time.Now().Add(7 * 24 * time.Hour).Unix(), // 7 days
		"type": "refresh",                              // Type of token
	})
	
	refreshTokenString, err := refreshToken.SignedString([]byte(refreshSecret))
	if err != nil {
		log.Printf("Error signing refresh token: %v", err)
		return
	}

	// ---- Set Access Token as HTTP-Only Cookie ----
	// This cookie is automatically sent with every request
	// JavaScript can't access it (HttpOnly = true)
	// Browsers enforce this automatically
	c.SetCookie(
		"karibu_access",                   // Name
		accessTokenString,                 // Value
		15*60,                            // MaxAge in seconds (15 minutes)
		"/",                              // Path (all paths)
		"localhost",                      // Domain
		false,                            // Secure: false for localhost, true for HTTPS
		true,                             // HttpOnly: JavaScript can't access
	)

	// ---- Set Refresh Token as HTTP-Only Cookie ----
	// Same as access token but with longer duration
	c.SetCookie(
		"karibu_refresh",
		refreshTokenString,
		7*24*60*60,                       // MaxAge in seconds (7 days)
		"/",
		"localhost",
		false,                            // false for localhost, true for HTTPS in production
		true,
	)

	log.Printf("✅ Tokens generated and set for user: %s", userID)
}

// ============================================
// ADDITIONAL HANDLERS (Placeholder)
// ============================================
// These are referenced in main.go
// You'll need to implement them


func UpdateProfile(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Profile update endpoint",
	})
}

func GetAllUsers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Admin only - get all users",
	})
}

func HealthCheck(c *gin.Context) {
	// Check if database is alive
	err := database.DB.Ping()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  "database unavailable",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now(),
	})
}

// Helper to generate a random hex token
func generateSecureToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func ForgotPassword(c *gin.Context) {
	var input ForgotPasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Valid email is required"})
		return
	}

	// 1. Generate a secure token and set expiration (1 hour from now)
	token, err := generateSecureToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	expiresAt := time.Now().Add(1 * time.Hour)

	// 2. Save token to database (only if user exists)
	query := `UPDATE users SET reset_token = $1, reset_token_expires_at = $2 WHERE email = $3 RETURNING id`
	var id string
	err = database.DB.QueryRow(query, token, expiresAt, input.Email).Scan(&id)

	if err != nil {
		// SECURITY: Even if the email doesn't exist, we return a success message!
		// This prevents hackers from using this endpoint to guess valid emails.
		c.JSON(http.StatusOK, gin.H{"message": "If an account exists, a reset link has been sent."})
		return
	}

	// 3. MOCK EMAIL DELIVERY: Print the link to your Go terminal!
	resetLink := fmt.Sprintf("http://localhost:5173/reset-password?token=%s", token)
	log.Println("=====================================================")
	log.Printf("📧 EMAIL SENT TO: %s\n", input.Email)
	log.Printf("🔗 CLICK TO RESET: %s\n", resetLink)
	log.Println("=====================================================")

	c.JSON(http.StatusOK, gin.H{"message": "If an account exists, a reset link has been sent."})
}

func ResetPassword(c *gin.Context) {
	var input ResetPasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token and new password are required"})
		return
	}

	// 1. Validate the new password strength using your exact utility
	passwordErrors := utils.ValidatePassword(input.NewPassword)
	if len(passwordErrors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password is too weak", "details": passwordErrors})
		return
	}

	// 2. Hash the new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), 12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// 3. Find the user by token, check expiration, and update password
	// We also immediately NULL the token so it can never be used again
	query := `
		UPDATE users 
		SET password_hash = $1, reset_token = NULL, reset_token_expires_at = NULL 
		WHERE reset_token = $2 AND reset_token_expires_at > NOW() 
		RETURNING id
	`
	
	var id string
	err = database.DB.QueryRow(query, string(hashedPassword), input.Token).Scan(&id)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset link. Please request a new one."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password successfully reset. You can now log in."})
}

