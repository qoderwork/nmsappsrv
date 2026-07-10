package initserver

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all init server routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// Init Server (零配置初始化服务器)
	rg.GET("/initserver/getConfig", h.GetConfig)
	rg.POST("/initserver/save", h.SaveConfig)
	rg.GET("/initserver/exportConfig", h.ExportConfig)
	rg.POST("/initserver/uploadConfig", h.UploadConfig)
}
