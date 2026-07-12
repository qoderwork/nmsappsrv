package alarm

import (
	"encoding/json"
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
	ConfirmAlarm(id int64) error
	UnconfirmAlarm(id int64) error
	BatchClearAlarms(alarmIds []int64, clearUser string) (int64, []int64, error)
	GetSeverityCount(licenseId int) ([]SeverityCount, error)
	CreateAlarm(a *Alarm) error
	CheckAlarmSuppression(alarm *Alarm) (bool, string)
	ListAlarmLibrary(tenancyId int) ([]AlarmLibrary, error)
	ImportAlarmLibrary(tenancyId int, items []AlarmLibrary) (int, error)
	ListAlarmTemplates(tenancyId int) ([]AlarmTemplate, error)
	CreateAlarmTemplate(t *AlarmTemplate) error
	UpdateAlarmTemplate(t *AlarmTemplate) error
	ListAlarmFilters(licenseId int) ([]AlarmFilter, error)
	CreateAlarmFilter(f *AlarmFilter) error
	UpdateAlarmFilter(f *AlarmFilter) error
	DeleteAlarmFilter(id int) error
	GetAlarmSyncConfig() (*AlarmSyncConfig, error)
	UpdateAlarmSyncConfig(config *AlarmSyncConfig) error
	AddCommentForAlarm(id int64, comment string) error
	QueryAlarmStatisticTopN(topN int, startTime, endTime *time.Time) ([]AlarmStatisticTopN, error)
	GetEmailNotificationConfig() (*EmailNotificationConfig, error)
	UpdateEmailNotificationConfig(config *EmailNotificationConfig) error
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
	// Align new-alarm defaults with nms-serv: Active + unacknowledged.
	if a.AlarmStatus == nil {
		status := AlarmStatusActiveUnconfirmed
		a.AlarmStatus = &status
	}
	if a.AlarmType == nil {
		t := AlarmTypeActive
		a.AlarmType = &t
	}
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

// ConfirmAlarm acknowledges an alarm, mirroring nms-serv confirmAlarm:
// status 1->3 (active) or 2->4 (history). Alarms that are already
// acknowledged (3/4) are left unchanged (no-op), matching Java behaviour.
func (s *service) ConfirmAlarm(id int64) error {
	a, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.ErrAlarmNotFound
		}
		return apperror.Wrap(err, "CONFIRM_ALARM_FAILED", 500, "failed to confirm alarm")
	}
	var next int
	switch {
	case a.AlarmStatus != nil && *a.AlarmStatus == AlarmStatusActiveUnconfirmed:
		next = AlarmStatusActiveConfirmed
	case a.AlarmStatus != nil && *a.AlarmStatus == AlarmStatusHistoryUnconfirmed:
		next = AlarmStatusHistoryConfirmed
	default:
		return nil // already confirmed or status unknown -> no-op
	}
	if err := s.repo.SetAlarmStatus(id, next, time.Now()); err != nil {
		return apperror.Wrap(err, "CONFIRM_ALARM_FAILED", 500, "failed to confirm alarm")
	}
	return nil
}

// UnconfirmAlarm reverses acknowledgement, mirroring nms-serv unconfirmAlarm:
// status 3->1 (active) or 4->2 (history). Unacknowledged alarms are a no-op.
func (s *service) UnconfirmAlarm(id int64) error {
	a, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.ErrAlarmNotFound
		}
		return apperror.Wrap(err, "UNCONFIRM_ALARM_FAILED", 500, "failed to unconfirm alarm")
	}
	var next int
	switch {
	case a.AlarmStatus != nil && *a.AlarmStatus == AlarmStatusActiveConfirmed:
		next = AlarmStatusActiveUnconfirmed
	case a.AlarmStatus != nil && *a.AlarmStatus == AlarmStatusHistoryConfirmed:
		next = AlarmStatusHistoryUnconfirmed
	default:
		return nil // already unacknowledged or status unknown -> no-op
	}
	if err := s.repo.SetAlarmStatus(id, next, time.Now()); err != nil {
		return apperror.Wrap(err, "UNCONFIRM_ALARM_FAILED", 500, "failed to unconfirm alarm")
	}
	return nil
}

// GetSeverityCount returns the per-severity tally of ACTIVE alarms, mirroring
// nms-serv getCountOfSeverity. All four severities are always returned, with a
// zero count for any that currently have no active alarms.
func (s *service) GetSeverityCount(licenseId int) ([]SeverityCount, error) {
	counts, err := s.repo.CountBySeverity(licenseId)
	if err != nil {
		return nil, apperror.Wrap(err, "SEVERITY_COUNT_FAILED", 500, "failed to count alarms by severity")
	}
	severities := []string{"Critical", "Major", "Minor", "Warning"}
	out := make([]SeverityCount, 0, len(severities))
	for _, sev := range severities {
		out = append(out, SeverityCount{Severity: sev, AlarmCount: counts[sev]})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// AlarmLibrary – import
// ---------------------------------------------------------------------------

// ImportAlarmLibrary persists a batch of alarm library entries for the given
// tenancy. Entries that already exist (matched by tenancy_id + alarm_identifier)
// are updated in place. Returns the number of entries processed.
func (s *service) ImportAlarmLibrary(tenancyId int, items []AlarmLibrary) (int, error) {
	// Stamp every row with the current tenancy.
	tenancyIdCopy := tenancyId
	for i := range items {
		items[i].TenancyId = &tenancyIdCopy
	}
	if err := s.repo.BulkCreateAlarmLibrary(items); err != nil {
		return 0, apperror.Wrap(err, "IMPORT_ALARM_LIBRARY_FAILED", 500, "failed to import alarm library")
	}
	return len(items), nil
}

// ---------------------------------------------------------------------------
// AlarmSyncConfig
// ---------------------------------------------------------------------------

const alarmSyncConfigKey = "alarm_sync_config"

// GetAlarmSyncConfig reads the alarm sync configuration from system_config.
// When no configuration has been saved yet it returns a zero-value struct.
func (s *service) GetAlarmSyncConfig() (*AlarmSyncConfig, error) {
	raw, err := s.repo.GetSystemConfig(alarmSyncConfigKey)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_ALARM_SYNC_CONFIG_FAILED", 500, "failed to get alarm sync config")
	}
	if raw == "" {
		return &AlarmSyncConfig{}, nil
	}
	var cfg AlarmSyncConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		logger.Errorf("GetAlarmSyncConfig: unmarshal error: %v", err)
		return nil, apperror.Wrap(err, "UNMARSHAL_ALARM_SYNC_CONFIG_FAILED", 500, "failed to unmarshal alarm sync config")
	}
	return &cfg, nil
}

// UpdateAlarmSyncConfig persists the alarm sync configuration to system_config.
func (s *service) UpdateAlarmSyncConfig(config *AlarmSyncConfig) error {
	// Merge with existing values so callers can send partial updates.
	existing, err := s.GetAlarmSyncConfig()
	if err != nil {
		return err
	}
	if config.Enabled != nil {
		existing.Enabled = config.Enabled
	}
	if config.SyncInterval != nil {
		existing.SyncInterval = config.SyncInterval
	}
	if config.SourceAddress != nil {
		existing.SourceAddress = config.SourceAddress
	}
	if config.LastSyncTime != nil {
		existing.LastSyncTime = config.LastSyncTime
	}

	data, err := json.Marshal(existing)
	if err != nil {
		return apperror.Wrap(err, "MARSHAL_ALARM_SYNC_CONFIG_FAILED", 500, "failed to marshal alarm sync config")
	}
	if err := s.repo.SaveSystemConfig(alarmSyncConfigKey, string(data)); err != nil {
		return apperror.Wrap(err, "UPDATE_ALARM_SYNC_CONFIG_FAILED", 500, "failed to update alarm sync config")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Alarm – comment
// ---------------------------------------------------------------------------

// AddCommentForAlarm appends or replaces the comment on a single alarm.
func (s *service) AddCommentForAlarm(id int64, comment string) error {
	// Verify the alarm exists first.
	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.ErrAlarmNotFound
		}
		return apperror.Wrap(err, "ADD_COMMENT_FAILED", 500, "failed to add comment for alarm")
	}
	if err := s.repo.UpdateAlarmComment(id, comment); err != nil {
		return apperror.Wrap(err, "ADD_COMMENT_FAILED", 500, "failed to add comment for alarm")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Alarm Statistic Top-N
// ---------------------------------------------------------------------------

// QueryAlarmStatisticTopN returns the top-N alarm identifiers grouped by count.
func (s *service) QueryAlarmStatisticTopN(topN int, startTime, endTime *time.Time) ([]AlarmStatisticTopN, error) {
	if topN < 1 {
		topN = 10
	}
	data, err := s.repo.FindAlarmStatisticTopN(topN, startTime, endTime)
	if err != nil {
		return nil, apperror.Wrap(err, "ALARM_STATISTIC_TOP_N_FAILED", 500, "failed to query alarm statistic top-n")
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// Email Notification Config
// ---------------------------------------------------------------------------

const emailNotificationConfigKey = "email_notification_config"

// GetEmailNotificationConfig reads the email notification configuration from
// system_config. When no configuration has been saved yet it returns a
// zero-value struct.
func (s *service) GetEmailNotificationConfig() (*EmailNotificationConfig, error) {
	raw, err := s.repo.GetSystemConfig(emailNotificationConfigKey)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_EMAIL_NOTIFICATION_CONFIG_FAILED", 500, "failed to get email notification config")
	}
	if raw == "" {
		return &EmailNotificationConfig{}, nil
	}
	var cfg EmailNotificationConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		logger.Errorf("GetEmailNotificationConfig: unmarshal error: %v", err)
		return nil, apperror.Wrap(err, "UNMARSHAL_EMAIL_NOTIFICATION_CONFIG_FAILED", 500, "failed to unmarshal email notification config")
	}
	return &cfg, nil
}

// UpdateEmailNotificationConfig persists the email notification configuration
// to system_config. Partial updates are merged with existing values.
func (s *service) UpdateEmailNotificationConfig(config *EmailNotificationConfig) error {
	existing, err := s.GetEmailNotificationConfig()
	if err != nil {
		return err
	}
	if config.Enabled != nil {
		existing.Enabled = config.Enabled
	}
	if config.SmtpHost != nil {
		existing.SmtpHost = config.SmtpHost
	}
	if config.SmtpPort != nil {
		existing.SmtpPort = config.SmtpPort
	}
	if config.SmtpUser != nil {
		existing.SmtpUser = config.SmtpUser
	}
	if config.SmtpPassword != nil {
		existing.SmtpPassword = config.SmtpPassword
	}
	if config.Recipients != nil {
		existing.Recipients = config.Recipients
	}

	data, err := json.Marshal(existing)
	if err != nil {
		return apperror.Wrap(err, "MARSHAL_EMAIL_NOTIFICATION_CONFIG_FAILED", 500, "failed to marshal email notification config")
	}
	if err := s.repo.SaveSystemConfig(emailNotificationConfigKey, string(data)); err != nil {
		return apperror.Wrap(err, "UPDATE_EMAIL_NOTIFICATION_CONFIG_FAILED", 500, "failed to update email notification config")
	}
	return nil
}
