package blacklist

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all device blacklist routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 设备黑名单
	rg.POST("/blacklist/list", h.ListDeviceBlackList)
	rg.POST("/blacklist/add", h.AddDeviceToBlackList)
	rg.POST("/blacklist/delete", h.DeleteDeviceFromBlackList)
	rg.POST("/blacklist/batch-delete", h.BatchDeleteDeviceFromBlackList)
	rg.POST("/blacklist/operation-logs", h.ListBlackListOperationLog)
}
