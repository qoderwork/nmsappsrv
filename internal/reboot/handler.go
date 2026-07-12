package reboot

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposesHTTP handlers for reboot endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// AddRebootTask handles POST /reboot-tasks
func (h *Handler) AddRebootTask(c *gin.Context) {
	var req AddRebootTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	tenancyId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)

	id, err := h.svc.AddRebootTask(&req, tenancyId, username)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, map[string]int{"id": id})
}

// DeleteRebootTask handles DELETE /reboot-tasks/:id
func (h *Handler) DeleteRebootTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}
	if err := h.svc.DeleteRebootTask(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// StartRebootTask handles POST /reboot-tasks/:id/start
func (h *Handler) StartRebootTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}
	username := middleware.GetUsername(c)
	if err := h.svc.StartRebootTask(id, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// CancelRebootTask handles POST /reboot-tasks/:id/cancel
func (h *Handler) CancelRebootTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}
	if err := h.svc.CancelRebootTask(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// ListRebootTasks handles GET /reboot-tasks
func (h *Handler) ListRebootTasks(c *gin.Context) {
	tenancyId := middleware.GetLicenseId(c)
	query := ListRebootTaskQuery{
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

	data, total, err := h.svc.ListTasks(tenancyId, query)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Paginated(c, data, total, query.Page, query.PageSize)
}

// ListRebootTaskResults handles GET /reboot-tasks/:id/results
func (h *Handler) ListRebootTaskResults(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}
	query := ListRebootTaskResultQuery{
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
