package pmfile

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all PM file routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/pm-files/upload", h.Upload)
	rg.GET("/pm-files", h.ListFiles)
	rg.GET("/pm-files/:id", h.GetFile)
	rg.GET("/pm-files/:id/download", h.DownloadFile)
	rg.DELETE("/pm-files/:id", h.DeleteFile)
}
