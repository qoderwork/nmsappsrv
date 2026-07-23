package monitor

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all monitor task routes on the given router group.
// Routes mirror Java ValueMonitorManagementController (base @RequestMapping("api/v2/")),
// all endpoints are POST as in the Java controller.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/addMonitorTask", h.CreateMonitorTask)
	rg.POST("/modifyMonitorTask", h.UpdateMonitorTask)
	rg.POST("/deleteMonitorTask", h.DeleteMonitorTask)
	rg.POST("/listMonitorTask", h.ListMonitorTasks)
	rg.POST("/viewMonitorTaskDetail", h.GetMonitorTask)
	rg.POST("/getMonitorStatistics", h.GetMonitorStatistics)
	rg.POST("/getElementInValueMonitorTask", h.GetMonitorElements)
	rg.POST("/getParametersInValueMonitorTask", h.GetMonitorParameters)
}
