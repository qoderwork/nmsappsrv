package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// maxAuditBodyLen caps the logged request body to avoid oversized rows.
const maxAuditBodyLen = 16384

// AuditLogEntry carries the data captured by AuditMiddleware.
type AuditLogEntry struct {
	Username           string
	IPAddress          string
	LogName            string
	RecordDetail       string
	Results            int // 1=success, 2=failure
	FailureReason      string
	OperationStartTime time.Time
	OperationEndTime   time.Time
	TenantID           int
}

// AuditLogWriter is the callback contract for persisting audit log entries.
type AuditLogWriter interface {
	Write(entry *AuditLogEntry)
}

// AuditMiddleware captures system-operator audit logs for every /api/v1 request.
// It writes entries asynchronously via the provided writer (never blocks).
func AuditMiddleware(writer AuditLogWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		route := c.FullPath()
		if route == "" {
			c.Next()
			return
		}

		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			_ = c.Request.Body.Close()
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		startTime := time.Now()
		c.Next()
		endTime := time.Now()

		status := c.Writer.Status()
		results := 1
		failureReason := ""
		if status >= 400 {
			results = 2
			if status == 401 || status == 403 {
				failureReason = "Unauthorized"
			} else if status >= 500 {
				failureReason = "Internal Server Error"
			} else {
				failureReason = "Request Failed"
			}
		}

		bodyBytes = maskSensitiveFields(bodyBytes)
		recordDetail := string(bodyBytes)
		if len(recordDetail) > maxAuditBodyLen {
			recordDetail = recordDetail[:maxAuditBodyLen]
		}

		writer.Write(&AuditLogEntry{
			Username:           GetUsername(c),
			IPAddress:          realClientIP(c),
			LogName:            c.Request.Method + " " + route,
			RecordDetail:       recordDetail,
			Results:            results,
			FailureReason:      failureReason,
			OperationStartTime: startTime,
			OperationEndTime:   endTime,
			TenantID:           tenantIDOf(c),
		})
	}
}

// realClientIP resolves the real client IP behind reverse proxies (nginx, compose, etc.).
// Checks X-Real-IP first, then X-Forwarded-For, then falls back to c.ClientIP().
func realClientIP(c *gin.Context) string {
	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		return ip
	}
	if fwd := c.GetHeader("X-Forwarded-For"); fwd != "" {
		// Take the first (original client) IP from the comma-separated list.
		if idx := strings.IndexByte(fwd, ','); idx > 0 {
			return strings.TrimSpace(fwd[:idx])
		}
		return strings.TrimSpace(fwd)
	}
	return c.ClientIP()
}

// maskSensitiveFields replaces values of known sensitive JSON keys (password,
// secret, token) with "***" to avoid storing credentials in audit logs.
func maskSensitiveFields(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return b
	}
	sensitive := map[string]bool{
		"password":      true,
		"oldPassword":   true,
		"newPassword":   true,
		"confirmPassword": true,
		"secret":        true,
		"token":         true,
		"secretKey":     true,
		"privateKey":    true,
	}
	masked := false
	for k := range m {
		if sensitive[k] && m[k] != nil {
			m[k] = "***"
			masked = true
		}
	}
	if !masked {
		return b
	}
	out, err := json.Marshal(m)
	if err != nil {
		return b
	}
	return out
}

// tenantIDOf resolves tenancy id from context.
func tenantIDOf(c *gin.Context) int {
	if v, ok := c.Get("tenant_id"); ok {
		switch t := v.(type) {
		case int:
			return t
		case int64:
			return int(t)
		case string:
			if n, err := strconv.Atoi(t); err == nil {
				return n
			}
		}
	}
	return GetTenantId(c)
}
