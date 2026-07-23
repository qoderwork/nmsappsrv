package diagnostics

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all diagnostics routes on the given router group.
// Aligned with Java CPEDiagnosticsController: @RequestMapping("/api/v1/") + @PostMapping("methodName")
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/listDiagnosticsResult", h.ListDiagnosticsResult)
	rg.POST("/listDiagnosticsStatus", h.ListDiagnosticsStatus)
	rg.POST("/diagnosticsPing", h.DiagnosticsPing)
	rg.POST("/diagnosticsTraceRoute", h.DiagnosticsTraceRoute)
	rg.POST("/diagnosticsDownload", h.DiagnosticsDownload)
	rg.POST("/diagnosticsUpload", h.DiagnosticsUpload)
	rg.POST("/listDiagnosticsTasks", h.ListDiagnosticsTasks)
	rg.POST("/getDiagnosticsTask", h.GetDiagnosticsTask)
}
