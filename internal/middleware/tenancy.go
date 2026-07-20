package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"
)

// publicPathPrefixes lists path prefixes that bypass TenancyMiddleware
// via prefix matching (e.g. /acs-file-server/ matches all sub-paths).
var publicPathPrefixes = []string{
	"/health",
	"/ready",
	"/metrics",
	"/swagger",
	"/acs-file-server/", // device-facing file server (Basic auth)
	"/ws",               // WebSocket handshake (token-bound)
	"/webssh",           // WebSSH
}

// publicExactPaths lists paths that bypass TenancyMiddleware via exact
// matching. Mirrors Java InterceptorConfig.excludePathPatterns
// (LicenseCheckInterceptor bypass) + Spring Security permitAll paths.
var publicExactPaths = []string{
	"/api/v1/login",
	"/api/v1/logout",
	"/api/v1/captchaImage",
	"/api/v1/users/login-failed-times",
	"/api/v1/users/need-change-password",
	"/api/v1/users/reset-password-by-link",
	"/api/v2/getLicenseInfo",
	"/api/v2/uploadLicenseFile",
	"/api/v2/getPermissionIdsForUser",
	"/api/v2/getLogo",
	"/api/v2/caFile/list",
	"/api/v2/downloadPasswordRSAPublicKey",
}

// isPublicPath returns true if the path matches a public prefix or exact path.
func isPublicPath(path string) bool {
	for _, p := range publicPathPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	for _, p := range publicExactPaths {
		if path == p {
			return true
		}
	}
	return false
}

// TenancyMiddleware extracts license_id and tenancy_id from request headers
// or JWT context, and sets tenancy_id in gin.Context for downstream handlers.
// Public paths (health, metrics, swagger, device-facing file server, etc.)
// bypass this check entirely — they have no per-tenant context.
func TenancyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}

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
