package mail

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/listMailConfig", h.ListMailConfig)
	rg.POST("/updateMailConfig", h.UpdateMailConfig)
	rg.POST("/testMail", h.TestMail)
	rg.POST("/getEmailCode", h.GetEmailCode)
	rg.POST("/checkEmailCode", h.CheckEmailCode)
	rg.POST("/isEnabledEmailAuthentication", h.IsEnabledEmailAuthentication)
}