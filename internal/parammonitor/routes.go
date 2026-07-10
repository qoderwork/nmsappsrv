package parammonitor

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all parameter monitor routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 参数监控 (Parameter Monitor)
	rg.POST("/param-monitor/configs", h.AddMonitorConfig)
	rg.POST("/param-monitor/configs/delete", h.DeleteMonitorConfig)
	rg.POST("/param-monitor/configs/view", h.ViewMonitorConfig)
	rg.PUT("/param-monitor/configs", h.UpdateMonitorConfig)
	rg.GET("/param-monitor/configs", h.ListMonitorConfigs)
	rg.POST("/param-monitor/configs/toggle", h.ToggleMonitorConfig)
	rg.POST("/param-monitor/realtime", h.GetRealtimeMonitorData)
	rg.POST("/param-monitor/reload", h.ReloadMonitorParameters)
	rg.POST("/param-monitor/batch-query", h.BatchQueryDeviceParameters)
	rg.POST("/param-monitor/batch-query-live", h.BatchQueryDeviceParametersLive)

	// 参数监控阈值告警 (Parameter Monitor Threshold Alerts)
	rg.POST("/param-monitor/threshold", h.CreateThresholdRule)
	rg.PUT("/param-monitor/threshold/:id", h.UpdateThresholdRule)
	rg.DELETE("/param-monitor/threshold/:id", h.DeleteThresholdRule)
	rg.GET("/param-monitor/threshold", h.ListThresholdRules)
	rg.GET("/param-monitor/threshold/:id", h.GetThresholdRule)
	rg.POST("/param-monitor/threshold/test", h.TestThresholdRule)
}
