package platform

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all platform settings routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// Platform Settings Module
	rg.GET("/getDate", h.GetDate)
	rg.GET("/getSupportedZone", h.GetSupportedZone)
	rg.GET("/getLogo", h.GetLogo)
	rg.POST("/listLogConfig", h.ListLogConfig)
	rg.POST("/updateLogConfig", h.UpdateLogConfig)
	rg.POST("/getFTPTransferLogConfig", h.GetFTPTransferLogConfig)
	rg.POST("/updateFTPTransferLogConfig", h.UpdateFTPTransferLogConfig)
	rg.POST("/getHECConfig", h.GetHECConfig)
	rg.POST("/updateHECConfig", h.UpdateHECConfig)
	rg.POST("/listNMSSecret", h.ListNMSSecret)
	rg.POST("/updateNMSSecret", h.UpdateNMSSecret)
	rg.GET("/downloadPasswordRSAPublicKey", h.DownloadPasswordRSAPublicKey)
	rg.GET("/downloadPlatformLogs", h.DownloadPlatformLogs)
	rg.GET("/downloadNMSManualDocument", h.DownloadNMSManualDocument)
}
