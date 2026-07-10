package parameter

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all parameter management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 参数管理
	rg.GET("/parameters/:elementId", h.GetParameters)
	rg.PUT("/parameters/:elementId", h.SetParameter)
	rg.GET("/parameter-logs", h.ListParameterLogs)
	rg.GET("/parameter-sets", h.ListParameterSets)
	rg.POST("/parameter-sets", h.CreateParameterSet)
	rg.PUT("/parameter-sets/:id", h.UpdateParameterSet)
	rg.DELETE("/parameter-sets/:id", h.DeleteParameterSet)
	rg.GET("/parameter-templates", h.ListParameterTemplates)
	rg.POST("/parameter-templates", h.CreateParameterTemplate)
	rg.PUT("/parameter-templates/:id", h.UpdateParameterTemplate)
	rg.POST("/parameter-templates/:templateId/deploy", h.DeployTemplate)
	rg.GET("/parameter-backup-logs", h.ListBackupLogs)
	rg.POST("/parameter-backup/:elementId", h.TriggerBackup)
	rg.POST("/parameter-tasks", h.BatchParameterConfigurationDirect)
	rg.GET("/batch-configurations", h.ListBatchConfigurations)
	rg.GET("/batch-configurations/:taskId/detail", h.ListBatchConfigurationDetail)
}
