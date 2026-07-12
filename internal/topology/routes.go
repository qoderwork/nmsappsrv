package topology

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all topology management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// LTE topology (BBU -> RU/ERU tree)
	rg.POST("/topology/lte", h.LteTopology)

	// NR topology (gNB -> EU/AU/RU tree)
	rg.POST("/topology/nr", h.NrTopology)

	// Batch upgrade EU and RU devices
	rg.POST("/topology/batch-upgrade", h.BatchUpgradeEUAndRU)

	// List batch upgrade logs
	rg.POST("/topology/batch-upgrade/logs", h.ListBatchUpgradeLog)

	// Reload LTE topology (clear cached params for re-collection)
	rg.POST("/topology/lte/reload", h.ReloadLTETopo)
}
