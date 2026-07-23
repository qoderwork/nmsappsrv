package devicelog

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all device log collection routes on the given router group.
// Routes mirror Java's CollectDeviceLogFileManagementController (base /api/v2/).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 设备日志采集 (Device Log Collection)
	rg.POST("/addDeviceLogCollectionTask", h.AddLogCollectionTask)
	// TODO: rg.POST("/addCPELogCollectionTask", h.AddCPELogCollectionTask) // handler not yet implemented
	// TODO: rg.POST("/listCPEDeviceLogCollectionResult", h.ListCPEDeviceLogCollectionResult) // handler not yet implemented
	rg.POST("/listGNBDeviceLogCollectionResult", h.ListLogCollectionResults)
	rg.POST("/deleteAllLogFileInDevice", h.DeleteAllLogFile)
	// TODO: rg.POST("/deleteAllLogFileInCPE", h.DeleteAllLogFileInCPE) // handler not yet implemented
	rg.POST("/deleteLogFile", h.DeleteLogFile)
	// TODO: rg.POST("/deleteCPELogFile", h.DeleteCPELogFile) // handler not yet implemented
	rg.GET("/downloadLogFile", h.DownloadLogFile)
	// TODO: rg.GET("/downloadCPELogFile", h.DownloadCPELogFile) // handler not yet implemented
	rg.POST("/listLogFile", h.ListLogFiles)
	// TODO: rg.POST("/listCPELogFile", h.ListCPELogFile) // handler not yet implemented
	rg.POST("/enableLogPeriodicUpload", h.EnablePeriodicUpload)
	rg.POST("/disableLogPeriodUpload", h.DisablePeriodicUpload)
}
