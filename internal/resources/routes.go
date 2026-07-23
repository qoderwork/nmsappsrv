package resources

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/getCpuAndMemUsage", h.GetCpuAndMemUsage)
	rg.POST("/getTableStatus", h.GetTableStatus)
	rg.POST("/getDiskUsage", h.GetDiskUsage)
	rg.POST("/updateServerResourceAlarmThreshold", h.SetCPUAndMemThreshold)
	rg.POST("/listServerResourceAlarmThreshold", h.ListCPUAndMemThreshold)
}