package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"nmsappsrv/pkg/logger"
)

// CORSMiddleware handles Cross-Origin Resource Sharing.
// If allowedOrigins is empty, all origins are allowed (development mode).
// In production, pass a specific list of allowed origins from config.
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	originSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = true
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		allowed := false
		if len(allowedOrigins) == 0 {
			// Dev mode: allow all
			allowed = true
			c.Header("Access-Control-Allow-Origin", "*")
		} else if originSet[origin] {
			allowed = true
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if !allowed && origin != "" {
			logger.Warnf("CORS: origin %s not allowed for %s %s",
				origin, c.Request.Method, c.Request.RequestURI)
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-License-Id, X-Tenancy-Id")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// RequestLogger logs each request's method, URI, client IP, status code, and latency.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		logger.Infof("[%s] %s %s %d %v",
			c.Request.Method, c.Request.RequestURI,
			c.ClientIP(), c.Writer.Status(), latency)
	}
}
