package alarm

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all alarm management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 告警
	rg.GET("/alarms", h.ListAlarms)
	rg.GET("/alarms/severity-count", h.GetSeverityCount)
	rg.GET("/alarms/statistic/top-n", h.QueryAlarmStatisticTopN)
	rg.GET("/alarms/:id", h.GetAlarm)
	rg.POST("/alarms/:id/clear", h.ClearAlarm)
	rg.POST("/alarms/:id/confirm", h.ConfirmAlarm)
	rg.POST("/alarms/:id/unconfirm", h.UnconfirmAlarm)
	rg.PUT("/alarms/batch-clear", h.BatchClearAlarms)
	rg.POST("/alarms/:id/comment", h.AddCommentForAlarm)
	rg.GET("/alarm-library", h.ListAlarmLibrary)
	rg.POST("/alarm-library/import", h.ImportAlarmLibrary)
	rg.GET("/alarm-library/template", h.DownloadAlarmLibraryTemplate)
	rg.GET("/alarm-templates", h.ListAlarmTemplates)
	rg.POST("/alarm-templates", h.CreateAlarmTemplate)
	rg.PUT("/alarm-templates/:id", h.UpdateAlarmTemplate)
	rg.GET("/alarm-filters", h.ListAlarmFilters)
	rg.POST("/alarm-filters", h.CreateAlarmFilter)
	rg.PUT("/alarm-filters/:id", h.UpdateAlarmFilter)
	rg.DELETE("/alarm-filters/:id", h.DeleteAlarmFilter)
	rg.GET("/alarm-sync-config", h.GetAlarmSyncConfig)
	rg.PUT("/alarm-sync-config", h.UpdateAlarmSyncConfig)

	// 邮件通知配置
	rg.GET("/email-notification/config", h.ListEmailNotificationConfig)
	rg.PUT("/email-notification/config", h.UpdateEmailNotificationConfig)
}
