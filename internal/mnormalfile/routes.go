package mnormalfile

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all MNormal file management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/mnormal-file/init-upload", h.InitUpload)
	rg.POST("/mnormal-file/upload-chunk", h.UploadChunk)
	rg.POST("/mnormal-file/check-chunk", h.CheckChunk)
	rg.POST("/mnormal-file/assemble", h.Assemble)
	rg.POST("/mnormal-file/upload", h.Upload)
	rg.POST("/mnormal-file/delete", h.Delete)
	rg.POST("/mnormal-file/list", h.List)
	rg.POST("/mnormal-file/detail", h.Detail)
	rg.POST("/mnormal-file/download-to-device", h.DownloadToDevice)
	rg.POST("/mnormal-file/download-results", h.DownloadResults)
}
