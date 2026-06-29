package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"nmsappsrv/pkg/logger"
)

// CORSMiddleware handles Cross-Origin Resource Sharing.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-License-Id")
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
