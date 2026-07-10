package restapi

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all REST API (Northbound) routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// Devices
	rg.GET("/devices", h.ListDevices)
	rg.GET("/devices/:id", h.GetDevice)
	rg.POST("/devices", h.AddDevice)
	rg.PUT("/devices/:id", h.ModifyDeviceById)
	rg.PUT("/devices/sn/:sn", h.ModifyDeviceBySN)
	rg.DELETE("/devices/:id", h.DeleteDevice)

	// Device Parameters
	rg.GET("/devices/:id/parameters", h.GetDeviceParams)
	rg.PUT("/devices/:id/parameters", h.SetDeviceParams)
	rg.POST("/devices/:id/parameters/preset", h.PresetDeviceParams)
	rg.GET("/request-status/:requestId", h.GetRequestStatus)

	// Alarms
	rg.GET("/alarms", h.ListAlarms)
	rg.POST("/alarms/sync", h.SyncAlarm)
	rg.POST("/alarms/clear", h.ClearAlarm)

	// Upgrade Files & Tasks
	rg.POST("/upgrade-files", h.UploadUpgradeFile)
	rg.GET("/upgrade-files", h.ListUpgradeFiles)
	rg.DELETE("/upgrade-files/:id", h.DeleteUpgradeFile)
	rg.POST("/upgrade-tasks", h.CreateUpgradeTask)
	rg.GET("/upgrade-tasks", h.ListUpgradeTasks)

	// TBG (Third-party Base Station Gateway)
	rg.POST("/tbg", h.AddTBGs)
	rg.PUT("/tbg", h.ModifyTBGs)
	rg.DELETE("/tbg", h.DeleteTBGs)
	rg.GET("/tbg", h.ListTBGs)
	rg.GET("/tbg/sn/:sn", h.GetTBGBySN)
	rg.GET("/tbg/wan-mac/:wanMac", h.GetTBGByWanMac)

	// Device Online Status
	rg.GET("/device/online-status", h.ListDeviceOnlineStatus)
	rg.GET("/device/:elementId/online-status", h.GetDeviceOnlineStatus)

	// ACS Settings
	rg.GET("/settings/acs", h.GetACSSettings)
	rg.PUT("/settings/acs", h.UpdateACSSettings)

	// SNMP Operations
	rg.POST("/snmp/get", h.SnmpGet)
	rg.POST("/snmp/set", h.SnmpSet)
	rg.GET("/snmp/operation-logs", h.ListSnmpOperationLogs)
}
