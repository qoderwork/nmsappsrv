package cbsd

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all CBSD (SAS) routes on the given router group.
// It mirrors the Java SASManagementController and CBSDCertFileManagementController
// (both with base @RequestMapping("/api/v2/")).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// -----------------------------------------------------------------------
	// SASManagementController
	// -----------------------------------------------------------------------

	// CBSD template / import
	rg.GET("/downloadBaseStationCBSDTemplate", h.DownloadCBSDTemplate)
	rg.POST("/importCBSDs", h.ImportCBSDs)

	// CBSD query / mutation
	rg.POST("/listCBSD", h.ListCBSD)
	rg.POST("/enableCBSD", h.EnableCBSD)
	rg.POST("/disableCBSD", h.DisableCBSD)
	rg.POST("/modifyCBSD", h.UpdateCBSD)
	rg.POST("/viewCBSD", h.GetCBSD)
	rg.POST("/deleteCBSD", h.DeleteCBSD)

	// SAS protocol lifecycle
	rg.POST("/register", h.RegisterCBSD)
	rg.POST("/spectrumInquiry", h.SpectrumInquiry)
	rg.POST("/grant", h.Grant)
	rg.POST("/relinquishment", h.Relinquishment)
	rg.POST("/deregister", h.DeregisterCBSD)

	// CBSD logs / statistics
	rg.POST("/listCBRSLog", h.ListCBSDLogs)
	rg.POST("/listCBSDStatusCount", h.ListCBSDStatusCount)

	// SAS config
	rg.GET("/listSasConfig", h.ListSasConfig)
	rg.POST("/updateSasConfig", h.UpdateSasConfig)

	// -----------------------------------------------------------------------
	// CBSDCertFileManagementController
	// -----------------------------------------------------------------------

	// Cert file upload / download / delete
	rg.POST("/uploadCBSDCertFile", h.UploadCbsdCertificate)
	// TODO: rg.GET("/downloadCBSDCertFile", h.DownloadCBSDCertFile) // handler not yet implemented
	rg.POST("/deleteCBSDCertFile", h.DeleteCbsdCertificate)

	// Cert file send task
	rg.POST("/addCBSDCertFileSendTask", h.CreateCertFileSendTask)
	rg.POST("/listCBSDCertFileSendTask", h.ListCertFileSendTasks)
	// TODO: rg.POST("/viewCBSDCertFileSendTaskDetail", h.ViewCBSDCertFileSendTaskDetail) // handler not yet implemented
	// TODO: rg.POST("/listCBSDCertFileSendDevice", h.ListCBSDCertFileSendDevice) // handler not yet implemented

	// Device cert file listing / packaging
	rg.POST("/listDeviceCBSDCertFile", h.ListCbsdCertificates)
	// TODO: rg.POST("/packageCPECBSDCertFile", h.PackageCPECBSDCertFile) // handler not yet implemented
}
