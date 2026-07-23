package parammonitor

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all parameter monitor routes on the given router group.
// Routes mirror the Java backend ParameterMonitorController under /api/v2/ base path.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// === ParameterMonitorController ===
	rg.POST("/parameterMonitorAdd", h.AddMonitorConfig)
	rg.POST("/parameterMonitorDelete", h.DeleteMonitorConfig)
	rg.POST("/parameterViewInfo", h.ViewMonitorConfig)
	rg.POST("/parameterMonitorUpdate", h.UpdateMonitorConfig)
	rg.POST("/parameterMonitorList", h.ListMonitorConfigs)
	rg.POST("/parameterMonitorToggle", h.ToggleMonitorConfig)
	rg.POST("/getRealtimeParameterMonitorData", h.GetRealtimeMonitorData)
	rg.POST("/parameterMonitorReload", h.ReloadMonitorParameters)
	rg.POST("/batchQueryDeviceParameters", h.BatchQueryDeviceParameters)
}
