package alarm

import (
	"time"

	"gorm.io/gorm"
)

// Service contains the business logic for alarm management.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ---------------------------------------------------------------------------
// Alarm
// ---------------------------------------------------------------------------

// ListAlarms returns a paginated alarm list. The page number (1-based) is
// converted to an offset before querying.
func (s *Service) ListAlarms(licenseId int, severity string, alarmType int, page, pageSize int) ([]Alarm, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindAlarms(licenseId, severity, alarmType, offset, pageSize)
}

// GetAlarm returns a single alarm by ID.
func (s *Service) GetAlarm(id int64) (*Alarm, error) {
	return s.repo.FindAlarmByID(id)
}

// ClearAlarm marks an alarm as cleared with the current timestamp.
func (s *Service) ClearAlarm(id int64) error {
	return s.repo.ClearAlarm(id, time.Now())
}

// ---------------------------------------------------------------------------
// AlarmLibrary
// ---------------------------------------------------------------------------

// ListAlarmLibrary returns all alarm library entries for the given tenancy.
func (s *Service) ListAlarmLibrary(tenancyId int) ([]AlarmLibrary, error) {
	return s.repo.FindAlarmLibrary(tenancyId)
}

// ---------------------------------------------------------------------------
// AlarmTemplate
// ---------------------------------------------------------------------------

// ListAlarmTemplates returns all alarm templates for the given tenancy.
func (s *Service) ListAlarmTemplates(tenancyId int) ([]AlarmTemplate, error) {
	return s.repo.FindAlarmTemplates(tenancyId)
}

// CreateAlarmTemplate persists a new alarm template.
func (s *Service) CreateAlarmTemplate(t *AlarmTemplate) error {
	return s.repo.CreateAlarmTemplate(t)
}

// UpdateAlarmTemplate persists changes to an existing alarm template.
func (s *Service) UpdateAlarmTemplate(t *AlarmTemplate) error {
	return s.repo.UpdateAlarmTemplate(t)
}

// ---------------------------------------------------------------------------
// AlarmFilter
// ---------------------------------------------------------------------------

// ListAlarmFilters returns all alarm filters for the given license.
func (s *Service) ListAlarmFilters(licenseId int) ([]AlarmFilter, error) {
	return s.repo.FindAlarmFilters(licenseId)
}

// CreateAlarmFilter persists a new alarm filter.
func (s *Service) CreateAlarmFilter(f *AlarmFilter) error {
	return s.repo.CreateAlarmFilter(f)
}

// UpdateAlarmFilter persists changes to an existing alarm filter.
func (s *Service) UpdateAlarmFilter(f *AlarmFilter) error {
	return s.repo.UpdateAlarmFilter(f)
}

// DeleteAlarmFilter removes an alarm filter by ID.
func (s *Service) DeleteAlarmFilter(id int) error {
	return s.repo.DeleteAlarmFilter(id)
}
