package deviceauth

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all device auth configuration routes on the given
// router group. These are mounted under the /api/v2/ group in cmd/main.go.
//
// In Java, device-facing ACS authentication is handled by ACSController at
// /acs, /cpeAcs (TR-069 CWMP endpoints), not by a dedicated config controller.
// The Go deviceauth module manages the device auth configuration via the web
// UI (GetConfig / SaveConfig / GetAuthInfo). Since there is no clear 1:1 Java
// controller mapping, the existing routes are preserved.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/device-auth/config", h.GetConfig)
	rg.PUT("/device-auth/config", h.SaveConfig)
	rg.GET("/device-auth/info", h.GetAuthInfo)
}
