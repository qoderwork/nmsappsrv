package misc

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/config"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for miscellaneous endpoints.
type Handler struct {
	svc Service
	// EnqueueZTPFunc is injected from main to avoid an import cycle with tr069.
	EnqueueZTPFunc func(ctx context.Context, elementId int64, serialNumber, operationType, operationUser string) error
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB, cfg *config.Config) *Handler {
	return &Handler{svc: NewService(db, cfg)}
}

// getTenancyId extracts tenancy_id from gin context as int.
func getTenancyId(c *gin.Context) int {
	v, ok := c.Get("tenancy_id")
	if !ok {
		return 0
	}
	s, ok := v.(string)
	if !ok {
		return 0
	}
	id, _ := strconv.Atoi(s)
	return id
}

func (h *Handler) ListBackupRestoreTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	tenancyId := getTenancyId(c)
	data, total, err := h.svc.ListBackupRestoreTasksVo(tenancyId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list backup/restore tasks")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

func (h *Handler) CreateBackup(c *gin.Context) {
	var req BackupRestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	username := middleware.GetUsername(c)
	tenancyId := getTenancyId(c)
	if err := h.svc.CreateBackupTask(&req, username, tenancyId); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) CreateRestore(c *gin.Context) {
	var req BackupRestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	username := middleware.GetUsername(c)
	tenancyId := getTenancyId(c)
	if err := h.svc.CreateRestoreTask(&req, username, tenancyId); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) StartBackupRestoreTask(c *gin.Context) {
	var body struct {
		TaskId int `json:"taskId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.TaskId == 0 {
		utils.Error(c, http.StatusBadRequest, "taskId is required")
		return
	}
	username := middleware.GetUsername(c)
	if err := h.svc.StartBackupRestoreTask(body.TaskId, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) CancelBackupRestoreTask(c *gin.Context) {
	var body struct {
		TaskId int `json:"taskId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.TaskId == 0 {
		utils.Error(c, http.StatusBadRequest, "taskId is required")
		return
	}
	if err := h.svc.CancelBackupRestoreTask(body.TaskId); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) ListBackupRestoreTaskDetail(c *gin.Context) {
	taskId, err := strconv.Atoi(c.Param("taskId"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid taskId")
		return
	}
	data, err := h.svc.ListBackupRestoreTaskDetail(taskId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list task detail")
		return
	}
	utils.Success(c, data)
}

func (h *Handler) ListBatchConfigLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	tenancyId := getTenancyId(c)
	data, total, err := h.svc.ListBatchConfigLogs(tenancyId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list batch config logs")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

func (h *Handler) ListMRData(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Query("element_id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element_id")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	data, total, err := h.svc.ListMRData(elementId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list MR data")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

func (h *Handler) ListZTPLogs(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Query("element_id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element_id")
		return
	}
	logs, err := h.svc.ListZTPLogs(elementId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list ZTP logs")
		return
	}
	utils.Success(c, logs)
}

func (h *Handler) GetZTPSetting(c *gin.Context) {
	setting, err := h.svc.GetZTPSetting()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to get ZTP setting")
		return
	}
	utils.Success(c, setting)
}

func (h *Handler) SaveZTPSetting(c *gin.Context) {
	var setting ZTPSetting
	if err := c.ShouldBindJSON(&setting); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.SaveZTPSetting(&setting); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to save ZTP setting")
		return
	}
	utils.Success(c, setting)
}

func (h *Handler) ListZTPResults(c *gin.Context) {
	var req ListZTPResultsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	data, total, err := h.svc.ListZTPResults(&req)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list ZTP results")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	utils.Paginated(c, data, total, page, pageSize)
}

func (h *Handler) ListZTPRetryLogs(c *gin.Context) {
	var body struct {
		ElementId int64 `json:"elementId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.ElementId == 0 {
		utils.Error(c, http.StatusBadRequest, "elementId is required")
		return
	}
	logs, err := h.svc.ListZTPRetryLogs(body.ElementId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list retry logs")
		return
	}
	utils.Success(c, logs)
}

func (h *Handler) ListHistoryZTPFiles(c *gin.Context) {
	var body struct {
		ElementId int64 `json:"elementId"`
		Page      int   `json:"page"`
		PageSize  int   `json:"pageSize"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.ElementId == 0 {
		utils.Error(c, http.StatusBadRequest, "elementId is required")
		return
	}
	data, total, err := h.svc.ListHistoryZTPFiles(body.ElementId, body.Page, body.PageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list history ZTP files")
		return
	}
	page := body.Page
	if page < 1 {
		page = 1
	}
	pageSize := body.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	utils.Paginated(c, data, total, page, pageSize)
}

func (h *Handler) SetZTPStatus(c *gin.Context) {
	var req SetZTPStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.SetZTPStatus(&req); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) BatchReZTP(c *gin.Context) {
	var req BatchReZTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.BatchReZTP(&req); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) DeleteZTPFiles(c *gin.Context) {
	var req DeleteZTPFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.DeleteZTPFiles(&req); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ProvisionZTP handles POST /ztp/provision — enqueues ZTP provisioning tasks.
func (h *Handler) ProvisionZTP(c *gin.Context) {
	var req struct {
		ElementIds    []int64 `json:"elementIds" binding:"required"`
		OperationUser string  `json:"operationUser"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.OK(c, map[string]string{"error": "invalid request: " + err.Error()})
		return
	}

	ctx := context.Background()
	enqueued := 0
	for _, elementId := range req.ElementIds {
		// Look up device serial number via the service layer.
		sn, err := h.svc.GetDeviceSerialNumber(elementId)
		if err != nil {
			logger.Warnf("ztp provision: device %d not found: %v", elementId, err)
			continue
		}
		if sn == "" {
			logger.Warnf("ztp provision: device %d has no serial number", elementId)
			continue
		}

		if err := h.EnqueueZTPFunc(ctx, elementId, sn, "provision", req.OperationUser); err != nil {
			logger.Errorf("ztp provision: failed to enqueue device %d: %v", elementId, err)
			continue
		}
		enqueued++
	}

	utils.OK(c, map[string]interface{}{
		"enqueued": enqueued,
		"total":    len(req.ElementIds),
	})
}

// GenerateAOSFile generates the AOS auto-config XML for a single device and
// records cpe_element.aos_file_name. Exposed on the Handler so the tr069 ZTP
// worker can invoke it without importing config (avoids a cycle).
func (h *Handler) GenerateAOSFile(elementId int64) (string, error) {
	return h.svc.GenerateAOSFile(elementId)
}

// ScanAndGenerateAOSFiles runs the ZTPTask-style scan for ready devices.
func (h *Handler) ScanAndGenerateAOSFiles() (int, error) {
	return h.svc.ScanAndGenerateAOSFiles()
}

func (h *Handler) ListNorthReports(c *gin.Context) {
	licenseId := middleware.GetLicenseId(c)
	reports, err := h.svc.ListNorthReports(licenseId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list north reports")
		return
	}
	utils.Success(c, reports)
}

func (h *Handler) CreateNorthReport(c *gin.Context) {
	var report NorthReport
	if err := c.ShouldBindJSON(&report); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.CreateNorthReport(&report); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create north report")
		return
	}
	utils.Success(c, report)
}

func (h *Handler) UpdateNorthReport(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid north report id")
		return
	}
	var report NorthReport
	if err := c.ShouldBindJSON(&report); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	report.Id = id
	if err := h.svc.UpdateNorthReport(&report); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to update north report")
		return
	}
	utils.Success(c, report)
}

func (h *Handler) DeleteNorthReport(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid north report id")
		return
	}
	if err := h.svc.DeleteNorthReport(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete north report")
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) ListRadius(c *gin.Context) {
	tenancyId := getTenancyId(c)
	list, err := h.svc.ListRadius(tenancyId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list RADIUS configs")
		return
	}
	utils.Success(c, list)
}

func (h *Handler) SaveRadius(c *gin.Context) {
	var rad Radius
	if err := c.ShouldBindJSON(&rad); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.SaveRadius(&rad); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to save RADIUS config")
		return
	}
	utils.Success(c, rad)
}

func (h *Handler) DeleteRadius(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid RADIUS id")
		return
	}
	if err := h.svc.DeleteRadius(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete RADIUS config")
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) ListOperatorLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	tenancyId := getTenancyId(c)
	data, total, err := h.svc.ListOperatorLogs(tenancyId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list operator logs")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

func (h *Handler) ListUploadFiles(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	data, total, err := h.svc.ListUploadFiles(page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list upload files")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

func (h *Handler) CreateUploadFile(c *gin.Context) {
	var f UploadFile
	if err := c.ShouldBindJSON(&f); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.CreateUploadFile(&f); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create upload file")
		return
	}
	utils.Success(c, f)
}

func (h *Handler) DeleteUploadFile(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteUploadFile(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete upload file")
		return
	}
	utils.Success(c, nil)
}

// ---------- BatchAddObject ----------

func (h *Handler) BatchAddObject(c *gin.Context) {
	var req BatchAddObjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	username := middleware.GetUsername(c)
	tenancyId := getTenancyId(c)
	if err := h.svc.BatchAddObject(&req, username, tenancyId); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) ListBatchAddObjectTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	tenancyId := getTenancyId(c)
	data, total, err := h.svc.ListBatchAddObjectTasks(tenancyId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list tasks")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

func (h *Handler) ListBatchAddObjectTaskDetail(c *gin.Context) {
	taskId, err := strconv.Atoi(c.Param("taskId"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid taskId")
		return
	}
	data, err := h.svc.ListBatchAddObjectTaskDetail(taskId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list task detail")
		return
	}
	utils.Success(c, data)
}
