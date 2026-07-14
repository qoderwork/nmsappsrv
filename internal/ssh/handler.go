package ssh

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"
)

// Handler exposes HTTP handlers for SSH endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// StartExpiredChecker starts the background goroutine that disables SSH on expired timers.
func (h *Handler) StartExpiredChecker() {
	h.svc.StartExpiredChecker()
}

// ---------- SSH Label ----------

// AddSSHLabel handles POST /addSSHLabel
func (h *Handler) AddSSHLabel(c *gin.Context) {
	var req AddSSHLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	tenancyId := middleware.GetLicenseId(c)
	if err := h.svc.AddLabel(&req, tenancyId); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// DeleteSSHLabel handles POST /deleteSSHLabel
func (h *Handler) DeleteSSHLabel(c *gin.Context) {
	var req DeleteSSHLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.DeleteLabel(req.Id); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// ListSSHLabels handles POST /listSSHLabels
func (h *Handler) ListSSHLabels(c *gin.Context) {
	tenancyId := middleware.GetLicenseId(c)
	labels, err := h.svc.ListLabels(tenancyId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, labels)
}

// UpdateSSHLabel handles POST /updateSSHLabel
func (h *Handler) UpdateSSHLabel(c *gin.Context) {
	var req UpdateSSHLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	tenancyId := middleware.GetLicenseId(c)
	if err := h.svc.UpdateLabel(&req, tenancyId); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// ---------- SSH Access Timer ----------

// SetSSHAccessTimer handles POST /sshAccessTimer
func (h *Handler) SetSSHAccessTimer(c *gin.Context) {
	var req SSHAccessTimerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	tenancyId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)
	operationIds, err := h.svc.SetAccessTimer(&req, tenancyId, username)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	// Return the per-device operation IDs so the frontend can poll progress,
	// mirroring Java's Result<List<Long>>.operationIds.
	utils.OK(c, operationIds)
}

// ListSSHAccessTimer handles POST /listSSHAccessTimer
func (h *Handler) ListSSHAccessTimer(c *gin.Context) {
	var req ListSSHAccessTimerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}
	data, total, err := h.svc.ListAccessTimers(&req)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Paginated(c, data, total, req.Page, req.PageSize)
}
