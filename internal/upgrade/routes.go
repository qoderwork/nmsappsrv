package upgrade

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all upgrade management routes on the given router group.
// Route paths mirror the Java backend's UpgradeManagementController and
// ManualUpgradeManagementController (base path @RequestMapping("/api/v2/")),
// which is already set up in cmd/main.go, so only the relative path is registered here.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// ---- 升级文件管理 (Upgrade File Management) ----
	rg.POST("/uploadUpgradeFile", h.UploadUpgradeFile)
	rg.POST("/listUpgradeFile", h.ListUpgradeFiles)
	// TODO: rg.POST("/listCPEUpgradeFile", h.ListCPEUpgradeFile) // handler not yet implemented
	rg.POST("/viewUpgradeFile", h.ViewUpgradeFile)
	// TODO: rg.POST("/viewCPEUpgradeFile", h.ViewCPEUpgradeFile) // handler not yet implemented
	rg.POST("/updateUpgradeFile", h.UpdateUpgradeFile)
	// TODO: rg.POST("/updateCPEUpgradeFile", h.UpdateCPEUpgradeFile) // handler not yet implemented
	rg.GET("/downloadUpgradeFile", h.DownloadUpgradeFile)
	rg.POST("/deleteDeviceUpgradeFile", h.DeleteUpgradeFile)
	// TODO: rg.POST("/deleteCPEDeviceUpgradeFile", h.DeleteCPEDeviceUpgradeFile) // handler not yet implemented
	rg.POST("/uploadUpgradeFileByPiecemeal", h.UploadUpgradeFileByPiecemeal)

	// ---- 升级任务管理 (Upgrade Task Management) ----
	rg.POST("/addUpgradeTask", h.CreateUpgradeTask)
	// TODO: rg.POST("/addCPEUpgradeTask", h.AddCPEUpgradeTask) // handler not yet implemented
	rg.POST("/viewUpgradeTask", h.GetUpgradeTask)
	// TODO: rg.POST("/viewCPEUpgradeTask", h.ViewCPEUpgradeTask) // handler not yet implemented
	rg.POST("/listUpgradeTask", h.ListUpgradeTasks)
	// TODO: rg.POST("/countUpgradeTaskSummary", h.CountUpgradeTaskSummary) // handler not yet implemented
	rg.POST("/listUpgradeResultDetail", h.ListUpgradeResultDetail)
	// TODO: rg.POST("/listCPEUpgradeTask", h.ListCPEUpgradeTask) // handler not yet implemented
	rg.POST("/startUpgradeTask", h.StartUpgradeTask)
	// TODO: rg.POST("/startCPEUpgradeTask", h.StartCPEUpgradeTask) // handler not yet implemented
	rg.POST("/listUpgradeResult", h.ListUpgradeResults)
	// TODO: rg.POST("/listCPEUpgradeResult", h.ListCPEUpgradeResult) // handler not yet implemented
	rg.POST("/cancelUpgradeTask", h.CancelUpgradeTask)

	// ---- 升级统计 (Upgrade Statistics) ----
	rg.POST("/listUpgradeTaskStatusCount", h.ListUpgradeTaskStatusCount)
	rg.POST("/listUpgradeDeviceResultCount", h.ListUpgradeDeviceResultCount)

	// ---- 回滚任务管理 (Rollback Task Management) ----
	rg.POST("/addRollbackTask", h.CreateRollbackTask)
	rg.POST("/listRollbackTask", h.ListRollbackTasks)
	rg.POST("/startRollbackTask", h.StartRollbackTask)
	rg.POST("/cancelRollbackTask", h.CancelRollbackTask)
	rg.POST("/listRollbackResult", h.ListRollbackResults)
	rg.POST("/viewRollbackTask", h.ViewRollbackTask)

	// ---- 自动升级任务管理 (Auto Upgrade Task Management) ----
	rg.POST("/autoUpgradeTaskList", h.AutoUpgradeTaskList)
	rg.POST("/modifyAutoUpgradeTask", h.ModifyAutoUpgradeTask)
	rg.POST("/addAutoUpgradeTask", h.AddAutoUpgradeTask)
	rg.POST("/deleteAutoUpgradeTask", h.DeleteAutoUpgradeTask)

	// ---- 升级确认 (Manual Confirmation) ----
	rg.POST("/manualConfirmationUpgrade", h.ManualConfirmationUpgrade)

	// ---- 手动升级任务管理 (Manual Upgrade Task Management) ----
	// Mirrors Java ManualUpgradeManagementController.
	// TODO: rg.POST("/addManualUpgradeTask", h.AddManualUpgradeTask) // handler not yet implemented
	// TODO: rg.POST("/listManualUpgradeTask", h.ListManualUpgradeTask) // handler not yet implemented
	// TODO: rg.POST("/viewManualUpgradeTask", h.ViewManualUpgradeTask) // handler not yet implemented
	// TODO: rg.POST("/cancelManualUpgradeTask", h.CancelManualUpgradeTask) // handler not yet implemented
	// TODO: rg.POST("/activeUpgradeFile", h.ActiveUpgradeFile) // handler not yet implemented
	// TODO: rg.POST("/listManualUpgradeDownloadDetail", h.ListManualUpgradeDownloadDetail) // handler not yet implemented

	// ---- NMS 系统升级管理 (NMS System Upgrade Management) ----
	// Mirrors Java NMSUpgradeManagementController.
	// TODO: rg.POST("/checkNMSUpgradeFileExist", h.CheckNMSUpgradeFileExist) // handler not yet implemented
	// TODO: rg.POST("/uploadNMSUpgradePartFile", h.UploadNMSUpgradePartFile) // handler not yet implemented
	// TODO: rg.POST("/checkNMSUpgradePartFile", h.CheckNMSUpgradePartFile) // handler not yet implemented
	// TODO: rg.POST("/assembleNMSUpgradeFiles", h.AssembleNMSUpgradeFiles) // handler not yet implemented
	// TODO: rg.POST("/upgrade", h.NMSUpgrade) // handler not yet implemented
	// TODO: rg.POST("/rollback", h.NMSRollback) // handler not yet implemented
	// TODO: rg.POST("/listUpgradeFiles", h.ListNMSUpgradeFiles) // handler not yet implemented (note: plural, distinct from listUpgradeFile)
	// TODO: rg.POST("/listUpgradeAndRollbackLog", h.ListNMSUpgradeAndRollbackLog) // handler not yet implemented
	// TODO: rg.POST("/deleteNMSUpgradeFile", h.DeleteNMSUpgradeFile) // handler not yet implemented
	// TODO: rg.GET("/getNMSUpgradeProgress", h.GetNMSUpgradeProgress) // handler not yet implemented
	// TODO: rg.GET("/getNMSUpgradeLog", h.GetNMSUpgradeLog) // handler not yet implemented
	// TODO: rg.GET("/checkDiskForNMSUpgradeFile", h.CheckDiskForNMSUpgradeFile) // handler not yet implemented
}

// RegisterShutdownRoutes registers all shutdown management routes on the given router group.
func RegisterShutdownRoutes(rg *gin.RouterGroup, h *ShutdownHandler) {
	// 关机管理 (Shutdown Management)
	rg.POST("/shutdown-tasks", h.AddShutdownTask)
	rg.GET("/shutdown-tasks", h.ListShutdownTasks)
	rg.GET("/shutdown-tasks/:id", h.ViewShutdownTask)
	rg.DELETE("/shutdown-tasks/:id", h.DeleteShutdownTask)
	rg.GET("/shutdown-tasks/:id/results", h.ListShutdownResults)
}

// RegisterPublicRoutes registers device-facing (no web-auth) upgrade file routes.
// The firmware is served to CPEs via TR-069 Download, which the device fetches
// directly over HTTP, so no bearer token is involved.
func RegisterPublicRoutes(rg gin.IRouter, h *Handler) {
	rg.GET("/acs-file-server/upgrade/downloadFile/:id", h.DownloadUpgradeFileRaw)
}
