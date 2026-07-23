package monitor

import "time"

// MonitorTask 对应 monitor_task 表
type MonitorTask struct {
	Id            int     `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskName      *string `gorm:"column:task_name;type:varchar(255)" json:"task_name"`
	TenantId     *int    `gorm:"column:license_id" json:"license_id"`
	Enable        *bool   `gorm:"column:enable" json:"enable"`
	ExecutionScope *int   `gorm:"column:execution_scope" json:"execution_scope"`
	ScopeData     *string `gorm:"column:scope_data;type:longtext" json:"scope_data"`
}

func (MonitorTask) TableName() string { return "monitor_task" }

// MonitorData 对应 monitor_data 表
type MonitorData struct {
	Id          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId   *int64     `gorm:"column:element_id;index" json:"element_id"`
	SampleTime  *time.Time `gorm:"column:sample_time;index" json:"sample_time"`
	ParameterId *string    `gorm:"column:parameter_id;type:varchar(255);index" json:"parameter_id"`
	Value       *float64   `gorm:"column:value" json:"value"`
}

func (MonitorData) TableName() string { return "monitor_data" }

// MonitorElements 对应 monitor_elements 表
type MonitorElements struct {
	Id        int    `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId *int64 `gorm:"column:element_id" json:"element_id"`
	TaskId    *int   `gorm:"column:task_id" json:"task_id"`
}

func (MonitorElements) TableName() string { return "monitor_elements" }

// MonitorParameters 对应 monitor_parameters 表
type MonitorParameters struct {
	Id          int     `gorm:"primaryKey;autoIncrement" json:"id"`
	ParameterId *string `gorm:"column:parameter_id;type:varchar(255)" json:"parameter_id"`
	TaskId      *int    `gorm:"column:task_id" json:"task_id"`
}

func (MonitorParameters) TableName() string { return "monitor_parameters" }
