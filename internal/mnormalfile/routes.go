package mnormalfile

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all MNormal file management routes on the given router group.
// Routes mirror the Java backend DeviceMNormalFileManagementController (all POST).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/initDeviceMNormalFileUpload", h.InitUpload)
	rg.POST("/uploadDeviceMNormalFileChunk", h.UploadChunk)
	rg.POST("/checkDeviceMNormalFileChunk", h.CheckChunk)
	rg.POST("/assembleDeviceMNormalFile", h.Assemble)
	rg.POST("/uploadDeviceMNormalFile", h.Upload)
	rg.POST("/deleteMNormalFile", h.Delete)
	rg.POST("/DeviceMNormalFileList", h.List)
	rg.POST("/detailMNormalFile", h.Detail)
	rg.POST("/downloadToDevice", h.DownloadToDevice)
	rg.POST("/downloadResults", h.DownloadResults)
}
