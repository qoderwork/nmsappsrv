package dashboard

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all dashboard management routes on the given router group.
// Routes mirror Java's DashboardManagementController (base /api/v2/) — all POST.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/listCpeOnlineStatistics", h.ListCpeOnlineStatistics)
	rg.POST("/listGNBOnlineStatistics", h.ListGNBOnlineStatistics)
	rg.POST("/listProductTypeAndDeviceCount", h.ListProductTypeAndDeviceCount)
	rg.POST("/listBaseStationStatistics", h.ListBaseStationStatistics)
	rg.POST("/listPDCPTrafficStatistic", h.ListPDCPTrafficStatistic)
	rg.POST("/listDeviceOnlineInfo", h.ListDeviceOnlineInfo)
	rg.POST("/statisticKPIForDevicelop", h.StatisticKPIForDevicelop)
}
