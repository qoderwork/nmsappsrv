package reset

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all reset task routes on the given router group.
// Mirrors ResetManagementController (base @RequestMapping("/api/v2/")) — all POST.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/addResetTask", h.AddResetTask)
	rg.POST("/deleteResetTask", h.DeleteResetTask)
	rg.POST("/cancelResetTask", h.CancelResetTask)
	rg.POST("/startResetTask", h.StartResetTask)
	rg.POST("/listResetTask", h.ListResetTasks)
	rg.POST("/listResetTaskResult", h.ListResetTaskResults)
}
