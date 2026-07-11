package dashboard

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all dashboard management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// --- REST-style routes (preferred) ---
	rg.POST("/dashboard/cpe-online-stats", h.ListCpeOnlineStatistics)
	rg.POST("/dashboard/gnb-online-stats", h.ListGNBOnlineStatistics)
	rg.POST("/dashboard/product-type-device-count", h.ListProductTypeAndDeviceCount)
	rg.POST("/dashboard/base-station-stats", h.ListBaseStationStatistics)
	rg.POST("/dashboard/pdcp-traffic", h.ListPDCPTrafficStatistic)
	rg.POST("/dashboard/device-online-info", h.ListDeviceOnlineInfo)
	rg.POST("/dashboard/kpi-device-loop", h.StatisticKPIForDevicelop)

	// --- Legacy RPC-style routes (deprecated) ---
	// Deprecated: use POST /dashboard/cpe-online-stats instead
	rg.POST("/listCpeOnlineStatistics", h.ListCpeOnlineStatistics)
	// Deprecated: use POST /dashboard/gnb-online-stats instead
	rg.POST("/listGNBOnlineStatistics", h.ListGNBOnlineStatistics)
	// Deprecated: use POST /dashboard/product-type-device-count instead
	rg.POST("/listProductTypeAndDeviceCount", h.ListProductTypeAndDeviceCount)
	// Deprecated: use POST /dashboard/base-station-stats instead
	rg.POST("/listBaseStationStatistics", h.ListBaseStationStatistics)
	// Deprecated: use POST /dashboard/pdcp-traffic instead
	rg.POST("/listPDCPTrafficStatistic", h.ListPDCPTrafficStatistic)
	// Deprecated: use POST /dashboard/device-online-info instead
	rg.POST("/listDeviceOnlineInfo", h.ListDeviceOnlineInfo)
	// Deprecated: use POST /dashboard/kpi-device-loop instead
	rg.POST("/statisticKPIForDevicelop", h.StatisticKPIForDevicelop)
}
