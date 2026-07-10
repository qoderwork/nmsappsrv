package alarm

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// mockRepository implements Repository with configurable function fields.
// Unset methods panic so tests fail fast if an unexpected call is made.
// ---------------------------------------------------------------------------

type mockRepository struct {
	findAlarmsFn             func(licenseId int, severity string, alarmType int, offset, limit int) ([]Alarm, int64, error)
	findAlarmByIDFn          func(id int64) (*Alarm, error)
	createAlarmFn            func(a *Alarm) error
	updateAlarmFn            func(a *Alarm) error
	clearAlarmFn             func(id int64, clearedTime time.Time) error
	batchClearAlarmsFn       func(ids []int64, clearUser string, clearedTime time.Time) (int64, []int64, error)
	findActiveAlarmFiltersFn func(licenseId int) ([]AlarmFilter, error)
	findAlarmFilterDevicesFn func(filterId int) ([]int64, error)
	findAlarmFilterAlarmsFn  func(filterId int) ([]string, error)
	findAlarmLibraryFn       func(tenancyId int) ([]AlarmLibrary, error)
	findAlarmTemplatesFn     func(tenancyId int) ([]AlarmTemplate, error)
	createAlarmTemplateFn    func(t *AlarmTemplate) error
	updateAlarmTemplateFn    func(t *AlarmTemplate) error
	findAlarmFiltersFn       func(licenseId int) ([]AlarmFilter, error)
	createAlarmFilterFn      func(f *AlarmFilter) error
	updateAlarmFilterFn      func(f *AlarmFilter) error
	deleteAlarmFilterFn      func(id int) error
}

func (m *mockRepository) FindAlarms(licenseId int, severity string, alarmType int, offset, limit int) ([]Alarm, int64, error) {
	if m.findAlarmsFn != nil {
		return m.findAlarmsFn(licenseId, severity, alarmType, offset, limit)
	}
	panic("mockRepository.FindAlarms not implemented")
}

func (m *mockRepository) FindAlarmByID(id int64) (*Alarm, error) {
	if m.findAlarmByIDFn != nil {
		return m.findAlarmByIDFn(id)
	}
	panic("mockRepository.FindAlarmByID not implemented")
}

func (m *mockRepository) CreateAlarm(a *Alarm) error {
	if m.createAlarmFn != nil {
		return m.createAlarmFn(a)
	}
	panic("mockRepository.CreateAlarm not implemented")
}

func (m *mockRepository) UpdateAlarm(a *Alarm) error {
	if m.updateAlarmFn != nil {
		return m.updateAlarmFn(a)
	}
	panic("mockRepository.UpdateAlarm not implemented")
}

func (m *mockRepository) ClearAlarm(id int64, clearedTime time.Time) error {
	if m.clearAlarmFn != nil {
		return m.clearAlarmFn(id, clearedTime)
	}
	panic("mockRepository.ClearAlarm not implemented")
}

func (m *mockRepository) BatchClearAlarms(ids []int64, clearUser string, clearedTime time.Time) (int64, []int64, error) {
	if m.batchClearAlarmsFn != nil {
		return m.batchClearAlarmsFn(ids, clearUser, clearedTime)
	}
	panic("mockRepository.BatchClearAlarms not implemented")
}

func (m *mockRepository) FindActiveAlarmFilters(licenseId int) ([]AlarmFilter, error) {
	if m.findActiveAlarmFiltersFn != nil {
		return m.findActiveAlarmFiltersFn(licenseId)
	}
	panic("mockRepository.FindActiveAlarmFilters not implemented")
}

func (m *mockRepository) FindAlarmFilterDevices(filterId int) ([]int64, error) {
	if m.findAlarmFilterDevicesFn != nil {
		return m.findAlarmFilterDevicesFn(filterId)
	}
	panic("mockRepository.FindAlarmFilterDevices not implemented")
}

func (m *mockRepository) FindAlarmFilterAlarms(filterId int) ([]string, error) {
	if m.findAlarmFilterAlarmsFn != nil {
		return m.findAlarmFilterAlarmsFn(filterId)
	}
	panic("mockRepository.FindAlarmFilterAlarms not implemented")
}

func (m *mockRepository) FindAlarmLibrary(tenancyId int) ([]AlarmLibrary, error) {
	if m.findAlarmLibraryFn != nil {
		return m.findAlarmLibraryFn(tenancyId)
	}
	panic("mockRepository.FindAlarmLibrary not implemented")
}

func (m *mockRepository) FindAlarmTemplates(tenancyId int) ([]AlarmTemplate, error) {
	if m.findAlarmTemplatesFn != nil {
		return m.findAlarmTemplatesFn(tenancyId)
	}
	panic("mockRepository.FindAlarmTemplates not implemented")
}

func (m *mockRepository) CreateAlarmTemplate(t *AlarmTemplate) error {
	if m.createAlarmTemplateFn != nil {
		return m.createAlarmTemplateFn(t)
	}
	panic("mockRepository.CreateAlarmTemplate not implemented")
}

func (m *mockRepository) UpdateAlarmTemplate(t *AlarmTemplate) error {
	if m.updateAlarmTemplateFn != nil {
		return m.updateAlarmTemplateFn(t)
	}
	panic("mockRepository.UpdateAlarmTemplate not implemented")
}

func (m *mockRepository) FindAlarmFilters(licenseId int) ([]AlarmFilter, error) {
	if m.findAlarmFiltersFn != nil {
		return m.findAlarmFiltersFn(licenseId)
	}
	panic("mockRepository.FindAlarmFilters not implemented")
}

func (m *mockRepository) CreateAlarmFilter(f *AlarmFilter) error {
	if m.createAlarmFilterFn != nil {
		return m.createAlarmFilterFn(f)
	}
	panic("mockRepository.CreateAlarmFilter not implemented")
}

func (m *mockRepository) UpdateAlarmFilter(f *AlarmFilter) error {
	if m.updateAlarmFilterFn != nil {
		return m.updateAlarmFilterFn(f)
	}
	panic("mockRepository.UpdateAlarmFilter not implemented")
}

func (m *mockRepository) DeleteAlarmFilter(id int) error {
	if m.deleteAlarmFilterFn != nil {
		return m.deleteAlarmFilterFn(id)
	}
	panic("mockRepository.DeleteAlarmFilter not implemented")
}

// ---------------------------------------------------------------------------
// Helper constructor
// ---------------------------------------------------------------------------

// newTestService creates a Service backed by the given Repository (no real DB).
func newTestService(repo Repository) Service {
	return &service{repo: repo}
}

// ---------------------------------------------------------------------------
// Service tests
// ---------------------------------------------------------------------------

func TestService_GetAlarm(t *testing.T) {
	severity := "CRITICAL"
	expected := &Alarm{Id: 42, Severity: &severity}

	repo := &mockRepository{
		findAlarmByIDFn: func(id int64) (*Alarm, error) {
			assert.Equal(t, int64(42), id)
			return expected, nil
		},
	}

	svc := newTestService(repo)
	result, err := svc.GetAlarm(42)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	assert.Equal(t, int64(42), result.Id)
	assert.Equal(t, "CRITICAL", *result.Severity)
}

func TestService_GetAlarm_NotFound(t *testing.T) {
	repo := &mockRepository{
		findAlarmByIDFn: func(id int64) (*Alarm, error) {
			return nil, errors.New("record not found")
		},
	}

	svc := newTestService(repo)
	result, err := svc.GetAlarm(999)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestService_ClearAlarm(t *testing.T) {
	var capturedID int64
	var capturedTime time.Time

	repo := &mockRepository{
		clearAlarmFn: func(id int64, clearedTime time.Time) error {
			capturedID = id
			capturedTime = clearedTime
			return nil
		},
	}

	svc := newTestService(repo)
	before := time.Now()
	err := svc.ClearAlarm(7)
	after := time.Now()

	assert.NoError(t, err)
	assert.Equal(t, int64(7), capturedID)
	// The service passes time.Now() -- verify it falls in the expected range.
	assert.True(t, !capturedTime.Before(before), "clearedTime should be >= time before call")
	assert.True(t, !capturedTime.After(after), "clearedTime should be <= time after call")
}

func TestService_ClearAlarm_Error(t *testing.T) {
	repo := &mockRepository{
		clearAlarmFn: func(id int64, clearedTime time.Time) error {
			return errors.New("db connection lost")
		},
	}

	svc := newTestService(repo)
	err := svc.ClearAlarm(1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db connection lost")
}

func TestService_ListAlarms_DefaultPagination(t *testing.T) {
	alarms := []Alarm{{Id: 1}, {Id: 2}}

	var capturedOffset, capturedLimit int

	repo := &mockRepository{
		findAlarmsFn: func(licenseId int, severity string, alarmType int, offset, limit int) ([]Alarm, int64, error) {
			capturedOffset = offset
			capturedLimit = limit
			assert.Equal(t, 1, licenseId)
			assert.Equal(t, "CRITICAL", severity)
			assert.Equal(t, 0, alarmType)
			return alarms, int64(10), nil
		},
	}

	svc := newTestService(repo)
	data, total, err := svc.ListAlarms(1, "CRITICAL", 0, 1, 20)

	assert.NoError(t, err)
	assert.Equal(t, int64(10), total)
	assert.Len(t, data, 2)
	// Page 1 with pageSize 20 -> offset=0, limit=20.
	assert.Equal(t, 0, capturedOffset)
	assert.Equal(t, 20, capturedLimit)
}

func TestService_ListAlarms_Page2(t *testing.T) {
	var capturedOffset, capturedLimit int

	repo := &mockRepository{
		findAlarmsFn: func(licenseId int, severity string, alarmType int, offset, limit int) ([]Alarm, int64, error) {
			capturedOffset = offset
			capturedLimit = limit
			return []Alarm{}, int64(0), nil
		},
	}

	svc := newTestService(repo)
	_, _, err := svc.ListAlarms(1, "", 0, 2, 10)

	assert.NoError(t, err)
	// Page 2 with pageSize 10 -> offset=10, limit=10.
	assert.Equal(t, 10, capturedOffset)
	assert.Equal(t, 10, capturedLimit)
}

func TestService_ListAlarms_InvalidPageDefaults(t *testing.T) {
	var capturedOffset, capturedLimit int

	repo := &mockRepository{
		findAlarmsFn: func(licenseId int, severity string, alarmType int, offset, limit int) ([]Alarm, int64, error) {
			capturedOffset = offset
			capturedLimit = limit
			return nil, int64(0), nil
		},
	}

	svc := newTestService(repo)
	// page=0 and pageSize=0 should default to page=1 and pageSize=20.
	_, _, err := svc.ListAlarms(1, "", 0, 0, 0)

	assert.NoError(t, err)
	assert.Equal(t, 0, capturedOffset)
	assert.Equal(t, 20, capturedLimit)
}
