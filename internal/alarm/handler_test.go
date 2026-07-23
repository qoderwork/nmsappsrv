package alarm

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"nmsappsrv/pkg/dto"
)

// ---------------------------------------------------------------------------
// mockService implements Service with configurable function fields.
// ---------------------------------------------------------------------------

type mockService struct {
	listAlarmsFn            func(tenantId int, severity string, alarmType int, page, pageSize int) ([]Alarm, int64, error)
	getAlarmFn              func(id int64) (*Alarm, error)
	clearAlarmFn            func(id int64) error
	batchClearAlarmsFn      func(alarmIds []int64, clearUser string) (int64, []int64, error)
	createAlarmFn           func(a *Alarm) error
	getByElementTypeAlarmIdFn func(elementId int64, alarmType int, alarmId string) (*Alarm, error)
	getByAlarmIdFn            func(alarmType int, alarmId string) (*Alarm, error)
	checkAlarmSuppressionFn func(alarm *Alarm) (bool, string)
	listAlarmLibraryFn      func(tenantId int) ([]AlarmLibrary, error)
	importAlarmLibraryFn    func(tenantId int, items []AlarmLibrary) (int, error)
	listAlarmTemplatesFn    func(tenantId int) ([]AlarmTemplate, error)
	createAlarmTemplateFn   func(t *AlarmTemplate) error
	updateAlarmTemplateFn   func(t *AlarmTemplate) error
	listAlarmFiltersFn      func(tenantId int) ([]AlarmFilter, error)
	createAlarmFilterFn     func(f *AlarmFilter) error
	updateAlarmFilterFn     func(f *AlarmFilter) error
	deleteAlarmFilterFn     func(id int) error
	confirmAlarmFn          func(id int64) error
	unconfirmAlarmFn        func(id int64) error
	getSeverityCountFn      func(tenantId int) ([]SeverityCount, error)
	getAlarmSyncConfigFn    func() (*AlarmSyncConfig, error)
	updateAlarmSyncConfigFn func(config *AlarmSyncConfig) error
	addCommentForAlarmFn    func(id int64, comment string) error
}

func (m *mockService) ListAlarms(tenantId int, severity string, alarmType int, page, pageSize int) ([]Alarm, int64, error) {
	if m.listAlarmsFn != nil {
		return m.listAlarmsFn(tenantId, severity, alarmType, page, pageSize)
	}
	panic("mockService.ListAlarms not implemented")
}

func (m *mockService) GetAlarm(id int64) (*Alarm, error) {
	if m.getAlarmFn != nil {
		return m.getAlarmFn(id)
	}
	panic("mockService.GetAlarm not implemented")
}

func (m *mockService) ClearAlarm(id int64) error {
	if m.clearAlarmFn != nil {
		return m.clearAlarmFn(id)
	}
	panic("mockService.ClearAlarm not implemented")
}

func (m *mockService) BatchClearAlarms(alarmIds []int64, clearUser string) (int64, []int64, error) {
	if m.batchClearAlarmsFn != nil {
		return m.batchClearAlarmsFn(alarmIds, clearUser)
	}
	panic("mockService.BatchClearAlarms not implemented")
}

func (m *mockService) CreateAlarm(a *Alarm) error {
	if m.createAlarmFn != nil {
		return m.createAlarmFn(a)
	}
	panic("mockService.CreateAlarm not implemented")
}

func (m *mockService) GetByElementTypeAlarmId(elementId int64, alarmType int, alarmId string) (*Alarm, error) {
	if m.getByElementTypeAlarmIdFn != nil {
		return m.getByElementTypeAlarmIdFn(elementId, alarmType, alarmId)
	}
	panic("mockService.GetByElementTypeAlarmId not implemented")
}

func (m *mockService) GetByAlarmId(alarmType int, alarmId string) (*Alarm, error) {
	if m.getByAlarmIdFn != nil {
		return m.getByAlarmIdFn(alarmType, alarmId)
	}
	panic("mockService.GetByAlarmId not implemented")
}

func (m *mockService) CheckAlarmSuppression(alarm *Alarm) (bool, string) {
	if m.checkAlarmSuppressionFn != nil {
		return m.checkAlarmSuppressionFn(alarm)
	}
	panic("mockService.CheckAlarmSuppression not implemented")
}

func (m *mockService) ListAlarmLibrary(tenantId int) ([]AlarmLibrary, error) {
	if m.listAlarmLibraryFn != nil {
		return m.listAlarmLibraryFn(tenantId)
	}
	panic("mockService.ListAlarmLibrary not implemented")
}

func (m *mockService) ListAlarmTemplates(tenantId int) ([]AlarmTemplate, error) {
	if m.listAlarmTemplatesFn != nil {
		return m.listAlarmTemplatesFn(tenantId)
	}
	panic("mockService.ListAlarmTemplates not implemented")
}

func (m *mockService) CreateAlarmTemplate(t *AlarmTemplate) error {
	if m.createAlarmTemplateFn != nil {
		return m.createAlarmTemplateFn(t)
	}
	panic("mockService.CreateAlarmTemplate not implemented")
}

func (m *mockService) UpdateAlarmTemplate(t *AlarmTemplate) error {
	if m.updateAlarmTemplateFn != nil {
		return m.updateAlarmTemplateFn(t)
	}
	panic("mockService.UpdateAlarmTemplate not implemented")
}

func (m *mockService) ListAlarmFilters(tenantId int) ([]AlarmFilter, error) {
	if m.listAlarmFiltersFn != nil {
		return m.listAlarmFiltersFn(tenantId)
	}
	panic("mockService.ListAlarmFilters not implemented")
}

func (m *mockService) CreateAlarmFilter(f *AlarmFilter) error {
	if m.createAlarmFilterFn != nil {
		return m.createAlarmFilterFn(f)
	}
	panic("mockService.CreateAlarmFilter not implemented")
}

func (m *mockService) UpdateAlarmFilter(f *AlarmFilter) error {
	if m.updateAlarmFilterFn != nil {
		return m.updateAlarmFilterFn(f)
	}
	panic("mockService.UpdateAlarmFilter not implemented")
}

func (m *mockService) DeleteAlarmFilter(id int) error {
	if m.deleteAlarmFilterFn != nil {
		return m.deleteAlarmFilterFn(id)
	}
	panic("mockService.DeleteAlarmFilter not implemented")
}

func (m *mockService) ConfirmAlarm(id int64) error {
	if m.confirmAlarmFn != nil {
		return m.confirmAlarmFn(id)
	}
	panic("mockService.ConfirmAlarm not implemented")
}

func (m *mockService) UnconfirmAlarm(id int64) error {
	if m.unconfirmAlarmFn != nil {
		return m.unconfirmAlarmFn(id)
	}
	panic("mockService.UnconfirmAlarm not implemented")
}

func (m *mockService) GetSeverityCount(tenantId int) ([]SeverityCount, error) {
	if m.getSeverityCountFn != nil {
		return m.getSeverityCountFn(tenantId)
	}
	panic("mockService.GetSeverityCount not implemented")
}

func (m *mockService) ImportAlarmLibrary(tenantId int, items []AlarmLibrary) (int, error) {
	if m.importAlarmLibraryFn != nil {
		return m.importAlarmLibraryFn(tenantId, items)
	}
	panic("mockService.ImportAlarmLibrary not implemented")
}

func (m *mockService) GetAlarmSyncConfig() (*AlarmSyncConfig, error) {
	if m.getAlarmSyncConfigFn != nil {
		return m.getAlarmSyncConfigFn()
	}
	panic("mockService.GetAlarmSyncConfig not implemented")
}

func (m *mockService) UpdateAlarmSyncConfig(config *AlarmSyncConfig) error {
	if m.updateAlarmSyncConfigFn != nil {
		return m.updateAlarmSyncConfigFn(config)
	}
	panic("mockService.UpdateAlarmSyncConfig not implemented")
}

func (m *mockService) AddCommentForAlarm(id int64, comment string) error {
	if m.addCommentForAlarmFn != nil {
		return m.addCommentForAlarmFn(id, comment)
	}
	panic("mockService.AddCommentForAlarm not implemented")
}

func (m *mockService) QueryAlarmStatisticTopN(topN int, startTime, endTime *time.Time) ([]AlarmStatisticTopN, error) {
	return nil, nil
}

func (m *mockService) GetEmailNotificationConfig() (*EmailNotificationConfig, error) {
	return &EmailNotificationConfig{}, nil
}

func (m *mockService) UpdateEmailNotificationConfig(config *EmailNotificationConfig) error {
	return nil
}

func (m *mockService) DeleteAlarm(id int64) error {
	return nil
}

func (m *mockService) GetAlarmTemplate(id int) (*AlarmTemplate, error) {
	return &AlarmTemplate{}, nil
}

func (m *mockService) DeleteAlarmTemplate(id int) error {
	return nil
}

func (m *mockService) GetAlarmFilter(id int) (*AlarmFilter, error) {
	return &AlarmFilter{}, nil
}

func (m *mockService) ToggleAlarmFilterEnable(id int, enable bool) error {
	return nil
}

func (m *mockService) UpdateAlarmTemplateEmailNotification(id int, enable bool) error {
	return nil
}

func (m *mockService) QueryAlarmStatisticResult(tenantId int) (*AlarmStatisticResult, error) {
	return &AlarmStatisticResult{}, nil
}

func (m *mockService) DeleteAlarmLibrary(id int) error {
	return nil
}

func (m *mockService) ListActiveAlarmProbableCause(tenantId int) ([]string, error) {
	return nil, nil
}

func (m *mockService) GetAlarmEventType(tenantId int) ([]string, error) {
	return nil, nil
}

func (m *mockService) ListEmailNoticeResult(query EmailNoticeResultQuery, page, pageSize int) ([]EmailNoticeResult, int64, error) {
	return nil, 0, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestRouter creates a gin engine with Java-style /api/v2 alarm routes wired
// to the given handler (route plan B — no RESTful resource paths).
func newTestRouter(h *Handler) *gin.Engine {
	r := gin.New()
	rg := r.Group("/api/v2")
	RegisterRoutes(rg, h)
	return r
}

// parseResult decodes the JSON body into a dto.Result envelope.
// Uses dto.Result[any] because the concrete data type varies per endpoint.
func parseResult(w *httptest.ResponseRecorder) dto.Result[any] {
	var resp dto.Result[any]
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp
}

// requestDataDTOJSON builds a RequestDataDTO body and returns its JSON bytes.
func requestDataDTOJSON[Q, D any](query Q, data D) []byte {
	req := dto.RequestDataDTO[Q, D]{
		Query: query,
		Data:  data,
	}
	b, _ := json.Marshal(req)
	return b
}

// ---------------------------------------------------------------------------
// Handler tests (Java route plan B)
// ---------------------------------------------------------------------------

func TestHandler_ListAlarms_Success(t *testing.T) {
	severity := "CRITICAL"
	source := "BS-001"
	alarm := Alarm{Id: 42, Severity: &severity, AlarmSource: &source}

	svc := &mockService{
		listAlarmsFn: func(tenantId int, severityArg string, alarmType int, page, pageSize int) ([]Alarm, int64, error) {
			assert.Equal(t, 0, tenantId)
			assert.Equal(t, "CRITICAL", severityArg)
			assert.Equal(t, 1, page)
			assert.Equal(t, 50, pageSize)
			return []Alarm{alarm}, 1, nil
		},
	}

	h := &Handler{svc: svc}
	router := newTestRouter(h)

	body := requestDataDTOJSON(
		AlarmQuery{Severity: &severity},
		struct{}{},
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/listAlarm", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	resp := parseResult(w)
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "success", resp.Msg)
	assert.NotNil(t, resp.Data)

	// Verify the page content inside Result.Data
	dataMap, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok, "data should be a SpringDataPage map")
	content, ok := dataMap["content"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, content, 1)
	assert.Equal(t, float64(1), dataMap["totalElements"])
}

func TestHandler_ConfirmAlarm_Success(t *testing.T) {
	var capturedID int64
	svc := &mockService{
		confirmAlarmFn: func(id int64) error {
			capturedID = id
			return nil
		},
	}

	h := &Handler{svc: svc}
	router := newTestRouter(h)

	idx := int64(7)
	body := requestDataDTOJSON(
		struct{}{},
		ConfirmAlarmDTO{Index: &idx},
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/confirmAlarm", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(7), capturedID)

	resp := parseResult(w)
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "success", resp.Msg)
}

func TestHandler_ConfirmAlarm_MissingIndex(t *testing.T) {
	svc := &mockService{}
	h := &Handler{svc: svc}
	router := newTestRouter(h)

	body := requestDataDTOJSON(struct{}{}, ConfirmAlarmDTO{})
	req := httptest.NewRequest(http.MethodPost, "/api/v2/confirmAlarm", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code) // HTTP=200 per Java contract
	resp := parseResult(w)
	assert.Equal(t, 400, resp.Code) // business BAD_REQUEST inside envelope
}

func TestHandler_ClearAlarm_Success(t *testing.T) {
	var capturedID int64
	svc := &mockService{
		clearAlarmFn: func(id int64) error {
			capturedID = id
			return nil
		},
	}

	h := &Handler{svc: svc}
	router := newTestRouter(h)

	idx := int64(7)
	body := requestDataDTOJSON(struct{}{}, ConfirmAlarmDTO{Index: &idx})
	req := httptest.NewRequest(http.MethodPost, "/api/v2/clearAlarm", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(7), capturedID)

	resp := parseResult(w)
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "success", resp.Msg)
}

func TestHandler_GetSeverityCount_Success(t *testing.T) {
	svc := &mockService{
		getSeverityCountFn: func(tenantId int) ([]SeverityCount, error) {
			return []SeverityCount{
				{Severity: "Critical", AlarmCount: 3},
				{Severity: "Major", AlarmCount: 0},
				{Severity: "Minor", AlarmCount: 0},
				{Severity: "Warning", AlarmCount: 0},
			}, nil
		},
	}
	h := &Handler{svc: svc}
	router := newTestRouter(h)

	body := requestDataDTOJSON(struct{}{}, struct{}{})
	req := httptest.NewRequest(http.MethodPost, "/api/v2/getCountOfSeverity", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResult(w)
	assert.Equal(t, 200, resp.Code)

	data, ok := resp.Data.([]interface{})
	assert.True(t, ok)
	assert.Len(t, data, 4)
}

func TestHandler_ListAlarmFilters_Success(t *testing.T) {
	name := "Filter1"
	svc := &mockService{
		listAlarmFiltersFn: func(tenantId int) ([]AlarmFilter, error) {
			return []AlarmFilter{
				{Id: 1, FilterRuleName: &name},
			}, nil
		},
	}
	h := &Handler{svc: svc}
	router := newTestRouter(h)

	body := requestDataDTOJSON(ListAlarmFilterTaskQuery{}, struct{}{})
	req := httptest.NewRequest(http.MethodPost, "/api/v2/listAlarmFilterTask", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResult(w)
	assert.Equal(t, 200, resp.Code)

	dataMap, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	content, ok := dataMap["content"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, content, 1)
}

func TestHandler_DeleteAlarmFilter_MissingId(t *testing.T) {
	svc := &mockService{}
	h := &Handler{svc: svc}
	router := newTestRouter(h)

	body := requestDataDTOJSON(struct{}{}, IntegerIdDto{})
	req := httptest.NewRequest(http.MethodPost, "/api/v2/deleteAlarmFilterTask", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResult(w)
	assert.Equal(t, 400, resp.Code)
}
