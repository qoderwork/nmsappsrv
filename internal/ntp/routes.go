package ntp

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all NTP configuration routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/ntp/config", h.ListNTPConfig)
	rg.PUT("/ntp/config", h.UpdateNTPConfig)
	rg.GET("/ntp/status", h.GetNTPStatus)
}
