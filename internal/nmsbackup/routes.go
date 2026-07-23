package nmsbackup

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all NMS backup and revert routes on the given router group.
// Routes mirror the Java backend controllers:
//   - NMSBackupAndRevertController (all POST)
//   - NMSBackupAndRevertLogManagementController (all POST)
//   - BaseStationBackupAndRestoreManagementController (handlers not yet implemented)
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// NMSBackupAndRevertController (all POST)
	rg.POST("/AddNMSBackupTask", h.AddNMSBackupTask)
	rg.POST("/ListNMSBackupTask", h.ListNMSBackupTask)
	rg.POST("/ModifyNMSBackupTask", h.ModifyNMSBackupTask)
	rg.POST("/RunNMSBackupTask", h.RunNMSBackupTask)
	rg.POST("/DeleteNMSBackupTask", h.DeleteNMSBackupTask)
	rg.POST("/RevertNMSBackupTask", h.RevertNMSBackupTask)
	rg.POST("/getBackupAndRestoreConfig", h.GetBackupAndRestoreConfig)
	rg.POST("/updateBackupAndRestoreConfig", h.UpdateBackupAndRestoreConfig)

	// NMSBackupAndRevertLogManagementController (all POST)
	rg.POST("/ListNMSBackupLogs", h.ListNMSBackupLogs)
	rg.POST("/getNMSBackupLogDetail", h.GetNMSBackupLogDetail)

	// BaseStationBackupAndRestoreManagementController
	// TODO: rg.POST("/listBaseStationBackupLatestFileInfo", h.ListBaseStationBackupLatestFileInfo) // handler not yet implemented
	// TODO: rg.POST("/importConfigFile", h.ImportConfigFile) // handler not yet implemented
	// TODO: rg.POST("/exportConfigFile", h.ExportConfigFile) // handler not yet implemented
	// TODO: rg.POST("/addBackupTask", h.AddBackupTask) // handler not yet implemented
	// TODO: rg.POST("/cancelBackupTask", h.CancelBackupTask) // handler not yet implemented
	// TODO: rg.POST("/cancelRestoreTask", h.CancelRestoreTask) // handler not yet implemented
	// TODO: rg.POST("/addRestoreTask", h.AddRestoreTask) // handler not yet implemented
	// TODO: rg.POST("/startBackupOrRestoreTask", h.StartBackupOrRestoreTask) // handler not yet implemented
	// TODO: rg.POST("/listBackupOrRestoreTask", h.ListBackupOrRestoreTask) // handler not yet implemented
	// TODO: rg.POST("/listDeviceBackupAndRestoreResult", h.ListDeviceBackupAndRestoreResult) // handler not yet implemented
	// TODO: rg.GET("/downloadConfigFile", h.DownloadConfigFile) // handler not yet implemented
}
