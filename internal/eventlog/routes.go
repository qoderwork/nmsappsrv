package eventlog

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all event log routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 事件日志
	rg.GET("/event-logs", h.ListEventLogs)
	rg.GET("/event-logs/:id", h.GetEventLog)
	rg.GET("/event-logs/task/:taskId", h.ListTaskEventLogs)
}
