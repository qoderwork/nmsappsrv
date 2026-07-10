package alarm

import (
	"time"

	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for alarm entities.
type Repository interface {
	FindAlarms(licenseId int, severity string, alarmType int, offset, limit int) ([]Alarm, int64, error)
	FindAlarmByID(id int64) (*Alarm, error)
	CreateAlarm(a *Alarm) error
	UpdateAlarm(a *Alarm) error
	ClearAlarm(id int64, clearedTime time.Time) error
	BatchClearAlarms(ids []int64, clearUser string, clearedTime time.Time) (int64, []int64, error)
	FindActiveAlarmFilters(licenseId int) ([]AlarmFilter, error)
	FindAlarmFilterDevices(filterId int) ([]int64, error)
	FindAlarmFilterAlarms(filterId int) ([]string, error)
	FindAlarmLibrary(tenancyId int) ([]AlarmLibrary, error)
	FindAlarmTemplates(tenancyId int) ([]AlarmTemplate, error)
	CreateAlarmTemplate(t *AlarmTemplate) error
	UpdateAlarmTemplate(t *AlarmTemplate) error
	FindAlarmFilters(licenseId int) ([]AlarmFilter, error)
	CreateAlarmFilter(f *AlarmFilter) error
	UpdateAlarmFilter(f *AlarmFilter) error
	DeleteAlarmFilter(id int) error
}

// repository handles database operations for alarm entities.
type repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// ---------------------------------------------------------------------------
// Alarm CRUD
// ---------------------------------------------------------------------------

// FindAlarms returns a paginated list of alarms with optional severity and
// alarm_type filters. The total count is returned for pagination metadata.
func (r *repository) FindAlarms(licenseId int, severity string, alarmType int, offset, limit int) ([]Alarm, int64, error) {
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
func (r *repository) FindAlarmByID(id int64) (*Alarm, error) {
	var a Alarm
	if err := r.db.Where("id = ?", id).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// CreateAlarm inserts a new alarm row.
func (r *repository) CreateAlarm(a *Alarm) error {
	return r.db.Create(a).Error
}

// UpdateAlarm saves all fields of an existing alarm.
func (r *repository) UpdateAlarm(a *Alarm) error {
	return r.db.Save(a).Error
}

// ClearAlarm marks an alarm as cleared by setting alarm_status to 0 and
// recording the cleared_time.
func (r *repository) ClearAlarm(id int64, clearedTime time.Time) error {
	return r.db.Model(&Alarm{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"alarm_status": 0,
			"cleared_time": clearedTime,
		}).Error
}

// BatchClearAlarms clears multiple alarms in a single transaction. It returns
// the number of alarms actually cleared and the IDs that were not found.
func (r *repository) BatchClearAlarms(ids []int64, clearUser string, clearedTime time.Time) (int64, []int64, error) {
	var clearedCount int64
	var notFoundIds []int64

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			result := tx.Model(&Alarm{}).Where("id = ?", id).
				Updates(map[string]interface{}{
					"alarm_status": 0,
					"cleared_time": clearedTime,
					"clear_user":   clearUser,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				notFoundIds = append(notFoundIds, id)
			} else {
				clearedCount++
			}
		}
		return nil
	})
	return clearedCount, notFoundIds, err
}

// FindActiveAlarmFilters returns all enabled alarm filters for the given
// license. These are used to check whether a new alarm should be suppressed.
func (r *repository) FindActiveAlarmFilters(licenseId int) ([]AlarmFilter, error) {
	var filters []AlarmFilter
	enabled := true
	if err := r.db.Where("license_id = ? AND enable = ?", licenseId, enabled).Find(&filters).Error; err != nil {
		return nil, err
	}
	return filters, nil
}

// FindAlarmFilterDevices returns the device IDs associated with a given filter
// from the alarm_filter_has_device table.
func (r *repository) FindAlarmFilterDevices(filterId int) ([]int64, error) {
	var devices []AlarmFilterHasDevice
	if err := r.db.Where("alarm_filter_id = ?", filterId).Find(&devices).Error; err != nil {
		return nil, err
	}
	var elementIds []int64
	for _, d := range devices {
		if d.ElementId != nil {
			elementIds = append(elementIds, *d.ElementId)
		}
	}
	return elementIds, nil
}

// FindAlarmFilterAlarms returns the alarm identifiers associated with a given
// filter from the alarm_filter_has_alarm table.
func (r *repository) FindAlarmFilterAlarms(filterId int) ([]string, error) {
	var alarms []AlarmFilterHasAlarm
	if err := r.db.Where("alarm_filter_id = ?", filterId).Find(&alarms).Error; err != nil {
		return nil, err
	}
	var alarmIds []string
	for _, a := range alarms {
		if a.AlarmId != nil {
			alarmIds = append(alarmIds, *a.AlarmId)
		}
	}
	return alarmIds, nil
}

// ---------------------------------------------------------------------------
// AlarmLibrary
// ---------------------------------------------------------------------------

// FindAlarmLibrary returns all alarm library entries for the given tenancy.
func (r *repository) FindAlarmLibrary(tenancyId int) ([]AlarmLibrary, error) {
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
func (r *repository) FindAlarmTemplates(tenancyId int) ([]AlarmTemplate, error) {
	var templates []AlarmTemplate
	if err := r.db.Where("tenancy_id = ?", tenancyId).Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

// CreateAlarmTemplate inserts a new alarm template.
func (r *repository) CreateAlarmTemplate(t *AlarmTemplate) error {
	return r.db.Create(t).Error
}

// UpdateAlarmTemplate saves changes to an existing alarm template.
func (r *repository) UpdateAlarmTemplate(t *AlarmTemplate) error {
	return r.db.Save(t).Error
}

// ---------------------------------------------------------------------------
// AlarmFilter
// ---------------------------------------------------------------------------

// FindAlarmFilters returns all alarm filters for the given license.
func (r *repository) FindAlarmFilters(licenseId int) ([]AlarmFilter, error) {
	var filters []AlarmFilter
	if err := r.db.Where("license_id = ?", licenseId).Find(&filters).Error; err != nil {
		return nil, err
	}
	return filters, nil
}

// CreateAlarmFilter inserts a new alarm filter.
func (r *repository) CreateAlarmFilter(f *AlarmFilter) error {
	return r.db.Create(f).Error
}

// UpdateAlarmFilter saves changes to an existing alarm filter.
func (r *repository) UpdateAlarmFilter(f *AlarmFilter) error {
	return r.db.Save(f).Error
}

// DeleteAlarmFilter removes an alarm filter by ID.
func (r *repository) DeleteAlarmFilter(id int) error {
	return r.db.Where("id = ?", id).Delete(&AlarmFilter{}).Error
}
