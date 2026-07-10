package monitor

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all monitor task routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 监控任务
	rg.GET("/monitor-tasks", h.ListMonitorTasks)
	rg.GET("/monitor-tasks/:id", h.GetMonitorTask)
	rg.POST("/monitor-tasks", h.CreateMonitorTask)
	rg.PUT("/monitor-tasks/:id", h.UpdateMonitorTask)
	rg.DELETE("/monitor-tasks/:id", h.DeleteMonitorTask)
	rg.GET("/monitor-data", h.GetMonitorData)
	rg.GET("/monitor-elements", h.GetMonitorElements)
	rg.PUT("/monitor-elements", h.SaveMonitorElements)
	rg.GET("/monitor-parameters", h.GetMonitorParameters)
	rg.PUT("/monitor-parameters", h.SaveMonitorParameters)
}
