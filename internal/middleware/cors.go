package middleware

import (
	"github.com/gin-gonic/gin"

	"nmsappsrv/pkg/logger"
)

// CORSMiddleware handles Cross-Origin Resource Sharing.
// If allowedOrigins is empty or contains "*", all origins are allowed.
// In production, pass a specific list of allowed origins from config.
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	// "*" as a list element means allow all origins (common convention
	// for env-var override like NMS_SERVER_CORS_ALLOWED_ORIGINS="*").
	allowAll := len(allowedOrigins) == 0
	originSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o == "*" {
			allowAll = true
		} else {
			originSet[o] = true
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		allowed := false
		if allowAll {
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
