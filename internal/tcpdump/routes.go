package tcpdump

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all tcpdump routes on the given router group.
// These map 1:1 onto Java's TcpdumpManagementController operations:
//   listNetworkCards   -> GET  /tcpdump/network-cards
//   doTcpdump          -> POST /tcpdump/capture
//   listTcpdumpFiles   -> GET  /tcpdump/files
//   downloadTcpdumpFile-> GET  /tcpdump/files/:name/download
//   deleteTcpdumpFile  -> DELETE /tcpdump/files/:name
//   batchDeleteTcpdump -> POST /tcpdump/files/batch-delete
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/tcpdump/network-cards", h.ListNetworkCards)
	rg.POST("/tcpdump/capture", h.DoCapture)
	rg.GET("/tcpdump/files", h.ListFiles)
	rg.GET("/tcpdump/files/:name/download", h.DownloadFile)
	rg.DELETE("/tcpdump/files/:name", h.DeleteFile)
	rg.POST("/tcpdump/files/batch-delete", h.BatchDeleteFiles)
}
