package health

import "github.com/gin-gonic/gin"

// RegisterRoutes registers health module routes that require authentication.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// --- REST-style routes (preferred) ---
	rg.GET("/health/mysql", h.GetMysqlInfo)
	rg.GET("/health/redis", h.GetRedisInfo)
	rg.GET("/health/queues", h.GetQueueInfo)
	rg.POST("/health/ha-status", h.ReportHAStatus)

	// --- Legacy RPC-style routes (deprecated) ---
	// Deprecated: use GET /health/mysql instead
	rg.GET("/getMysqlInfo", h.GetMysqlInfo)
	// Deprecated: use GET /health/redis instead
	rg.GET("/getRedisInfo", h.GetRedisInfo)
	// Deprecated: use GET /health/queues instead
	rg.GET("/getQueueInfo", h.GetQueueInfo)
	// Deprecated: use POST /health/ha-status instead
	rg.POST("/reportHAStatus", h.ReportHAStatus)
}

// RegisterPublicRoutes registers health module routes that do not require authentication.
func RegisterPublicRoutes(rg *gin.Engine, h *Handler) {
	rg.GET("/healthCheck", h.HealthCheck)
	rg.HEAD("/reportHAStatus", h.ReportHAStatusHead)

	// Kubernetes-style probes (see docs/DEPLOYMENT_CHECKLIST.md §6).
	rg.GET("/healthz", h.Liveness)  // liveness — process alive, no deps
	rg.GET("/readyz", h.Readiness)  // readiness — pings DB + Redis (503 if down)
}
