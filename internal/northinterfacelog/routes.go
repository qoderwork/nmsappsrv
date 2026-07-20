package northinterfacelog

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/north-interface-logs", h.ListLogs)
}
