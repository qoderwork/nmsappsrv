package mml

import "time"

// MmlCommand 对应 mml_command 表
type MmlCommand struct {
	Id       int     `gorm:"primaryKey;autoIncrement" json:"id"`
	Command  *string `gorm:"column:command;type:varchar(255)" json:"command"`
	Hint     *string `gorm:"column:hint;type:varchar(1000)" json:"hint"`
	Name     *string `gorm:"column:name;type:varchar(255)" json:"name"`
	HelpFile *string `gorm:"column:help_file;type:varchar(255)" json:"help_file"`
	MmlSetId *int    `gorm:"column:mml_set_id" json:"mml_set_id"`
	Type     *string `gorm:"column:type;type:varchar(255)" json:"type"`
}

func (MmlCommand) TableName() string { return "mml_command" }

// MmlCommandParam 对应 mml_command_param 表
type MmlCommandParam struct {
	Id           int     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         *string `gorm:"column:name;type:varchar(255)" json:"name"`
	Parameter    *string `gorm:"column:parameter;type:varchar(255)" json:"parameter"`
	ValueRange   *string `gorm:"column:value_range;type:varchar(255)" json:"value_range"`
	Type         *string `gorm:"column:type;type:varchar(255)" json:"type"`
	Od           int     `gorm:"column:od" json:"od"`
	Necessity    bool    `gorm:"column:necessity" json:"necessity"`
	MmlCommandId *int    `gorm:"column:mml_command_id" json:"mml_command_id"`
	Writable     bool    `gorm:"column:writable" json:"writable"`
	Option       *string `gorm:"column:op;type:longtext" json:"option"`
	DefaultValue *string `gorm:"column:default_value;type:varchar(255)" json:"default_value"`
	Relate       *string `gorm:"column:relate;type:varchar(255)" json:"relate"`
}

func (MmlCommandParam) TableName() string { return "mml_command_param" }

// MmlSet 对应 mml_set 表
type MmlSet struct {
	Id        int     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      *string `gorm:"column:name;type:varchar(255)" json:"name"`
	ParentId  *int    `gorm:"column:parent_id" json:"parent_id"`
	LicenseId *int    `gorm:"column:license_id" json:"license_id"`
	Version   *string `gorm:"column:version;type:varchar(255)" json:"version"`
}

func (MmlSet) TableName() string { return "mml_set" }

// MmlExecuteResult 对应 mml_execute_result 表
type MmlExecuteResult struct {
	Id                int        `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId         *int64     `gorm:"column:element_id" json:"element_id"`
	ResultReturnedTime *time.Time `gorm:"column:result_returned_time" json:"result_returned_time"`
	Command           *string    `gorm:"column:command;type:varchar(500)" json:"command"`
	Result            *string    `gorm:"column:result;type:longtext" json:"result"`
	Uid               *string    `gorm:"column:uid;type:varchar(255)" json:"uid"`
	Status            int        `gorm:"column:status" json:"status"`
	HasFault          bool       `gorm:"column:has_fault" json:"has_fault"`
	FaultString       *string    `gorm:"column:fault_string;type:text" json:"fault_string"`
	User              *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	EventLogId        *int64     `gorm:"column:event_log_id" json:"event_log_id"`
	OperationTime     *time.Time `gorm:"column:operation_time" json:"operation_time"`
	SendTime          *time.Time `gorm:"column:send_time" json:"send_time"`
}

func (MmlExecuteResult) TableName() string { return "mml_execute_result" }

// GetMMLResultByEventLogIdsVO 对齐 Java GetMMLResultByEventLogIdsVO：
// 按 eventLogId 列表轮询 MML 执行结果时返回的标量视图。
type GetMMLResultByEventLogIdsVO struct {
	Mml          string `json:"mml"`
	DeviceName   string `json:"deviceName"`
	SerialNumber string `json:"serialNumber"`
	Result       string `json:"result"`
}
