package corenet

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all core network routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 核心网
	rg.GET("/core-networks", h.ListCoreNetworks)
	rg.GET("/core-networks/:id", h.GetCoreNetwork)
	rg.POST("/core-networks", h.CreateCoreNetwork)
	rg.PUT("/core-networks/:id", h.UpdateCoreNetwork)
	rg.DELETE("/core-networks/:id", h.DeleteCoreNetwork)
	rg.GET("/core-networks/:id/data", h.GetCoreNetworkData)
	rg.PUT("/core-networks/:id/data", h.SaveCoreNetworkData)
	rg.GET("/core-networks/:id/kpis", h.GetCoreNetworkKpis)
	rg.GET("/core-networks/:id/statistics", h.GetStatisticData)
	rg.GET("/core-networks/:id/logs", h.ListOperationLogs)

	// Tier 1 corenet KPI batch
	rg.POST("/core-networks/alarms", h.GetCoreNetworkAlarms)
	rg.POST("/core-networks/ue-list", h.ListUEList)
	rg.POST("/core-networks/ue-number-statistic", h.ListUENumberStatistic)
	rg.POST("/core-networks/ue-infos", h.GetUeInfos)
	rg.POST("/core-networks/switch", h.ChangeCoreNetworkSwitch)
	rg.POST("/core-networks/kpi/user-info", h.GetCoreNetworkUserInfo)
	rg.POST("/core-networks/kpi/upf-traffic", h.GetCoreNetworkUpfTraffic)
	rg.POST("/core-networks/kpi/upf-traffic/built-in", h.GetBuiltInCoreNetworkUpfTraffic)
	rg.POST("/core-networks/kpi/user-info/built-in", h.GetBuiltInCoreNetworkUserInfo)
	rg.POST("/core-networks/kpi/report", h.GetKpiReport)
}
