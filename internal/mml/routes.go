package mml

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all MML command routes on the given router group.
// Routes are aligned with the Java MmlController (base @RequestMapping("/api/v2/")).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// MML命令 (对齐 Java MmlController)
	rg.GET("/getMmlCommand", h.GetMmlCommandTree)
	rg.POST("/getParametersInMML", h.GetMmlCommandParams)
	// TODO: rg.POST("/batchExecuteMML", h.BatchExecuteMML) // handler not yet implemented
	rg.POST("/listMmlResult", h.ListMmlResults)
	rg.POST("/getMMLResultByEventLogIds", h.GetMMLResultByEventLogIds)
	rg.POST("/importMMLAndParameter", h.ImportMML)
	rg.POST("/getMmlCommandsByVersion", h.GetMmlCommandsByVersion)
	rg.POST("/getMmlVersions", h.GetMmlVersions)
	rg.POST("/deleteMmlByVersion", h.DeleteMmlByVersion)

	// MML脚本批量处理
	RegisterBatchProcessRoutes(rg, h)
}
