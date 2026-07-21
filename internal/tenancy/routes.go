package tenancy

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all tenancy management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/tenancies", h.AddTenancy)
	rg.PUT("/tenancies", h.UpdateTenancy)
	rg.GET("/tenancies", h.ListTenancy)
	rg.DELETE("/tenancies", h.DeleteTenancy)
	rg.POST("/tenancies/view", h.ViewTenancy)
}
