package nmsbackup

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all NMS backup and revert routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// NMS备份与回退 (NMS Backup & Revert)
	rg.POST("/nms-backup/tasks", h.AddNMSBackupTask)
	rg.POST("/nms-backup/tasks/list", h.ListNMSBackupTask)
	rg.POST("/nms-backup/tasks/modify", h.ModifyNMSBackupTask)
	rg.POST("/nms-backup/tasks/run", h.RunNMSBackupTask)
	rg.POST("/nms-backup/tasks/delete", h.DeleteNMSBackupTask)
	rg.POST("/nms-backup/tasks/revert", h.RevertNMSBackupTask)
	rg.GET("/nms-backup/config", h.GetBackupAndRestoreConfig)
	rg.PUT("/nms-backup/config", h.UpdateBackupAndRestoreConfig)
	rg.POST("/nms-backup/logs", h.ListNMSBackupLogs)
	rg.POST("/nms-backup/logs/detail", h.GetNMSBackupLogDetail)
}
