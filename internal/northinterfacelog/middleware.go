package northinterfacelog

import (
	"bytes"
	"io"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"
)

// maxRequestDataLen caps the logged request body to avoid oversized rows.
const maxRequestDataLen = 16384

// AuditMiddleware logs every northbound call to north_interface_log. Unlike the
// Java implementation (which only instruments a handful of endpoints), this
// covers the full northbound surface. It buffers the request body so it can
// both be logged and still reach the downstream handler, then records the
// outcome after the handler chain completes (so the HTTP status is known).
func AuditMiddleware(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		route := c.FullPath()
		if route == "" {
			// No matching route (e.g. 404) — nothing meaningful to audit.
			c.Next()
			return
		}

		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			_ = c.Request.Body.Close()
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		c.Next()

		status := c.Writer.Status()
		result := 0
		if status < 400 {
			result = 1
		}

		reqData := string(bodyBytes)
		if len(reqData) > maxRequestDataLen {
			reqData = reqData[:maxRequestDataLen]
		}

		log := &NorthInterfaceLog{
			LogName:       c.Request.Method + " " + route,
			User:          middleware.GetUsername(c),
			OperationTime: time.Now(),
			Result:        result,
			RequestData:   reqData,
			ElementID:     0,
			PresetTaskID:  0,
			Info:          "HTTP " + strconv.Itoa(status),
			TenancyID:     tenancyIDOf(c),
		}
		utils.SafeGo("north-interface-audit", func() {
			_ = svc.Save(log)
		})
	}
}

// tenancyIDOf resolves the tenancy id from context, falling back to the JWT
// license id. In the Java northbound context tenancyId == licenseId, so the
// JWT claim is the right fallback when no explicit tenancy header is present.
func tenancyIDOf(c *gin.Context) int {
	if v, ok := c.Get("tenancy_id"); ok {
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
	return middleware.GetLicenseId(c)
}
