package reset

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for reset endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// AddResetTask handles POST /reset-tasks
func (h *Handler) AddResetTask(c *gin.Context) {
	var req AddResetTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	tenantId := middleware.GetTenantId(c)
	username := middleware.GetUsername(c)
	id, err := h.svc.AddResetTask(&req, tenantId, username)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, map[string]int{"id": id})
}

// DeleteResetTask handles DELETE /reset-tasks/:id
func (h *Handler) DeleteResetTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}
	if err := h.svc.DeleteResetTask(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// StartResetTask handles POST /reset-tasks/:id/start
func (h *Handler) StartResetTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}
	username := middleware.GetUsername(c)
	if err := h.svc.StartResetTask(id, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// CancelResetTask handles POST /reset-tasks/:id/cancel
func (h *Handler) CancelResetTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}
	if err := h.svc.CancelResetTask(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// ListResetTasks handles GET /reset-tasks
func (h *Handler) ListResetTasks(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	query := ListResetTaskQuery{
		TaskName:   c.Query("taskName"),
		DeviceType: c.Query("deviceType"),
	}
	if v := c.Query("startTime"); v != "" {
		query.StartTime = &v
	}
	if v := c.Query("endTime"); v != "" {
		query.EndTime = &v
	}
	if v, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil {
		query.Page = v
	}
	if v, err := strconv.Atoi(c.DefaultQuery("pageSize", "20")); err == nil {
		query.PageSize = v
	}
	data, total, err := h.svc.ListTasks(tenantId, query)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Paginated(c, data, total, query.Page, query.PageSize)
}

// ListResetTaskResults handles GET /reset-tasks/:id/results
func (h *Handler) ListResetTaskResults(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}
	query := ListResetTaskResultQuery{
		TaskId:       id,
		SerialNumber: c.Query("serialNumber"),
	}
	if v, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil {
		query.Page = v
	}
	if v, err := strconv.Atoi(c.DefaultQuery("pageSize", "20")); err == nil {
		query.PageSize = v
	}
	data, total, err := h.svc.ListTaskResults(query)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Paginated(c, data, total, query.Page, query.PageSize)
}
