package mail

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all mail configuration routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// --- REST-style routes (preferred) ---
	rg.GET("/mail/config", h.ListMailConfig)
	rg.PUT("/mail/config", h.UpdateMailConfig)
	rg.POST("/mail/test", h.TestMail)
	rg.POST("/mail/email-code", h.GetEmailCode)
	rg.POST("/mail/check-email-code", h.CheckEmailCode)
	rg.GET("/mail/email-auth-enabled", h.IsEnabledEmailAuthentication)

	// --- Legacy RPC-style routes (deprecated) ---
	// Deprecated: use GET /mail/config instead
	rg.POST("/listMailConfig", h.ListMailConfig)
	// Deprecated: use PUT /mail/config instead
	rg.POST("/updateMailConfig", h.UpdateMailConfig)
	// Deprecated: use POST /mail/test instead
	rg.POST("/testMail", h.TestMail)
	// Deprecated: use POST /mail/email-code instead
	rg.POST("/getEmailCode", h.GetEmailCode)
	// Deprecated: use POST /mail/check-email-code instead
	rg.POST("/checkEmailCode", h.CheckEmailCode)
	// Deprecated: use GET /mail/email-auth-enabled instead
	rg.POST("/isEnabledEmailAuthentication", h.IsEnabledEmailAuthentication)
}
