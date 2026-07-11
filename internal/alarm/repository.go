package alarm

import (
	"time"

	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository provides data access for alarm entities.
// It embeds BaseRepository[Alarm, int64] for standard CRUD on Alarm,
// and retains module-specific methods for custom queries and other entity types.
type Repository struct {
	*baserepo.BaseRepository[Alarm, int64]
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		BaseRepository: baserepo.New[Alarm, int64](db, "id"),
		db:             db,
	}
}

// ---------------------------------------------------------------------------
// Alarm – module-specific queries
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

// ClearAlarm marks an alarm as cleared by setting alarm_status to 0 and
// recording the cleared_time.
func (r *Repository) ClearAlarm(id int64, clearedTime time.Time) error {
	return r.db.Model(&Alarm{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"alarm_status": 0,
			"cleared_time": clearedTime,
		}).Error
}

// BatchClearAlarms clears multiple alarms in a single transaction. It returns
// the number of alarms actually cleared and the IDs that were not found.
func (r *Repository) BatchClearAlarms(ids []int64, clearUser string, clearedTime time.Time) (int64, []int64, error) {
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
func (r *Repository) FindActiveAlarmFilters(licenseId int) ([]AlarmFilter, error) {
	var filters []AlarmFilter
	enabled := true
	if err := r.db.Where("license_id = ? AND enable = ?", licenseId, enabled).Find(&filters).Error; err != nil {
		return nil, err
	}
	return filters, nil
}

// FindAlarmFilterDevices returns the device IDs associated with a given filter
// from the alarm_filter_has_device table.
func (r *Repository) FindAlarmFilterDevices(filterId int) ([]int64, error) {
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
func (r *Repository) FindAlarmFilterAlarms(filterId int) ([]string, error) {
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
