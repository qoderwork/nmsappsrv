package mml

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for MML endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// ListMmlSets handles GET /mml-sets
func (h *Handler) ListMmlSets(c *gin.Context) {
	licenseId := middleware.GetLicenseId(c)

	sets, err := h.svc.ListMmlSets(licenseId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list MML sets")
		return
	}
	utils.Success(c, sets)
}

// ListMmlCommands handles GET /mml-commands?mml_set_id=
func (h *Handler) ListMmlCommands(c *gin.Context) {
	setId, err := strconv.Atoi(c.Query("mml_set_id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid mml_set_id")
		return
	}

	cmds, err := h.svc.ListMmlCommands(setId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list MML commands")
		return
	}
	utils.Success(c, cmds)
}

// GetMmlCommandParams handles GET /mml-commands/:id/params
func (h *Handler) GetMmlCommandParams(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid command id")
		return
	}

	params, err := h.svc.GetMmlCommandParams(id)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to get command params")
		return
	}
	utils.Success(c, params)
}

// ExecuteMml handles POST /mml-execute
func (h *Handler) ExecuteMml(c *gin.Context) {
	var req struct {
		ElementId int64                  `json:"element_id"`
		Command   string                 `json:"command"`
		Uid       string                 `json:"uid"`
		Params    map[string]interface{} `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	uid := middleware.GetUserId(c)
	username := middleware.GetUsername(c)

	result, err := h.svc.ExecuteMml(req.ElementId, req.Command, req.Uid, username, req.Params)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to execute MML command")
		return
	}
	_ = uid
	utils.Success(c, result)
}

// ListMmlResults handles GET /mml-results?element_id=&page=&pageSize=
func (h *Handler) ListMmlResults(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Query("element_id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element_id")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	data, total, err := h.svc.ListMmlResults(elementId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list MML results")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// GetMmlResult handles GET /mml-results/:id
func (h *Handler) GetMmlResult(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid result id")
		return
	}

	result, err := h.svc.GetMmlResult(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "MML result not found")
		return
	}
	utils.Success(c, result)
}

// GetMMLResultByEventLogIds handles POST /mml-results-by-event-log-ids
// 对齐 Java getMMLResultByEventLogIds：按 eventLogId 列表轮询已送达的 MML 执行结果。
func (h *Handler) GetMMLResultByEventLogIds(c *gin.Context) {
	var req struct {
		EventLogIds []int64 `json:"event_log_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.EventLogIds) == 0 {
		utils.Error(c, http.StatusBadRequest, "event_log_ids is required")
		return
	}

	data, err := h.svc.GetMMLResultByEventLogIds(req.EventLogIds)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to get MML results by event log ids")
		return
	}
	utils.Success(c, data)
}
