//go:build !no_license

package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"nmsappsrv/pkg/utils"
)

// LicenseMiddleware gates authenticated endpoints unless a valid, non-expired
// license is active (or enforcement is disabled at runtime).
//
// Whitelisting is achieved by only attaching this middleware to the groups that
// need gating (the authenticated API group and the REST group). Public routes
// such as /health, /ready, /login, /license/upload and /license/info are simply
// never registered under this middleware, so they stay reachable even when no
// license is present — an admin can log in and upload a license.
func LicenseMiddleware(gate LicenseGate) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !gate.Required() {
			c.Next()
			return
		}
		if gate.IsValid() {
			c.Next()
			return
		}
		utils.Error(c, http.StatusForbidden, "NMS License expired")
		c.Abort()
	}
}
