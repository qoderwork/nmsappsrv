package nmsbackup

import "time"

// NMSBackupAndRevertTask 对应 nms_backup_and_revert_task 表 (对齐Java实体)
type NMSBackupAndRevertTask struct {
	Id          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	TaskType    *string    `gorm:"column:task_type;type:varchar(255)" json:"task_type"` // "backup" or "revert"
	ExecuteMode *int       `gorm:"column:execute_mode" json:"execute_mode"`             // 1=immediately, 2=awaiting start, 3=schedule time
	Period      *int       `gorm:"column:period" json:"period"`                         // interval in days (0=once)
	StartTime   *time.Time `gorm:"column:start_time" json:"start_time"`                 // actual execution start
	EndTime     *time.Time `gorm:"column:end_time" json:"end_time"`                     // actual execution end
	Status      *int       `gorm:"column:status" json:"status"`                         // 1=waiting, 2=running, 3=done, 4=cancelled
	CreateTime  *time.Time `gorm:"column:create_time" json:"create_time"`
	UpdateTime  *time.Time `gorm:"column:update_time" json:"update_time"`
	User        *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	LicenseId   *int       `gorm:"column:license_id" json:"license_id"`
	NmsBackupId *int       `gorm:"column:nms_backup_id" json:"nms_backup_id"` // FK to nms_backup_and_revert.id
}

func (NMSBackupAndRevertTask) TableName() string { return "nms_backup_and_revert_task" }

// NMSBackupAndRevert 对应 nms_backup_and_revert 表 (对齐Java实体，含调度定义)
type NMSBackupAndRevert struct {
	Id               int        `gorm:"primaryKey;autoIncrement" json:"id"`
	CreateTime       *time.Time `gorm:"column:create_time" json:"create_time"`
	CreateUserName   *string    `gorm:"column:create_user_name;type:varchar(255)" json:"create_user_name"`
	BackupName       *string    `gorm:"column:backup_name;type:varchar(255)" json:"backup_name"`
	BackupType       *int       `gorm:"column:backup_type" json:"backup_type"`             // 0=once(manual), 1=schedule(recurring)
	BackupInterval   *int       `gorm:"column:backup_interval" json:"backup_interval"`     // interval in days
	PmInterval       *int       `gorm:"column:pm_interval" json:"pm_interval"`             // PM files retention days
	MrInterval       *int       `gorm:"column:mr_interval" json:"mr_interval"`             // MR files retention days
	XlogInterval     *int       `gorm:"column:xlog_interval" json:"xlog_interval"`         // xlog files retention days
	BackupBeginTime  *time.Time `gorm:"column:backup_begin_time" json:"backup_begin_time"` // first scheduled execution time
	BackupStatus     *int       `gorm:"column:backup_status" json:"backup_status"`         // 0=ready, 1=running, 2=completed, 3=failed
	BackupTimeCost   *int       `gorm:"column:backup_time_cost" json:"backup_time_cost"`   // duration in seconds
	RevertBeginTime  *time.Time `gorm:"column:revert_begin_time" json:"revert_begin_time"`
	RevertEndTime    *time.Time `gorm:"column:revert_end_time" json:"revert_end_time"`
	RevertStatus     *int       `gorm:"column:revert_status" json:"revert_status"`
	FileName         *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`   // semicolon-separated zip paths
	FileSize         *string    `gorm:"column:file_size;type:varchar(255)" json:"file_size"`
	LicenseId        *int       `gorm:"column:license_id" json:"license_id"`
}

func (NMSBackupAndRevert) TableName() string { return "nms_backup_and_revert" }

// NMSBackupAndRevertLog 对应 nms_backup_and_revert_log 表
type NMSBackupAndRevertLog struct {
	Id            int        `gorm:"primaryKey;autoIncrement" json:"id"`
	FileName      *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	Time          *time.Time `gorm:"column:time" json:"time"`
	OperationUser *string    `gorm:"column:operation_user;type:varchar(255)" json:"operation_user"`
	Result        *int       `gorm:"column:result" json:"result"` // 0=success, 1=failure
	Reason        *string    `gorm:"column:reason;type:text" json:"reason"`
}

func (NMSBackupAndRevertLog) TableName() string { return "nms_backup_and_revert_log" }

// BackupRetentionConfig stored in system_config as key "nms_backup_retention"
type BackupRetentionConfig struct {
	BackupFileSavedDays *int `json:"backupFileSavedDays"` // matches Java GetBackupAndRestoreConfigDTO
}

// --- DTOs (对齐Java API) ---

type AddNMSBackupTaskRequest struct {
	BackupName      string `json:"backupName" binding:"required"`
	BackupType      int    `json:"backupType"`                // 0=once, 1=schedule
	BackupInterval  int    `json:"backupInterval,omitempty"`  // days
	BackupBeginTime string `json:"backupBeginTime,omitempty"` // RFC3339 or "2006-01-02 15:04:05"
	PmInterval      int    `json:"pmInterval,omitempty"`
	MrInterval      int    `json:"mrInterval,omitempty"`
	XlogInterval    int    `json:"xlogInterval,omitempty"`
}

type ModifyNMSBackupTaskRequest struct {
	Id              int     `json:"id" binding:"required"`
	BackupName      string  `json:"backupName"`
	BackupType      *int    `json:"backupType"`
	BackupInterval  *int    `json:"backupInterval"`
	BackupBeginTime *string `json:"backupBeginTime"` // RFC3339 or "2006-01-02 15:04:05"
	PmInterval      *int    `json:"pmInterval"`
	MrInterval      *int    `json:"mrInterval"`
	XlogInterval    *int    `json:"xlogInterval"`
}

type RunNMSBackupTaskRequest struct {
	Id int `json:"id" binding:"required"` // nms_backup_and_revert.id
}

type DeleteNMSBackupTaskRequest struct {
	Id int `json:"id" binding:"required"`
}

type RevertNMSBackupTaskRequest struct {
	Id int `json:"id" binding:"required"`
}

type ListNMSBackupTaskRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

type NMSBackupTaskVo struct {
	Id              int    `json:"id"`
	BackupName      string `json:"backupName"`
	BackupType      int    `json:"backupType"`
	BackupInterval  int    `json:"backupInterval"`
	BackupBeginTime string `json:"backupBeginTime"`
	BackupStatus    int    `json:"backupStatus"`
	PmInterval      int    `json:"pmInterval"`
	MrInterval      int    `json:"mrInterval"`
	XlogInterval    int    `json:"xlogInterval"`
	CreateTime      string `json:"createTime"`
}

type ListNMSBackupLogsRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

type NMSBackupLogVo struct {
	Id            int    `json:"id"`
	FileName      string `json:"fileName"`
	Time          string `json:"time"`
	OperationUser string `json:"operationUser"`
	Result        int    `json:"result"`
	Reason        string `json:"reason"`
}

type GetNMSBackupLogDetailRequest struct {
	LogId int `json:"logId" binding:"required"`
}

type UpdateBackupRetentionRequest struct {
	BackupFileSavedDays *int `json:"backupFileSavedDays"`
}

type GetBackupAndRestoreConfigDTO struct {
	BackupFileSavedDays int `json:"backupFileSavedDays"`
}
