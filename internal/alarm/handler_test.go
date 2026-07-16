package alarm

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/utils"
)

// ---------------------------------------------------------------------------
// mockService implements Service with configurable function fields.
// ---------------------------------------------------------------------------

type mockService struct {
	listAlarmsFn            func(licenseId int, severity string, alarmType int, page, pageSize int) ([]Alarm, int64, error)
	getAlarmFn              func(id int64) (*Alarm, error)
	clearAlarmFn            func(id int64) error
	batchClearAlarmsFn      func(alarmIds []int64, clearUser string) (int64, []int64, error)
	createAlarmFn           func(a *Alarm) error
	getByElementTypeAlarmIdFn func(elementId int64, alarmType int, alarmId string) (*Alarm, error)
	getByAlarmIdFn            func(alarmType int, alarmId string) (*Alarm, error)
	checkAlarmSuppressionFn func(alarm *Alarm) (bool, string)
	listAlarmLibraryFn      func(tenancyId int) ([]AlarmLibrary, error)
	importAlarmLibraryFn    func(tenancyId int, items []AlarmLibrary) (int, error)
	listAlarmTemplatesFn    func(tenancyId int) ([]AlarmTemplate, error)
	createAlarmTemplateFn   func(t *AlarmTemplate) error
	updateAlarmTemplateFn   func(t *AlarmTemplate) error
	listAlarmFiltersFn      func(licenseId int) ([]AlarmFilter, error)
	createAlarmFilterFn     func(f *AlarmFilter) error
	updateAlarmFilterFn     func(f *AlarmFilter) error
	deleteAlarmFilterFn     func(id int) error
	confirmAlarmFn          func(id int64) error
	unconfirmAlarmFn        func(id int64) error
	getSeverityCountFn      func(licenseId int) ([]SeverityCount, error)
	getAlarmSyncConfigFn    func() (*AlarmSyncConfig, error)
	updateAlarmSyncConfigFn func(config *AlarmSyncConfig) error
	addCommentForAlarmFn    func(id int64, comment string) error
}

func (m *mockService) ListAlarms(licenseId int, severity string, alarmType int, page, pageSize int) ([]Alarm, int64, error) {
	if m.listAlarmsFn != nil {
		return m.listAlarmsFn(licenseId, severity, alarmType, page, pageSize)
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

func (m *mockService) ListAlarmLibrary(tenancyId int) ([]AlarmLibrary, error) {
	if m.listAlarmLibraryFn != nil {
		return m.listAlarmLibraryFn(tenancyId)
	}
	panic("mockService.ListAlarmLibrary not implemented")
}

func (m *mockService) ListAlarmTemplates(tenancyId int) ([]AlarmTemplate, error) {
	if m.listAlarmTemplatesFn != nil {
		return m.listAlarmTemplatesFn(tenancyId)
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

func (m *mockService) ListAlarmFilters(licenseId int) ([]AlarmFilter, error) {
	if m.listAlarmFiltersFn != nil {
		return m.listAlarmFiltersFn(licenseId)
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

func (m *mockService) GetSeverityCount(licenseId int) ([]SeverityCount, error) {
	if m.getSeverityCountFn != nil {
		return m.getSeverityCountFn(licenseId)
	}
	panic("mockService.GetSeverityCount not implemented")
}

func (m *mockService) ImportAlarmLibrary(tenancyId int, items []AlarmLibrary) (int, error) {
	if m.importAlarmLibraryFn != nil {
		return m.importAlarmLibraryFn(tenancyId, items)
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

func (m *mockService) QueryAlarmStatisticResult(licenseId int) (*AlarmStatisticResult, error) {
	return &AlarmStatisticResult{}, nil
}

func (m *mockService) DeleteAlarmLibrary(id int) error {
	return nil
}

func (m *mockService) ListActiveAlarmProbableCause(licenseId int) ([]string, error) {
	return nil, nil
}

func (m *mockService) GetAlarmEventType(licenseId int) ([]string, error) {
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

// newTestRouter creates a gin engine with alarm routes wired to the given handler.
func newTestRouter(h *Handler) *gin.Engine {
	r := gin.New()
	r.GET("/alarms/:id", h.GetAlarm)
	r.PUT("/alarms/:id/clear", h.ClearAlarm)
	r.GET("/alarms", h.ListAlarms)
	r.PUT("/alarms/batch-clear", h.BatchClearAlarms)
	r.POST("/alarms/:id/confirm", h.ConfirmAlarm)
	r.POST("/alarms/:id/unconfirm", h.UnconfirmAlarm)
	r.GET("/alarms/severity-count", h.GetSeverityCount)
	return r
}

// parseResponse decodes the JSON body into a utils.Response.
func parseResponse(w *httptest.ResponseRecorder) utils.Response {
	var resp utils.Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp
}

// ---------------------------------------------------------------------------
// Handler tests
// ---------------------------------------------------------------------------

func TestHandler_GetAlarm_Success(t *testing.T) {
	severity := "CRITICAL"
	source := "BS-001"
	expected := &Alarm{Id: 42, Severity: &severity, AlarmSource: &source}

	svc := &mockService{
		getAlarmFn: func(id int64) (*Alarm, error) {
			assert.Equal(t, int64(42), id)
			return expected, nil
		},
	}

	h := &Handler{svc: svc}
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/alarms/42", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	resp := parseResponse(w)
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "Success", resp.Message)
	assert.NotNil(t, resp.Data)

	// Verify the alarm data in the response.
	dataMap, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(42), dataMap["id"])
	assert.Equal(t, "CRITICAL", dataMap["severity"])
}

func TestHandler_GetAlarm_NotFound(t *testing.T) {
	svc := &mockService{
		getAlarmFn: func(id int64) (*Alarm, error) {
			return nil, apperror.ErrAlarmNotFound
		},
	}

	h := &Handler{svc: svc}
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/alarms/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	resp := parseResponse(w)
	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "alarm not found", resp.Message)
}

func TestHandler_GetAlarm_InvalidID(t *testing.T) {
	svc := &mockService{}
	h := &Handler{svc: svc}
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/alarms/not-a-number", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	resp := parseResponse(w)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "invalid alarm id", resp.Message)
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

	req := httptest.NewRequest(http.MethodPut, "/alarms/7/clear", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(7), capturedID)

	resp := parseResponse(w)
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "Success", resp.Message)
}

func TestHandler_ClearAlarm_ServiceError(t *testing.T) {
	svc := &mockService{
		clearAlarmFn: func(id int64) error {
			return apperror.Wrap(errors.New("db connection lost"), "CLEAR_ALARM_FAILED", 500, "failed to clear alarm")
		},
	}

	h := &Handler{svc: svc}
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPut, "/alarms/1/clear", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	resp := parseResponse(w)
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "failed to clear alarm", resp.Message)
}

func TestHandler_ClearAlarm_InvalidID(t *testing.T) {
	svc := &mockService{}
	h := &Handler{svc: svc}
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPut, "/alarms/abc/clear", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	resp := parseResponse(w)
	assert.Equal(t, "invalid alarm id", resp.Message)
}

func TestHandler_BatchClearAlarms_Success(t *testing.T) {
	svc := &mockService{
		batchClearAlarmsFn: func(alarmIds []int64, clearUser string) (int64, []int64, error) {
			assert.Equal(t, []int64{1, 2, 3}, alarmIds)
			assert.Equal(t, "admin", clearUser)
			return 2, []int64{3}, nil
		},
	}

	h := &Handler{svc: svc}
	router := newTestRouter(h)

	body := `{"alarmIds":[1,2,3],"clearUser":"admin"}`
	req := httptest.NewRequest(http.MethodPut, "/alarms/batch-clear", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
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

	req := httptest.NewRequest(http.MethodPost, "/alarms/5/confirm", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(5), capturedID)
}

func TestHandler_ConfirmAlarm_NotFound(t *testing.T) {
	svc := &mockService{
		confirmAlarmFn: func(id int64) error {
			return apperror.ErrAlarmNotFound
		},
	}
	h := &Handler{svc: svc}
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/alarms/404/confirm", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_ConfirmAlarm_InvalidID(t *testing.T) {
	svc := &mockService{}
	h := &Handler{svc: svc}
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/alarms/not-a-number/confirm", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_UnconfirmAlarm_Success(t *testing.T) {
	var capturedID int64
	svc := &mockService{
		unconfirmAlarmFn: func(id int64) error {
			capturedID = id
			return nil
		},
	}
	h := &Handler{svc: svc}
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/alarms/9/unconfirm", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(9), capturedID)
}

func TestHandler_GetSeverityCount_Success(t *testing.T) {
	svc := &mockService{
		getSeverityCountFn: func(licenseId int) ([]SeverityCount, error) {
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

	req := httptest.NewRequest(http.MethodGet, "/alarms/severity-count", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(w)
	assert.Equal(t, 200, resp.Code)

	data, ok := resp.Data.([]interface{})
	assert.True(t, ok)
	assert.Len(t, data, 4)
}
