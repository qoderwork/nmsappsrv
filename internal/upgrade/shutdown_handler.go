package upgrade

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"
)

// ShutdownHandler handles shutdown management endpoints.
type ShutdownHandler struct {
	svc *ShutdownService
}

// NewShutdownHandler creates a new ShutdownHandler.
func NewShutdownHandler(db *gorm.DB) *ShutdownHandler {
	return &ShutdownHandler{
		svc: NewShutdownService(db),
	}
}

// AddShutdownTask creates a new shutdown task.
func (h *ShutdownHandler) AddShutdownTask(c *gin.Context) {
	var req AddShutdownTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	username := middleware.GetUsername(c)
	tenantId := middleware.GetTenantId(c)

	taskId, err := h.svc.CreateShutdownTask(&req, username, tenantId)
	if err != nil {
		logger.Errorf("Failed to create shutdown task: %v", err)
		utils.Error(c, 500, "Failed to create shutdown task")
		return
	}

	utils.Success(c, gin.H{"taskId": taskId})
}

// ListShutdownTasks returns a paginated list of shutdown tasks.
func (h *ShutdownHandler) ListShutdownTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	req := ListShutdownTaskRequest{Page: page, PageSize: pageSize}

	tenantId := middleware.GetTenantId(c)

	tasks, total, err := h.svc.ListShutdownTasks(req.Page, req.PageSize, tenantId)
	if err != nil {
		logger.Errorf("Failed to list shutdown tasks: %v", err)
		utils.Error(c, 500, "Failed to list shutdown tasks")
		return
	}

	utils.Paginated(c, tasks, total, req.Page, req.PageSize)
}

// ViewShutdownTask returns the detail of a shutdown task with device list.
func (h *ShutdownHandler) ViewShutdownTask(c *gin.Context) {
	taskId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "Invalid task id")
		return
	}

	task, err := h.svc.ViewShutdownTask(taskId)
	if err != nil {
		logger.Errorf("Failed to view shutdown task: %v", err)
		utils.Error(c, 500, "Failed to view shutdown task")
		return
	}

	utils.Success(c, task)
}

// DeleteShutdownTask deletes a shutdown task and associated logs.
func (h *ShutdownHandler) DeleteShutdownTask(c *gin.Context) {
	taskId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "Invalid task id")
		return
	}

	if err := h.svc.DeleteShutdownTask(taskId); err != nil {
		logger.Errorf("Failed to delete shutdown task: %v", err)
		utils.Error(c, 500, "Failed to delete shutdown task")
		return
	}

	utils.OK(c, nil)
}

// ListShutdownResults returns the per-device shutdown results for a task.
func (h *ShutdownHandler) ListShutdownResults(c *gin.Context) {
	taskId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "Invalid task id")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	results, total, err := h.svc.ListShutdownResults(taskId, page, pageSize)
	if err != nil {
		logger.Errorf("Failed to list shutdown results: %v", err)
		utils.Error(c, 500, "Failed to list shutdown results")
		return
	}

	utils.Paginated(c, results, total, page, pageSize)
}
