package license

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all license management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// License
	rg.GET("/license/:id", h.GetLicense)
	rg.GET("/licenses", h.ListLicenses)
	rg.PUT("/license/:id", h.UpdateLicense)
	rg.GET("/license/sas-config", h.GetSASConfig)
	rg.POST("/license/sas-config", h.SaveSASConfig)
	rg.GET("/license/entra-endpoints", h.ListEntraEndpoints)
	rg.POST("/license/entra-endpoints", h.CreateEntraEndpoint)
	rg.PUT("/license/entra-endpoints/:id", h.UpdateEntraEndpoint)
	rg.DELETE("/license/entra-endpoints/:id", h.DeleteEntraEndpoint)
}

// RegisterPublicRoutes registers license endpoints that must stay open before a
// license is activated, so an administrator can upload one and inspect status.
// These are NOT placed behind the license gate.
// Aligned with Java LicenseCreatorController: @RequestMapping("/api/v2")
func RegisterPublicRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/uploadLicenseFile", h.UploadLicenseFile)
	rg.POST("/getLicenseInfo", h.GetLicenseInfo)
}
