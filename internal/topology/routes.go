package topology

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all topology management routes on the given router group.
// Routes mirror Java's TopologyManagementController (base /api/v2/) — all POST.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/lteTopology", h.LteTopology)
	rg.POST("/batchUpgradeEUAndRU", h.BatchUpgradeEUAndRU)
	rg.POST("/listBatchUpgradeLog", h.ListBatchUpgradeLog)
	rg.POST("/reloadLTETopo", h.ReloadLTETopo)
	rg.POST("/nrTopology", h.NrTopology)
}
