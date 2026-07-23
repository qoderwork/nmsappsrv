package reboot

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all reboot task routes on the given router group.
// Mirrors RebootManagementController (base @RequestMapping("/api/v2/")) — all POST.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/addRebootTask", h.AddRebootTask)
	// TODO: rg.POST("/addCPERebootTask", h.AddCPERebootTask) // handler not yet implemented
	rg.POST("/deleteRebootTask", h.DeleteRebootTask)
	rg.POST("/cancelRebootTask", h.CancelRebootTask)
	// TODO: rg.POST("/deleteCPERebootTask", h.DeleteCPERebootTask) // handler not yet implemented
	rg.POST("/startRebootTask", h.StartRebootTask)
	// TODO: rg.POST("/startCPERebootTask", h.StartCPERebootTask) // handler not yet implemented
	rg.POST("/listRebootTask", h.ListRebootTasks)
	// TODO: rg.POST("/listCPERebootTask", h.ListCPERebootTasks) // handler not yet implemented
	rg.POST("/listRebootTaskResult", h.ListRebootTaskResults)
	// TODO: rg.POST("/listCPERebootTaskResult", h.ListCPERebootTaskResults) // handler not yet implemented
}
