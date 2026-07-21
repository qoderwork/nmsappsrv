package dashboard

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all dashboard management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/dashboard/cpe-online-stats", h.ListCpeOnlineStatistics)
	rg.POST("/dashboard/gnb-online-stats", h.ListGNBOnlineStatistics)
	rg.POST("/dashboard/product-type-device-count", h.ListProductTypeAndDeviceCount)
	rg.POST("/dashboard/base-station-stats", h.ListBaseStationStatistics)
	rg.POST("/dashboard/pdcp-traffic", h.ListPDCPTrafficStatistic)
	rg.POST("/dashboard/device-online-info", h.ListDeviceOnlineInfo)
	rg.POST("/dashboard/kpi-device-loop", h.StatisticKPIForDevicelop)
}
