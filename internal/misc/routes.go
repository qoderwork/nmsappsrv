package misc

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all miscellaneous routes on the given router group.
// Routes mirror the Java AOSManagementController (base @RequestMapping("/api/v2/"))).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// AOS Management — matching Java AOSManagementController exactly
	rg.GET("/updateEnableGeofence", h.UpdateEnableGeofence)
	rg.POST("/importNrAOSFile", h.ImportNrAOSFile)
	rg.GET("/downloadZTPTemplate", h.DownloadZTPTemplate)
	rg.POST("/downloadAOSFile", h.DownloadAOSFile)
	rg.POST("/getGenerateAOSFileTaskProgress", h.GetGenerateAOSFileTaskProgress)
	rg.POST("/listZTPResults", h.ListZTPResults)
	// TODO: rg.GET("/downloadZTPFile", h.DownloadZTPFile) // handler not yet implemented
	rg.POST("/setZTPStatus", h.SetZTPStatus)
	rg.POST("/deleteZTPFile", h.DeleteZTPFiles)
	rg.GET("/getZTPSetting", h.GetZTPSetting)
	rg.POST("/modifyZTPSetting", h.SaveZTPSetting)
	rg.GET("/listSpatialFileMarkets", h.ListSpatialFileMarkets)
	rg.POST("/getMarketCoordinates", h.GetMarketCoordinates)
	rg.POST("/listZTPRetryLog", h.ListZTPRetryLogs)
	rg.POST("/listTBG", h.ListTBG)
	rg.POST("/addTBG", h.AddTBG)
	rg.POST("/modifyTBG", h.ModifyTBG)
	rg.POST("/deleteTBG", h.DeleteTBG)
	rg.GET("/downloadTBGTemplate", h.DownloadTBGTemplate)
	rg.POST("/importTBGFile", h.ImportTBGFile)
	rg.POST("/batchReztp", h.BatchReZTP)
	rg.POST("/listHistoryZTPFiles", h.ListHistoryZTPFiles)
	rg.POST("/downloadHistoryZTPFile", h.DownloadHistoryZTPFile)
	rg.POST("/listPSAPId", h.ListPSAPID)
	rg.POST("/syncPSADID", h.SyncPSAPID)
	rg.POST("/listSyncPSAPIDLog", h.ListPSAPIDSyncLogs)
}
