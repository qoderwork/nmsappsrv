package parameter

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all parameter management routes on the given router group.
// Routes mirror the Java backend controllers under /api/v2/ base path:
//   - TR069ParameterManagementController
//   - ParameterTemplateManagementController
//   - ParameterDeploymentTemplateController
//   - ConfigurationManagementController
//   - DeviceConfigurationManagementController
//   - ObjectManagementController
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// === TR069ParameterManagementController ===
	rg.POST("/addTR069Parameter", h.AddTR069Parameter)
	rg.POST("/listTR069Parameter", h.ListTR069Parameters)
	rg.POST("/deleteTR069Parameter", h.DeleteTR069Parameter)
	rg.POST("/viewTR069Parameter", h.ViewTR069Parameter)
	rg.POST("/updateTR069Parameter", h.UpdateTR069Parameter)
	rg.POST("/addParameterSet", h.CreateParameterSet)
	rg.POST("/updateParameterSet", h.UpdateParameterSet)
	rg.POST("/deleteParameterSet", h.DeleteParameterSet)
	// TODO: rg.POST("/getParameterSet", h.GetParameterSet) // handler not yet implemented
	// TODO: rg.POST("/listParameterInParameterSet", h.ListParameterInParameterSet) // handler not yet implemented
	// TODO: rg.POST("/addParameterToParameterSet", h.AddParameterToParameterSet) // handler not yet implemented
	// TODO: rg.POST("/deleteParameterToParameterSet", h.DeleteParameterToParameterSet) // handler not yet implemented

	// === ParameterTemplateManagementController ===
	rg.POST("/createParameterTemplate", h.CreateParameterTemplate)
	rg.POST("/updateParameterTemplate", h.UpdateParameterTemplate)
	rg.POST("/deleteParameterTemplate", h.DeleteParameterTemplate)
	rg.POST("/getParameterTemplateInfo", h.GetParameterTemplate)
	rg.POST("/listParameterTemplates", h.ListParameterTemplates)

	// === ParameterDeploymentTemplateController ===
	// TODO: rg.POST("/addParameterDeployTemplate", h.AddParameterDeployTemplate) // handler not yet implemented
	// TODO: rg.POST("/updateParameterDeployTemplate", h.UpdateParameterDeployTemplate) // handler not yet implemented
	// TODO: rg.POST("/deleteParameterDeployTemplate", h.DeleteParameterDeployTemplate) // handler not yet implemented
	// TODO: rg.POST("/getParameterDeployTemplateInfo", h.GetParameterDeployTemplateInfo) // handler not yet implemented
	// TODO: rg.POST("/listParameterDeployTemplates", h.ListParameterDeployTemplates) // handler not yet implemented
	// TODO: rg.POST("/parameterDeployTemplateLogs", h.ParameterDeployTemplateLogs) // handler not yet implemented

	// === ConfigurationManagementController ===
	rg.POST("/exportConfigurationTemplateFile", h.ExportParameterTemplate)
	rg.POST("/batchParameterConfiguration", h.BatchParameterConfiguration)
	rg.POST("/listBatchConfiguration", h.ListBatchConfigurations)
	rg.POST("/listBatchConfigurationDetail", h.ListBatchConfigurationDetail)
	rg.POST("/batchParameterConfigurationDirect", h.BatchParameterConfigurationDirect)

	// === DeviceConfigurationManagementController ===
	// TODO: rg.POST("/listParameterSetForDevice", h.ListParameterSetForDevice) // handler not yet implemented
	// TODO: rg.POST("/listParameterForDevice", h.ListParameterForDevice) // handler not yet implemented
	rg.POST("/queryParameterValues", h.GetParameters)
	rg.POST("/setParameterValues", h.SetParameter)
	// TODO: rg.GET("/exportNeighbourCellInfo", h.ExportNeighbourCellInfo) // handler not yet implemented
	// TODO: rg.GET("/setWebEnable", h.SetWebEnable) // handler not yet implemented

	// === ObjectManagementController ===
	// TODO: rg.POST("/batchAddObject", h.BatchAddObject) // handler not yet implemented
	// TODO: rg.POST("/listBatchAddObjectTask", h.ListBatchAddObjectTask) // handler not yet implemented
	// TODO: rg.POST("/listBatchAddObjectTaskDetail", h.ListBatchAddObjectTaskDetail) // handler not yet implemented
}
