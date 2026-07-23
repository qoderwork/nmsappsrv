package alarm

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/dto"
	"nmsappsrv/pkg/enums"
	"nmsappsrv/pkg/logger"

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
// Helpers: write dto.Result envelope + always HTTP 200 (Java contract)
// ---------------------------------------------------------------------------

func writeOK[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, dto.OKData(data))
}

func writeFail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, dto.Failure(code, msg))
}

func writeFailInvalid(c *gin.Context, msg string) {
	writeFail(c, enums.BAD_REQUEST.Code, msg)
}

func writeFailService(c *gin.Context, err error) {
	var appErr *apperror.AppError
	if err == nil {
		writeFail(c, enums.ERROR.Code, "nil service error")
		return
	}
	if errors.As(err, &appErr) {
		writeFail(c, appErr.StatusCode, appErr.Message)
		return
	}
	logger.Errorf("unhandled service error in %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	writeFail(c, enums.ERROR.Code, "internal server error")
}

// ptr is a tiny helper to lift a value into *T when the service layer needs a pointer.
func ptr[T any](v T) *T { return &v }

// intPtrZeroNil returns nil when v is zero, otherwise a pointer to v.
// Helps bridge empty-JSON-zero to nil for optional service params.
func intPtrZeroNil(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

// ---------------------------------------------------------------------------
// Alarm state transitions + list
// ---------------------------------------------------------------------------

// ListAlarms handles POST /api/v2/listAlarm
// Java: listAlarm(@RequestBody RequestDataDTO<AlarmQuery, Object>)
// → Result<Page<ListAlarmVO>>
func (h *Handler) ListAlarms(c *gin.Context) {
	var req dto.RequestDataDTO[AlarmQuery, any]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	req.Normalize()

	severity := ""
	if req.Query.Severity != nil {
		severity = *req.Query.Severity
	}
	alarmType := 0
	if req.Query.AlarmType != nil {
		if v, err := parseIntOptional(*req.Query.AlarmType); err == nil {
			alarmType = v
		}
	}
	tenantId := middleware.GetTenantId(c)

	list, total, err := h.svc.ListAlarms(tenantId, severity, alarmType, req.Page.PageNumber, req.Page.PageSize)
	if err != nil {
		writeFailService(c, err)
		return
	}
	writeOK(c, NewSpringDataPage(list, total, req.Page.PageNumber, req.Page.PageSize))
}

// parseIntOptional attempts to parse a numeric string. Returns the value on
// success, or an error. Empty string yields 0 with no error, matching the
// prior behaviour of strconv.Atoi(DefaultQuery(..., "0")).
func parseIntOptional(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("not a number: %q", s)
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, nil
}

// ConfirmAlarm handles POST /api/v2/confirmAlarm
// Java: confirmAlarm(RequestDataDTO<Object, ConfirmAlarmDTO>)
func (h *Handler) ConfirmAlarm(c *gin.Context) {
	var req dto.RequestDataDTO[any, ConfirmAlarmDTO]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Data.Index == nil {
		writeFailInvalid(c, "data.index is required")
		return
	}
	if err := h.svc.ConfirmAlarm(*req.Data.Index); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK[any](c, nil)
}

// UnconfirmAlarm handles POST /api/v2/unconfirmAlarm
func (h *Handler) UnconfirmAlarm(c *gin.Context) {
	var req dto.RequestDataDTO[any, ConfirmAlarmDTO]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Data.Index == nil {
		writeFailInvalid(c, "data.index is required")
		return
	}
	if err := h.svc.UnconfirmAlarm(*req.Data.Index); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK[any](c, nil)
}

// ClearAlarm handles POST /api/v2/clearAlarm
func (h *Handler) ClearAlarm(c *gin.Context) {
	var req dto.RequestDataDTO[any, ConfirmAlarmDTO]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Data.Index == nil {
		writeFailInvalid(c, "data.index is required")
		return
	}
	if err := h.svc.ClearAlarm(*req.Data.Index); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK[any](c, nil)
}

// DeleteAlarm handles POST /api/v2/deleteAlarm — hard-deletes one alarm row.
// Java: deleteAlarm(RequestDataDTO<Object, ConfirmAlarmDTO>)
func (h *Handler) DeleteAlarm(c *gin.Context) {
	var req dto.RequestDataDTO[any, ConfirmAlarmDTO]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Data.Index == nil {
		writeFailInvalid(c, "data.index is required")
		return
	}
	if err := h.svc.DeleteAlarm(*req.Data.Index); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK[any](c, nil)
}

// ---------------------------------------------------------------------------
// Alarm statistics + counts
// ---------------------------------------------------------------------------

// QueryAlarmStatisticResult handles POST /api/v2/queryAlarmStatisticResult
// Java: queryAlarmStatisticResult(RequestDataDTO<AlarmStatisticResultQuery, Object>)
func (h *Handler) QueryAlarmStatisticResult(c *gin.Context) {
	var req dto.RequestDataDTO[AlarmStatisticResultQuery, any]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	tenantId := middleware.GetTenantId(c)
	result, err := h.svc.QueryAlarmStatisticResult(tenantId)
	if err != nil {
		writeFailService(c, err)
		return
	}
	writeOK(c, []*AlarmStatisticResult{result})
}

// QueryAlarmStatisticTopN handles POST /api/v2/queryAlarmStatisticTopN
// Java: queryAlarmStatisticTopN(RequestDataDTO<AlarmStatisticTopNQuery, Object>)
func (h *Handler) QueryAlarmStatisticTopN(c *gin.Context) {
	var req dto.RequestDataDTO[AlarmStatisticTopNQuery, any]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	topN := 10
	var startTime, endTime *time.Time
	if req.Query.StartTime != nil {
		t := time.UnixMilli(*req.Query.StartTime)
		startTime = &t
	}
	if req.Query.EndTime != nil {
		t := time.UnixMilli(*req.Query.EndTime)
		endTime = &t
	}
	list, err := h.svc.QueryAlarmStatisticTopN(topN, startTime, endTime)
	if err != nil {
		writeFailService(c, err)
		return
	}
	writeOK(c, list)
}

// GetSeverityCount handles POST /api/v2/getCountOfSeverity
func (h *Handler) GetSeverityCount(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	data, err := h.svc.GetSeverityCount(tenantId)
	if err != nil {
		writeFailService(c, err)
		return
	}
	writeOK(c, data)
}

// ---------------------------------------------------------------------------
// Alarm filter tasks (7 endpoints)
// ---------------------------------------------------------------------------

// AddAlarmFilterTask handles POST /api/v2/addAlarmFilterTask
func (h *Handler) AddAlarmFilterTask(c *gin.Context) {
	var req dto.RequestDataDTO[any, AddAlarmFilterTaskDTO]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	f := AddAlarmFilterDTOToEntity(&req.Data)
	f.TenantId = intPtr(middleware.GetTenantId(c))
	if err := h.svc.CreateAlarmFilter(f); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK(c, f)
}

// ListAlarmFilters handles POST /api/v2/listAlarmFilterTask
func (h *Handler) ListAlarmFilters(c *gin.Context) {
	var req dto.RequestDataDTO[ListAlarmFilterTaskQuery, any]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	req.Normalize()
	tenantId := middleware.GetTenantId(c)
	list, err := h.svc.ListAlarmFilters(tenantId)
	if err != nil {
		writeFailService(c, err)
		return
	}
	total := int64(len(list))
	writeOK(c, NewSpringDataPage(list, total, req.Page.PageNumber, req.Page.PageSize))
}

// UpdateAlarmFilter handles POST /api/v2/updateAlarmFilterTask
func (h *Handler) UpdateAlarmFilter(c *gin.Context) {
	var req dto.RequestDataDTO[any, UpdateAlarmFilterTaskDTO]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Data.Id == nil {
		writeFailInvalid(c, "data.id is required")
		return
	}
	f := AddAlarmFilterDTOToEntity(&req.Data.AddAlarmFilterTaskDTO)
	f.Id = *req.Data.Id
	f.TenantId = intPtr(middleware.GetTenantId(c))
	if err := h.svc.UpdateAlarmFilter(f); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK(c, f)
}

// DeleteAlarmFilter handles POST /api/v2/deleteAlarmFilterTask
func (h *Handler) DeleteAlarmFilter(c *gin.Context) {
	var req dto.RequestDataDTO[any, IntegerIdDto]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Data.Id == nil {
		writeFailInvalid(c, "data.id is required")
		return
	}
	if err := h.svc.DeleteAlarmFilter(*req.Data.Id); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK[any](c, nil)
}

// GetAlarmFilter handles POST /api/v2/viewAlarmFilterTask
func (h *Handler) GetAlarmFilter(c *gin.Context) {
	var req dto.RequestDataDTO[IntegerIdDto, any]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Query.Id == nil {
		writeFailInvalid(c, "query.id is required")
		return
	}
	f, err := h.svc.GetAlarmFilter(*req.Query.Id)
	if err != nil {
		writeFailService(c, err)
		return
	}
	writeOK(c, f)
}

// EnableAlarmFilterTask handles POST /api/v2/enableAlarmFilterTask
func (h *Handler) EnableAlarmFilterTask(c *gin.Context) {
	var req dto.RequestDataDTO[any, IntegerIdDto]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Data.Id == nil {
		writeFailInvalid(c, "data.id is required")
		return
	}
	if err := h.svc.ToggleAlarmFilterEnable(*req.Data.Id, true); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK[any](c, nil)
}

// DisableAlarmFilterTask handles POST /api/v2/disableAlarmFilterTask
func (h *Handler) DisableAlarmFilterTask(c *gin.Context) {
	var req dto.RequestDataDTO[any, IntegerIdDto]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Data.Id == nil {
		writeFailInvalid(c, "data.id is required")
		return
	}
	if err := h.svc.ToggleAlarmFilterEnable(*req.Data.Id, false); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK[any](c, nil)
}

// ---------------------------------------------------------------------------
// Alarm library (import / list / delete) + template download
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

// ImportAlarmLibrary handles POST /api/v2/importAlarmLibrary (multipart file)
// NOTE: Java uses @RequestParam("file") MultipartFile — this is the only
// non-RequestDataDTO endpoint in AlarmManagementController (file upload
// cannot travel inside JSON envelope).
func (h *Handler) ImportAlarmLibrary(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		writeFailInvalid(c, "missing or invalid file upload: "+err.Error())
		return
	}
	defer file.Close()

	f, err := excelize.OpenReader(file)
	if err != nil {
		writeFailInvalid(c, "failed to open Excel file: "+err.Error())
		return
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		writeFailInvalid(c, "Excel file has no sheets")
		return
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		writeFailInvalid(c, "failed to read sheet rows: "+err.Error())
		return
	}
	if len(rows) < 2 {
		writeFailInvalid(c, "Excel file must have a header row and at least one data row")
		return
	}

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
		writeFailInvalid(c, "no valid data rows found in the uploaded file")
		return
	}

	tenantId := middleware.GetTenantId(c)
	imported, err := h.svc.ImportAlarmLibrary(tenantId, items)
	if err != nil {
		writeFailService(c, err)
		return
	}
	writeOK(c, map[string]interface{}{"importedCount": imported})
}

// DeleteAlarmLibrary handles POST /api/v2/deleteAlarmLibrary
func (h *Handler) DeleteAlarmLibrary(c *gin.Context) {
	var req dto.RequestDataDTO[any, IntegerIdDto]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Data.Id == nil {
		writeFailInvalid(c, "data.id is required")
		return
	}
	if err := h.svc.DeleteAlarmLibrary(*req.Data.Id); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK[any](c, nil)
}

// ListAlarmLibrary handles POST /api/v2/listAlarmLibrary
func (h *Handler) ListAlarmLibrary(c *gin.Context) {
	var req dto.RequestDataDTO[AlarmLibraryQuery, any]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	req.Normalize()
	tenantId := middleware.GetTenantId(c)
	list, err := h.svc.ListAlarmLibrary(tenantId)
	if err != nil {
		writeFailService(c, err)
		return
	}
	total := int64(len(list))
	writeOK(c, NewSpringDataPage(list, total, req.Page.PageNumber, req.Page.PageSize))
}

// DownloadAlarmLibraryTemplate handles GET /api/v2/downloadAlarmLibraryTemplate
func (h *Handler) DownloadAlarmLibraryTemplate(c *gin.Context) {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "AlarmLibrary"
	index, _ := f.NewSheet(sheetName)
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	for i, header := range alarmLibraryHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	fileName := "alarm_library_template.xlsx"
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	if err := f.Write(c.Writer); err != nil {
		writeFail(c, enums.ERROR.Code, "failed to write template file: "+err.Error())
	}
}

// ---------------------------------------------------------------------------
// Alarm templates (6 endpoints) + email-notification enable per template
// ---------------------------------------------------------------------------

// CreateAlarmTemplate handles POST /api/v2/addAlarmTemplate
func (h *Handler) CreateAlarmTemplate(c *gin.Context) {
	var req dto.RequestDataDTO[any, AddAlarmTemplateDTO]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	t := AddAlarmTemplateDTOToEntity(&req.Data)
	t.TenantId = intPtr(middleware.GetTenantId(c))
	if err := h.svc.CreateAlarmTemplate(t); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK(c, IntegerIdDto{Id: &t.Id})
}

// ListAlarmTemplates handles POST /api/v2/listAlarmTemplate
func (h *Handler) ListAlarmTemplates(c *gin.Context) {
	var req dto.RequestDataDTO[any, any]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	req.Normalize()
	tenantId := middleware.GetTenantId(c)
	list, err := h.svc.ListAlarmTemplates(tenantId)
	if err != nil {
		writeFailService(c, err)
		return
	}
	total := int64(len(list))
	writeOK(c, NewSpringDataPage(list, total, req.Page.PageNumber, req.Page.PageSize))
}

// GetAlarmTemplate handles POST /api/v2/viewAlarmTemplate
func (h *Handler) GetAlarmTemplate(c *gin.Context) {
	var req dto.RequestDataDTO[IntegerIdDto, any]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Query.Id == nil {
		writeFailInvalid(c, "query.id is required")
		return
	}
	t, err := h.svc.GetAlarmTemplate(*req.Query.Id)
	if err != nil {
		writeFailService(c, err)
		return
	}
	writeOK(c, t)
}

// UpdateAlarmTemplate handles POST /api/v2/updateAlarmTemplate
func (h *Handler) UpdateAlarmTemplate(c *gin.Context) {
	var req dto.RequestDataDTO[any, UpdateAlarmTemplateDTO]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Data.Id == nil {
		writeFailInvalid(c, "data.id is required")
		return
	}
	t := UpdateAlarmTemplateDTOToEntity(&req.Data)
	t.TenantId = intPtr(middleware.GetTenantId(c))
	if err := h.svc.UpdateAlarmTemplate(t); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK[any](c, nil)
}

// DeleteAlarmTemplate handles POST /api/v2/deleteAlarmTemplate
func (h *Handler) DeleteAlarmTemplate(c *gin.Context) {
	var req dto.RequestDataDTO[any, IntegerIdDto]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Data.Id == nil {
		writeFailInvalid(c, "data.id is required")
		return
	}
	if err := h.svc.DeleteAlarmTemplate(*req.Data.Id); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK[any](c, nil)
}

// UpdateEmailNotificationEnableInTemplate handles POST /api/v2/updateEmailNotificationEnableInTemplate
func (h *Handler) UpdateEmailNotificationEnableInTemplate(c *gin.Context) {
	var req dto.RequestDataDTO[any, UpdateEmailNotificationEnableInTemplateDTO]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Data.TemplateId == nil {
		writeFailInvalid(c, "data.templateId is required")
		return
	}
	enable := false
	if req.Data.EnableEmailNotification != nil {
		enable = *req.Data.EnableEmailNotification
	}
	if err := h.svc.UpdateAlarmTemplateEmailNotification(*req.Data.TemplateId, enable); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK[any](c, nil)
}

// ---------------------------------------------------------------------------
// Email notification config + email notice result
// ---------------------------------------------------------------------------

// UpdateEmailNotificationConfig handles POST /api/v2/updateEmailNotificationConfig
// Java: updateEmailNotificationConfig(RequestDataDTO<Object, EmailNotificationConfigDTO>)
func (h *Handler) UpdateEmailNotificationConfig(c *gin.Context) {
	var req dto.RequestDataDTO[any, EmailNotificationConfigDTO]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	// Shape transformation: Java EmailNotificationConfigDTO(emailNotification, defaultRecipients)
	// → Go EmailNotificationConfig(enabled, smtpHost...recipients). We persist what Java sends:
	// enabled <- emailNotification, defaultRecipients (CSV) <- recipients list (1 string element).
	enabled := req.Data.EmailNotification
	var recipients []string
	if req.Data.DefaultRecipients != nil && *req.Data.DefaultRecipients != "" {
		recipients = []string{*req.Data.DefaultRecipients}
	}
	cfg := &EmailNotificationConfig{
		Enabled:    enabled,
		Recipients: recipients,
	}
	if err := h.svc.UpdateEmailNotificationConfig(cfg); err != nil {
		writeFailService(c, err)
		return
	}
	updated, err := h.svc.GetEmailNotificationConfig()
	if err != nil {
		writeFailService(c, err)
		return
	}
	out := EmailNotificationConfigDTO{}
	if updated.Enabled != nil {
		out.EmailNotification = updated.Enabled
	}
	if len(updated.Recipients) > 0 {
		// Join the CSV back to a single string for Java frontend.
		s := ""
		for i, r := range updated.Recipients {
			if i > 0 {
				s += ","
			}
			s += r
		}
		out.DefaultRecipients = &s
	}
	writeOK(c, out)
}

// ListEmailNotificationConfig handles POST /api/v2/listEmailNotificationConfig
// Java: Result<EmailNotificationConfigDTO> with no RequestDataDTO envelope.
func (h *Handler) ListEmailNotificationConfig(c *gin.Context) {
	cfg, err := h.svc.GetEmailNotificationConfig()
	if err != nil {
		writeFailService(c, err)
		return
	}
	out := EmailNotificationConfigDTO{}
	if cfg.Enabled != nil {
		out.EmailNotification = cfg.Enabled
	}
	if len(cfg.Recipients) > 0 {
		s := ""
		for i, r := range cfg.Recipients {
			if i > 0 {
				s += ","
			}
			s += r
		}
		out.DefaultRecipients = &s
	}
	writeOK(c, out)
}

// ListEmailNoticeResult handles POST /api/v2/listEmailNoticeResult
// Java: listEmailNoticeResult(RequestDataDTO<ListEmailNoticeResultQuery, Object>)
func (h *Handler) ListEmailNoticeResult(c *gin.Context) {
	var req dto.RequestDataDTO[ListEmailNoticeResultQuery, any]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	req.Normalize()
	q := EmailNoticeResultQuery{}
	if req.Query.AlarmTemplateId != nil {
		q.AlarmTemplateId = req.Query.AlarmTemplateId
	}
	if req.Query.EmailSubject != nil {
		q.EmailSubject = *req.Query.EmailSubject
	}
	list, total, err := h.svc.ListEmailNoticeResult(q, req.Page.PageNumber, req.Page.PageSize)
	if err != nil {
		writeFailService(c, err)
		return
	}
	writeOK(c, NewSpringDataPage(list, total, req.Page.PageNumber, req.Page.PageSize))
}

// ---------------------------------------------------------------------------
// Misc alarm helpers
// ---------------------------------------------------------------------------

// ListActiveAlarmProbableCause handles POST /api/v2/listActiveAlarmProbableCause
func (h *Handler) ListActiveAlarmProbableCause(c *gin.Context) {
	var req dto.RequestDataDTO[any, any]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	tenantId := middleware.GetTenantId(c)
	causes, err := h.svc.ListActiveAlarmProbableCause(tenantId)
	if err != nil {
		writeFailService(c, err)
		return
	}
	writeOK(c, causes)
}

// GetAlarmEventType handles POST /api/v2/getAlarmEventType
// Java: no envelope, returns Result<List<String>>
func (h *Handler) GetAlarmEventType(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	types, err := h.svc.GetAlarmEventType(tenantId)
	if err != nil {
		writeFailService(c, err)
		return
	}
	writeOK(c, types)
}

// GetAlarmSyncConfig handles POST /api/v2/getAlarmSyncConfig
// Java: Result<GetAlarmSyncConfigVO>
func (h *Handler) GetAlarmSyncConfig(c *gin.Context) {
	cfg, err := h.svc.GetAlarmSyncConfig()
	if err != nil {
		writeFailService(c, err)
		return
	}
	vo := GetAlarmSyncConfigVO{}
	if cfg.Enabled != nil {
		vo.Enable = cfg.Enabled
	}
	if cfg.SyncInterval != nil {
		vo.Period = cfg.SyncInterval
	}
	writeOK(c, vo)
}

// UpdateAlarmSyncConfig handles POST /api/v2/updateAlarmSyncConfig
// Java: updateAlarmSyncConfig(RequestDataDTO<Object, GetAlarmSyncConfigVO>)
func (h *Handler) UpdateAlarmSyncConfig(c *gin.Context) {
	var req dto.RequestDataDTO[any, GetAlarmSyncConfigVO]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	cfg := &AlarmSyncConfig{
		Enabled:      req.Data.Enable,
		SyncInterval: req.Data.Period,
	}
	if err := h.svc.UpdateAlarmSyncConfig(cfg); err != nil {
		writeFailService(c, err)
		return
	}
	updated, err := h.svc.GetAlarmSyncConfig()
	if err != nil {
		writeFailService(c, err)
		return
	}
	out := GetAlarmSyncConfigVO{}
	if updated.Enabled != nil {
		out.Enable = updated.Enabled
	}
	if updated.SyncInterval != nil {
		out.Period = updated.SyncInterval
	}
	writeOK(c, out)
}

// AddCommentForAlarm handles POST /api/v2/addCommentForAlarm
func (h *Handler) AddCommentForAlarm(c *gin.Context) {
	var req dto.RequestDataDTO[any, AddCommentForAlarmDTO]
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailInvalid(c, "invalid request body: "+err.Error())
		return
	}
	if req.Data.AlarmIndex == nil {
		writeFailInvalid(c, "data.alarmIndex is required")
		return
	}
	if req.Data.Comment == nil || *req.Data.Comment == "" {
		writeFailInvalid(c, "data.comment is required")
		return
	}
	if err := h.svc.AddCommentForAlarm(*req.Data.AlarmIndex, *req.Data.Comment); err != nil {
		writeFailService(c, err)
		return
	}
	writeOK[any](c, nil)
}

// ---------------------------------------------------------------------------
// DTO → entity bridge helpers
// ---------------------------------------------------------------------------

func intPtr(v int) *int { return &v }

// AddAlarmFilterDTOToEntity converts the wire DTO to the DB entity.
// Multi-value string / int64 fields are joined with "," because the core
// entity columns are legacy comma-separated varchars / longtexts.
func AddAlarmFilterDTOToEntity(d *AddAlarmFilterTaskDTO) *AlarmFilter {
	f := &AlarmFilter{
		FilterRuleName:         d.FilterRuleName,
		Enable:                 d.Enable,
		ExecutionAction:        d.ExecutionAction,
		ExecutionOnAllAlarm:    d.ExecutionOnAllAlarm,
		ExecuteOnAllBaseStation: d.ExecuteOnAllBaseStation,
		ExecuteOnAllCPE:        d.ExecuteOnAllCPE,
	}
	if len(d.AlarmSources) > 0 {
		s := joinStr(d.AlarmSources)
		f.AlarmSources = &s
	}
	if d.StartTime != nil {
		t := time.UnixMilli(*d.StartTime)
		f.StartTime = &t
	}
	if d.EndTime != nil {
		t := time.UnixMilli(*d.EndTime)
		f.EndTime = &t
	}
	if len(d.BaseStationIds) > 0 {
		s := joinInt64(d.BaseStationIds)
		f.BaseStationIds = &s
	}
	if len(d.CpeIds) > 0 {
		s := joinInt64(d.CpeIds)
		f.CpeIds = &s
	}
	if len(d.BaseStationDeviceGroupIds) > 0 {
		s := joinStr(d.BaseStationDeviceGroupIds)
		f.BaseStationDeviceGroupIds = &s
	}
	if len(d.CpeDeviceGroupIds) > 0 {
		s := joinStr(d.CpeDeviceGroupIds)
		f.CpeDeviceGroupIds = &s
	}
	if len(d.AlarmIds) > 0 {
		s := joinStr(d.AlarmIds)
		f.AlarmIds = &s
	}
	return f
}

func AddAlarmTemplateDTOToEntity(d *AddAlarmTemplateDTO) *AlarmTemplate {
	t := &AlarmTemplate{
		Name:                           d.Name,
		Description:                    d.Description,
		ExecuteOnAllBaseStation:        d.ExecuteOnAllBaseStation,
		ExecuteOnAllCPE:                d.ExecuteOnAllCPE,
		ExecuteOnAllAlarm:              d.ExecuteOnAllAlarm,
		EnableEmailNotification:        d.EnableEmailNotification,
		ToleranceDuration:              d.ToleranceDuration,
		Interval_:                      d.Interval,
		EnableNotifyDefaultRecipients:  d.EnableNotifyDefaultRecipients,
		Emails:                         d.Emails,
	}
	if len(d.BaseStationIds) > 0 {
		s := joinInt64(d.BaseStationIds)
		t.BaseStationIds = &s
	}
	if len(d.CpeIds) > 0 {
		s := joinInt64(d.CpeIds)
		t.CpeIds = &s
	}
	if len(d.BaseStationDeviceGroupIds) > 0 {
		s := joinStr(d.BaseStationDeviceGroupIds)
		t.BaseStationDeviceGroupIds = &s
	}
	if len(d.CpeDeviceGroupIds) > 0 {
		s := joinStr(d.CpeDeviceGroupIds)
		t.CpeDeviceGroupIds = &s
	}
	if len(d.AlarmIds) > 0 {
		s := joinStr(d.AlarmIds)
		t.AlarmIds = &s
	}
	if len(d.AlarmSources) > 0 {
		s := joinStr(d.AlarmSources)
		t.AlarmSources = &s
	}
	return t
}

func UpdateAlarmTemplateDTOToEntity(d *UpdateAlarmTemplateDTO) *AlarmTemplate {
	inner := AddAlarmTemplateDTO{
		Name:                           d.Name,
		Description:                    d.Description,
		ExecuteOnAllBaseStation:        d.ExecuteOnAllBaseStation,
		ExecuteOnAllCPE:                d.ExecuteOnAllCPE,
		BaseStationIds:                 d.BaseStationIds,
		CpeIds:                         d.CpeIds,
		BaseStationDeviceGroupIds:      d.BaseStationDeviceGroupIds,
		CpeDeviceGroupIds:              d.CpeDeviceGroupIds,
		ExecuteOnAllAlarm:              d.ExecuteOnAllAlarm,
		AlarmIds:                       d.AlarmIds,
		EnableEmailNotification:        d.EnableEmailNotification,
		ToleranceDuration:              d.ToleranceDuration,
		Interval:                       d.Interval,
		EnableNotifyDefaultRecipients:  d.EnableNotifyDefaultRecipients,
		AlarmSources:                   d.AlarmSources,
		Emails:                         d.Emails,
	}
	t := AddAlarmTemplateDTOToEntity(&inner)
	if d.Id != nil {
		t.Id = *d.Id
	}
	return t
}

func joinStr(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	s := xs[0]
	for i := 1; i < len(xs); i++ {
		s += "," + xs[i]
	}
	return s
}

func joinInt64(xs []int64) string {
	if len(xs) == 0 {
		return ""
	}
	s := fmt.Sprintf("%d", xs[0])
	for i := 1; i < len(xs); i++ {
		s += fmt.Sprintf(",%d", xs[i])
	}
	return s
}
