package northinterfacelog

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all north interface log routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// NorthInterfaceLogManagementController
	rg.POST("/listNorthInterfaceLog", h.ListLogs)
}
