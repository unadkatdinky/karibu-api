package middleware

import (
	"context"
	
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// Create a rate limiter for auth endpoints (5 requests per minute per IP)
func AuthRateLimiter() gin.HandlerFunc {
	store := memory.NewStore()
	
	// 5 requests per minute
	rate := limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  5,
	}
	
	instance := limiter.New(store, rate)
	
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		
		context, err := instance.Get(context.Background(), clientIP)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Rate limit service error",
			})
			c.Abort()
			return
		}
		
		if context.Reached {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many attempts. Please try again in 1 minute.",
				"retry_after": 60,
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// General API rate limiter (100 requests per minute)
func APIRateLimiter() gin.HandlerFunc {
	store := memory.NewStore()
	
	rate := limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  100,
	}
	
	instance := limiter.New(store, rate)
	
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		context, err := instance.Get(context.Background(), clientIP)
		
		if err != nil {
			c.Next()
			return
		}
		
		if context.Reached {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "API rate limit exceeded",
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}