package ssh

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all SSH label and access timer routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// --- REST-style routes (preferred) ---
	rg.POST("/ssh-labels", h.AddSSHLabel)
	rg.DELETE("/ssh-labels", h.DeleteSSHLabel)
	rg.GET("/ssh-labels", h.ListSSHLabels)
	rg.PUT("/ssh-labels", h.UpdateSSHLabel)
	rg.POST("/ssh-access-timer", h.SetSSHAccessTimer)
	rg.GET("/ssh-access-timer", h.ListSSHAccessTimer)

	// --- Legacy RPC-style routes (deprecated) ---
	// Deprecated: use POST /ssh-labels instead
	rg.POST("/addSSHLabel", h.AddSSHLabel)
	// Deprecated: use DELETE /ssh-labels instead
	rg.POST("/deleteSSHLabel", h.DeleteSSHLabel)
	// Deprecated: use GET /ssh-labels instead
	rg.POST("/listSSHLabels", h.ListSSHLabels)
	// Deprecated: use PUT /ssh-labels instead
	rg.POST("/updateSSHLabel", h.UpdateSSHLabel)
	// Deprecated: use POST /ssh-access-timer instead
	rg.POST("/sshAccessTimer", h.SetSSHAccessTimer)
	// Deprecated: use GET /ssh-access-timer instead
	rg.POST("/listSSHAccessTimer", h.ListSSHAccessTimer)
}
