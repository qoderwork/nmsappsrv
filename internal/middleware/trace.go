package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nmsappsrv/pkg/logger"
)

const TraceIDKey = "trace_id"

// TraceID generates or extracts a trace ID for each request.
// It reads X-Request-Id from the request header; if absent, generates a new UUID.
// The trace ID is stored in gin.Context and added to the response header.
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Request-Id")
		if traceID == "" {
			traceID = uuid.New().String()
		}
		c.Set(TraceIDKey, traceID)
		c.Header("X-Request-Id", traceID)
		c.Next()
	}
}

// GetTraceID extracts the trace ID from gin.Context.
// Returns empty string if not set.
func GetTraceID(c *gin.Context) string {
	v, exists := c.Get(TraceIDKey)
	if !exists {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// RequestLogger logs each request with trace_id, method, URI, client IP, status code, and latency.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		traceID := GetTraceID(c)
		logger.Infof("[trace=%s] %s %s %s %d %v",
			traceID, c.Request.Method, c.Request.RequestURI,
			c.ClientIP(), c.Writer.Status(), latency)
	}
}
