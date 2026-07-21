package deviceauth

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all device auth routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/device-auth/config", h.GetConfig)
	rg.PUT("/device-auth/config", h.SaveConfig)
	rg.GET("/device-auth/info", h.GetAuthInfo)
}
