package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"karibu-api/internal/database"
	"karibu-api/internal/handlers"
	"karibu-api/internal/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// ============================================
	// 1. LOAD ENVIRONMENT VARIABLES
	// ============================================
	// godotenv reads from .env file
	// If .env doesn't exist, it's OK (uses OS environment variables instead)
	godotenv.Load()

	 // ✅ ADD THIS BLOCK immediately after godotenv.Load()
    if os.Getenv("ACCESS_SECRET") == "" || os.Getenv("REFRESH_SECRET") == "" {
        log.Fatal("❌ FATAL: ACCESS_SECRET and REFRESH_SECRET must be set. Check your .env file.")
    }


	// ============================================
	// 2. CONNECT TO DATABASE
	// ============================================
	// This initializes the PostgreSQL connection pool
	// If it fails, the program exits here (log.Fatalf)
	database.Connect()

	// ============================================
	// 3. SETUP GIN FRAMEWORK
	// ============================================
	// gin.Default() creates a router with basic logging
	// In production, consider gin.New() for more control
	r := gin.Default()

	// ============================================
	// 4. GLOBAL MIDDLEWARE (Applied to ALL routes)
	// ============================================

	// Logger Middleware
	// This logs every request: method, path, status code, duration
	// Useful for debugging and monitoring
	r.Use(middleware.LoggerMiddleware())

	// CORS Middleware (Cross-Origin Resource Sharing)
	// Allows your React app (localhost:5173) to talk to this API (localhost:8080)
	//
	// Why CORS?
	// Browsers block requests from different domains by default (security)
	// CORS tells the browser: "It's OK, I allow requests from React"
	//
	// In production, change AllowOrigins to your actual domain:
	// AllowOrigins: []string{"https://karibu.example.com"}
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:5173"}, // Your React dev server
		AllowMethods: []string{"POST", "GET", "OPTIONS", "PUT", "DELETE"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		// AllowCredentials is CRITICAL for HTTP-only cookies to work
		// Without this, cookies won't be sent with requests
		AllowCredentials: true,
		// MaxAge tells browser to cache CORS response (improves performance)
		MaxAge: 86400, // 24 hours
	}))

	// Error Recovery Middleware
	// If a handler panics, this catches it and returns 500 instead of crashing
	r.Use(gin.Recovery())

	// ============================================
	// 5. API ROUTES
	// ============================================

	// Create API group with version prefix
	// Routes will be /api/v1/...
	api := r.Group("/api/v1")
	{
		// ---- HEALTH CHECK ----
		// This endpoint is used by load balancers to check if server is alive
		// Returns 200 if server + database are working
		api.GET("/health", handlers.HealthCheck)

		// ---- AUTHENTICATION ROUTES ----
		// These routes handle user registration and login
		// We apply rate limiting to prevent brute force attacks
		authRoutes := api.Group("/auth")
		{
			// Register: Create new account
			// Rate limited to 5 attempts per minute per IP address
			// Why? Prevent attackers from registering thousands of fake accounts
			authRoutes.POST("/register",
				middleware.AuthRateLimiter(), // Rate limit middleware
				handlers.Register,
			)

			// Login: Authenticate user
			// Rate limited to 5 attempts per minute per IP address
			// Why? Prevent brute force attacks (trying many passwords)
			authRoutes.POST("/login",
				middleware.AuthRateLimiter(),
				handlers.Login,
			)

			// Refresh: Get new access token using refresh token
			// No rate limiting here (users might use this frequently)
			authRoutes.POST("/refresh",
				handlers.RefreshToken,
			)

			// Logout: Invalidate tokens and clear cookies
			// Requires authentication (user must be logged in)
			authRoutes.POST("/logout",
				middleware.AuthRequired(), // Only authenticated users
				handlers.Logout,
			)

			authRoutes.POST("/forgot-password", handlers.ForgotPassword)
			authRoutes.POST("/reset-password", handlers.ResetPassword)
			authRoutes.POST("/verify", handlers.VerifyAccount)
			authRoutes.POST("/resend-otp",
				middleware.AuthRateLimiter(), // Rate limited like other auth-creation endpoints
				handlers.ResendOTP,
			)
		}

		// ---- PROTECTED ROUTES EXAMPLE ----
		// These routes require authentication
		// Only users with valid tokens can access them
		usersRoutes := api.Group("/users")
		usersRoutes.Use(middleware.AuthRequired()) // Apply to entire group
		{
			// Get current user's profile
			usersRoutes.GET("/me", handlers.GetCurrentUser)

			// Update user's profile
			usersRoutes.PUT("/me", handlers.UpdateProfile)
		}

		// ---- ADMIN ROUTES EXAMPLE ----
		// These routes require authentication AND admin role
		adminRoutes := api.Group("/admin")
		adminRoutes.Use(middleware.AuthRequired()) // Must be logged in
		{
			// Get all users (admin only)
			adminRoutes.GET("/users",
				middleware.RequireRole("Admin"), // Only admins
				handlers.GetAllUsers,
			)
		}
	}

	// ============================================
	// 6. 404 HANDLER
	// ============================================
	// If no route matches, return 404
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Endpoint not found",
			"path":  c.Request.RequestURI,
		})
	})

	// ============================================
	// 7. START SERVER WITH GRACEFUL SHUTDOWN
	// ============================================

	// Get port from environment, default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create HTTP server with timeouts
	// These prevent slow clients from holding connections forever
	server := &http.Server{
		Addr:           ":" + port,
		Handler:        r,
		ReadTimeout:    15 * time.Second, // Max time to read request
		WriteTimeout:   15 * time.Second, // Max time to write response
		MaxHeaderBytes: 1 << 20,          // 1MB max header size
	}

	// Start server in a separate goroutine
	// This allows the main goroutine to listen for shutdown signals
	go func() {
		log.Printf("🚀 Server starting on http://localhost:%s", port)
		log.Printf("📍 API available at http://localhost:%s/api/v1", port)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server error: %v", err)
		}
	}()

	// ============================================
	// 8. GRACEFUL SHUTDOWN
	// ============================================
	// This waits for interrupt signal (Ctrl+C, SIGTERM)
	// Then cleanly shuts down the server

	quit := make(chan os.Signal, 1)
	// Listen for SIGINT (Ctrl+C) and SIGTERM (kill signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until signal is received
	sig := <-quit
	log.Printf("📍 Received signal: %v", sig)
	log.Println("⏳ Shutting down server gracefully...")

	// Create context with 5 second timeout for shutdown
	// If server doesn't shut down in 5 seconds, force it
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown server
	// This stops accepting new connections and waits for existing ones to complete
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("❌ Server shutdown error: %v", err)
	}

	// Close database connection
	if database.DB != nil {
		database.DB.Close()
	}

	log.Println("✅ Server stopped")
}
