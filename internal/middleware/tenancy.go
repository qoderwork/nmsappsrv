package middleware

import (
	"strconv"
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

// defaultTenantIdStr is the string representation of the default license ID.
// Corresponds to pkg/database.DefaultTenantId ("1").
const defaultTenantIdStr = "1"

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
	"/api/v1/license/info",
	"/api/v1/license/upload",
	"/api/v1/auth/permissions/user-ids",
	"/api/v1/getLogo",
	"/api/v1/caFile/list",
	"/api/v1/downloadPasswordRSAPublicKey",
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

// extractTenantIDFromJWT parses the JWT from Authorization header and
// extracts the tenant_id claim. Used by TenancyMiddleware when neither
// X-License-Id header nor gin context (set by AuthMiddleware) is available.
func extractTenantIDFromJWT(c *gin.Context) int {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return 0
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return 0
	}
	tokenString := strings.TrimSpace(parts[1])
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return 0
	}
	return claims.TenantID
}

// TenancyMiddleware extracts tenant_id and tenant_id from request headers
// or JWT context, and sets tenant_id in gin.Context for downstream handlers.
// Public paths (health, metrics, swagger, device-facing file server, etc.)
// bypass this check entirely — they have no per-tenant context.
func TenancyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Extract tenant_id: header first, then JWT context, then JWT from Authorization header
		tenantIDStr := c.GetHeader("X-License-Id")
		var tenantID int
		if tenantIDStr == "" {
			// Try JWT context (int type from AuthMiddleware)
			if val, exists := c.Get("tenant_id"); exists {
				switch v := val.(type) {
				case int:
					tenantID = v
				case float64:
					tenantID = int(v)
				}
			}
			// If AuthMiddleware hasn't run yet (e.g. global middleware order),
			// parse JWT directly from Authorization header
			if tenantID <= 0 {
				tenantID = extractTenantIDFromJWT(c)
			}
		} else {
			// Handle "default" as special case (maps to ID 1)
			if tenantIDStr == "default" || tenantIDStr == defaultTenantIdStr {
				tenantID = 1
			} else {
				// Try parsing as integer
				id, err := strconv.Atoi(tenantIDStr)
				if err != nil {
					logger.Warnf("invalid tenant_id header '%s' for request %s %s from %s",
						tenantIDStr, c.Request.Method, c.Request.RequestURI, c.ClientIP())
					utils.Error(c, 403, "invalid tenant_id")
					c.Abort()
					return
				}
				tenantID = id
			}
		}

		// tenant_id is required (must be > 0)
		if tenantID <= 0 {
			logger.Warnf("missing or invalid tenant_id for request %s %s from %s",
				c.Request.Method, c.Request.RequestURI, c.ClientIP())
			utils.Error(c, 403, "tenant_id required")
			c.Abort()
			return
		}

		// Set tenant_id in context for downstream handlers as int
		c.Set("tenant_id", tenantID)

		c.Next()
	}
}
