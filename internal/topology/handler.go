package topology

import (
	"nmsappsrv/internal/middleware"
	"nmsappsrv/internal/tr069"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler exposes topology HTTP endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a new Handler.
func NewHandler(db *gorm.DB, opSender *tr069.OperationSender) *Handler {
	return &Handler{svc: NewService(db, opSender)}
}

// LteTopology handles GET /topology/lte?id=123
func (h *Handler) LteTopology(c *gin.Context) {
	var req LongIdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request: id is required")
		return
	}
	data, err := h.svc.LteTopology(req.Id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// NrTopology handles GET /topology/nr?id=123
func (h *Handler) NrTopology(c *gin.Context) {
	var req LongIdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request: id is required")
		return
	}
	data, err := h.svc.NrTopology(req.Id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// BatchUpgradeEUAndRU handles POST /topology/batch-upgrade
func (h *Handler) BatchUpgradeEUAndRU(c *gin.Context) {
	var req BatchUpgradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}
	tenantId := middleware.GetTenantId(c)
	username := middleware.GetUsername(c)
	if err := h.svc.BatchUpgradeEUAndRU(&req, tenantId, username); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ListBatchUpgradeLog handles GET /topology/batch-upgrade/logs
func (h *Handler) ListBatchUpgradeLog(c *gin.Context) {
	var q ListBatchUpgradeLogQuery
	if err := c.ShouldBindJSON(&q); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}
	data, total, err := h.svc.ListBatchUpgradeLog(&q)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Paginated(c, data, total, q.Page, q.PageSize)
}

// ReloadLTETopo handles POST /topology/lte/reload
func (h *Handler) ReloadLTETopo(c *gin.Context) {
	var req LongIdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request: id is required")
		return
	}
	if err := h.svc.ReloadLTETopo(req.Id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}
