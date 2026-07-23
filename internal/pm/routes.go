package pm

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all performance monitoring routes on the given router group.
// Routes mirror Java PerformanceManagementController (base @RequestMapping("/api/v2/")),
// using Java path names directly so the frontend can call the same endpoints.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// KPI Set
	rg.POST("/listKPISet", h.ListKPISets)
	rg.POST("/addKPISet", h.CreateKPISet)
	// TODO: rg.POST("/updateKPISet", h.UpdateKPISet) // handler not yet implemented
	rg.POST("/deleteKPISet", h.DeleteKPISet)

	// KPI
	rg.POST("/listKPI", h.ListKPIs)
	rg.POST("/listAllKPI", h.ListAllKPIs)
	rg.POST("/addKPI", h.CreateKPI)
	rg.POST("/updateKPI", h.UpdateKPI)
	rg.POST("/deleteKPI", h.DeleteKPI)
	rg.POST("/viewKPI", h.GetKPI)
	rg.POST("/importKPI", h.ImportKPIs)
	rg.POST("/downloadKPITemplate", h.DownloadKPITemplate)

	// KPI Template
	rg.POST("/addKPITemplate", h.CreateKPITemplate)
	rg.POST("/listKPITemplate", h.ListKPITemplates)
	rg.POST("/viewKPITemplate", h.GetKPITemplate)
	rg.POST("/updateKPITemplate", h.UpdateKPITemplate)
	rg.POST("/deleteKPITemplate", h.DeleteKPITemplate)
	rg.POST("/listTemplateTableData", h.GetDashboardData)
	// TODO: rg.POST("/listTemplateChartData", h.ListTemplateChartData) // handler not yet implemented

	// KPI Measurement
	rg.POST("/listKPIMeas", h.ListKPIMeas)
	rg.POST("/updateMeasTaskSwitch", h.UpdateMeasTaskSwitch)

	// PM File
	rg.GET("/downloadPMFile", h.DownloadPMFile)
	rg.POST("/exportPMExcel", h.ExportPMExcel)

	// KPI Alarm Template
	rg.POST("/addKPIAlarmTemplate", h.CreateKPIAlarm)
	rg.POST("/updateKPIAlarmTemplate", h.UpdateKPIAlarm)
	rg.POST("/listKPIAlarmTemplate", h.ListKPIAlarms)
	rg.POST("/deleteKPIAlarmTemplate", h.DeleteKPIAlarm)
	rg.POST("/viewKPIAlarmTemplate", h.GetKPIAlarmTemplate)
	rg.POST("/updateKPIAlarmTemplateStatus", h.UpdateKPIAlarmTemplateStatus)

	// Replenish Task
	rg.POST("/addReplenishTask", h.AddReplenishTask)
	rg.POST("/listReplenishTask", h.ListReplenishTask)
	rg.POST("/viewReplenishTask", h.ViewReplenishTask)
	rg.POST("/listDeviceReplenish", h.ListDeviceReplenish)
}
