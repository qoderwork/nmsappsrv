package reboot

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all reboot task routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/reboot-tasks", h.AddRebootTask)
	rg.GET("/reboot-tasks", h.ListRebootTasks)
	rg.DELETE("/reboot-tasks/:id", h.DeleteRebootTask)
	rg.POST("/reboot-tasks/:id/start", h.StartRebootTask)
	rg.POST("/reboot-tasks/:id/cancel", h.CancelRebootTask)
	rg.GET("/reboot-tasks/:id/results", h.ListRebootTaskResults)
}
