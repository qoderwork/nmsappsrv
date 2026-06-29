package parameter

import "time"

// Parameter 对应 parameter 表 (UUID主键)
type Parameter struct {
	Id   string  `gorm:"primaryKey;type:varchar(32)" json:"id"`
	Path *string `gorm:"column:path;type:varchar(255)" json:"path"`
}

func (Parameter) TableName() string { return "parameter" }

// ParameterAttributes 对应 parameter_attributes 表 (UUID主键)
type ParameterAttributes struct {
	Id             string  `gorm:"primaryKey;type:varchar(32)" json:"id"`
	ParameterName  *string `gorm:"column:parameter_name;type:varchar(255);uniqueIndex:idx_elem_param" json:"parameter_name"`
	ElementId      *int64  `gorm:"column:element_id;uniqueIndex:idx_elem_param" json:"element_id"`
	Notification   *int    `gorm:"column:notification" json:"notification"`
	AccessList     *string `gorm:"column:access_list;type:varchar(255)" json:"access_list"`
}

func (ParameterAttributes) TableName() string { return "parameter_attributes" }

// ParameterLog 对应 parameter_log 表 (UUID主键)
type ParameterLog struct {
	Id            string     `gorm:"primaryKey;type:varchar(32)" json:"id"`
	ParameterName *string    `gorm:"column:parameter_name;type:varchar(255)" json:"parameter_name"`
	OldValue      *string    `gorm:"column:old_value;type:mediumtext" json:"old_value"`
	NewValue      *string    `gorm:"column:new_value;type:mediumtext" json:"new_value"`
	ChangeUser    *string    `gorm:"column:change_user;type:varchar(255)" json:"change_user"`
	ChangeTime    *time.Time `gorm:"column:change_time" json:"change_time"`
	ElementId     *int64     `gorm:"column:element_id" json:"element_id"`
	HasFault      bool       `gorm:"column:has_fault" json:"has_fault"`
	FaultMsg      *string    `gorm:"column:fault_msg;type:text" json:"fault_msg"`
}

func (ParameterLog) TableName() string { return "parameter_log" }

// ParameterBackupLog 对应 parameter_backup_log 表
type ParameterBackupLog struct {
	Id            int    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId        *string `gorm:"column:task_id;type:varchar(255)" json:"task_id"`
	ElementId     *int64  `gorm:"column:element_id" json:"element_id"`
	Filename      *string `gorm:"column:filename;type:varchar(255)" json:"filename"`
	GenerateTime  *int64  `gorm:"column:generate_time" json:"generate_time"`
}

func (ParameterBackupLog) TableName() string { return "parameter_backup_log" }

// ParameterSet 对应 parameter_set 表 (UUID主键)
type ParameterSet struct {
	Id             string `gorm:"primaryKey;type:varchar(32)" json:"id"`
	Name           *string `gorm:"column:name;type:varchar(255)" json:"name"`
	ParentId       *string `gorm:"column:parent_id;type:varchar(32)" json:"parent_id"`
	StandardSet    bool    `gorm:"column:standard_set" json:"standard_set"`
	Sort           int     `gorm:"column:sort" json:"sort"`
	LicenseId      *int    `gorm:"column:license_id" json:"license_id"`
	SelfDevelopSet bool    `gorm:"column:self_develop_set" json:"self_develop_set"`
	DeviceType     *string `gorm:"column:device_type;type:varchar(255)" json:"device_type"`
	Chinese        *string `gorm:"column:chinese;type:varchar(255)" json:"chinese"`
	Addable        bool    `gorm:"column:addable" json:"addable"`
	Deletable      bool    `gorm:"column:deletable" json:"deletable"`
	HelpFile       *string `gorm:"column:help_file;type:varchar(255)" json:"help_file"`
}

func (ParameterSet) TableName() string { return "parameter_set" }

// ParameterSetHasParameter 对应 parameter_set_has_parameter 关联表
type ParameterSetHasParameter struct {
	Id            int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	ParameterSetId *string `gorm:"column:parameter_set_id;type:varchar(32)" json:"parameter_set_id"`
	ParameterId   *string `gorm:"column:parameter_id;type:varchar(32)" json:"parameter_id"`
}

func (ParameterSetHasParameter) TableName() string { return "parameter_set_has_parameter" }

// ParameterTemplate 对应 parameter_template 表
type ParameterTemplate struct {
	Id          int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        *string `gorm:"column:name;type:varchar(255)" json:"name"`
	Description *string `gorm:"column:description;type:varchar(255)" json:"description"`
	TenancyId   *int    `gorm:"column:tenancy_id" json:"tenancy_id"`
	IsDefault   *bool   `gorm:"column:is_default" json:"is_default"`
}

func (ParameterTemplate) TableName() string { return "parameter_template" }

// ParameterTemplateHasParameter 对应 parameter_template_has_parameter 表
type ParameterTemplateHasParameter struct {
	Id          int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateId  *int64  `gorm:"column:template_id" json:"template_id"`
	ParameterId *string `gorm:"column:parameter_id;type:varchar(32)" json:"parameter_id"`
}

func (ParameterTemplateHasParameter) TableName() string { return "parameter_template_has_parameter" }

// ---------- Batch Parameter Configuration DTOs ----------

// BatchParamValue is a single key-value pair in a batch parameter config request.
type BatchParamValue struct {
	ParamKey   string `json:"paramKey"`
	ParamValue string `json:"paramValue"`
}

// BatchParameterConfigRequest is the JSON body for POST /parameter-tasks.
type BatchParameterConfigRequest struct {
	ParamValues []BatchParamValue `json:"paramValues" binding:"required"`
	ElementIds  []int64           `json:"elementIds"`
	GroupIds    []string          `json:"groupIds"`
}

// BatchConfigTaskVo is the response item for the batch configuration task list.
type BatchConfigTaskVo struct {
	Id            int64  `json:"id"`
	Name          string `json:"name"`
	OperationUser string `json:"operationUser"`
	OperationTime string `json:"operationTime"`
	DeviceCount   int    `json:"deviceCount"`
	Progress      string `json:"progress"` // e.g. "3/5"
}

// BatchConfigTaskDetailVo is the response item for per-device results.
type BatchConfigTaskDetailVo struct {
	DeviceName   string `json:"deviceName"`
	SerialNumber string `json:"serialNumber"`
	ElementId    int64  `json:"elementId"`
	Result       *int   `json:"result"` // event_log status: 1=pending, 3=success, 4=fail
	FaultInfo    string `json:"faultInfo"`
	Data         string `json:"data"`
}
