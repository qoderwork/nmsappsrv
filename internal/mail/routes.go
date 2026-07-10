package mail

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all mail configuration routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// Mail
	rg.POST("/listMailConfig", h.ListMailConfig)
	rg.POST("/updateMailConfig", h.UpdateMailConfig)
	rg.POST("/testMail", h.TestMail)
	rg.POST("/getEmailCode", h.GetEmailCode)
	rg.POST("/checkEmailCode", h.CheckEmailCode)
	rg.POST("/isEnabledEmailAuthentication", h.IsEnabledEmailAuthentication)
}
