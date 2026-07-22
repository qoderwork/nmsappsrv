package misc

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

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

// getTenantId extracts tenant_id from gin context as int.
func getTenantId(c *gin.Context) int {
	v, ok := c.Get("tenant_id")
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
	tenantId := getTenantId(c)
	data, total, err := h.svc.ListBackupRestoreTasksVo(tenantId, page, pageSize)
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
	tenantId := getTenantId(c)
	if err := h.svc.CreateBackupTask(&req, username, tenantId); err != nil {
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
	tenantId := getTenantId(c)
	if err := h.svc.CreateRestoreTask(&req, username, tenantId); err != nil {
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
	tenantId := getTenantId(c)
	data, total, err := h.svc.ListBatchConfigLogs(tenantId, page, pageSize)
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
	tenantId := middleware.GetTenantId(c)
	reports, err := h.svc.ListNorthReports(tenantId)
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
	tenantId := getTenantId(c)
	list, err := h.svc.ListRadius(tenantId)
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
	tenantId := getTenantId(c)
	data, total, err := h.svc.ListOperatorLogs(tenantId, page, pageSize)
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
	tenantId := getTenantId(c)
	if err := h.svc.BatchAddObject(&req, username, tenantId); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) ListBatchAddObjectTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	tenantId := getTenantId(c)
	data, total, err := h.svc.ListBatchAddObjectTasks(tenantId, page, pageSize)
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

// CheckSFTPCredentials verifies the SFTP username/password against the
// persisted ZTPSetting (plaintext, per the "Go 自管密钥" decision — see
// 2026-07-15 daily memory). Returns true only when both fields are set
// and match exactly. The ZTPSetting is re-read on every call so config
// changes take effect without a server restart.
func (h *Handler) CheckSFTPCredentials(username, password string) bool {
	if username == "" || password == "" {
		return false
	}
	setting, err := h.svc.GetZTPSetting()
	if err != nil || setting == nil {
		return false
	}
	if setting.SFTPUsername == nil || setting.SFTPPassword == nil {
		return false
	}
	return *setting.SFTPUsername == username && *setting.SFTPPassword == password
}

// ---------- AOS Management — TBG ----------

func (h *Handler) ListTBG(c *gin.Context) {
	var req ListTBGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	tenantId := middleware.GetTenantId(c)
	data, total, err := h.svc.ListTBGs(tenantId, &req)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list TBG")
		return
	}
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	utils.Paginated(c, data, total, page, pageSize)
}

func (h *Handler) AddTBG(c *gin.Context) {
	var req AddTBGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	tenantId := middleware.GetTenantId(c)
	tbg, err := h.svc.AddTBG(tenantId, &req)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to add TBG")
		return
	}
	utils.Success(c, tbg)
}

func (h *Handler) ModifyTBG(c *gin.Context) {
	var req ModifyTBGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.ModifyTBG(&req); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to modify TBG")
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) DeleteTBG(c *gin.Context) {
	var req DeleteTBGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.DeleteTBGs(req.Ids); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete TBG")
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) ImportTBGFile(c *gin.Context) {
	_, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "file is required")
		return
	}
	tenantId := middleware.GetTenantId(c)

	// Open the uploaded file
	src, err := header.Open()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to open uploaded file")
		return
	}
	defer src.Close()

	// Parse Excel file
	f, err := excelize.OpenReader(src)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid Excel file")
		return
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "failed to read Excel rows")
		return
	}

	var tbgs []TBG
	now := time.Now()
	for i, row := range rows {
		if i == 0 { // skip header
			continue
		}
		if len(row) < 3 {
			continue
		}
		port, _ := strconv.Atoi(row[2])
		name := row[0]
		ip := row[1]
		tbgs = append(tbgs, TBG{
			Name:       &name,
			IP:         &ip,
			Port:       &port,
			TenantId:  &tenantId,
			CreateTime: &now,
			UpdateTime: &now,
		})
	}

	count, err := h.svc.ImportTBGs(tenantId, tbgs)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to import TBG file")
		return
	}
	utils.Success(c, map[string]interface{}{"count": count})
}

func (h *Handler) DownloadTBGTemplate(c *gin.Context) {
	f := excelize.NewFile()
	sheet := "TBG"
	f.SetSheetName("Sheet1", sheet)
	f.SetCellValue(sheet, "A1", "Name")
	f.SetCellValue(sheet, "B1", "IP")
	f.SetCellValue(sheet, "C1", "Port")

	buf, err := f.WriteToBuffer()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to generate template")
		return
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=tbg_template.xlsx")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

// ---------- AOS Management — PSAPID ----------

func (h *Handler) ListPSAPID(c *gin.Context) {
	var req ListPSAPIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	tenantId := middleware.GetTenantId(c)
	data, total, err := h.svc.ListPSAPIDs(tenantId, &req)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list PSAP ID")
		return
	}
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	utils.Paginated(c, data, total, page, pageSize)
}

func (h *Handler) SyncPSAPID(c *gin.Context) {
	var req SyncPSAPIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	username := middleware.GetUsername(c)
	tenantId := middleware.GetTenantId(c)
	count, err := h.svc.SyncPSAPIDs(tenantId, username)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to sync PSAP ID")
		return
	}
	utils.Success(c, map[string]interface{}{"count": count})
}

func (h *Handler) ListPSAPIDSyncLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	data, total, err := h.svc.ListPSAPIDSyncLogs(page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list PSAP ID sync logs")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// ---------- AOS Management — SpatialFile ----------

func (h *Handler) ListSpatialFileMarkets(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	data, err := h.svc.ListSpatialFileMarkets(tenantId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list spatial file markets")
		return
	}
	utils.Success(c, data)
}

func (h *Handler) GetMarketCoordinates(c *gin.Context) {
	var req MarketCoordinateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	data, err := h.svc.GetMarketCoordinates(req.MarketId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to get market coordinates")
		return
	}
	utils.Success(c, data)
}
