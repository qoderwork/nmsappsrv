package eventlog

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for event-log endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// ---------------------------------------------------------------------------
// EventLog endpoints
// ---------------------------------------------------------------------------

// ListEventLogs handles GET /?element_id=&event_type=&page=&pageSize=
func (h *Handler) ListEventLogs(c *gin.Context) {
	elementIdStr := c.DefaultQuery("element_id", "0")
	elementId, err := strconv.ParseInt(elementIdStr, 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element_id")
		return
	}

	eventType := c.Query("event_type")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	data, total, err := h.svc.ListEventLogs(elementId, eventType, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list event logs")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// GetEventLog handles GET /:id
func (h *Handler) GetEventLog(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid event log id")
		return
	}

	data, err := h.svc.GetEventLog(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "event log not found")
		return
	}
	utils.Success(c, data)
}

// ---------------------------------------------------------------------------
// TaskToEventLog endpoints
// ---------------------------------------------------------------------------

// ListTaskEventLogs handles GET /task/:taskId?task_type=
func (h *Handler) ListTaskEventLogs(c *gin.Context) {
	taskId, err := strconv.Atoi(c.Param("taskId"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	taskType := c.Query("task_type")

	data, err := h.svc.ListTaskEventLogs(taskId, taskType)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list task event logs")
		return
	}
	utils.Success(c, data)
}
