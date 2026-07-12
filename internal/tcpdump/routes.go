package tcpdump

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all tcpdump routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/tcpdump/start", h.StartCapture)
	rg.POST("/tcpdump/stop/:taskId", h.StopCapture)
	rg.GET("/tcpdump/tasks", h.ListTasks)
	rg.GET("/tcpdump/tasks/:id", h.GetTask)
	rg.GET("/tcpdump/tasks/:id/download", h.DownloadCapture)
	rg.DELETE("/tcpdump/tasks/:id", h.DeleteTask)
}
