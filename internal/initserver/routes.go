package initserver

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all init server routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/initserver/config", h.GetConfig)
	rg.PUT("/initserver/config", h.SaveConfig)
	rg.GET("/initserver/config/export", h.ExportConfig)
	rg.POST("/initserver/config/import", h.UploadConfig)
}
