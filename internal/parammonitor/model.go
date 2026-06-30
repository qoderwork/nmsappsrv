package parammonitor

import "time"

// ParameterMonitorConfig 对应 parameter_monitor_config 表
type ParameterMonitorConfig struct {
	Id         int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name       *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	LicenseId  *int       `gorm:"column:license_id" json:"license_id"`
	Enable     *bool      `gorm:"column:enable" json:"enable"`
	Scope      *int       `gorm:"column:scope" json:"scope"` // 1=all devices, 2=selected
	ScopeData  *string    `gorm:"column:scope_data;type:longtext" json:"scope_data"` // JSON array of elementIds or groupIds
	Interval   *int       `gorm:"column:interval_seconds" json:"interval_seconds"`   // polling interval
	CreateTime *time.Time `gorm:"column:create_time" json:"create_time"`
	UpdateTime *time.Time `gorm:"column:update_time" json:"update_time"`
}

func (ParameterMonitorConfig) TableName() string { return "parameter_monitor_config" }

// MonitorConfigHasParameter 对应 monitor_config_has_parameter 关联表
type MonitorConfigHasParameter struct {
	Id          int     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigId    *int    `gorm:"column:config_id" json:"config_id"`
	ParameterId *string `gorm:"column:parameter_id;type:varchar(32)" json:"parameter_id"`
}

func (MonitorConfigHasParameter) TableName() string { return "monitor_config_has_parameter" }

// --- DTOs ---

type AddMonitorConfigRequest struct {
	Name         string   `json:"name" binding:"required"`
	Enable       bool     `json:"enable"`
	Scope        int      `json:"scope"`
	ScopeData    string   `json:"scopeData"`
	Interval     int      `json:"interval"`
	ParameterIds []string `json:"parameterIds" binding:"required"`
}

type UpdateMonitorConfigRequest struct {
	Id           int      `json:"id" binding:"required"`
	Name         string   `json:"name"`
	Enable       *bool    `json:"enable"`
	Scope        *int     `json:"scope"`
	ScopeData    *string  `json:"scopeData"`
	Interval     *int     `json:"interval"`
	ParameterIds []string `json:"parameterIds"`
}

type DeleteMonitorConfigRequest struct {
	Id int `json:"id" binding:"required"`
}

type ViewMonitorConfigRequest struct {
	Id int `json:"id" binding:"required"`
}

type ToggleMonitorConfigRequest struct {
	Id     int  `json:"id" binding:"required"`
	Enable bool `json:"enable"`
}

type ListMonitorConfigRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

type MonitorConfigVo struct {
	Id           int      `json:"id"`
	Name         string   `json:"name"`
	Enable       bool     `json:"enable"`
	Scope        int      `json:"scope"`
	Interval     int      `json:"interval"`
	ParameterIds []string `json:"parameterIds"`
	DeviceCount  int      `json:"deviceCount"`
	CreateTime   string   `json:"createTime"`
}

type MonitorConfigDetailVo struct {
	Id         int           `json:"id"`
	Name       string        `json:"name"`
	Enable     bool          `json:"enable"`
	Scope      int           `json:"scope"`
	ScopeData  string        `json:"scopeData"`
	Interval   int           `json:"interval"`
	Parameters []ParameterVo `json:"parameters"`
}

type ParameterVo struct {
	Id   string `json:"id"`
	Path string `json:"path"`
	Name string `json:"name"`
}

type RealtimeMonitorDataRequest struct {
	ConfigId int `json:"configId" binding:"required"`
}

type RealtimeMonitorDataVo struct {
	ElementId    int64              `json:"elementId"`
	DeviceName   string             `json:"deviceName"`
	SerialNumber string             `json:"serialNumber"`
	Online       bool               `json:"online"`
	Parameters   []ParameterValueVo `json:"parameters"`
}

type ParameterValueVo struct {
	ParameterName string `json:"parameterName"`
	Value         string `json:"value"`
}

type ReloadMonitorRequest struct {
	ConfigId   int     `json:"configId" binding:"required"`
	ElementIds []int64 `json:"elementIds"`
}

type BatchQueryDeviceParamRequest struct {
	ElementIds   []int64  `json:"elementIds" binding:"required"`
	ParameterIds []string `json:"parameterIds" binding:"required"`
}

type BatchQueryResultVo struct {
	ElementId    int64              `json:"elementId"`
	DeviceName   string             `json:"deviceName"`
	SerialNumber string             `json:"serialNumber"`
	Parameters   []ParameterValueVo `json:"parameters"`
}

// BatchQueryLiveRequest is the request body for live GPV batch query.
type BatchQueryLiveRequest struct {
	ElementIds   []int64  `json:"elementIds" binding:"required"`
	ParameterIds []string `json:"parameterIds" binding:"required"`
}

// BatchQueryLiveResult holds the dispatch status for a single device.
type BatchQueryLiveResult struct {
	ElementId    int64  `json:"elementId"`
	SerialNumber string `json:"serialNumber"`
	DeviceName   string `json:"deviceName"`
	Dispatched   bool   `json:"dispatched"`
	Error        string `json:"error,omitempty"`
	EventLogId   int64  `json:"eventLogId,omitempty"`
}
