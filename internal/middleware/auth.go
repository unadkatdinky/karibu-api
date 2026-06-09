package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ============================================
// AUTH REQUIRED MIDDLEWARE
// ============================================
// This middleware verifies the JWT token
// Every request to a protected route must have a valid token
// 
// How it works:
// 1. Check if token exists (in Authorization header or cookie)
// 2. Verify token signature
// 3. Extract user ID and role
// 4. Store in context for handlers to use
// 5. Allow request to continue or reject
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// ---- STEP 1: Get token from Authorization header ----
		// Format: "Bearer <token>"
		authHeader := c.GetHeader("Authorization")
		
		var tokenString string
		
		// Check if Authorization header has Bearer token
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}

		// ---- STEP 2: If no Bearer token, check cookies ----
		// Browser automatically sends cookies, so check there too
		if tokenString == "" {
			cookie, err := c.Cookie("karibu_access")
			if err != nil {
				// No token found anywhere
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "No authentication token provided",
				})
				c.Abort()
				return
			}
			tokenString = cookie
		}

		// ---- STEP 3: Parse and verify token ----
		// This validates:
		// 1. Token signature matches our secret
		// 2. Token hasn't been tampered with
		// 3. Token hasn't expired
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Verify signing method is HMAC (what we use)
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			// Return secret key for verification
			return []byte(os.Getenv("ACCESS_SECRET")), nil
		})

		// Check if there was an error parsing
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token: " + err.Error(),
			})
			c.Abort()
			return
		}

		// Check if token is valid
		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Token is invalid or expired",
			})
			c.Abort()
			return
		}

		// ---- STEP 4: Extract claims from token ----
		// Claims are the data inside the token (user ID, role, expiry)
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token claims",
			})
			c.Abort()
			return
		}

		// ---- STEP 5: Extract user ID from claims ----
		// "sub" = subject = who the token is for (the user ID)
		userID, ok := claims["sub"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid user ID in token",
			})
			c.Abort()
			return
		}

		// ---- STEP 6: Extract role from claims ----
		role, ok := claims["role"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid role in token",
			})
			c.Abort()
			return
		}

		// ---- STEP 7: Store in context ----
		// Handlers can now access user info:
		// userID, _ := c.Get("user_id")
		// role, _ := c.Get("user_role")
		c.Set("user_id", userID)
		c.Set("user_role", role)
		c.Set("claims", claims)

		// ---- STEP 8: Allow request to continue ----
		c.Next()
	}
}

// ============================================
// REQUIRE ROLE MIDDLEWARE
// ============================================
// This middleware checks if user has required role
// Must be used AFTER AuthRequired middleware
// 
// Example:
// authRoutes.GET("/admin/users",
//   middleware.AuthRequired(),
//   middleware.RequireRole("Admin"),
//   handlers.GetAllUsers,
// )
func RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ---- STEP 1: Get user role from context ----
		// AuthRequired middleware already put it there
		userRole, exists := c.Get("user_role")
		if !exists {
			// This shouldn't happen if AuthRequired is applied first
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "No user role in context",
			})
			c.Abort()
			return
		}

		role, ok := userRole.(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid role type",
			})
			c.Abort()
			return
		}

		// ---- STEP 2: Check if user has required role ----
		// Allow multiple roles for flexibility
		// Example: Admin can access traveler routes too
		allowedRoles := []string{requiredRole}
		
		// Admin can access everything
		if role == "Admin" {
			c.Next()
			return
		}

		// Check if user has required role
		hasRole := false
		for _, allowed := range allowedRoles {
			if role == allowed {
				hasRole = true
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("This endpoint requires %s role, but you are %s", requiredRole, role),
			})
			c.Abort()
			return
		}

		// ---- STEP 3: Allow request to continue ----
		c.Next()
	}
}

// ============================================
// OPTIONAL AUTH MIDDLEWARE
// ============================================
// This middleware checks for token but doesn't require it
// Useful for endpoints that work both authenticated and unauthenticated
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		
		var tokenString string
		
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}

		// If no Bearer token, check cookies
		if tokenString == "" {
			cookie, err := c.Cookie("karibu_access")
			if err != nil {
				// No token, that's OK - just continue
				c.Next()
				return
			}
			tokenString = cookie
		}

		// Try to parse and verify token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(os.Getenv("ACCESS_SECRET")), nil
		})

		// If token is valid, extract info
		if err == nil && token.Valid {
			claims, ok := token.Claims.(jwt.MapClaims)
			if ok {
				if userID, ok := claims["sub"].(string); ok {
					c.Set("user_id", userID)
				}
				if role, ok := claims["role"].(string); ok {
					c.Set("user_role", role)
				}
			}
		}

		// Continue regardless (authentication is optional)
		c.Next()
	}
}