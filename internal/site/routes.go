package site

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all site and system configuration routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 站点
	rg.GET("/sites", h.ListSites)
	rg.GET("/sites/basic", h.ListSiteBasicInfo)
	rg.POST("/sites", h.CreateSite)
	rg.PUT("/sites/:id", h.UpdateSite)
	rg.DELETE("/sites/:id", h.DeleteSite)

	// 系统
	rg.GET("/system/config", h.GetSystemConfig)
	rg.PUT("/system/config", h.UpdateSystemConfig)
	rg.GET("/system/areas", h.ListAreas)
	rg.GET("/system/areas/:id", h.GetArea)
	rg.POST("/system/areas", h.CreateArea)
	rg.PUT("/system/areas/:id", h.UpdateArea)
	rg.DELETE("/system/areas/:id", h.DeleteArea)
	rg.GET("/system/parameters", h.ListSystemParameters)
	rg.PUT("/system/parameters", h.UpdateSystemParameter)
}
