package misc

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all miscellaneous routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 批量操作
	rg.POST("/batch-backup", h.CreateBackup)
	rg.POST("/batch-restore", h.CreateRestore)
	rg.GET("/batch-tasks", h.ListBackupRestoreTasks)
	rg.POST("/batch-tasks/start", h.StartBackupRestoreTask)
	rg.POST("/batch-tasks/cancel", h.CancelBackupRestoreTask)
	rg.GET("/batch-tasks/:taskId/detail", h.ListBackupRestoreTaskDetail)
	rg.GET("/batch-logs", h.ListBatchConfigLogs)

	// Batch Add Object (TR069)
	rg.POST("/batch-add-object", h.BatchAddObject)
	rg.GET("/batch-add-object/tasks", h.ListBatchAddObjectTasks)
	rg.GET("/batch-add-object/tasks/:taskId/detail", h.ListBatchAddObjectTaskDetail)

	// ZTP
	rg.POST("/ztp/provision", h.ProvisionZTP)
	rg.GET("/ztp/logs", h.ListZTPLogs)
	rg.GET("/ztp/setting", h.GetZTPSetting)
	rg.POST("/ztp/setting", h.SaveZTPSetting)
	rg.POST("/ztp/results", h.ListZTPResults)
	rg.POST("/ztp/retry-logs", h.ListZTPRetryLogs)
	rg.POST("/ztp/history-files", h.ListHistoryZTPFiles)
	rg.POST("/ztp/status", h.SetZTPStatus)
	rg.POST("/ztp/batch-reztp", h.BatchReZTP)
	rg.POST("/ztp/delete-files", h.DeleteZTPFiles)

	// 北向接口
	rg.GET("/north-reports", h.ListNorthReports)
	rg.POST("/north-reports", h.CreateNorthReport)
	rg.PUT("/north-reports/:id", h.UpdateNorthReport)
	rg.DELETE("/north-reports/:id", h.DeleteNorthReport)

	// RADIUS
	rg.GET("/radius", h.ListRadius)
	rg.POST("/radius", h.SaveRadius)
	rg.DELETE("/radius/:id", h.DeleteRadius)

	// 文件上传
	rg.GET("/upload-files", h.ListUploadFiles)
	rg.POST("/upload-files", h.CreateUploadFile)
	rg.DELETE("/upload-files/:id", h.DeleteUploadFile)

	// MR (Measurement Report)
	rg.GET("/mr-data", h.ListMRData)

	// System operator logs
	rg.GET("/system/operator-logs", h.ListOperatorLogs)

	// 基站配置备份与恢复 (Base Station Backup & Restore)
	rg.GET("/bs-backup/info", h.ListBaseStationBackupInfo)
	rg.POST("/bs-backup/import-config", h.ImportConfigFile)
	rg.POST("/bs-backup/export-config", h.ExportConfigFile)
	rg.POST("/bs-backup/backup-tasks", h.AddBSBackupTask)
	rg.POST("/bs-backup/backup-tasks/cancel", h.CancelBackupTask)
	rg.POST("/bs-backup/restore-tasks/cancel", h.CancelRestoreTask)
	rg.POST("/bs-backup/restore-tasks", h.AddBSRestoreTask)
	rg.POST("/bs-backup/tasks/start", h.StartBackupOrRestoreTask)
	rg.POST("/bs-backup/tasks", h.ListBSBackupTasks)
	rg.POST("/bs-backup/tasks/results", h.ListDeviceBackupResult)
	rg.POST("/bs-backup/download-config", h.DownloadConfigFile)

	// AOS Management — TBG (Tunnel Border Gateway)
	rg.POST("/tbg/list", h.ListTBG)
	rg.POST("/tbg/add", h.AddTBG)
	rg.POST("/tbg/modify", h.ModifyTBG)
	rg.POST("/tbg/delete", h.DeleteTBG)
	rg.POST("/tbg/import", h.ImportTBGFile)
	rg.GET("/tbg/template", h.DownloadTBGTemplate)

	// AOS Management — PSAPID
	rg.POST("/psap-id/list", h.ListPSAPID)
	rg.POST("/psap-id/sync", h.SyncPSAPID)
	rg.POST("/psap-id/sync-logs", h.ListPSAPIDSyncLogs)

	// AOS Management — SpatialFile
	rg.GET("/spatial-file/markets", h.ListSpatialFileMarkets)
	rg.POST("/spatial-file/market-coordinates", h.GetMarketCoordinates)
}
