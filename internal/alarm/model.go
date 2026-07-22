package alarm

import "time"

// Alarm 对应 alarm 表
type Alarm struct {
	Id                    int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Severity              *string    `gorm:"column:severity;type:varchar(255);index:idx_elem_alarm" json:"severity"`
	AlarmIdentifier       *string    `gorm:"column:alarm_identifier;type:varchar(255);index:idx_elem_alarm" json:"alarm_identifier"`
	ProbableCause         *string    `gorm:"column:probable_cause;type:varchar(255)" json:"probable_cause"`
	AlarmSource           *string    `gorm:"column:alarm_source;type:varchar(255)" json:"alarm_source"`
	NetworkElement        *string    `gorm:"column:network_element;type:varchar(255)" json:"network_element"`
	EventType             *string    `gorm:"column:event_type;type:varchar(255)" json:"event_type"`
	AlarmStatus           *int       `gorm:"column:alarm_status" json:"alarm_status"`
	AlarmType             *int       `gorm:"column:alarm_type;index:idx_elem_alarm" json:"alarm_type"`
	EventTime             *time.Time `gorm:"column:event_time" json:"event_time"`
	UpdateTime            *time.Time `gorm:"column:update_time" json:"update_time"`
	ClearedTime           *time.Time `gorm:"column:cleared_time" json:"cleared_time"`
	SpecificProblem       *string    `gorm:"column:specific_problem;type:varchar(255)" json:"specific_problem"`
	AlarmId               *string    `gorm:"column:alarm_id;type:varchar(255);index:idx_alarm_id_type" json:"alarm_id"`
	ElementId             *int64     `gorm:"column:element_id;index:idx_elem_alarm" json:"element_id"`
	TenantId             *int       `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime            *time.Time `gorm:"column:create_time" json:"create_time"`
	AdditionalInformation *string    `gorm:"column:additional_information;type:text" json:"additional_information"`
	AlarmTemplateId       *int       `gorm:"column:alarm_template_id" json:"alarm_template_id"`
	ClearUser             *string    `gorm:"column:clear_user;type:varchar(255)" json:"clear_user"`
	IsSuppressed          *bool      `gorm:"column:is_suppressed" json:"is_suppressed"`
	SuppressionReason     *string    `gorm:"column:suppression_reason;type:varchar(255)" json:"suppression_reason"`
	Comment               *string    `gorm:"column:comment;type:longtext" json:"comment"`
	// Exported + HandleSuggestion are part of the Java api/core Alarm DTO
	// (the north-bound response shape) but not the corefunction domain
	// entity. They are response-only enrichment fields; populated by a
	// future join once the columns land in the shared schema.
	Exported         *bool   `gorm:"column:exported" json:"exported"`
	HandleSuggestion *string `gorm:"column:handle_suggestion;type:varchar(1024)" json:"handleSuggestion"`
}

// Alarm status values, mirroring nms-serv's alarm_status 4-state model.
// nms-serv encodes both "cleared/history" and "acknowledged" into a single
// integer, which is the contract the REST clients depend on.
const (
	AlarmStatusActiveUnconfirmed  = 1 // Active, not acknowledged
	AlarmStatusHistoryUnconfirmed = 2 // Cleared (history), not acknowledged
	AlarmStatusActiveConfirmed    = 3 // Active, acknowledged
	AlarmStatusHistoryConfirmed   = 4 // Cleared (history), acknowledged
)

// Alarm type values, mirroring nms-serv's alarm_type field.
const (
	AlarmTypeActive  = 1 // ACTIVE
	AlarmTypeHistory = 2 // HISTORY
)

// SeverityCount is one row of the per-severity active-alarm tally returned by
// GetSeverityCount, mirroring nms-serv's getCountOfSeverity response.
type SeverityCount struct {
	Severity   string `json:"severity"`
	AlarmCount int64  `json:"alarmCount"`
}

func (Alarm) TableName() string { return "alarm" }

// AlarmFilter 对应 alarm_filter 表
type AlarmFilter struct {
	Id                          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	FilterRuleName              *string    `gorm:"column:filter_rule_name;type:varchar(255)" json:"filter_rule_name"`
	Enable                      *bool      `gorm:"column:enable" json:"enable"`
	ExecutionAction             *int       `gorm:"column:execution_action" json:"execution_action"`
	AlarmSources                *string    `gorm:"column:alarm_sources;type:varchar(255)" json:"alarm_sources"`
	ExecuteOnAllBaseStation     *bool      `gorm:"column:execute_on_all_base_station" json:"execute_on_all_base_station"`
	ExecuteOnAllCPE             *bool      `gorm:"column:execute_on_all_cpe" json:"execute_on_all_cpe"`
	ExecutionOnAllAlarm         *bool      `gorm:"column:execution_on_all_alarm" json:"execution_on_all_alarm"`
	StartTime                   *time.Time `gorm:"column:start_time" json:"start_time"`
	EndTime                     *time.Time `gorm:"column:end_time" json:"end_time"`
	TenantId                   *int       `gorm:"column:tenant_id" json:"tenant_id"`
	User                        *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	UpdateTime                  *time.Time `gorm:"column:update_time" json:"update_time"`
	BaseStationIds              *string    `gorm:"column:base_station_ids;type:text" json:"base_station_ids"`
	CpeIds                      *string    `gorm:"column:cpe_ids;type:text" json:"cpe_ids"`
	BaseStationDeviceGroupIds   *string    `gorm:"column:base_station_device_group_ids;type:text" json:"base_station_device_group_ids"`
	CpeDeviceGroupIds           *string    `gorm:"column:cpe_device_group_ids;type:text" json:"cpe_device_group_ids"`
	AlarmIds                    *string    `gorm:"column:alarm_ids;type:text" json:"alarm_ids"`
}

func (AlarmFilter) TableName() string { return "alarm_filter" }

// AlarmFilterHasAlarm 对应 alarm_filter_has_alarm 表
type AlarmFilterHasAlarm struct {
	Id            int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	AlarmFilterId *int    `gorm:"column:alarm_filter_id" json:"alarm_filter_id"`
	AlarmId       *string `gorm:"column:alarm_id;type:varchar(255)" json:"alarm_id"`
}

func (AlarmFilterHasAlarm) TableName() string { return "alarm_filter_has_alarm" }

// AlarmFilterHasDevice 对应 alarm_filter_has_device 表
type AlarmFilterHasDevice struct {
	Id            int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	AlarmFilterId *int   `gorm:"column:alarm_filter_id" json:"alarm_filter_id"`
	ElementId     *int64 `gorm:"column:element_id" json:"element_id"`
}

func (AlarmFilterHasDevice) TableName() string { return "alarm_filter_has_device" }

// AlarmFilterHasDeviceGroup 对应 alarm_filter_has_device_group 表
type AlarmFilterHasDeviceGroup struct {
	Id            int64 `gorm:"primaryKey;autoIncrement" json:"id"`
	AlarmFilterId *int  `gorm:"column:alarm_filter_id" json:"alarm_filter_id"`
	DeviceGroupId *int  `gorm:"column:device_group_id" json:"device_group_id"`
}

func (AlarmFilterHasDeviceGroup) TableName() string { return "alarm_filter_has_device_group" }

// AlarmLibrary 对应 alarm_library 表
type AlarmLibrary struct {
	Id              int     `gorm:"primaryKey;autoIncrement" json:"id"`
	AlarmIdentifier *string `gorm:"column:alarm_identifier;type:varchar(255);uniqueIndex:idx_tenancy_alarm_id" json:"alarm_identifier"`
	ProbableCause   *string `gorm:"column:probable_cause;type:varchar(255)" json:"probable_cause"`
	Severity        *string `gorm:"column:severity;type:varchar(255)" json:"severity"`
	EventType       *string `gorm:"column:event_type;type:varchar(255)" json:"event_type"`
	Explanation     *string `gorm:"column:explanation;type:text" json:"explanation"`
	SpecificProblem *string `gorm:"column:specific_problem;type:varchar(255)" json:"specific_problem"`
	TenantId       *int    `gorm:"column:tenant_id;uniqueIndex:idx_tenancy_alarm_id" json:"tenant_id"`
	AlarmSource     *string `gorm:"column:alarm_source;type:varchar(255)" json:"alarm_source"`
}

func (AlarmLibrary) TableName() string { return "alarm_library" }

// AlarmTemplate 对应 alarm_template 表
type AlarmTemplate struct {
	Id                          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantId                   *int       `gorm:"column:tenant_id" json:"tenant_id"`
	Name                        *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	Description                 *string    `gorm:"column:description;type:varchar(255)" json:"description"`
	ExecuteOnAllBaseStation     *bool      `gorm:"column:execute_on_all_base_station" json:"execute_on_all_base_station"`
	ExecuteOnAllCPE             *bool      `gorm:"column:execute_on_all_cpe" json:"execute_on_all_cpe"`
	BaseStationIds              *string    `gorm:"column:base_station_ids;type:longtext" json:"base_station_ids"`
	CpeIds                      *string    `gorm:"column:cpe_ids;type:longtext" json:"cpe_ids"`
	BaseStationDeviceGroupIds   *string    `gorm:"column:base_station_device_group_ids;type:longtext" json:"base_station_device_group_ids"`
	CpeDeviceGroupIds           *string    `gorm:"column:cpe_device_group_ids;type:longtext" json:"cpe_device_group_ids"`
	ExecuteOnAllAlarm           *bool      `gorm:"column:execute_on_all_alarm" json:"execute_on_all_alarm"`
	AlarmIds                    *string    `gorm:"column:alarm_ids;type:longtext" json:"alarm_ids"`
	EnableEmailNotification     *bool      `gorm:"column:enable_email_notification" json:"enable_email_notification"`
	ToleranceDuration           *int       `gorm:"column:tolerance_duration" json:"tolerance_duration"`
	Interval_                   *int       `gorm:"column:interval" json:"interval"`
	EnableNotifyDefaultRecipients *bool    `gorm:"column:enable_notify_default_recipients" json:"enable_notify_default_recipients"`
	AlarmSources                *string    `gorm:"column:alarm_sources;type:varchar(255)" json:"alarm_sources"`
	Emails                      *string    `gorm:"column:emails;type:varchar(255)" json:"emails"`
	LastSendEmailDate           *time.Time `gorm:"column:last_send_email_date" json:"last_send_email_date"`
}

func (AlarmTemplate) TableName() string { return "alarm_template" }

// SystemConfig maps to the system_config table for key-value configuration storage.
// The live table (migrated by site.SystemConfig) uses an "id" varchar PK and a
// "config" longtext column — not config_key/config_value.
type SystemConfig struct {
	Id     string  `gorm:"primaryKey;type:varchar(32)" json:"id"`
	Config *string `gorm:"column:config;type:longtext" json:"config"`
}

func (SystemConfig) TableName() string { return "system_config" }

// AddCommentRequest is the JSON body for POST /alarms/:id/comment.
type AddCommentRequest struct {
	Comment string `json:"comment" binding:"required"`
}

// AlarmSyncConfig represents the alarm sync configuration stored in system_config.
type AlarmSyncConfig struct {
	Enabled       *bool   `json:"enabled"`
	SyncInterval  *int    `json:"syncInterval"`
	SourceAddress *string `json:"sourceAddress"`
	LastSyncTime  *string `json:"lastSyncTime"`
}

// AlarmStatisticTopN is one row of the top-N alarm statistics returned by
// QueryAlarmStatisticTopN, grouping alarms by alarm_identifier and counting.
type AlarmStatisticTopN struct {
	AlarmIdentifier string `json:"alarmIdentifier" gorm:"column:alarm_identifier"`
	AlarmCount      int64  `json:"alarmCount" gorm:"column:alarm_count"`
}

// AlarmStatisticResult is the aggregated alarm counts returned by
// QueryAlarmStatisticResult. Mirrors the Java response shape
// (total/active/history + per-severity + per-type + per-source maps).
type AlarmStatisticResult struct {
	TotalCount   int64            `json:"totalCount"`
	ActiveCount  int64            `json:"activeCount"`
	HistoryCount int64            `json:"historyCount"`
	BySeverity   map[string]int64 `json:"bySeverity"`
	ByAlarmType  map[string]int64 `json:"byAlarmType"`
	BySource     map[string]int64 `json:"bySource"`
}

// EmailNotificationConfig represents the email notification configuration
// stored in system_config with key "email_notification_config".
type EmailNotificationConfig struct {
	Enabled      *bool    `json:"enabled"`
	SmtpHost     *string  `json:"smtpHost"`
	SmtpPort     *int     `json:"smtpPort"`
	SmtpUser     *string  `json:"smtpUser"`
	SmtpPassword *string  `json:"smtpPassword"`
	Recipients   []string `json:"recipients"`
}

// EmailNoticeResult maps the email_notice_result table (one row per
// email notification attempt -- populated by the email notifier when
// an alarm template fires). Schema is the 6 columns defined in the
// V1_0__baseline migration. Mirrors Java
// com.waveoss.core.dao.entity.EmailNoticeResult.
type EmailNoticeResult struct {
	Id              int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	AlarmTemplateId *int       `gorm:"column:alarm_template_id" json:"alarmTemplateId"`
	EmailRecipient  *string    `gorm:"column:email_recipient;type:varchar(255)" json:"emailRecipient"`
	EmailSubject    *string    `gorm:"column:email_subject;type:longtext" json:"emailSubject"`
	FailureReason   *string    `gorm:"column:failure_reason;type:longtext" json:"failureReason"`
	PostTime        *time.Time `gorm:"column:post_time" json:"postTime"`
	Result          *int       `gorm:"column:result" json:"result"`
}

func (EmailNoticeResult) TableName() string { return "email_notice_result" }

// EmailNoticeResultQuery is the JSON body for POST /alarms/email-notice-results
// (mirrors Java ListEmailNoticeResultQuery).
type EmailNoticeResultQuery struct {
	AlarmTemplateId *int   `json:"alarmTemplateId"`
	EmailSubject    string `json:"emailSubject"`
}
