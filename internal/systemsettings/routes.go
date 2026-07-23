package systemsettings

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all system settings routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *SystemSettingsHandler) {
	// SystemSettingsManagementController
	rg.POST("/listDeviceSettings", h.ListDeviceSettings)
	rg.POST("/updateDeviceSettings", h.UpdateDeviceSettings)
	rg.POST("/listACSSettings", h.ListACSSettings)
	rg.POST("/updateACSSettings", h.UpdateACSSettings)
	rg.POST("/updateLogSettings", h.UpdateLogSettings)
	rg.POST("/listLogSettings", h.ListLogSettings)
}
