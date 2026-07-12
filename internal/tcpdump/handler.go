package tcpdump

import (
	"net/http"
	"strconv"

	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for tcpdump endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// StartCapture handles POST /tcpdump/start
func (h *Handler) StartCapture(c *gin.Context) {
	var req StartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: elementId and duration are required")
		return
	}

	// Sanitize the filter expression
	if req.Filter != "" {
		req.Filter = sanitizeFilter(req.Filter)
	}

	taskId, err := h.svc.StartCapture(&req)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.Success(c, StartResponse{TaskId: taskId})
}

// StopCapture handles POST /tcpdump/stop/:taskId
func (h *Handler) StopCapture(c *gin.Context) {
	taskId, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	if err := h.svc.StopCapture(taskId); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.OK(c, "ok")
}

// ListTasks handles GET /tcpdump/tasks
func (h *Handler) ListTasks(c *gin.Context) {
	tasks, err := h.svc.ListTasks()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, tasks)
}

// GetTask handles GET /tcpdump/tasks/:id
func (h *Handler) GetTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	task, err := h.svc.GetTask(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "task not found")
		return
	}

	utils.Success(c, task)
}

// DownloadCapture handles GET /tcpdump/tasks/:id/download
func (h *Handler) DownloadCapture(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	filePath, err := h.svc.GetTaskFilePath(id)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Header("Content-Type", "application/vnd.tcpdump.pcap")
	c.Header("Content-Disposition", "attachment; filename=capture.pcap")
	c.File(filePath)
}

// DeleteTask handles DELETE /tcpdump/tasks/:id
func (h *Handler) DeleteTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	if err := h.svc.DeleteTask(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.OK(c, "deleted")
}
