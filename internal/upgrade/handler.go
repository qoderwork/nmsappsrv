package upgrade

import (
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for upgrade endpoints.
type Handler struct {
	svc Service
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
	tenantId, _ := strconv.Atoi(c.DefaultQuery("tenant_id", "0"))

	data, total, err := h.svc.ListUpgradeFiles(tenantId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list upgrade files")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// UploadUpgradeFile handles POST /upgrade-files
// Accepts a multipart form with the file under the "file" field plus optional
// metadata form fields (version, device_type, product_type, file_type). The file
// bytes are persisted to local storage and the absolute path is recorded so the
// worker can hand a device-reachable URL to TR-069 Download.
func (h *Handler) UploadUpgradeFile(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "file is required")
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to open uploaded file")
		return
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to read uploaded file")
		return
	}

	tenantId := middleware.GetTenantId(c)
	username := middleware.GetUsername(c)

	storedPath, err := saveUpgradeFile(tenantId, fileHeader.Filename, data)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to persist upgrade file: "+err.Error())
		return
	}

	now := time.Now()
	fileName := fileHeader.Filename
	version := c.PostForm("version")
	deviceType := c.PostForm("device_type")
	productType := c.PostForm("product_type")
	fileType := c.PostForm("file_type")

	f := &UpgradeFile{
		FileName:         &fileName,
		FilePath:         &storedPath,
		Version:          &version,
		DeviceType:       &deviceType,
		ProductType:      &productType,
		FileType:         &fileType,
		FileSize:         int64Ptr(int64(len(data))),
		OriginalFileName: &fileName,
		TenantId:        &tenantId,
		User:             &username,
		UploadTime:       &now,
	}

	if err := h.svc.UploadUpgradeFile(f); err != nil {
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
	tenantId := middleware.GetTenantId(c)

	filter := UpgradeTaskFilter{
		SearchText: c.Query("searchText"),
		TaskName:   c.Query("taskName"),
		StartTime:  c.Query("startTime"),
		EndTime:    c.Query("endTime"),
		DeviceType: c.Query("deviceType"),
	}

	data, total, err := h.svc.ListUpgradeTasks(tenantId, filter, page, pageSize)
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
	tenantId := middleware.GetTenantId(c)

	data, total, err := h.svc.ListRebootTasks(tenantId, page, pageSize)
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
	tenantId := middleware.GetTenantId(c)

	data, total, err := h.svc.ListRollbackTasks(tenantId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list rollback tasks")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// ---------------------------------------------------------------------------
// Upgrade task lifecycle
// ---------------------------------------------------------------------------

// StartUpgradeTask handles POST /upgrade-tasks/:id/start
func (h *Handler) StartUpgradeTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	if err := h.svc.StartUpgradeTask(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// CancelUpgradeTask handles POST /upgrade-tasks/:id/cancel
func (h *Handler) CancelUpgradeTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	if err := h.svc.CancelUpgradeTask(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ListUpgradeResults handles GET /upgrade-tasks/:id/results
func (h *Handler) ListUpgradeResults(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	data, total, err := h.svc.ListUpgradeResults(id, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list upgrade results")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// ListUpgradeResultDetail handles GET /upgrade-tasks/:id/results/detail
func (h *Handler) ListUpgradeResultDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	data, total, err := h.svc.ListUpgradeResultDetail(id, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list upgrade result detail")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// ---------------------------------------------------------------------------
// Rollback task lifecycle
// ---------------------------------------------------------------------------

// StartRollbackTask handles POST /rollback-tasks/:id/start
func (h *Handler) StartRollbackTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	if err := h.svc.StartRollbackTask(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// CancelRollbackTask handles POST /rollback-tasks/:id/cancel
func (h *Handler) CancelRollbackTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	if err := h.svc.CancelRollbackTask(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ListRollbackResults handles GET /rollback-tasks/:id/results
func (h *Handler) ListRollbackResults(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	data, total, err := h.svc.ListRollbackResults(id, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list rollback results")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// ViewRollbackTask handles GET /rollback-tasks/:id/view
// Mirrors Java UpgradeManagementController.viewRollbackTask.
func (h *Handler) ViewRollbackTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}
	data, err := h.svc.ViewRollbackTask(id)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to view rollback task")
		return
	}
	utils.Success(c, data)
}

// ManualConfirmationUpgrade handles POST /upgrade/manual-confirmation
// body: {"logId": "..."}
// Mirrors Java UpgradeManagementController.manualConfirmationUpgrade.
func (h *Handler) ManualConfirmationUpgrade(c *gin.Context) {
	var req struct {
		LogId string `json:"logId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "logId is required")
		return
	}
	if err := h.svc.ManualConfirmationUpgrade(req.LogId); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// Statistics
// ---------------------------------------------------------------------------

// ListUpgradeTaskStatusCount handles POST /upgrade-tasks/status-count
func (h *Handler) ListUpgradeTaskStatusCount(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)

	data, err := h.svc.ListUpgradeTaskStatusCount(tenantId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to get status count")
		return
	}
	utils.Success(c, data)
}

// ListUpgradeDeviceResultCount handles POST /upgrade-tasks/device-result-count
func (h *Handler) ListUpgradeDeviceResultCount(c *gin.Context) {
	var req struct {
		TaskId int `json:"task_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	data, err := h.svc.ListUpgradeDeviceResultCount(req.TaskId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to get device result count")
		return
	}
	utils.Success(c, data)
}

// ---------------------------------------------------------------------------
// AutoUpgradeTask CRUD
// ---------------------------------------------------------------------------

// AutoUpgradeTaskList handles GET /auto-upgrade-tasks
func (h *Handler) AutoUpgradeTaskList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	data, total, err := h.svc.ListAutoUpgradeTasks(page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list auto upgrade tasks")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// AddAutoUpgradeTask handles POST /auto-upgrade-tasks
func (h *Handler) AddAutoUpgradeTask(c *gin.Context) {
	var t AutoUpgradeTask
	if err := c.ShouldBindJSON(&t); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.AddAutoUpgradeTask(&t); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to add auto upgrade task")
		return
	}
	utils.Success(c, t)
}

// ModifyAutoUpgradeTask handles PUT /auto-upgrade-tasks/:id
func (h *Handler) ModifyAutoUpgradeTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	var t AutoUpgradeTask
	if err := c.ShouldBindJSON(&t); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	t.Id = int(id)

	if err := h.svc.ModifyAutoUpgradeTask(&t); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to modify auto upgrade task")
		return
	}
	utils.Success(c, t)
}

// DeleteAutoUpgradeTask handles DELETE /auto-upgrade-tasks/:id
func (h *Handler) DeleteAutoUpgradeTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	if err := h.svc.DeleteAutoUpgradeTask(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete auto upgrade task")
		return
	}
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// File operations
// ---------------------------------------------------------------------------

// DownloadUpgradeFile handles POST /upgrade-files/:id/download
func (h *Handler) DownloadUpgradeFile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	file, err := h.svc.DownloadUpgradeFile(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "upgrade file not found")
		return
	}

	filePath := ""
	if file.FilePath != nil {
		filePath = *file.FilePath
	}
	fileName := "upgrade_file"
	if file.FileName != nil {
		fileName = *file.FileName
	}

	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.File(filePath)
}

// DownloadUpgradeFileRaw serves the upgrade file bytes to devices (CPE) over the
// file-server endpoint. It is device-facing (no web-auth) and is the URL handed to
// TR-069 Download, so the device can fetch the firmware directly.
func (h *Handler) DownloadUpgradeFileRaw(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	file, err := h.svc.ViewUpgradeFile(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "upgrade file not found")
		return
	}

	filePath := ""
	if file.FilePath != nil {
		filePath = *file.FilePath
	}
	if filePath == "" {
		utils.Error(c, http.StatusNotFound, "upgrade file not available")
		return
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		utils.Error(c, http.StatusNotFound, "upgrade file not found on disk")
		return
	}

	c.File(filePath)
}

// ViewUpgradeFile handles GET /upgrade-files/:id
func (h *Handler) ViewUpgradeFile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	file, err := h.svc.ViewUpgradeFile(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "upgrade file not found")
		return
	}
	utils.Success(c, file)
}

// UpdateUpgradeFile handles PUT /upgrade-files/:id
func (h *Handler) UpdateUpgradeFile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	var f UpgradeFile
	if err := c.ShouldBindJSON(&f); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	f.Id = id

	if err := h.svc.UpdateUpgradeFile(&f); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to update upgrade file")
		return
	}
	utils.Success(c, f)
}

// UploadUpgradeFileByPiecemeal handles POST /upgrade-files/piecemeal
func (h *Handler) UploadUpgradeFileByPiecemeal(c *gin.Context) {
	var req PiecemealUploadRequest
	req.FileName = c.PostForm("file_name")
	req.UploadId = c.PostForm("upload_id")
	req.DeviceType = c.PostForm("device_type")
	req.Version = c.PostForm("version")
	req.ProductType = c.PostForm("product_type")
	req.ChunkIndex, _ = strconv.Atoi(c.PostForm("chunk_index"))
	req.TotalChunks, _ = strconv.Atoi(c.PostForm("total_chunks"))
	req.ChunkSize, _ = strconv.ParseInt(c.PostForm("chunk_size"), 10, 64)
	req.TotalSize, _ = strconv.ParseInt(c.PostForm("total_size"), 10, 64)

	if req.FileName == "" || req.UploadId == "" {
		utils.Error(c, http.StatusBadRequest, "file_name and upload_id are required")
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "file chunk is required")
		return
	}
	defer file.Close()

	chunkData, err := io.ReadAll(file)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to read file chunk")
		return
	}

	tenantId := middleware.GetTenantId(c)
	username := middleware.GetUsername(c)

	if err := h.svc.UploadUpgradeFileByPiecemeal(&req, chunkData, tenantId, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}
