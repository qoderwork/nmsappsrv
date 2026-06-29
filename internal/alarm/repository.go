package alarm

import (
	"time"

	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository handles database operations for alarm entities.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ---------------------------------------------------------------------------
// Alarm CRUD
// ---------------------------------------------------------------------------

// FindAlarms returns a paginated list of alarms with optional severity and
// alarm_type filters. The total count is returned for pagination metadata.
func (r *Repository) FindAlarms(licenseId int, severity string, alarmType int, offset, limit int) ([]Alarm, int64, error) {
	var alarms []Alarm
	var total int64

	query := r.db.Model(&Alarm{}).Where("license_id = ?", licenseId)

	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if alarmType > 0 {
		query = query.Where("alarm_type = ?", alarmType)
	}

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindAlarms count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&alarms).Error; err != nil {
		logger.Errorf("FindAlarms query error: %v", err)
		return nil, 0, err
	}
	return alarms, total, nil
}

// FindAlarmByID returns a single alarm by its primary key.
func (r *Repository) FindAlarmByID(id int64) (*Alarm, error) {
	var a Alarm
	if err := r.db.Where("id = ?", id).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// CreateAlarm inserts a new alarm row.
func (r *Repository) CreateAlarm(a *Alarm) error {
	return r.db.Create(a).Error
}

// UpdateAlarm saves all fields of an existing alarm.
func (r *Repository) UpdateAlarm(a *Alarm) error {
	return r.db.Save(a).Error
}

// ClearAlarm marks an alarm as cleared by setting alarm_status to 0 and
// recording the cleared_time.
func (r *Repository) ClearAlarm(id int64, clearedTime time.Time) error {
	return r.db.Model(&Alarm{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"alarm_status": 0,
			"cleared_time": clearedTime,
		}).Error
}

// ---------------------------------------------------------------------------
// AlarmLibrary
// ---------------------------------------------------------------------------

// FindAlarmLibrary returns all alarm library entries for the given tenancy.
func (r *Repository) FindAlarmLibrary(tenancyId int) ([]AlarmLibrary, error) {
	var libs []AlarmLibrary
	if err := r.db.Where("tenancy_id = ?", tenancyId).Find(&libs).Error; err != nil {
		return nil, err
	}
	return libs, nil
}

// ---------------------------------------------------------------------------
// AlarmTemplate
// ---------------------------------------------------------------------------

// FindAlarmTemplates returns all alarm templates for the given tenancy.
func (r *Repository) FindAlarmTemplates(tenancyId int) ([]AlarmTemplate, error) {
	var templates []AlarmTemplate
	if err := r.db.Where("tenancy_id = ?", tenancyId).Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

// CreateAlarmTemplate inserts a new alarm template.
func (r *Repository) CreateAlarmTemplate(t *AlarmTemplate) error {
	return r.db.Create(t).Error
}

// UpdateAlarmTemplate saves changes to an existing alarm template.
func (r *Repository) UpdateAlarmTemplate(t *AlarmTemplate) error {
	return r.db.Save(t).Error
}

// ---------------------------------------------------------------------------
// AlarmFilter
// ---------------------------------------------------------------------------

// FindAlarmFilters returns all alarm filters for the given license.
func (r *Repository) FindAlarmFilters(licenseId int) ([]AlarmFilter, error) {
	var filters []AlarmFilter
	if err := r.db.Where("license_id = ?", licenseId).Find(&filters).Error; err != nil {
		return nil, err
	}
	return filters, nil
}

// CreateAlarmFilter inserts a new alarm filter.
func (r *Repository) CreateAlarmFilter(f *AlarmFilter) error {
	return r.db.Create(f).Error
}

// UpdateAlarmFilter saves changes to an existing alarm filter.
func (r *Repository) UpdateAlarmFilter(f *AlarmFilter) error {
	return r.db.Save(f).Error
}

// DeleteAlarmFilter removes an alarm filter by ID.
func (r *Repository) DeleteAlarmFilter(id int) error {
	return r.db.Where("id = ?", id).Delete(&AlarmFilter{}).Error
}
