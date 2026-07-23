package tcpdump

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all tcpdump routes on the given router group.
// Routes mirror Java's TcpdumpManagementController (base /api/v2/).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/listNetworkCards", h.ListNetworkCards)
	rg.POST("/doTcpdump", h.DoCapture)
	rg.POST("/listTcpdumpFiles", h.ListFiles)
	rg.GET("/downloadTcpdumpFile", h.DownloadFile)
	rg.POST("/deleteTcpdumpFile", h.DeleteFile)
	rg.POST("/batchDeleteTcpdumpFile", h.BatchDeleteFiles)
}
