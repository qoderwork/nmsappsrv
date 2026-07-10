package devicelog

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all device log collection routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 设备日志采集 (Device Log Collection)
	rg.POST("/device-log/collection", h.AddLogCollectionTask)
	rg.POST("/device-log/collection/results", h.ListLogCollectionResults)
	rg.POST("/device-log/delete-all", h.DeleteAllLogFile)
	rg.POST("/device-log/delete", h.DeleteLogFile)
	rg.POST("/device-log/download", h.DownloadLogFile)
	rg.POST("/device-log/files", h.ListLogFiles)
	rg.POST("/device-log/periodic/enable", h.EnablePeriodicUpload)
	rg.POST("/device-log/periodic/disable", h.DisablePeriodicUpload)
}
