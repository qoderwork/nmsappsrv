package deviceauth

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all device auth routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// Device Auth (设备认证策略)
	rg.GET("/deviceAuth/getConfig", h.GetConfig)
	rg.POST("/deviceAuth/save", h.SaveConfig)
	rg.GET("/deviceAuth/getAuthInfo", h.GetAuthInfo)
}
