package reboot

import "time"

// ---------- Database entities ----------

// RebootTask corresponds to the reboot_task table.
type RebootTask struct {
	Id             int        `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	Name           string     `gorm:"column:name;type:varchar(255)" json:"name"`
	User           string     `gorm:"column:user;type:varchar(255)" json:"user"`
	OperationTime  time.Time  `gorm:"column:operation_time" json:"operationTime"`
	Status         int        `gorm:"column:status" json:"status"` // 1=Waiting 2=Executing 3=Executed 4=Cancelled
	StartTime      *time.Time `gorm:"column:start_time" json:"startTime"`
	EndTime        *time.Time `gorm:"column:end_time" json:"endTime"`
	ExecuteMode    int        `gorm:"column:execute_mode" json:"executeMode"` // 1=immediate 2=wait 3=scheduled
	TriggerTime    *time.Time `gorm:"column:trigger_time" json:"triggerTime"`
	TenantId      int        `gorm:"column:tenant_id" json:"tenantId"`
	ElementIds     string     `gorm:"column:element_ids;type:longtext" json:"elementIds"`
	DeviceType     string     `gorm:"column:device_type;type:varchar(50)" json:"deviceType"`
	Scope          string     `gorm:"column:scope;type:varchar(50)" json:"scope"` // element / deviceGroup
	DeviceGroupIds string     `gorm:"column:device_group_ids;type:longtext" json:"deviceGroupIds"`
	SoftReboot     bool       `gorm:"column:soft_reboot" json:"softReboot"`
}

func (RebootTask) TableName() string { return "reboot_task" }

// TaskToEventLog links a task to event_log entries.
type TaskToEventLog struct {
	Id         int64  `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TaskId     int    `gorm:"column:task_id" json:"taskId"`
	EventLogId int64  `gorm:"column:event_log_id" json:"eventLogId"`
	TaskType   string `gorm:"column:task_type;type:varchar(50)" json:"taskType"`
}

func (TaskToEventLog) TableName() string { return "task_to_event_log" }

// ---------- Request DTOs ----------

type AddRebootTaskRequest struct {
	Name           string   `json:"name" binding:"required"`
	ExecuteMode    int      `json:"executeMode" binding:"required"`
	TriggerTime    *string  `json:"triggerTime"`
	ElementIds     []int64  `json:"elementIds"`
	DeviceType     string   `json:"deviceType" binding:"required"`
	Scope          string   `json:"scope" binding:"required"`
	DeviceGroupIds []string `json:"deviceGroupIds"`
	SoftReboot     bool     `json:"softReboot"`
}

type ListRebootTaskQuery struct {
	TaskName   string  `json:"taskName"`
	StartTime  *string `json:"startTime"`
	EndTime    *string `json:"endTime"`
	DeviceType string  `json:"deviceType"`
	Page       int     `json:"page"`
	PageSize   int     `json:"pageSize"`
}

type ListRebootTaskResultQuery struct {
	TaskId       int    `json:"taskId" binding:"required"`
	SerialNumber string `json:"serialNumber"`
	Page         int    `json:"page"`
	PageSize     int    `json:"pageSize"`
}

// ---------- Response VOs ----------

type RebootTaskVO struct {
	Id            int        `json:"id"`
	Name          string     `json:"name"`
	User          string     `json:"user"`
	OperationTime time.Time  `json:"operationTime"`
	Status        int        `json:"status"`
	Progress      string     `json:"progress"`
	Results       *int       `json:"results"`
	StartTime     *time.Time `json:"startTime"`
	EndTime       *time.Time `json:"endTime"`
	TenancyName   string     `json:"tenancyName"`
}

type RebootTaskResultVO struct {
	SerialNumber  string     `json:"serialNumber"`
	DeviceName    string     `json:"deviceName"`
	Status        int        `json:"status"`
	Results       *int       `json:"results"`
	FailureReason string     `json:"failureReason"`
	Time          *time.Time `json:"time"`
	ElementId     int64      `json:"elementId"`
}

// ---------- Internal ----------

type operationMessage struct {
	EventType      string `json:"eventType"`
	NeNeid         int64  `json:"neNeid"`
	Operation      string `json:"operation"`
	OperationParam string `json:"operationParam"`
	OperationUser  string `json:"operationUser"`
	CommandTrackId int64  `json:"commandTrackId"`
	ExpiredAt      int64  `json:"expiredAt"`
}
