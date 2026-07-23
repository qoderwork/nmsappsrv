package security

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/getSecurityRule", h.GetSecurityRule)
	rg.POST("/updateSecurityRule", h.UpdateSecurityRule)
	rg.GET("/getPasswordStrategy", h.GetPasswordStrategy)
}