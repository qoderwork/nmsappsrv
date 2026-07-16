package pm

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all performance monitoring routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 性能监控
	rg.GET("/pm/kpis", h.ListKPIs)
	rg.GET("/pm/kpis/:id", h.GetKPI)
	rg.POST("/pm/kpis", h.CreateKPI)
	rg.PUT("/pm/kpis/:id", h.UpdateKPI)
	rg.DELETE("/pm/kpis/:id", h.DeleteKPI)
	rg.POST("/pm/kpis/all", h.ListAllKPIs)
	rg.GET("/pm/kpi-sets", h.ListKPISets)
	rg.POST("/pm/kpi-sets", h.CreateKPISet)
	rg.DELETE("/pm/kpi-sets/:id", h.DeleteKPISet)
	rg.GET("/pm/kpi-templates", h.ListKPITemplates)
	rg.POST("/pm/kpi-templates", h.CreateKPITemplate)
	rg.PUT("/pm/kpi-templates/:id", h.UpdateKPITemplate)
	rg.GET("/pm/kpi-templates/:id", h.GetKPITemplate)
	rg.DELETE("/pm/kpi-templates/:id", h.DeleteKPITemplate)
	rg.GET("/pm/kpi-templates/:id/download", h.DownloadKPITemplate)
	rg.GET("/pm/data", h.GetDashboardData)
	rg.GET("/pm/file-logs", h.ListPMFileLogs)
	rg.GET("/pm/kpi-alarms", h.ListKPIAlarms)
	rg.POST("/pm/kpi-alarms", h.CreateKPIAlarm)
	rg.PUT("/pm/kpi-alarms/:id", h.UpdateKPIAlarm)
	rg.DELETE("/pm/kpi-alarms/:id", h.DeleteKPIAlarm)
	rg.GET("/pm/kpi-alarms/:id", h.GetKPIAlarmTemplate)
	rg.PUT("/pm/kpi-alarms/:id/status", h.UpdateKPIAlarmTemplateStatus)
	rg.GET("/pm/dashboard", h.GetDashboardData)
	rg.GET("/pm/pdcp-traffic", h.GetPDCPTraffic)
	rg.GET("/pm/device-online-info", h.GetDeviceOnlineInfo)
	rg.GET("/pm/product-type-device-count", h.GetProductTypeAndDeviceCount)
	rg.POST("/pm/export-excel", h.ExportPMExcel)
	rg.POST("/pm/kpis/import", h.ImportKPIs)
	rg.GET("/pm/file-logs/download", h.DownloadPMFile)
	rg.POST("/pm/kpi-meas", h.ListKPIMeas)
	rg.POST("/pm/kpi-meas/switch", h.UpdateMeasTaskSwitch)
}
