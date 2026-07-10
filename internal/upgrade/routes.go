package upgrade

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all upgrade management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 升级管理
	rg.GET("/upgrade-files", h.ListUpgradeFiles)
	rg.POST("/upgrade-files", h.UploadUpgradeFile)
	rg.DELETE("/upgrade-files/:id", h.DeleteUpgradeFile)
	rg.POST("/upgrade-tasks", h.CreateUpgradeTask)
	rg.GET("/upgrade-tasks", h.ListUpgradeTasks)
	rg.GET("/upgrade-tasks/:id", h.GetUpgradeTask)
	rg.GET("/upgrade-logs", h.ListUpgradeLogs)
	rg.POST("/rollback-tasks", h.CreateRollbackTask)
	rg.GET("/rollback-tasks", h.ListRollbackTasks)
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
