package security

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all security rule routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/security/rules", h.GetSecurityRule)
	rg.PUT("/security/rules", h.UpdateSecurityRule)
	rg.GET("/security/password-strategy", h.GetPasswordStrategy)
}
