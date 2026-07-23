package site

import "github.com/gin-gonic/gin"

// RegisterRoutes registers site and area management routes aligned with Java
// SiteManagementController and AreaManagementController (base @RequestMapping("/api/v2/")).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// ===== SiteManagementController =====
	rg.POST("/listSites", h.ListSites)
	rg.GET("/listSiteBasicInfo", h.ListSiteBasicInfo)
	rg.POST("/addSite", h.CreateSite)
	rg.POST("/updateSite", h.UpdateSite)
	rg.POST("/deleteSite", h.DeleteSite)

	// ===== AreaManagementController =====
	rg.POST("/listArea", h.ListAreas)
	rg.POST("/addArea", h.CreateArea)
	rg.POST("/updateArea", h.UpdateArea)
	rg.POST("/deleteArea", h.DeleteArea)

	// NOTE: The following routes have no Java counterpart and are removed per
	// 1:1 replication requirement:
	// - GET  /system/config        (no Java controller)
	// - PUT  /system/config        (no Java controller)
	// - POST /viewAreaDetail       (not in AreaManagementController)
	// - GET  /system/parameters    (no Java controller)
	// - PUT  /system/parameters    (no Java controller)
}