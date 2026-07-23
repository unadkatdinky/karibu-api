package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"karibu-api/internal/utils"
	"github.com/sirupsen/logrus"
)

// LoggerMiddleware logs all incoming requests
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		path := c.Request.RequestURI
		method := c.Request.Method

		c.Next()

		duration := time.Since(startTime)
		statusCode := c.Writer.Status()

		utils.Log.WithFields(logrus.Fields{
			"method":       method,
			"path":         path,
			"status_code":  statusCode,
			"duration_ms":  duration.Milliseconds(),
			"ip":           c.ClientIP(),
			"user_agent":   c.Request.UserAgent(),
			"content_type": c.ContentType(),
		}).Info("HTTP Request")
	}
}