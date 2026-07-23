package blacklist

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all device blacklist routes on the given router group.
// Aligned with Java ElementBlackListManagementController: @RequestMapping("api/v1/")
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/listDeviceBlackList", h.ListDeviceBlackList)
	rg.POST("/addDeviceToBlackList", h.AddDeviceToBlackList)
	rg.POST("/deleteDeviceFromBlackList", h.DeleteDeviceFromBlackList)
	rg.POST("/batchDeleteDeviceFromBlackList", h.BatchDeleteDeviceFromBlackList)
	rg.POST("/listBlackListOperationLog", h.ListBlackListOperationLog)
}
