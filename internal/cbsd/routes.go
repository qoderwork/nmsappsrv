package cbsd

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all CBSD (SAS) routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// CBSD (SAS)
	rg.GET("/cbsd", h.ListCBSD)
	rg.GET("/cbsd/:id", h.GetCBSD)
	rg.POST("/cbsd/register", h.RegisterCBSD)
	rg.PUT("/cbsd/:id", h.UpdateCBSD)
	rg.POST("/cbsd/deregister", h.DeregisterCBSD)
	rg.GET("/cbsd-logs", h.ListCBSDLogs)
	rg.POST("/cbsd/cert-tasks", h.CreateCertFileSendTask)
	rg.GET("/cbsd/cert-tasks", h.ListCertFileSendTasks)
}
