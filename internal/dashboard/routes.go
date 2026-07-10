package dashboard

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all dashboard management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// Dashboard Management Module
	rg.POST("/listCpeOnlineStatistics", h.ListCpeOnlineStatistics)
	rg.POST("/listGNBOnlineStatistics", h.ListGNBOnlineStatistics)
	rg.POST("/listProductTypeAndDeviceCount", h.ListProductTypeAndDeviceCount)
	rg.POST("/listBaseStationStatistics", h.ListBaseStationStatistics)
	rg.POST("/listPDCPTrafficStatistic", h.ListPDCPTrafficStatistic)
	rg.POST("/listDeviceOnlineInfo", h.ListDeviceOnlineInfo)
	rg.POST("/statisticKPIForDevicelop", h.StatisticKPIForDevicelop)
}
