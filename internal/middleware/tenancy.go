package middleware

import (
	"github.com/gin-gonic/gin"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"
)

// TenancyMiddleware extracts license_id and tenancy_id from request headers
// or JWT context, and sets tenancy_id in gin.Context for downstream handlers.
func TenancyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract license_id: header first, then JWT context
		licenseID := c.GetHeader("X-License-Id")
		if licenseID == "" {
			if val, exists := c.Get("license_id"); exists {
				if s, ok := val.(string); ok {
					licenseID = s
				}
			}
		}

		// Extract tenancy_id: header first, then JWT context
		tenancyID := c.GetHeader("X-Tenancy-Id")
		if tenancyID == "" {
			if val, exists := c.Get("tenancy_id"); exists {
				if s, ok := val.(string); ok {
					tenancyID = s
				}
			}
		}

		// license_id is required
		if licenseID == "" {
			logger.Warnf("missing license_id for request %s %s from %s",
				c.Request.Method, c.Request.RequestURI, c.ClientIP())
			utils.Error(c, 403, "license_id required")
			c.Abort()
			return
		}

		// Set values in context for downstream handlers
		c.Set("license_id", licenseID)
		if tenancyID != "" {
			c.Set("tenancy_id", tenancyID)
		}

		c.Next()
	}
}
