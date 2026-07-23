package health

import "github.com/gin-gonic/gin"

// RegisterRoutes registers health module routes that require authentication.
// These are mounted under the /api/v2/ group in cmd/main.go, matching the
// Java HealthController paths (@RequestMapping without class-level base path).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// Java: HealthController @GetMapping("/api/v2/getMysqlInfo")
	rg.GET("/getMysqlInfo", h.GetMysqlInfo)
	// Java: HealthController @GetMapping("/api/v2/getRedisInfo")
	rg.GET("/getRedisInfo", h.GetRedisInfo)
	// TODO: Java: HealthController @GetMapping("/api/v2/getRabbitMQInfo")
	// RabbitMQ is excluded per project memory; handler not implemented.
	// rg.GET("/getRabbitMQInfo", h.GetRabbitMQInfo)
}

// RegisterPublicRoutes registers health module routes that do not require
// authentication. These are mounted on the root router in cmd/main.go,
// matching the Java HealthController public paths.
func RegisterPublicRoutes(rg *gin.Engine, h *Handler) {
	// Java: HealthController @GetMapping("/healthCheck")
	rg.GET("/healthCheck", h.HealthCheck)
	// Java: HealthController @PostMapping("/reportHAStatus")
	rg.POST("/reportHAStatus", h.ReportHAStatus)
	// Java: HealthController @RequestMapping(value = "/reportHAStatus", method = HEAD)
	rg.HEAD("/reportHAStatus", h.ReportHAStatusHead)

	// Kubernetes-style probes (Go-specific additions, not in Java backend).
	// See docs/DEPLOYMENT_CHECKLIST.md §6.
	rg.GET("/healthz", h.Liveness) // liveness — process alive, no deps
	rg.GET("/readyz", h.Readiness) // readiness — pings DB + Redis (503 if down)
}
