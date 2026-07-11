package alarm

import (
	"errors"
	"time"

	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Service defines the business-logic contract for alarm management.
type Service interface {
	ListAlarms(licenseId int, severity string, alarmType int, page, pageSize int) ([]Alarm, int64, error)
	GetAlarm(id int64) (*Alarm, error)
	ClearAlarm(id int64) error
	BatchClearAlarms(alarmIds []int64, clearUser string) (int64, []int64, error)
	CreateAlarm(a *Alarm) error
	CheckAlarmSuppression(alarm *Alarm) (bool, string)
	ListAlarmLibrary(tenancyId int) ([]AlarmLibrary, error)
	ListAlarmTemplates(tenancyId int) ([]AlarmTemplate, error)
	CreateAlarmTemplate(t *AlarmTemplate) error
	UpdateAlarmTemplate(t *AlarmTemplate) error
	ListAlarmFilters(licenseId int) ([]AlarmFilter, error)
	CreateAlarmFilter(f *AlarmFilter) error
	UpdateAlarmFilter(f *AlarmFilter) error
	DeleteAlarmFilter(id int) error
}

// service contains the business logic for alarm management.
type service struct {
	repo *Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// ---------------------------------------------------------------------------
// Alarm
// ---------------------------------------------------------------------------

// ListAlarms returns a paginated alarm list. The page number (1-based) is
// converted to an offset before querying.
func (s *service) ListAlarms(licenseId int, severity string, alarmType int, page, pageSize int) ([]Alarm, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	data, total, err := s.repo.FindAlarms(licenseId, severity, alarmType, offset, pageSize)
	if err != nil {
		return nil, 0, apperror.Wrap(err, "LIST_ALARMS_FAILED", 500, "failed to list alarms")
	}
	return data, total, nil
}

// GetAlarm returns a single alarm by ID.
func (s *service) GetAlarm(id int64) (*Alarm, error) {
	alarm, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrAlarmNotFound
		}
		return nil, apperror.Wrap(err, "GET_ALARM_FAILED", 500, "failed to get alarm")
	}
	return alarm, nil
}

// ClearAlarm marks an alarm as cleared with the current timestamp.
func (s *service) ClearAlarm(id int64) error {
	if err := s.repo.ClearAlarm(id, time.Now()); err != nil {
		return apperror.Wrap(err, "CLEAR_ALARM_FAILED", 500, "failed to clear alarm")
	}
	return nil
}

// BatchClearAlarms clears multiple alarms in a single transaction. It returns
// the count of cleared alarms and any IDs that were not found in the database.
func (s *service) BatchClearAlarms(alarmIds []int64, clearUser string) (int64, []int64, error) {
	cleared, notFound, err := s.repo.BatchClearAlarms(alarmIds, clearUser, time.Now())
	if err != nil {
		return 0, nil, apperror.Wrap(err, "BATCH_CLEAR_ALARMS_FAILED", 500, "failed to batch clear alarms")
	}
	return cleared, notFound, nil
}

// CreateAlarm inserts a new alarm after checking alarm suppression rules. If
// the alarm matches an active filter it is still created but marked as
// suppressed so the creation is not silently lost.
func (s *service) CreateAlarm(a *Alarm) error {
	// Check suppression before inserting.
	suppressed, reason := s.CheckAlarmSuppression(a)
	if suppressed {
		t := true
		a.IsSuppressed = &t
		a.SuppressionReason = &reason
		logger.Infof("Alarm suppressed: reason=%s, source=%v, identifier=%v",
			reason, a.AlarmSource, a.AlarmIdentifier)
	}
	if err := s.repo.Create(a); err != nil {
		return apperror.Wrap(err, "CREATE_ALARM_FAILED", 500, "failed to create alarm")
	}
	return nil
}

// CheckAlarmSuppression checks whether the given alarm matches any active
// alarm filter. It returns suppressed=true and the matching filter name when
// a match is found.
//
// Matching rules:
//  1. Time range: the alarm's event_time must fall within the filter's
//     start_time / end_time window (if both are set).
//  2. Alarm source / device: if the filter has ExecutionOnAllAlarm=false,
//     the alarm's element_id must appear in the filter's device list
//     (alarm_filter_has_device) or the alarm's alarm_source must match one
//     of the filter's AlarmSources entries.
//  3. Alarm identifier: if the filter is not set to match all alarms, the
//     alarm's alarm_id must appear in the filter's alarm list
//     (alarm_filter_has_alarm).
func (s *service) CheckAlarmSuppression(alarm *Alarm) (bool, string) {
	licenseId := 0
	if alarm.LicenseId != nil {
		licenseId = *alarm.LicenseId
	}

	filters, err := s.repo.FindActiveAlarmFilters(licenseId)
	if err != nil {
		logger.Errorf("CheckAlarmSuppression: failed to query filters: %v", err)
		return false, ""
	}

	for _, f := range filters {
		// 1. Time range check.
		if f.StartTime != nil && alarm.EventTime != nil && alarm.EventTime.Before(*f.StartTime) {
			continue
		}
		if f.EndTime != nil && alarm.EventTime != nil && alarm.EventTime.After(*f.EndTime) {
			continue
		}

		// 2. Alarm identifier check (skip if filter matches all alarms).
		allAlarms := f.ExecutionOnAllAlarm != nil && *f.ExecutionOnAllAlarm
		if !allAlarms {
			filterAlarmIds, err := s.repo.FindAlarmFilterAlarms(f.Id)
			if err != nil {
				logger.Errorf("CheckAlarmSuppression: failed to query filter alarms: %v", err)
				continue
			}
			if alarm.AlarmId != nil && !containsString(filterAlarmIds, *alarm.AlarmId) {
				continue
			}
			if alarm.AlarmId == nil {
				continue
			}
		}

		// 3. Device / source check (skip if filter targets all devices).
		allBaseStation := f.ExecuteOnAllBaseStation != nil && *f.ExecuteOnAllBaseStation
		allCPE := f.ExecuteOnAllCPE != nil && *f.ExecuteOnAllCPE
		if !allBaseStation && !allCPE {
			// Check explicit device list.
			filterDevices, err := s.repo.FindAlarmFilterDevices(f.Id)
			if err != nil {
				logger.Errorf("CheckAlarmSuppression: failed to query filter devices: %v", err)
				continue
			}
			deviceMatch := false
			if alarm.ElementId != nil && containsInt64(filterDevices, *alarm.ElementId) {
				deviceMatch = true
			}
			// Check alarm source string match.
			if !deviceMatch && f.AlarmSources != nil && alarm.AlarmSource != nil {
				if containsString(splitCSV(*f.AlarmSources), *alarm.AlarmSource) {
					deviceMatch = true
				}
			}
			if !deviceMatch {
				continue
			}
		}

		// All conditions matched — alarm is suppressed.
		filterName := ""
		if f.FilterRuleName != nil {
			filterName = *f.FilterRuleName
		}
		return true, filterName
	}

	return false, ""
}

// containsString checks whether a string slice contains the target.
func containsString(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

// containsInt64 checks whether an int64 slice contains the target.
func containsInt64(slice []int64, target int64) bool {
	for _, v := range slice {
		if v == target {
			return true
		}
	}
	return false
}

// splitCSV splits a comma-separated string into trimmed non-empty tokens.
func splitCSV(s string) []string {
	var result []string
	for _, part := range splitString(s, ',') {
		trimmed := trimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// splitString splits s on the given separator without importing strings.
func splitString(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// trimSpace removes leading/trailing whitespace.
func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// ---------------------------------------------------------------------------
// AlarmLibrary
// ---------------------------------------------------------------------------

// ListAlarmLibrary returns all alarm library entries for the given tenancy.
func (s *service) ListAlarmLibrary(tenancyId int) ([]AlarmLibrary, error) {
	data, err := s.repo.FindAlarmLibrary(tenancyId)
	if err != nil {
		return nil, apperror.Wrap(err, "LIST_ALARM_LIBRARY_FAILED", 500, "failed to list alarm library")
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// AlarmTemplate
// ---------------------------------------------------------------------------

// ListAlarmTemplates returns all alarm templates for the given tenancy.
func (s *service) ListAlarmTemplates(tenancyId int) ([]AlarmTemplate, error) {
	data, err := s.repo.FindAlarmTemplates(tenancyId)
	if err != nil {
		return nil, apperror.Wrap(err, "LIST_ALARM_TEMPLATES_FAILED", 500, "failed to list alarm templates")
	}
	return data, nil
}

// CreateAlarmTemplate persists a new alarm template.
func (s *service) CreateAlarmTemplate(t *AlarmTemplate) error {
	if err := s.repo.CreateAlarmTemplate(t); err != nil {
		return apperror.Wrap(err, "CREATE_ALARM_TEMPLATE_FAILED", 500, "failed to create alarm template")
	}
	return nil
}

// UpdateAlarmTemplate persists changes to an existing alarm template.
func (s *service) UpdateAlarmTemplate(t *AlarmTemplate) error {
	if err := s.repo.UpdateAlarmTemplate(t); err != nil {
		return apperror.Wrap(err, "UPDATE_ALARM_TEMPLATE_FAILED", 500, "failed to update alarm template")
	}
	return nil
}

// ---------------------------------------------------------------------------
// AlarmFilter
// ---------------------------------------------------------------------------

// ListAlarmFilters returns all alarm filters for the given license.
func (s *service) ListAlarmFilters(licenseId int) ([]AlarmFilter, error) {
	data, err := s.repo.FindAlarmFilters(licenseId)
	if err != nil {
		return nil, apperror.Wrap(err, "LIST_ALARM_FILTERS_FAILED", 500, "failed to list alarm filters")
	}
	return data, nil
}

// CreateAlarmFilter persists a new alarm filter.
func (s *service) CreateAlarmFilter(f *AlarmFilter) error {
	if err := s.repo.CreateAlarmFilter(f); err != nil {
		return apperror.Wrap(err, "CREATE_ALARM_FILTER_FAILED", 500, "failed to create alarm filter")
	}
	return nil
}

// UpdateAlarmFilter persists changes to an existing alarm filter.
func (s *service) UpdateAlarmFilter(f *AlarmFilter) error {
	if err := s.repo.UpdateAlarmFilter(f); err != nil {
		return apperror.Wrap(err, "UPDATE_ALARM_FILTER_FAILED", 500, "failed to update alarm filter")
	}
	return nil
}

// DeleteAlarmFilter removes an alarm filter by ID.
func (s *service) DeleteAlarmFilter(id int) error {
	if err := s.repo.DeleteAlarmFilter(id); err != nil {
		return apperror.Wrap(err, "DELETE_ALARM_FILTER_FAILED", 500, "failed to delete alarm filter")
	}
	return nil
}
