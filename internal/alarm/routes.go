package alarm

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all alarm management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 告警
	rg.GET("/alarms", h.ListAlarms)
	rg.GET("/alarms/:id", h.GetAlarm)
	rg.POST("/alarms/:id/clear", h.ClearAlarm)
	rg.PUT("/alarms/batch-clear", h.BatchClearAlarms)
	rg.GET("/alarm-library", h.ListAlarmLibrary)
	rg.GET("/alarm-templates", h.ListAlarmTemplates)
	rg.POST("/alarm-templates", h.CreateAlarmTemplate)
	rg.PUT("/alarm-templates/:id", h.UpdateAlarmTemplate)
	rg.GET("/alarm-filters", h.ListAlarmFilters)
	rg.POST("/alarm-filters", h.CreateAlarmFilter)
	rg.PUT("/alarm-filters/:id", h.UpdateAlarmFilter)
	rg.DELETE("/alarm-filters/:id", h.DeleteAlarmFilter)
}
