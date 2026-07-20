package systemsettings

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all system settings routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *SystemSettingsHandler) {
	rg.GET("/settings/device", h.ListDeviceSettings)
	rg.PUT("/settings/device", h.UpdateDeviceSettings)
	rg.GET("/settings/acs", h.ListACSSettings)
	rg.PUT("/settings/acs", h.UpdateACSSettings)
	rg.GET("/settings/log", h.ListLogSettings)
	rg.PUT("/settings/log", h.UpdateLogSettings)
	rg.GET("/settings/north-bound", h.GetNorthBoundConfig)
	rg.PUT("/settings/north-bound", h.UpdateNorthBoundConfig)
}
