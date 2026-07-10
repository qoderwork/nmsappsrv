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
	rg.GET("/core-network-logs", h.ListOperationLogs)
}
