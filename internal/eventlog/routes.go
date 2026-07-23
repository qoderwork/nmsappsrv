package eventlog

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all event log routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// GNBLogManagementController
	// TODO: rg.POST("/listGNBEventLog", h.ListGNBEventLog) // handler not yet implemented
	// TODO: rg.POST("/listBaseStationValueChangeLog", h.ListBaseStationValueChangeLog) // handler not yet implemented

	// CPELogManagementController
	// TODO: rg.POST("/listCpeEventLog", h.ListCpeEventLog) // handler not yet implemented
	// TODO: rg.POST("/listCPEValueChangeLog", h.ListCPEValueChangeLog) // handler not yet implemented
}
