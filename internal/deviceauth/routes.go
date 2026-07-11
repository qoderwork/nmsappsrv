package deviceauth

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all device auth routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// --- REST-style routes (preferred) ---
	rg.GET("/device-auth/config", h.GetConfig)
	rg.PUT("/device-auth/config", h.SaveConfig)
	rg.GET("/device-auth/info", h.GetAuthInfo)

	// --- Legacy RPC-style routes (deprecated) ---
	// Deprecated: use GET /device-auth/config instead
	rg.GET("/deviceAuth/getConfig", h.GetConfig)
	// Deprecated: use PUT /device-auth/config instead
	rg.POST("/deviceAuth/save", h.SaveConfig)
	// Deprecated: use GET /device-auth/info instead
	rg.GET("/deviceAuth/getAuthInfo", h.GetAuthInfo)
}
