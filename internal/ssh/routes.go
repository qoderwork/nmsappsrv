package ssh

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all SSH label and access timer routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/ssh-labels", h.AddSSHLabel)
	rg.DELETE("/ssh-labels", h.DeleteSSHLabel)
	rg.GET("/ssh-labels", h.ListSSHLabels)
	rg.PUT("/ssh-labels", h.UpdateSSHLabel)
	rg.POST("/ssh-access-timer", h.SetSSHAccessTimer)
	rg.POST("/ssh-access-timer/list", h.ListSSHAccessTimer)
}
