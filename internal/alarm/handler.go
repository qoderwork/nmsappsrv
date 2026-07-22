package alarm

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// SyncAlarmsFunc is a function type for alarm sync.
type SyncAlarmsFunc func()

// syncAlarmsFn stores the alarm sync function injected from main.go.
var syncAlarmsFn SyncAlarmsFunc

// SetSyncAlarmsFunc sets the alarm sync function.
func SetSyncAlarmsFunc(fn SyncAlarmsFunc) {
	syncAlarmsFn = fn
}

// Handler exposes HTTP handlers for alarm-related endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// ---------------------------------------------------------------------------
// Alarm endpoints
// ---------------------------------------------------------------------------

// ListAlarms handles GET /alarms?page=1&pageSize=20&severity=&alarm_type=
func (h *Handler) ListAlarms(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	severity := c.Query("severity")
	alarmType, _ := strconv.Atoi(c.DefaultQuery("alarm_type", "0"))
	tenantId := middleware.GetTenantId(c)

	data, total, err := h.svc.ListAlarms(tenantId, severity, alarmType, page, pageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// GetAlarm handles GET /alarms/:id
func (h *Handler) GetAlarm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm id")
		return
	}

	alarm, err := h.svc.GetAlarm(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, alarm)
}

// ClearAlarm handles PUT /alarms/:id/clear
func (h *Handler) ClearAlarm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm id")
		return
	}

	if err := h.svc.ClearAlarm(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// DeleteAlarm handles DELETE /alarms/:id
// Hard-deletes a single alarm row. Mirrors Java deleteAlarm.
func (h *Handler) DeleteAlarm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm id")
		return
	}

	if err := h.svc.DeleteAlarm(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// BatchClearRequest is the JSON body for PUT /alarms/batch-clear.
type BatchClearRequest struct {
	AlarmIds  []int64 `json:"alarmIds" binding:"required"`
	ClearUser string  `json:"clearUser"`
}

// BatchClearAlarms handles PUT /alarms/batch-clear
func (h *Handler) BatchClearAlarms(c *gin.Context) {
	var req BatchClearRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body: alarmIds is required")
		return
	}

	cleared, notFound, err := h.svc.BatchClearAlarms(req.AlarmIds, req.ClearUser)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.OK(c, map[string]interface{}{
		"clearedCount": cleared,
		"notFoundIds":  notFound,
	})
}

// ConfirmAlarm handles POST /alarms/:id/confirm
func (h *Handler) ConfirmAlarm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm id")
		return
	}
	if err := h.svc.ConfirmAlarm(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// UnconfirmAlarm handles POST /alarms/:id/unconfirm
func (h *Handler) UnconfirmAlarm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm id")
		return
	}
	if err := h.svc.UnconfirmAlarm(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// GetSeverityCount handles GET /alarms/severity-count
func (h *Handler) GetSeverityCount(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	data, err := h.svc.GetSeverityCount(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// ---------------------------------------------------------------------------
// AlarmLibrary endpoints
// ---------------------------------------------------------------------------

// ListAlarmLibrary handles GET /alarm-library
func (h *Handler) ListAlarmLibrary(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)

	data, err := h.svc.ListAlarmLibrary(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// ---------------------------------------------------------------------------
// AlarmTemplate endpoints
// ---------------------------------------------------------------------------

// ListAlarmTemplates handles GET /alarm-templates
func (h *Handler) ListAlarmTemplates(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)

	data, err := h.svc.ListAlarmTemplates(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// GetAlarmTemplate handles GET /alarm-templates/:id
// Mirrors Java viewAlarmTemplate.
func (h *Handler) GetAlarmTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid template id")
		return
	}

	data, err := h.svc.GetAlarmTemplate(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// CreateAlarmTemplate handles POST /alarm-templates
func (h *Handler) CreateAlarmTemplate(c *gin.Context) {
	var t AlarmTemplate
	if err := c.ShouldBindJSON(&t); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateAlarmTemplate(&t); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, &t)
}

// UpdateAlarmTemplate handles PUT /alarm-templates/:id
func (h *Handler) UpdateAlarmTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm template id")
		return
	}

	var t AlarmTemplate
	if err := c.ShouldBindJSON(&t); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	t.Id = id

	if err := h.svc.UpdateAlarmTemplate(&t); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, &t)
}

// DeleteAlarmTemplate handles DELETE /alarm-templates/:id
// Mirrors Java deleteAlarmTemplate.
func (h *Handler) DeleteAlarmTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm template id")
		return
	}

	if err := h.svc.DeleteAlarmTemplate(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// UpdateAlarmTemplateEmailNotification handles PUT /alarm-templates/:id/email-notification
// Body: {"enable": true|false}. Mirrors Java updateEmailNotificationEnableInTemplate.
func (h *Handler) UpdateAlarmTemplateEmailNotification(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm template id")
		return
	}
	var body struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.UpdateAlarmTemplateEmailNotification(id, body.Enable); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// AlarmFilter endpoints
// ---------------------------------------------------------------------------

// ListAlarmFilters handles GET /alarm-filters
func (h *Handler) ListAlarmFilters(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)

	data, err := h.svc.ListAlarmFilters(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// GetAlarmFilter handles GET /alarm-filters/:id
// Mirrors Java viewAlarmFilterTask.
func (h *Handler) GetAlarmFilter(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm filter id")
		return
	}

	data, err := h.svc.GetAlarmFilter(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// CreateAlarmFilter handles POST /alarm-filters
func (h *Handler) CreateAlarmFilter(c *gin.Context) {
	var f AlarmFilter
	if err := c.ShouldBindJSON(&f); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateAlarmFilter(&f); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, &f)
}

// UpdateAlarmFilter handles PUT /alarm-filters/:id
func (h *Handler) UpdateAlarmFilter(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm filter id")
		return
	}

	var f AlarmFilter
	if err := c.ShouldBindJSON(&f); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	f.Id = id

	if err := h.svc.UpdateAlarmFilter(&f); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, &f)
}

// DeleteAlarmFilter handles DELETE /alarm-filters/:id
func (h *Handler) DeleteAlarmFilter(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm filter id")
		return
	}

	if err := h.svc.DeleteAlarmFilter(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ToggleAlarmFilterEnable handles PUT /alarm-filters/:id/enable
// Body: {"enable": true|false}. Mirrors Java enableAlarmFilterTask /
// disableAlarmFilterTask (one endpoint with a body flag rather than two).
func (h *Handler) ToggleAlarmFilterEnable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm filter id")
		return
	}
	var body struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.ToggleAlarmFilterEnable(id, body.Enable); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// AlarmLibrary – import / template
// ---------------------------------------------------------------------------

// alarmLibraryHeaders defines the column order for the import template.
var alarmLibraryHeaders = []string{
	"Alarm Identifier",
	"Probable Cause",
	"Severity",
	"Event Type",
	"Explanation",
	"Specific Problem",
	"Alarm Source",
}

// ImportAlarmLibrary handles POST /alarm-library/import
//
// Accepts a multipart file upload (field name "file") containing an Excel
// workbook whose first sheet has a header row followed by data rows.
func (h *Handler) ImportAlarmLibrary(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "missing or invalid file upload")
		return
	}
	defer file.Close()

	f, err := excelize.OpenReader(file)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "failed to open Excel file: "+err.Error())
		return
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		utils.Error(c, http.StatusBadRequest, "Excel file has no sheets")
		return
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "failed to read sheet rows: "+err.Error())
		return
	}
	if len(rows) < 2 {
		utils.Error(c, http.StatusBadRequest, "Excel file must have a header row and at least one data row")
		return
	}

	// Build a column-index map from the header row so column order is flexible.
	headerRow := rows[0]
	colIdx := make(map[string]int, len(headerRow))
	for i, hdr := range headerRow {
		colIdx[hdr] = i
	}

	strPtr := func(row []string, col string) *string {
		idx, ok := colIdx[col]
		if !ok || idx >= len(row) {
			return nil
		}
		v := row[idx]
		return &v
	}

	var items []AlarmLibrary
	for _, row := range rows[1:] {
		// Skip completely empty rows.
		allEmpty := true
		for _, cell := range row {
			if cell != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			continue
		}
		items = append(items, AlarmLibrary{
			AlarmIdentifier: strPtr(row, "Alarm Identifier"),
			ProbableCause:   strPtr(row, "Probable Cause"),
			Severity:        strPtr(row, "Severity"),
			EventType:       strPtr(row, "Event Type"),
			Explanation:     strPtr(row, "Explanation"),
			SpecificProblem: strPtr(row, "Specific Problem"),
			AlarmSource:     strPtr(row, "Alarm Source"),
		})
	}

	if len(items) == 0 {
		utils.Error(c, http.StatusBadRequest, "no valid data rows found in the uploaded file")
		return
	}

	tenantId := middleware.GetTenantId(c)
	imported, err := h.svc.ImportAlarmLibrary(tenantId, items)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, map[string]interface{}{"importedCount": imported})
}

// DownloadAlarmLibraryTemplate handles GET /alarm-library/template
//
// Generates an Excel workbook with the alarm library column headers and serves
// it as a downloadable file.
func (h *Handler) DownloadAlarmLibraryTemplate(c *gin.Context) {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "AlarmLibrary"
	index, _ := f.NewSheet(sheetName)
	f.SetActiveSheet(index)
	// Remove the default "Sheet1".
	f.DeleteSheet("Sheet1")

	for i, header := range alarmLibraryHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	fileName := "alarm_library_template.xlsx"
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	if err := f.Write(c.Writer); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to write template file")
	}
}

// ---------------------------------------------------------------------------
// AlarmSyncConfig
// ---------------------------------------------------------------------------

// GetAlarmSyncConfig handles GET /alarm-sync-config
func (h *Handler) GetAlarmSyncConfig(c *gin.Context) {
	cfg, err := h.svc.GetAlarmSyncConfig()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, cfg)
}

// UpdateAlarmSyncConfig handles PUT /alarm-sync-config
func (h *Handler) UpdateAlarmSyncConfig(c *gin.Context) {
	var cfg AlarmSyncConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.UpdateAlarmSyncConfig(&cfg); err != nil {
		utils.HandleError(c, err)
		return
	}

	// Return the merged config so the UI can refresh.
	updated, err := h.svc.GetAlarmSyncConfig()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, updated)
}

// SyncAlarms handles POST /alarms/sync
// Manually triggers alarm sync for all online enb devices.
func (h *Handler) SyncAlarms(c *gin.Context) {
	if syncAlarmsFn == nil {
		utils.Error(c, http.StatusInternalServerError, "alarm sync not initialized")
		return
	}
	syncAlarmsFn()
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// Alarm – comment
// ---------------------------------------------------------------------------

// AddCommentForAlarm handles POST /alarms/:id/comment
func (h *Handler) AddCommentForAlarm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm id")
		return
	}

	var req AddCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body: comment is required")
		return
	}

	if err := h.svc.AddCommentForAlarm(id, req.Comment); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// Alarm Statistic Top-N
// ---------------------------------------------------------------------------

// QueryAlarmStatisticTopN handles GET /alarms/statistic/top-n
func (h *Handler) QueryAlarmStatisticTopN(c *gin.Context) {
	topN, _ := strconv.Atoi(c.DefaultQuery("topN", "10"))
	if topN < 1 {
		topN = 10
	}

	var startTime, endTime *time.Time
	if v := c.Query("startTime"); v != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
			startTime = &t
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			startTime = &t
		}
	}
	if v := c.Query("endTime"); v != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
			endTime = &t
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			endTime = &t
		}
	}

	data, err := h.svc.QueryAlarmStatisticTopN(topN, startTime, endTime)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// ---------------------------------------------------------------------------
// Email Notification Config
// ---------------------------------------------------------------------------

// ListEmailNotificationConfig handles GET /email-notification/config
func (h *Handler) ListEmailNotificationConfig(c *gin.Context) {
	cfg, err := h.svc.GetEmailNotificationConfig()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, cfg)
}

// UpdateEmailNotificationConfig handles PUT /email-notification/config
func (h *Handler) UpdateEmailNotificationConfig(c *gin.Context) {
	var cfg EmailNotificationConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.UpdateEmailNotificationConfig(&cfg); err != nil {
		utils.HandleError(c, err)
		return
	}

	// Return the merged config so the UI can refresh.
	updated, err := h.svc.GetEmailNotificationConfig()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, updated)
}

// QueryAlarmStatisticResult handles POST /alarms/statistic
// Mirrors Java queryAlarmStatisticResult.
func (h *Handler) QueryAlarmStatisticResult(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	result, err := h.svc.QueryAlarmStatisticResult(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, result)
}

// DeleteAlarmLibrary handles DELETE /alarm-library/:id
// Mirrors Java deleteAlarmLibrary.
func (h *Handler) DeleteAlarmLibrary(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm library id")
		return
	}
	if err := h.svc.DeleteAlarmLibrary(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ListActiveAlarmProbableCause handles GET /alarms/active-probable-causes
// Mirrors Java listActiveAlarmProbableCause.
func (h *Handler) ListActiveAlarmProbableCause(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	causes, err := h.svc.ListActiveAlarmProbableCause(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, causes)
}

// GetAlarmEventType handles POST /alarms/event-type
// Mirrors Java getAlarmEventType.
func (h *Handler) GetAlarmEventType(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	types, err := h.svc.GetAlarmEventType(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, types)
}

// ListEmailNoticeResult handles POST /alarms/email-notice-results
// Body: {query: {alarmTemplateId?, emailSubject?}, page, pageSize}.
// Mirrors Java listEmailNoticeResult.
func (h *Handler) ListEmailNoticeResult(c *gin.Context) {
	var body struct {
		Query    EmailNoticeResultQuery `json:"query"`
		Page     int                    `json:"page"`
		PageSize int                    `json:"pageSize"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}
	data, total, err := h.svc.ListEmailNoticeResult(body.Query, body.Page, body.PageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Paginated(c, data, total, body.Page, body.PageSize)
}
