package heartbeat

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all heartbeat (SAS/CBSD) routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *HeartbeatHandler) {
	// Heartbeat (SAS/CBSD)
	rg.POST("/heartbeat/process", h.ProcessHeartbeat)
	rg.GET("/heartbeat/status", h.ListHeartbeatStatus)
	rg.POST("/heartbeat/send/:sn", h.SendHeartbeat)
}
