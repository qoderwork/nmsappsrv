package diagnostics

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all diagnostics routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// CPE 诊断
	rg.POST("/diagnostics/result", h.ListDiagnosticsResult)
	rg.POST("/diagnostics/status", h.ListDiagnosticsStatus)
	rg.POST("/diagnostics/ping", h.DiagnosticsPing)
	rg.POST("/diagnostics/trace-route", h.DiagnosticsTraceRoute)
	rg.POST("/diagnostics/download", h.DiagnosticsDownload)
	rg.POST("/diagnostics/upload", h.DiagnosticsUpload)

	// Diagnostics task history
	rg.GET("/diagnostics/tasks", h.ListDiagnosticsTasks)
	rg.GET("/diagnostics/tasks/:id", h.GetDiagnosticsTask)
}
