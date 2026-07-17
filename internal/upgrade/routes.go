package upgrade

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all upgrade management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 升级文件管理
	rg.GET("/upgrade-files", h.ListUpgradeFiles)
	rg.POST("/upgrade-files", h.UploadUpgradeFile)
	rg.POST("/upgrade-files/piecemeal", h.UploadUpgradeFileByPiecemeal)
	rg.GET("/upgrade-files/:id", h.ViewUpgradeFile)
	rg.PUT("/upgrade-files/:id", h.UpdateUpgradeFile)
	rg.DELETE("/upgrade-files/:id", h.DeleteUpgradeFile)
	rg.POST("/upgrade-files/:id/download", h.DownloadUpgradeFile)

	// 升级任务管理
	rg.POST("/upgrade-tasks", h.CreateUpgradeTask)
	rg.GET("/upgrade-tasks", h.ListUpgradeTasks)
	rg.GET("/upgrade-tasks/:id", h.GetUpgradeTask)
	rg.POST("/upgrade-tasks/:id/start", h.StartUpgradeTask)
	rg.POST("/upgrade-tasks/:id/cancel", h.CancelUpgradeTask)
	rg.GET("/upgrade-tasks/:id/results", h.ListUpgradeResults)
	rg.GET("/upgrade-tasks/:id/results/detail", h.ListUpgradeResultDetail)

	// 升级统计
	rg.POST("/upgrade-tasks/status-count", h.ListUpgradeTaskStatusCount)
	rg.POST("/upgrade-tasks/device-result-count", h.ListUpgradeDeviceResultCount)

	// 升级日志
	rg.GET("/upgrade-logs", h.ListUpgradeLogs)

	// 回滚任务管理
	rg.POST("/rollback-tasks", h.CreateRollbackTask)
	rg.GET("/rollback-tasks", h.ListRollbackTasks)
	rg.GET("/rollback-tasks/:id/view", h.ViewRollbackTask)
	rg.POST("/rollback-tasks/:id/start", h.StartRollbackTask)
	rg.POST("/rollback-tasks/:id/cancel", h.CancelRollbackTask)
	rg.GET("/rollback-tasks/:id/results", h.ListRollbackResults)

	// 升级结果管理
	rg.POST("/upgrade/manual-confirmation", h.ManualConfirmationUpgrade)

	// 自动升级任务管理
	rg.GET("/auto-upgrade-tasks", h.AutoUpgradeTaskList)
	rg.POST("/auto-upgrade-tasks", h.AddAutoUpgradeTask)
	rg.PUT("/auto-upgrade-tasks/:id", h.ModifyAutoUpgradeTask)
	rg.DELETE("/auto-upgrade-tasks/:id", h.DeleteAutoUpgradeTask)
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
