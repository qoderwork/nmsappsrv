package upgrade

import "time"

// TaskFields 任务基类字段 (Java: Task MappedSuperclass)
// 所有任务类型都包含这些字段

// UpgradeTask 对应 upgrade_task 表
type UpgradeTask struct {
	Id                      int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name                    *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	User                    *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	OperationTime           *time.Time `gorm:"column:operation_time" json:"operation_time"`
	Status                  *int       `gorm:"column:status" json:"status"`
	StartTime               *time.Time `gorm:"column:start_time" json:"start_time"`
	EndTime                 *time.Time `gorm:"column:end_time" json:"end_time"`
	ExecuteMode             *int       `gorm:"column:execute_mode" json:"execute_mode"`
	TriggerTime             *time.Time `gorm:"column:trigger_time" json:"trigger_time"`
	TenancyId               *int       `gorm:"column:tenancy_id" json:"tenancy_id"`
	ElementIds              *string    `gorm:"column:element_ids;type:text" json:"element_ids"`
	DeviceType              *string    `gorm:"column:device_type;type:varchar(255)" json:"device_type"`
	UpgradeType             *string    `gorm:"column:upgrade_type;type:varchar(255)" json:"upgrade_type"`
	UpgradeFileId           *int       `gorm:"column:upgrade_file_id" json:"upgrade_file_id"`
	Source                  *string    `gorm:"column:source;type:varchar(255)" json:"source"`
	ActivationMode          *int       `gorm:"column:activation_mode" json:"activation_mode"`
	ActivationTime          *time.Time `gorm:"column:activation_time" json:"activation_time"`
	Scope                   *string    `gorm:"column:scope;type:varchar(255)" json:"scope"`
	DeviceGroupIds          *string    `gorm:"column:device_group_ids;type:longtext" json:"device_group_ids"`
	ConcurrentNumber        *int       `gorm:"column:concurrent_number" json:"concurrent_number"`
	MaxRetryTimes           *int       `gorm:"column:max_retry_times" json:"max_retry_times"`
	ManualUpgrade           *bool      `gorm:"column:manual_upgrade" json:"manual_upgrade"`
	AllowSameVersionUpgrade *int       `gorm:"column:allow_same_version_upgrade" json:"allow_same_version_upgrade"`
}

func (UpgradeTask) TableName() string { return "upgrade_task" }

// UpgradeTaskVo is the API response VO for upgrade task queries (computed fields).
type UpgradeTaskVo struct {
	Id            int        `json:"id"`
	Name          string     `json:"name"`
	User          string     `json:"user"`
	OperationTime string     `json:"operationTime"`
	Status        *int       `json:"status"`
	StartTime     string     `json:"startTime"`
	EndTime       string     `json:"endTime"`
	ExecuteMode   *int       `json:"executeMode"`
	DeviceType    string     `json:"deviceType"`
	UpgradeType   string     `json:"upgradeType"`
	Version       string     `json:"version"`       // from UpgradeFile join
	DeviceCount   int        `json:"deviceCount"`    // len(elementIds)
	Progress      string     `json:"progress"`       // e.g. "3/10"
	SuccessCount  int        `json:"successCount"`
	FailCount     int        `json:"failCount"`
	TenancyName   string     `json:"tenancyName"`
}

// UpgradeTaskFilter holds query parameters for filtering upgrade task lists.
type UpgradeTaskFilter struct {
	SearchText  string
	TaskName    string
	StartTime   string
	EndTime     string
	DeviceType  string
}

// UpgradeFile 对应 upgrade_file 表
type UpgradeFile struct {
	Id               int        `gorm:"primaryKey;autoIncrement" json:"id"`
	FileName         *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	FilePath         *string    `gorm:"column:file_path;type:varchar(255)" json:"file_path"`
	Version          *string    `gorm:"column:version;type:varchar(255)" json:"version"`
	DeviceType       *string    `gorm:"column:device_type;type:varchar(255)" json:"device_type"`
	FileSize         *int64     `gorm:"column:file_size" json:"file_size"`
	FileType         *string    `gorm:"column:file_type;type:varchar(255)" json:"file_type"`
	TenancyId        *int       `gorm:"column:tenancy_id" json:"tenancy_id"`
	UploadTime       *time.Time `gorm:"column:upload_time" json:"upload_time"`
	User             *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	ProductType      *string    `gorm:"column:product_type;type:varchar(255)" json:"product_type"`
	OriginalFileName *string    `gorm:"column:original_file_name;type:varchar(255)" json:"original_file_name"`
}

func (UpgradeFile) TableName() string { return "upgrade_file" }

// UpgradeLog 对应 upgrade_log 表 (UUID主键)
type UpgradeLog struct {
	Id               string     `gorm:"primaryKey;type:varchar(32)" json:"id"`
	NeId             *int64     `gorm:"column:ne_id" json:"ne_id"`
	CreationTime     *time.Time `gorm:"column:creation_time" json:"creation_time"`
	OldVersion       *string    `gorm:"column:old_version;type:varchar(255)" json:"old_version"`
	NewVersion       *string    `gorm:"column:new_version;type:varchar(255)" json:"new_version"`
	DoneTime         *time.Time `gorm:"column:done_time;index" json:"done_time"`
	IsDone           *bool      `gorm:"column:is_done" json:"is_done"`
	Message          *string    `gorm:"column:message;type:text" json:"message"`
	CommandTrackId   *int64     `gorm:"column:command_track_id" json:"command_track_id"`
	IsDownloaded     *bool      `gorm:"column:is_downloaded;default:false" json:"is_downloaded"`
	DownloadedTime   *time.Time `gorm:"column:downloaded_time" json:"downloaded_time"`
	UpgradeType      *string    `gorm:"column:upgrade_type;type:varchar(255)" json:"upgrade_type"`
	RruVersionInfo   *string    `gorm:"column:rru_version_info;type:longtext" json:"rru_version_info"`
	TaskId           *int       `gorm:"column:task_id" json:"task_id"`
	Upgrade          *bool      `gorm:"column:upgrade" json:"upgrade"`
	TenancyId        *int       `gorm:"column:tenancy_id" json:"tenancy_id"`
	DeviceType       *string    `gorm:"column:device_type;type:varchar(255)" json:"device_type"`
	Success          *bool      `gorm:"column:success" json:"success"`
	RetryTimes       *int       `gorm:"column:retry_times" json:"retry_times"`
	AutoUpgradeTaskId *int      `gorm:"column:auto_upgrade_task_id" json:"auto_upgrade_task_id"`
}

func (UpgradeLog) TableName() string { return "upgrade_log" }

// RollbackTask 对应 rollback_task 表
type RollbackTask struct {
	Id             int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name           *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	User           *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	OperationTime  *time.Time `gorm:"column:operation_time" json:"operation_time"`
	Status         *int       `gorm:"column:status" json:"status"`
	StartTime      *time.Time `gorm:"column:start_time" json:"start_time"`
	EndTime        *time.Time `gorm:"column:end_time" json:"end_time"`
	ExecuteMode    *int       `gorm:"column:execute_mode" json:"execute_mode"`
	TriggerTime    *time.Time `gorm:"column:trigger_time" json:"trigger_time"`
	TenancyId      *int       `gorm:"column:tenancy_id" json:"tenancy_id"`
	ElementIds     *string    `gorm:"column:element_ids;type:text" json:"element_ids"`
	Scope          *string    `gorm:"column:scope;type:varchar(255)" json:"scope"`
	DeviceGroupIds *string    `gorm:"column:device_group_ids;type:longtext" json:"device_group_ids"`
}

func (RollbackTask) TableName() string { return "rollback_task" }

// RebootTask 对应 reboot_task 表
type RebootTask struct {
	Id             int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name           *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	User           *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	OperationTime  *time.Time `gorm:"column:operation_time" json:"operation_time"`
	Status         *int       `gorm:"column:status" json:"status"`
	StartTime      *time.Time `gorm:"column:start_time" json:"start_time"`
	EndTime        *time.Time `gorm:"column:end_time" json:"end_time"`
	ExecuteMode    *int       `gorm:"column:execute_mode" json:"execute_mode"`
	TriggerTime    *time.Time `gorm:"column:trigger_time" json:"trigger_time"`
	TenancyId      *int       `gorm:"column:tenancy_id" json:"tenancy_id"`
	ElementIds     *string    `gorm:"column:element_ids;type:longtext" json:"element_ids"`
	DeviceType     *string    `gorm:"column:device_type;type:varchar(255)" json:"device_type"`
	Scope          *string    `gorm:"column:scope;type:varchar(255)" json:"scope"`
	DeviceGroupIds *string    `gorm:"column:device_group_ids;type:longtext" json:"device_group_ids"`
	SoftReboot     *bool      `gorm:"column:soft_reboot" json:"soft_reboot"`
}

func (RebootTask) TableName() string { return "reboot_task" }

// ShutdownMyTask 对应 shutdown_my_task 表
type ShutdownMyTask struct {
	Id             int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name           *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	User           *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	OperationTime  *time.Time `gorm:"column:operation_time" json:"operation_time"`
	Status         *int       `gorm:"column:status" json:"status"`
	StartTime      *time.Time `gorm:"column:start_time" json:"start_time"`
	EndTime        *time.Time `gorm:"column:end_time" json:"end_time"`
	ExecuteMode    *int       `gorm:"column:execute_mode" json:"execute_mode"`
	TriggerTime    *time.Time `gorm:"column:trigger_time" json:"trigger_time"`
	TenancyId      *int       `gorm:"column:tenancy_id" json:"tenancy_id"`
	ElementIds     *string    `gorm:"column:element_ids;type:text" json:"element_ids"`
}

func (ShutdownMyTask) TableName() string { return "shutdown_my_task" }

// ShutdownLog 对应 shutdown_log 表
type ShutdownLog struct {
	Id         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId     *int       `gorm:"column:task_id" json:"task_id"`
	EventLogId *int64     `gorm:"column:event_log_id" json:"event_log_id"`
	ElementId  *int64     `gorm:"column:element_id" json:"element_id"`
	Status     *int       `gorm:"column:status" json:"status"`
	Time       *time.Time `gorm:"column:time" json:"time"`
}

func (ShutdownLog) TableName() string { return "shutdown_log" }

// EUAndRUBatchUpgradeLog 对应 eu_and_ru_batch_upgrade_log 表
type EUAndRUBatchUpgradeLog struct {
	Id               int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	TenancyId        *int       `gorm:"column:tenancy_id" json:"tenancy_id"`
	ElementId        *int64     `gorm:"column:element_id" json:"element_id"`
	User             *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	OriginalVersion  *string    `gorm:"column:original_version;type:longtext" json:"original_version"`
	UpgradedVersion  *string    `gorm:"column:upgraded_version;type:longtext" json:"upgraded_version"`
	OperationTime    *time.Time `gorm:"column:operation_time" json:"operation_time"`
	DownloadedTime   *time.Time `gorm:"column:downloaded_time" json:"downloaded_time"`
	UpgradedTime     *time.Time `gorm:"column:upgraded_time" json:"upgraded_time"`
	EventLogId       *int64     `gorm:"column:event_log_id" json:"event_log_id"`
	Result           *int       `gorm:"column:result" json:"result"`
	FaultInfo        *string    `gorm:"column:fault_info;type:text" json:"fault_info"`
}

func (EUAndRUBatchUpgradeLog) TableName() string { return "eu_and_ru_batch_upgrade_log" }
