package platform

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/getDate", h.GetDate)
	rg.GET("/getSupportedZone", h.GetSupportedZone)
	rg.GET("/getLogo", h.GetLogo)
	rg.GET("/listLogConfig", h.ListLogConfig)
	rg.POST("/updateLogConfig", h.UpdateLogConfig)
	rg.GET("/getFTPTransferLogConfig", h.GetFTPTransferLogConfig)
	rg.POST("/updateFTPTransferLogConfig", h.UpdateFTPTransferLogConfig)
	rg.GET("/getHECConfig", h.GetHECConfig)
	rg.POST("/updateHECConfig", h.UpdateHECConfig)
	rg.GET("/listNMSSecret", h.ListNMSSecret)
	rg.POST("/updateNMSSecret", h.UpdateNMSSecret)
	rg.GET("/downloadPasswordRSAPublicKey", h.DownloadPasswordRSAPublicKey)
	rg.POST("/downloadPlatformLogs", h.DownloadPlatformLogs)
	rg.GET("/downloadNMSManualDocument", h.DownloadNMSManualDocument)
}