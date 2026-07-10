package tenancy

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all tenancy management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// Tenancy Management Module
	rg.POST("/addTenancy", h.AddTenancy)
	rg.POST("/updateTenancy", h.UpdateTenancy)
	rg.POST("/listTenancy", h.ListTenancy)
	rg.POST("/deleteTenancy", h.DeleteTenancy)
	rg.POST("/viewTenancy", h.ViewTenancy)
}
