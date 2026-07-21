package mail

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all mail configuration routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/mail/config", h.ListMailConfig)
	rg.PUT("/mail/config", h.UpdateMailConfig)
	rg.POST("/mail/test", h.TestMail)
	rg.POST("/mail/email-code", h.GetEmailCode)
	rg.POST("/mail/check-email-code", h.CheckEmailCode)
	rg.GET("/mail/email-auth-enabled", h.IsEnabledEmailAuthentication)
}
