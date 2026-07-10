package reset

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all reset task routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 重启恢复
	rg.POST("/reset-tasks", h.AddResetTask)
	rg.GET("/reset-tasks", h.ListResetTasks)
	rg.DELETE("/reset-tasks/:id", h.DeleteResetTask)
	rg.POST("/reset-tasks/:id/start", h.StartResetTask)
	rg.POST("/reset-tasks/:id/cancel", h.CancelResetTask)
	rg.GET("/reset-tasks/:id/results", h.ListResetTaskResults)
}
