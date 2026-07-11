package security

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all security rule routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// --- REST-style routes (preferred) ---
	rg.GET("/security/rules", h.GetSecurityRule)
	rg.PUT("/security/rules", h.UpdateSecurityRule)
	rg.GET("/security/password-strategy", h.GetPasswordStrategy)

	// --- Legacy RPC-style routes (deprecated) ---
	// Deprecated: use GET /security/rules instead
	rg.POST("/getSecurityRule", h.GetSecurityRule)
	// Deprecated: use PUT /security/rules instead
	rg.POST("/updateSecurityRule", h.UpdateSecurityRule)
	// Deprecated: use GET /security/password-strategy instead
	rg.GET("/getPasswordStrategy", h.GetPasswordStrategy)
}
