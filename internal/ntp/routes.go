package ntp

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all NTP configuration routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// --- REST-style routes (preferred) ---
	rg.GET("/ntp/config", h.ListNTPConfig)
	rg.PUT("/ntp/config", h.UpdateNTPConfig)
	rg.GET("/ntp/status", h.GetNTPStatus)

	// --- Legacy RPC-style routes (deprecated) ---
	// Deprecated: use GET /ntp/config instead
	rg.POST("/listNTPConfig", h.ListNTPConfig)
	// Deprecated: use PUT /ntp/config instead
	rg.POST("/updateNTPConfig", h.UpdateNTPConfig)
	// Deprecated: use GET /ntp/status instead
	rg.POST("/getNTPStatus", h.GetNTPStatus)
}
