package health

import "github.com/gin-gonic/gin"

// RegisterRoutes registers health module routes that require authentication.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/getMysqlInfo", h.GetMysqlInfo)
	rg.GET("/getRedisInfo", h.GetRedisInfo)
	rg.GET("/getQueueInfo", h.GetQueueInfo)
	rg.POST("/reportHAStatus", h.ReportHAStatus)
}

// RegisterPublicRoutes registers health module routes that do not require authentication.
func RegisterPublicRoutes(rg *gin.Engine, h *Handler) {
	rg.GET("/healthCheck", h.HealthCheck)
	rg.HEAD("/reportHAStatus", h.ReportHAStatusHead)
}
