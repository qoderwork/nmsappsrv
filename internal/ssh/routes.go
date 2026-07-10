package ssh

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all SSH label and access timer routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// SSH Label
	rg.POST("/addSSHLabel", h.AddSSHLabel)
	rg.POST("/deleteSSHLabel", h.DeleteSSHLabel)
	rg.POST("/listSSHLabels", h.ListSSHLabels)
	rg.POST("/updateSSHLabel", h.UpdateSSHLabel)

	// SSH Access Timer
	rg.POST("/sshAccessTimer", h.SetSSHAccessTimer)
	rg.POST("/listSSHAccessTimer", h.ListSSHAccessTimer)
}
