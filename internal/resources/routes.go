package resources

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all resource monitoring routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// Resources Module
	rg.POST("/cpu-mem-usage", h.GetCpuAndMemUsage)
	rg.GET("/table-status", h.GetTableStatus)
	rg.GET("/disk-usage", h.GetDiskUsage)
	rg.POST("/cpu-mem-threshold", h.SetCPUAndMemThreshold)
	rg.POST("/list-cpu-mem-threshold", h.ListCPUAndMemThreshold)
}
