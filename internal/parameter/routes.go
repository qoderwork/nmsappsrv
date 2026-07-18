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
	rg.GET("/parameter-templates/:id", h.GetParameterTemplate)
	rg.DELETE("/parameter-templates/:id", h.DeleteParameterTemplate)
	rg.POST("/parameter-templates/:id/deploy", h.DeployTemplate)
	rg.GET("/parameter-templates/:id/deploy-logs", h.ListDeployTemplateLogs)
	rg.GET("/parameter-backup-logs", h.ListBackupLogs)
	rg.POST("/parameter-backup/:elementId", h.TriggerBackup)
	rg.POST("/parameter-tasks", h.BatchParameterConfigurationDirect)
	rg.POST("/batch-configuration", h.BatchParameterConfiguration)
	rg.GET("/batch-configurations", h.ListBatchConfigurations)
	rg.GET("/batch-configurations/:taskId/detail", h.ListBatchConfigurationDetail)

	// TR-069 参数定义 CRUD
	rg.POST("/tr069-parameters", h.AddTR069Parameter)
	rg.GET("/tr069-parameters", h.ListTR069Parameters)
	rg.GET("/tr069-parameters/:id", h.ViewTR069Parameter)
	rg.PUT("/tr069-parameters/:id", h.UpdateTR069Parameter)
	rg.DELETE("/tr069-parameters/:id", h.DeleteTR069Parameter)

	// Model Tree 设备参数树管理
	rg.GET("/model-tree/:elementId", h.GetModelTree)
	rg.POST("/model-tree/:elementId/refresh", h.RefreshParameter)
	rg.POST("/model-tree/:elementId/reload", h.ReloadParameter)
	rg.POST("/model-tree/:elementId/add-object", h.AddObject)
	rg.POST("/model-tree/:elementId/delete-object", h.DeleteObject)
	rg.POST("/model-tree/:elementId/batch-delete-object", h.BatchDeleteObject)
	rg.POST("/model-tree/:elementId/delete-object-after-need-reboot", h.DeleteObjectAfterNeedReboot)

	// Export 参数模板导出
	rg.GET("/parameter-templates/:id/export", h.ExportParameterTemplate)
}
