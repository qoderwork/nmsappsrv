package alarm

import (
	"errors"
	"fmt"
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

// GetByElementTypeAlarmId returns the most recently updated active alarm for a
// device matching the given alarm_type and alarm_id, or (nil, nil) when none
// exists. Used by callers that must de-duplicate / update a single logical
// alarm (e.g. the ZTP "ztp_failed" alarm raised per element).
func (r *Repository) GetByElementTypeAlarmId(elementId int64, alarmType int, alarmId string) (*Alarm, error) {
	var a Alarm
	err := r.db.Where("element_id = ? AND alarm_type = ? AND alarm_id = ?", elementId, alarmType, alarmId).
		Order("update_time DESC").First(&a).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// GetByAlarmId returns the most recently updated alarm matching the given
// alarm_type and alarm_id, ignoring element_id. Used for SYSTEM-level alarms
// that are not tied to a single device (e.g. T-Platform's "t_platform_unavailable"
// alarm, which mirrors Java alarmService.getByAlarmTypeAndAlarmId). Returns
// (nil, nil) when none exists.
func (r *Repository) GetByAlarmId(alarmType int, alarmId string) (*Alarm, error) {
	var a Alarm
	err := r.db.Where("alarm_type = ? AND alarm_id = ?", alarmType, alarmId).
		Order("update_time DESC").First(&a).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// FindAlarms returns a paginated list of alarms with optional severity and
// alarm_type filters. The total count is returned for pagination metadata.
func (r *Repository) FindAlarms(licenseId int, severity string, alarmType int, offset, limit int) ([]Alarm, int64, error) {
	var alarms []Alarm
	var total int64

	query := r.db.Model(&Alarm{})
	if licenseId > 0 {
		query = query.Where("license_id = ?", licenseId)
	}

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
	if err := query.Order("event_time DESC, id DESC").Offset(offset).Limit(limit).Find(&alarms).Error; err != nil {
		logger.Errorf("FindAlarms query error: %v", err)
		return nil, 0, err
	}
	return alarms, total, nil
}

// ClearAlarm marks an alarm as cleared, mirroring nms-serv clearAlarm:
// alarm_status migrates 1->2 (active) or 3->4 (history), alarm_type is set to
// HISTORY, and cleared_time + update_time are recorded. If the alarm is
// already cleared (status 2 or 4) its status is left unchanged while the
// history flag and timestamps are still refreshed.
func (r *Repository) ClearAlarm(id int64, clearedTime time.Time) error {
	return r.db.Model(&Alarm{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"alarm_status": gorm.Expr("CASE WHEN alarm_status = 1 THEN 2 WHEN alarm_status = 3 THEN 4 ELSE alarm_status END"),
			"alarm_type":   AlarmTypeHistory,
			"cleared_time": clearedTime,
			"update_time":  clearedTime,
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
					"alarm_status": gorm.Expr("CASE WHEN alarm_status = 1 THEN 2 WHEN alarm_status = 3 THEN 4 ELSE alarm_status END"),
					"alarm_type":   AlarmTypeHistory,
					"cleared_time": clearedTime,
					"clear_user":   clearUser,
					"update_time":  clearedTime,
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
	q := r.db.Where("enable = ?", enabled)
	if licenseId > 0 {
		q = q.Where("license_id = ?", licenseId)
	}
	if err := q.Find(&filters).Error; err != nil {
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
// If licenseId is 0 (platform admin), returns filters across all tenants.
func (r *Repository) FindAlarmFilters(licenseId int) ([]AlarmFilter, error) {
	var filters []AlarmFilter
	q := r.db.Model(&AlarmFilter{})
	if licenseId > 0 {
		q = q.Where("license_id = ?", licenseId)
	}
	if err := q.Find(&filters).Error; err != nil {
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

// SetAlarmStatus updates only the alarm_status and update_time columns. It is
// used by ConfirmAlarm/UnconfirmAlarm to drive the alarm_status state machine.
func (r *Repository) SetAlarmStatus(id int64, status int, updateTime time.Time) error {
	return r.db.Model(&Alarm{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"alarm_status": status,
			"update_time":  updateTime,
		}).Error
}

// CountBySeverity returns the per-severity count of ACTIVE alarms
// (alarm_type = AlarmTypeActive). It mirrors nms-serv getCountOfSeverity,
// which groups only active alarms by severity.
func (r *Repository) CountBySeverity(licenseId int) (map[string]int64, error) {
	type statRow struct {
		Severity string
		Cnt      int64
	}
	var rows []statRow
	query := r.db.Model(&Alarm{}).
		Select("severity, COUNT(*) as cnt").
		Where("alarm_type = ?", AlarmTypeActive)
	if licenseId > 0 {
		query = query.Where("license_id = ?", licenseId)
	}
	if err := query.Group("severity").Scan(&rows).Error; err != nil {
		logger.Errorf("CountBySeverity error: %v", err)
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		key := row.Severity
		if key == "" {
			key = "(unknown)"
		}
		out[key] = row.Cnt
	}
	return out, nil
}

// FindAlarmStatisticTopN returns the top-N alarm identifiers by count,
// optionally filtered by a time range.
func (r *Repository) FindAlarmStatisticTopN(topN int, startTime, endTime *time.Time) ([]AlarmStatisticTopN, error) {
	var rows []AlarmStatisticTopN
	query := r.db.Model(&Alarm{}).
		Select("COALESCE(alarm_identifier, '(unknown)') as alarm_identifier, COUNT(*) as alarm_count").
		Where("alarm_identifier IS NOT NULL")
	if startTime != nil {
		query = query.Where("event_time >= ?", *startTime)
	}
	if endTime != nil {
		query = query.Where("event_time <= ?", *endTime)
	}
	if err := query.Group("alarm_identifier").
		Order("alarm_count DESC").
		Limit(topN).
		Scan(&rows).Error; err != nil {
		logger.Errorf("FindAlarmStatisticTopN error: %v", err)
		return nil, err
	}
	return rows, nil
}

// ---------------------------------------------------------------------------
// AlarmLibrary – import
// ---------------------------------------------------------------------------

// BulkCreateAlarmLibrary inserts multiple alarm library entries in a single
// transaction, using upsert (ON DUPLICATE KEY UPDATE) semantics based on the
// unique index (tenancy_id, alarm_identifier).
func (r *Repository) BulkCreateAlarmLibrary(items []AlarmLibrary) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i := range items {
			// Try to find existing entry by tenancy_id + alarm_identifier.
			var existing AlarmLibrary
			err := tx.Where("tenancy_id = ? AND alarm_identifier = ?",
				items[i].TenancyId, items[i].AlarmIdentifier).First(&existing).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					if createErr := tx.Create(&items[i]).Error; createErr != nil {
						return createErr
					}
					continue
				}
				return err
			}
			// Update existing entry.
			existing.ProbableCause = items[i].ProbableCause
			existing.Severity = items[i].Severity
			existing.EventType = items[i].EventType
			existing.Explanation = items[i].Explanation
			existing.SpecificProblem = items[i].SpecificProblem
			existing.AlarmSource = items[i].AlarmSource
			if saveErr := tx.Save(&existing).Error; saveErr != nil {
				return saveErr
			}
		}
		return nil
	})
}

// ---------------------------------------------------------------------------
// Alarm – comment
// ---------------------------------------------------------------------------

// UpdateAlarmComment sets the comment field on a single alarm.
func (r *Repository) UpdateAlarmComment(id int64, comment string) error {
	return r.db.Model(&Alarm{}).Where("id = ?", id).
		Update("comment", comment).Error
}

// ---------------------------------------------------------------------------
// SystemConfig – alarm sync config
// ---------------------------------------------------------------------------

// GetSystemConfig reads a system_config entry by id (the key).
// Returns ("", nil) when the key does not exist.
func (r *Repository) GetSystemConfig(key string) (string, error) {
	var cfg SystemConfig
	if err := r.db.Where("id = ?", key).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	if cfg.Config == nil {
		return "", nil
	}
	return *cfg.Config, nil
}

// SaveSystemConfig upserts a system_config entry by id (the key).
func (r *Repository) SaveSystemConfig(key, value string) error {
	var cfg SystemConfig
	err := r.db.Where("id = ?", key).First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cfg = SystemConfig{
				Id:     key,
				Config: &value,
			}
			return r.db.Create(&cfg).Error
		}
		return err
	}
	cfg.Config = &value
	return r.db.Save(&cfg).Error
}

// DeleteAlarm hard-deletes a single alarm row. Mirrors Java deleteAlarm.
func (r *Repository) DeleteAlarm(id int64) error {
	return r.db.Where("id = ?", id).Delete(&Alarm{}).Error
}

// FindAlarmTemplate returns one alarm template by id. Mirrors Java viewAlarmTemplate.
func (r *Repository) FindAlarmTemplate(id int) (*AlarmTemplate, error) {
	var t AlarmTemplate
	if err := r.db.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// DeleteAlarmTemplate hard-deletes an alarm template. Mirrors Java deleteAlarmTemplate.
func (r *Repository) DeleteAlarmTemplate(id int) error {
	return r.db.Where("id = ?", id).Delete(&AlarmTemplate{}).Error
}

// FindAlarmFilter returns one alarm filter by id. Mirrors Java viewAlarmFilterTask.
func (r *Repository) FindAlarmFilter(id int) (*AlarmFilter, error) {
	var f AlarmFilter
	if err := r.db.Where("id = ?", id).First(&f).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

// ToggleAlarmFilterEnable flips the `enable` flag on an alarm filter.
// Mirrors Java enableAlarmFilterTask / disableAlarmFilterTask.
func (r *Repository) ToggleAlarmFilterEnable(id int, enable bool) error {
	return r.db.Model(&AlarmFilter{}).
		Where("id = ?", id).
		Update("enable", enable).Error
}

// UpdateAlarmTemplateEmailNotification flips the `enable_email_notification`
// flag on an alarm template. Mirrors Java updateEmailNotificationEnableInTemplate.
func (r *Repository) UpdateAlarmTemplateEmailNotification(id int, enable bool) error {
	return r.db.Model(&AlarmTemplate{}).
		Where("id = ?", id).
		Update("enable_email_notification", enable).Error
}

// AlarmStatistic aggregates by licenseId.
type AlarmStatistic struct {
	Total     int64
	Active    int64
	History   int64
	BySeverity map[string]int64
	ByType    map[string]int64
	BySource  map[string]int64
}

// FindAlarmStatistic aggregates the alarm table by licenseId. Mirrors Java
// queryAlarmStatisticResult. Counts split by alarm_type (1=active, 2=history),
// severity, and alarm_source.
func (r *Repository) FindAlarmStatistic(licenseId int) (*AlarmStatistic, error) {
	out := &AlarmStatistic{
		BySeverity: map[string]int64{},
		ByType:     map[string]int64{},
		BySource:   map[string]int64{},
	}
	// Totals.
	totalsQ := r.db.Model(&Alarm{})
	if licenseId > 0 {
		totalsQ = totalsQ.Where("license_id = ?", licenseId)
	}
	if err := totalsQ.Count(new(int64)).Error; err != nil {
		return nil, err
	}
	// Use a single grouped query to avoid three round-trips.
	type row struct {
		AlarmType  *int
		Severity   *string
		AlarmSource *string
		Cnt        int64
	}
	var rows []row
	groupQ := r.db.Model(&Alarm{}).
		Select("alarm_type, severity, alarm_source, COUNT(*) AS cnt")
	if licenseId > 0 {
		groupQ = groupQ.Where("license_id = ?", licenseId)
	}
	if err := groupQ.Group("alarm_type, severity, alarm_source").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out.Total += r.Cnt
		// alarm_type: 1=active, 2=history.
		if r.AlarmType != nil {
			switch *r.AlarmType {
			case 1:
				out.Active += r.Cnt
				out.ByType["active"] += r.Cnt
			case 2:
				out.History += r.Cnt
				out.ByType["history"] += r.Cnt
			default:
				out.ByType[fmt.Sprintf("%d", *r.AlarmType)] += r.Cnt
			}
		}
		if r.Severity != nil && *r.Severity != "" {
			out.BySeverity[*r.Severity] += r.Cnt
		}
		if r.AlarmSource != nil && *r.AlarmSource != "" {
			out.BySource[*r.AlarmSource] += r.Cnt
		}
	}
	return out, nil
}

// DeleteAlarmLibrary hard-deletes an alarm library entry by id. Mirrors Java
// deleteAlarmLibrary.
func (r *Repository) DeleteAlarmLibrary(id int) error {
	return r.db.Where("id = ?", id).Delete(&AlarmLibrary{}).Error
}

// FindActiveProbableCauses returns the distinct probable_cause values for
// active alarms (alarm_type=1) in the given license. Mirrors Java
// listActiveAlarmProbableCause.
func (r *Repository) FindActiveProbableCauses(licenseId int) ([]string, error) {
	var causes []string
	q := r.db.Model(&Alarm{}).
		Where("alarm_type = ? AND probable_cause IS NOT NULL AND probable_cause <> ''", AlarmTypeActive)
	if licenseId > 0 {
		q = q.Where("license_id = ?", licenseId)
	}
	err := q.Distinct("probable_cause").
		Pluck("probable_cause", &causes).Error
	return causes, err
}

// FindAlarmEventTypes returns the distinct event_type values for the given
// license. Mirrors Java getAlarmEventType.
func (r *Repository) FindAlarmEventTypes(licenseId int) ([]string, error) {
	var types []string
	q := r.db.Model(&Alarm{}).
		Where("event_type IS NOT NULL AND event_type <> ''")
	if licenseId > 0 {
		q = q.Where("license_id = ?", licenseId)
	}
	err := q.Distinct("event_type").
		Pluck("event_type", &types).Error
	return types, err
}

// FindEmailNoticeResults returns paginated email_notice_result rows
// filtered by template id and email subject (LIKE). Mirrors Java
// listEmailNoticeResult. Sort: id DESC.
func (r *Repository) FindEmailNoticeResults(alarmTemplateId *int, emailSubject string, offset, limit int) ([]EmailNoticeResult, int64, error) {
	var items []EmailNoticeResult
	var total int64
	q := r.db.Model(&EmailNoticeResult{})
	if alarmTemplateId != nil {
		q = q.Where("alarm_template_id = ?", *alarmTemplateId)
	}
	if emailSubject != "" {
		q = q.Where("email_subject LIKE ?", "%"+emailSubject+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}
