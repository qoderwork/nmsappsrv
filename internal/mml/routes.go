package mml

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all MML command routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// MML命令
	rg.GET("/mml-sets", h.ListMmlSets)
	rg.GET("/mml-commands", h.ListMmlCommands)
	rg.GET("/mml-commands/:id/params", h.GetMmlCommandParams)
	rg.POST("/mml-execute", h.ExecuteMml)
	rg.GET("/mml-results", h.ListMmlResults)
	rg.GET("/mml-results/:id", h.GetMmlResult)

	// MML import + version lifecycle (对齐 Java importMMLAndParameter / getMmlVersions / getMmlCommandsByVersion / getMmlCommand / deleteMmlByVersion)
	rg.POST("/mml/import", h.ImportMML)
	rg.GET("/mml/versions", h.GetMmlVersions)
	rg.GET("/mml/commands-by-version", h.GetMmlCommandsByVersion)
	rg.GET("/mml/command-tree", h.GetMmlCommandTree)
	rg.DELETE("/mml/version", h.DeleteMmlByVersion)

	// MML脚本批量处理
	RegisterBatchProcessRoutes(rg, h)
}
