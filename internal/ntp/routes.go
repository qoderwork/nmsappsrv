package ntp

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all NTP configuration routes on the given router group.
// Aligned with Java NTPController: @RequestMapping("/api/v1/") + @PostMapping("methodName")
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/listNTPConfig", h.ListNTPConfig)
	rg.POST("/updateNTPConfig", h.UpdateNTPConfig)
	rg.POST("/getNTPStatus", h.GetNTPStatus)
}
