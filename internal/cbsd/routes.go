package cbsd

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all CBSD (SAS) routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// CBSD CRUD
	rg.GET("/cbsd", h.ListCBSD)
	rg.GET("/cbsd/:id", h.GetCBSD)
	rg.POST("/cbsd/register", h.RegisterCBSD)
	rg.PUT("/cbsd/:id", h.UpdateCBSD)
	rg.DELETE("/cbsd/:id", h.DeleteCBSD)
	rg.POST("/cbsd/deregister", h.DeregisterCBSD)

	// CBSD lifecycle
	rg.POST("/cbsd/:id/enable", h.EnableCBSD)
	rg.POST("/cbsd/:id/disable", h.DisableCBSD)

	// SAS protocol
	rg.POST("/cbsd/spectrum-inquiry", h.SpectrumInquiry)
	rg.POST("/cbsd/:id/grant", h.Grant)
	rg.POST("/cbsd/:id/relinquishment", h.Relinquishment)
	rg.POST("/cbsd/:id/sas-heartbeat", h.SasHeartbeat)

	// Import / Template
	rg.POST("/cbsd/import", h.ImportCBSDs)
	rg.GET("/cbsd/template", h.DownloadCBSDTemplate)

	// Statistics
	rg.POST("/cbsd/status-count", h.ListCBSDStatusCount)

	// SAS config
	rg.GET("/cbsd/sas-config", h.ListSasConfig)
	rg.PUT("/cbsd/sas-config", h.UpdateSasConfig)

	// CBSD logs
	rg.GET("/cbsd-logs", h.ListCBSDLogs)

	// Cert file send tasks
	rg.POST("/cbsd/cert-tasks", h.CreateCertFileSendTask)
	rg.GET("/cbsd/cert-tasks", h.ListCertFileSendTasks)

	// Certificate file management
	rg.GET("/cbsd/certificate", h.ListCbsdCertificates)
	rg.POST("/cbsd/certificate", h.UploadCbsdCertificate)
	rg.DELETE("/cbsd/certificate/:id", h.DeleteCbsdCertificate)
}
