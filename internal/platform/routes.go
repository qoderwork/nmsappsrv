package platform

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all platform settings routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// --- REST-style routes (preferred) ---
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

	// --- Legacy RPC-style routes (deprecated) ---
	// Deprecated: use GET /platform/date instead
	rg.GET("/getDate", h.GetDate)
	// Deprecated: use GET /platform/zones instead
	rg.GET("/getSupportedZone", h.GetSupportedZone)
	// Deprecated: use GET /platform/logo instead
	rg.GET("/getLogo", h.GetLogo)
	// Deprecated: use GET /platform/log-config instead
	rg.POST("/listLogConfig", h.ListLogConfig)
	// Deprecated: use PUT /platform/log-config instead
	rg.POST("/updateLogConfig", h.UpdateLogConfig)
	// Deprecated: use GET /platform/ftp-log-config instead
	rg.POST("/getFTPTransferLogConfig", h.GetFTPTransferLogConfig)
	// Deprecated: use PUT /platform/ftp-log-config instead
	rg.POST("/updateFTPTransferLogConfig", h.UpdateFTPTransferLogConfig)
	// Deprecated: use GET /platform/hec-config instead
	rg.POST("/getHECConfig", h.GetHECConfig)
	// Deprecated: use PUT /platform/hec-config instead
	rg.POST("/updateHECConfig", h.UpdateHECConfig)
	// Deprecated: use GET /platform/secrets instead
	rg.POST("/listNMSSecret", h.ListNMSSecret)
	// Deprecated: use PUT /platform/secrets instead
	rg.POST("/updateNMSSecret", h.UpdateNMSSecret)
	// Deprecated: use GET /platform/downloads/rsa-public-key instead
	rg.GET("/downloadPasswordRSAPublicKey", h.DownloadPasswordRSAPublicKey)
	// Deprecated: use GET /platform/downloads/logs instead
	rg.GET("/downloadPlatformLogs", h.DownloadPlatformLogs)
	// Deprecated: use GET /platform/downloads/manual instead
	rg.GET("/downloadNMSManualDocument", h.DownloadNMSManualDocument)
}
