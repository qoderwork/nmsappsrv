package device

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all device management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 设备管理
	rg.GET("/devices", h.ListDevices)
	rg.GET("/devices/:id", h.GetDevice)
	rg.POST("/devices", h.CreateDevice)
	rg.PUT("/devices/:id", h.UpdateDevice)
	rg.DELETE("/devices/:id", h.DeleteDevice)
	rg.POST("/devices/import", h.ImportDevices)
	rg.POST("/devices/:id/empty-commands", h.EmptyCommands)

	// 设备组
	rg.GET("/device-groups", h.ListGroups)
	rg.POST("/device-groups", h.CreateGroup)
	rg.PUT("/device-groups/:id", h.UpdateGroup)
	rg.DELETE("/device-groups/:id", h.DeleteGroup)
}
