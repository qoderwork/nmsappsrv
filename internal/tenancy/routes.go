package tenancy

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all tenancy management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// --- REST-style routes (preferred) ---
	rg.POST("/tenancies", h.AddTenancy)
	rg.PUT("/tenancies", h.UpdateTenancy)
	rg.GET("/tenancies", h.ListTenancy)
	rg.DELETE("/tenancies", h.DeleteTenancy)
	rg.POST("/tenancies/view", h.ViewTenancy)

	// --- Legacy RPC-style routes (deprecated) ---
	// Deprecated: use POST /tenancies instead
	rg.POST("/addTenancy", h.AddTenancy)
	// Deprecated: use PUT /tenancies instead
	rg.POST("/updateTenancy", h.UpdateTenancy)
	// Deprecated: use GET /tenancies instead
	rg.POST("/listTenancy", h.ListTenancy)
	// Deprecated: use DELETE /tenancies instead
	rg.POST("/deleteTenancy", h.DeleteTenancy)
	// Deprecated: use POST /tenancies/view instead
	rg.POST("/viewTenancy", h.ViewTenancy)
}
