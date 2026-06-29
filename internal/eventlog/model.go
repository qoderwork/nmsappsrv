package eventlog

import "time"

// EventLog 对应 event_log 表
type EventLog struct {
	Id                  int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	EventType           *string    `gorm:"column:event_type;type:varchar(255)" json:"event_type"`
	OperationTime       *time.Time `gorm:"column:operation_time" json:"operation_time"`
	CommandIssueTime    *time.Time `gorm:"column:command_issue_time" json:"command_issue_time"`
	CommandResponseTime *time.Time `gorm:"column:command_response_time" json:"command_response_time"`
	User                *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	ElementId           *int64     `gorm:"column:element_id;index" json:"element_id"`
	Status              *int       `gorm:"column:status" json:"status"`
	FaultInfo           *string    `gorm:"column:fault_info;type:text" json:"fault_info"`
	CommandTrackData    *string    `gorm:"column:command_track_data;type:longtext" json:"command_track_data"`
}

func (EventLog) TableName() string { return "event_log" }

// TaskToEventLog 对应 task_to_event_log 表
type TaskToEventLog struct {
	Id         int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId     *int    `gorm:"column:task_id" json:"task_id"`
	EventLogId *int64  `gorm:"column:event_log_id" json:"event_log_id"`
	TaskType   *string `gorm:"column:task_type;type:varchar(255)" json:"task_type"`
}

func (TaskToEventLog) TableName() string { return "task_to_event_log" }
