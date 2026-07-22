package cacert

import "time"

// CaFile represents a CA certificate file record (ca_file table)
type CaFile struct {
	Id          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	FileName    *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	URL         *string    `gorm:"column:url;type:varchar(512)" json:"url"`
	DelFlag     *string    `gorm:"column:del_flag;type:varchar(10);default:'N'" json:"del_flag"`
	CreateBy    *string    `gorm:"column:create_by;type:varchar(255)" json:"create_by"`
	CreateTime  *time.Time `gorm:"column:create_time" json:"create_time"`
	Description *string    `gorm:"column:description;type:varchar(512)" json:"description"`
}

func (CaFile) TableName() string { return "ca_file" }

// CaTask represents a CA certificate deployment task (ca_task table)
type CaTask struct {
	Id         int        `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskName   *string    `gorm:"column:task_name;type:varchar(255)" json:"task_name"`
	CaFileId   *int       `gorm:"column:ca_file_id" json:"ca_file_id"`
	Status     *string    `gorm:"column:stasus;type:varchar(50)" json:"status"` // note: Java has typo "stasus"
	CreateBy   *string    `gorm:"column:create_by;type:varchar(255)" json:"create_by"`
	CreateTime *time.Time `gorm:"column:create_time" json:"create_time"`
	UpdateBy   *string    `gorm:"column:update_by;type:varchar(255)" json:"update_by"`
	UpdateTime *time.Time `gorm:"column:update_time" json:"update_time"`
	TenantId  *int       `gorm:"column:tenant_id" json:"tenant_id"`
}

func (CaTask) TableName() string { return "ca_task" }

// DeviceSendCaLog represents per-device CA delivery log (device_send_ca_log table)
type DeviceSendCaLog struct {
	Id            int    `gorm:"primaryKey;autoIncrement" json:"id"`
	DeviceId      *int64 `gorm:"column:device_id" json:"device_id"`
	Result        *int   `gorm:"column:result" json:"result"`
	Scope         *string `gorm:"column:scope;type:varchar(50)" json:"scope"`
	DeviceGroupId *string `gorm:"column:device_group_id;type:varchar(255)" json:"device_group_id"`
	TaskId        *int   `gorm:"column:task_id" json:"task_id"`
	Info          *string `gorm:"column:info;type:varchar(512)" json:"info"`
	EventLogId    *int64 `gorm:"column:event_log_id" json:"event_log_id"`
}

func (DeviceSendCaLog) TableName() string { return "device_send_ca_log" }

// ---------- DTOs ----------

// CaFileListQuery represents the query for listing CA files
type CaFileListQuery struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

// CaFileDeleteRequest represents the request to delete a CA file
type CaFileDeleteRequest struct {
	Id int `json:"id" binding:"required"`
}

// CaTaskSaveRequest represents the request to create a CA task
type CaTaskSaveRequest struct {
	TaskName   string  `json:"taskName" binding:"required"`
	CaFileId   int     `json:"caFileId" binding:"required"`
	Scope      string  `json:"scope"` // "device_group" or "device"
	DeviceIds  []int64 `json:"deviceIds"`
	GroupIds   []string `json:"groupIds"`
}

// CaTaskListQuery represents the query for listing CA tasks
type CaTaskListQuery struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

// CaTaskDetailQuery represents the query for CA task detail
type CaTaskDetailQuery struct {
	Id int `json:"id" binding:"required"`
}

// CaTaskDeleteRequest represents the request to delete a CA task
type CaTaskDeleteRequest struct {
	Id int `json:"id" binding:"required"`
}

// DeviceCaLogQuery represents the query for device CA delivery logs
type DeviceCaLogQuery struct {
	TaskId   int `json:"taskId"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}
