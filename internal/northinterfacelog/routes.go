package northinterfacelog

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the audit-log query endpoint. The audit *capture* is
// installed separately as middleware (AuditMiddleware) on the northbound group
// in cmd/main.go so that every northbound call is recorded.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/listNorthInterfaceLog", h.ListLogs)
}
