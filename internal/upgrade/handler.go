package upgrade

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for upgrade endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// ---------------------------------------------------------------------------
// UpgradeFile endpoints
// ---------------------------------------------------------------------------

// ListUpgradeFiles handles GET /files?page=&pageSize=
func (h *Handler) ListUpgradeFiles(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	tenancyId, _ := strconv.Atoi(c.DefaultQuery("tenancy_id", "0"))

	data, total, err := h.svc.ListUpgradeFiles(tenancyId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list upgrade files")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// UploadUpgradeFile handles POST /files
func (h *Handler) UploadUpgradeFile(c *gin.Context) {
	var f UpgradeFile
	if err := c.ShouldBindJSON(&f); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.UploadUpgradeFile(&f); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to upload upgrade file")
		return
	}
	utils.Success(c, f)
}

// DeleteUpgradeFile handles DELETE /files/:id
func (h *Handler) DeleteUpgradeFile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	if err := h.svc.DeleteUpgradeFile(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete upgrade file")
		return
	}
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// UpgradeTask endpoints
// ---------------------------------------------------------------------------

// ListUpgradeTasks handles GET /tasks?page=&pageSize=&searchText=&taskName=&startTime=&endTime=&deviceType=
func (h *Handler) ListUpgradeTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	tenancyId := middleware.GetLicenseId(c)

	filter := UpgradeTaskFilter{
		SearchText: c.Query("searchText"),
		TaskName:   c.Query("taskName"),
		StartTime:  c.Query("startTime"),
		EndTime:    c.Query("endTime"),
		DeviceType: c.Query("deviceType"),
	}

	data, total, err := h.svc.ListUpgradeTasks(tenancyId, filter, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list upgrade tasks")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// GetUpgradeTask handles GET /tasks/:id
func (h *Handler) GetUpgradeTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	data, err := h.svc.GetUpgradeTask(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "upgrade task not found")
		return
	}
	utils.Success(c, data)
}

// CreateUpgradeTask handles POST /tasks
func (h *Handler) CreateUpgradeTask(c *gin.Context) {
	var t UpgradeTask
	if err := c.ShouldBindJSON(&t); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateUpgradeTask(&t); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create upgrade task")
		return
	}
	utils.Success(c, t)
}

// ---------------------------------------------------------------------------
// UpgradeLog endpoints
// ---------------------------------------------------------------------------

// ListUpgradeLogs handles GET /task/:taskId/logs?page=&pageSize=
func (h *Handler) ListUpgradeLogs(c *gin.Context) {
	taskId, err := strconv.Atoi(c.Param("taskId"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	data, total, err := h.svc.ListUpgradeLogs(taskId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list upgrade logs")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// ---------------------------------------------------------------------------
// RebootTask endpoints
// ---------------------------------------------------------------------------

// CreateRebootTask handles POST /reboot
func (h *Handler) CreateRebootTask(c *gin.Context) {
	var t RebootTask
	if err := c.ShouldBindJSON(&t); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateRebootTask(&t); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create reboot task")
		return
	}
	utils.Success(c, t)
}

// ListRebootTasks handles GET /reboot?page=&pageSize=
func (h *Handler) ListRebootTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	tenancyId := middleware.GetLicenseId(c)

	data, total, err := h.svc.ListRebootTasks(tenancyId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list reboot tasks")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// ---------------------------------------------------------------------------
// RollbackTask endpoints
// ---------------------------------------------------------------------------

// CreateRollbackTask handles POST /rollback
func (h *Handler) CreateRollbackTask(c *gin.Context) {
	var t RollbackTask
	if err := c.ShouldBindJSON(&t); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateRollbackTask(&t); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create rollback task")
		return
	}
	utils.Success(c, t)
}

// ListRollbackTasks handles GET /rollback?page=&pageSize=
func (h *Handler) ListRollbackTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	tenancyId := middleware.GetLicenseId(c)

	data, total, err := h.svc.ListRollbackTasks(tenancyId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list rollback tasks")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}
