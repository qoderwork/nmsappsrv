package platform

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all platform settings routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/platform/date", h.GetDate)
	rg.GET("/platform/zones", h.GetSupportedZone)
	rg.GET("/platform/logo", h.GetLogo)
	rg.GET("/platform/log-config", h.ListLogConfig)
	rg.PUT("/platform/log-config", h.UpdateLogConfig)
	rg.GET("/platform/ftp-log-config", h.GetFTPTransferLogConfig)
	rg.PUT("/platform/ftp-log-config", h.UpdateFTPTransferLogConfig)
	rg.GET("/platform/hec-config", h.GetHECConfig)
	rg.PUT("/platform/hec-config", h.UpdateHECConfig)
	rg.GET("/platform/secrets", h.ListNMSSecret)
	rg.PUT("/platform/secrets", h.UpdateNMSSecret)
	rg.GET("/platform/downloads/rsa-public-key", h.DownloadPasswordRSAPublicKey)
	rg.GET("/platform/downloads/logs", h.DownloadPlatformLogs)
	rg.GET("/platform/downloads/manual", h.DownloadNMSManualDocument)
}
