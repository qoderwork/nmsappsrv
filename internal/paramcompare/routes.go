package paramcompare

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all parameter compare routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// Parameter Compare
	rg.POST("/param-compare/compare", h.Compare)
	rg.POST("/param-compare/batch", h.BatchCompare)
	rg.GET("/param-compare/templates", h.ListTemplates)
}
