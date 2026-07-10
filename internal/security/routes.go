package security

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all security rule routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// Security Rules
	rg.POST("/getSecurityRule", h.GetSecurityRule)
	rg.POST("/updateSecurityRule", h.UpdateSecurityRule)
	rg.GET("/getPasswordStrategy", h.GetPasswordStrategy)
}
