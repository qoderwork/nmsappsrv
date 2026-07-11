package initserver

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all init server routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// --- REST-style routes (preferred) ---
	rg.GET("/initserver/config", h.GetConfig)
	rg.PUT("/initserver/config", h.SaveConfig)
	rg.GET("/initserver/config/export", h.ExportConfig)
	rg.POST("/initserver/config/import", h.UploadConfig)

	// --- Legacy RPC-style routes (deprecated) ---
	// Deprecated: use GET /initserver/config instead
	rg.GET("/initserver/getConfig", h.GetConfig)
	// Deprecated: use PUT /initserver/config instead
	rg.POST("/initserver/save", h.SaveConfig)
	// Deprecated: use GET /initserver/config/export instead
	rg.GET("/initserver/exportConfig", h.ExportConfig)
	// Deprecated: use POST /initserver/config/import instead
	rg.POST("/initserver/uploadConfig", h.UploadConfig)
}
