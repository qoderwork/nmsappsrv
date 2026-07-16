package parameter

import "time"

// Parameter 对应 parameter 表 (UUID主键, 对齐Java 25字段)
type Parameter struct {
	Id                   string     `gorm:"primaryKey;type:varchar(32)" json:"id"`
	Name                 *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	Remark               *string    `gorm:"column:remark;type:longtext" json:"remark"`
	Path                 *string    `gorm:"column:path;type:varchar(255)" json:"path"`
	ParamType            *string    `gorm:"column:param_type;type:varchar(255)" json:"param_type"`
	Length               *int       `gorm:"column:length" json:"length"`
	ParamRange           *string    `gorm:"column:param_range;type:longtext" json:"param_range"`
	Unit                 *string    `gorm:"column:unit;type:varchar(255)" json:"unit"`
	Note                 *string    `gorm:"column:note;type:varchar(255)" json:"note"`
	IsWritable           *bool      `gorm:"column:is_writable" json:"is_writable"`
	IsArray              *bool      `gorm:"column:is_array" json:"is_array"`
	Hint                 *string    `gorm:"column:hint;type:longtext" json:"hint"`
	Sort                 int        `gorm:"column:sort" json:"sort"`
	Range                *string    `gorm:"column:rg;type:longtext" json:"range"`
	IsOpen               bool       `gorm:"column:is_open" json:"is_open"`
	StandardParameter    bool       `gorm:"column:standard_parameter" json:"standard_parameter"`
	LicenseId            *int       `gorm:"column:license_id" json:"license_id"`
	RegularExpression    *string    `gorm:"column:regular_expression;type:varchar(255)" json:"regular_expression"`
	SelfDevelopParameter bool       `gorm:"column:self_develop_parameter" json:"self_develop_parameter"`
	NeedReboot           *bool      `gorm:"column:need_reboot" json:"need_reboot"`
	MultipleCheck        *bool      `gorm:"column:multiple_check" json:"multiple_check"`
	Separator            *string    `gorm:"column:separator;type:varchar(255)" json:"separator"`
	User                 *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	UpdateTime           *time.Time `gorm:"column:update_time" json:"update_time"`
}

func (Parameter) TableName() string { return "parameter" }

// ParameterVo is the API response VO for parameter queries (对齐Java 16字段响应).
type ParameterVo struct {
	ParamName         string `json:"paramName"`
	CustomName        string `json:"customName"`
	Tr069Name         string `json:"tr069Name"`
	Value             string `json:"value"`
	Type              string `json:"type"`
	RegularExpression string `json:"regularExpression"`
	Length            *int   `json:"length"`
	Unit              string `json:"unit"`
	UpdateTime        string `json:"updateTime"`
	ParameterId       string `json:"parameterId"`
	MappingValue      string `json:"mappingValue"`
	Writable          bool   `json:"writable"`
	Remark            string `json:"remark"`
	NeedReboot        bool   `json:"needReboot"`
	Hint              string `json:"hint"`
	MultipleCheck     bool   `json:"multipleCheck"`
	Separator         string `json:"separator"`
}

// DeviceParameterDetailVo wraps device metadata with its parameters (对齐Java DeviceParameterDetailVO).
type DeviceParameterDetailVo struct {
	ElementId    int64         `json:"elementId"`
	SerialNumber string        `json:"serialNumber"`
	DeviceName   string        `json:"deviceName"`
	DeviceType   string        `json:"deviceType"`
	Online       bool          `json:"online"`
	Parameters   []ParameterVo `json:"parameters"`
}

// ParameterAttributes 对应 parameter_attributes 表 (UUID主键)
type ParameterAttributes struct {
	Id            string  `gorm:"primaryKey;type:varchar(32)" json:"id"`
	ParameterName *string `gorm:"column:parameter_name;type:varchar(255);uniqueIndex:idx_elem_param" json:"parameter_name"`
	ElementId     *int64  `gorm:"column:element_id;uniqueIndex:idx_elem_param" json:"element_id"`
	Notification  *int    `gorm:"column:notification" json:"notification"`
	AccessList    *string `gorm:"column:access_list;type:varchar(255)" json:"access_list"`
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
	Id           int     `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId       *string `gorm:"column:task_id;type:varchar(255)" json:"task_id"`
	ElementId    *int64  `gorm:"column:element_id" json:"element_id"`
	Filename     *string `gorm:"column:filename;type:varchar(255)" json:"filename"`
	GenerateTime *int64  `gorm:"column:generate_time" json:"generate_time"`
}

func (ParameterBackupLog) TableName() string { return "parameter_backup_log" }

// ParameterSet 对应 parameter_set 表 (UUID主键)
type ParameterSet struct {
	Id             string  `gorm:"primaryKey;type:varchar(32)" json:"id"`
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
	Revisable      bool    `gorm:"column:revisable" json:"revisable"`
	HelpFile       *string `gorm:"column:help_file;type:varchar(255)" json:"help_file"`
}

func (ParameterSet) TableName() string { return "parameter_set" }

// ParameterSetHasParameter 对应 parameter_set_has_parameter 关联表
type ParameterSetHasParameter struct {
	Id             int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	ParameterSetId *string `gorm:"column:parameter_set_id;type:varchar(32)" json:"parameter_set_id"`
	ParameterId    *string `gorm:"column:parameter_id;type:varchar(32)" json:"parameter_id"`
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
// parameter_value 存储该模板为该参数定义的(目标)值 —— 对齐 Java
// ParameterDeploymentTemplate.parameters 中 path->value 的语义: 下发时推送
// 的是模板"定义值", 而非设备当前值.
type ParameterTemplateHasParameter struct {
	Id             int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateId     *int64  `gorm:"column:template_id" json:"template_id"`
	ParameterId    *string `gorm:"column:parameter_id;type:varchar(32)" json:"parameter_id"`
	ParameterValue *string `gorm:"column:parameter_value;type:varchar(1024)" json:"parameter_value"`
}

func (ParameterTemplateHasParameter) TableName() string { return "parameter_template_has_parameter" }

// TemplateParameter 是部署模板中单个参数条目, 携带其 DEFINED(目标)值.
type TemplateParameter struct {
	ParameterId string `json:"parameterId" binding:"required"`
	Value       string `json:"value"`
	// Path is populated by repo queries that join `parameter` (e.g.
	// FindParameterTemplate); it is not part of the create/update request
	// payload and is left empty on writes.
	Path string `json:"path,omitempty" gorm:"column:path"`
}

// ParameterTemplateRequest 是创建/更新参数模板的 JSON 请求体: 模板头 + 参数列表(含定义值).
type ParameterTemplateRequest struct {
	ID          int64               `json:"id"`
	Name        *string             `json:"name"`
	Description *string             `json:"description"`
	TenancyId   *int                `json:"tenancy_id"`
	IsDefault   *bool               `json:"is_default"`
	Parameters  []TemplateParameter `json:"parameters"`
}

// ParameterDeploymentLog 对应 parameter_deployment_log 表 (对齐Java ParameterDeploymentLog entity)
// TemplateParameterVo is the API response item for a parameter template's
// parameter list, returned by `GetParameterTemplate` (mirrors Java
// `getParameterDeployTemplateInfo`).
type TemplateParameterVo struct {
	ParameterId string `json:"parameterId"`
	Path        string `json:"path"`
	Value       string `json:"value"`
}

// ParameterTemplateDetailVo is the API response for the single-template
// detail endpoint (`GET /parameter-templates/:id`). Includes the template
// metadata plus its full parameter list (with paths from the `parameter` table
// via the `parameter_template_has_parameter` join).
type ParameterTemplateDetailVo struct {
	Id          int64                `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	TenancyId   int                  `json:"tenancyId"`
	IsDefault   bool                 `json:"isDefault"`
	Parameters  []TemplateParameterVo `json:"parameters"`
}

type ParameterDeploymentLog struct {
	Id            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateId    *int64     `gorm:"column:template_id" json:"template_id"`
	ElementId     *int64     `gorm:"column:element_id" json:"element_id"`
	EventLogId    *int64     `gorm:"column:event_log_id" json:"event_log_id"`
	Result        *bool      `gorm:"column:result" json:"result"`
	Info          *string    `gorm:"column:info;type:text" json:"info"`
	OperationTime *time.Time `gorm:"column:operation_time" json:"operation_time"`
	TenancyId     *int       `gorm:"column:tenancy_id" json:"tenancy_id"`
}

func (ParameterDeploymentLog) TableName() string { return "parameter_deployment_log" }

// DeployTemplateLogVo is the API response VO for deploy template log queries (对齐Java ParameterDeploymentLogVO).
type DeployTemplateLogVo struct {
	Id            int64  `json:"id"`
	TenancyName   string `json:"tenancyName"`
	TemplateName  string `json:"templateName"`
	DeviceName    string `json:"deviceName"`
	SerialNumber  string `json:"serialNumber"`
	OperationTime string `json:"operationTime"`
	Info          string `json:"info"`
	Result        *bool  `json:"result"`
}

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

// ---------- Batch SPV (SetParameterValues) DTOs ----------

// SetParameterRecord is a single parameter entry in a batch SPV request.
type SetParameterRecord struct {
	ParamName string `json:"paramName" binding:"required"`
	Value     string `json:"value" binding:"required"`
}

// BatchSetParameterRequest is the JSON body for POST /parameters/batch-set.
// Sets multiple parameters atomically on a single device.
type BatchSetParameterRequest struct {
	ElementId int64                `json:"elementId" binding:"required"`
	Values    []SetParameterRecord `json:"values" binding:"required"`
}

// ---------- TR-069 Parameter Definition ----------

// TR069Parameter 对应 tr069_parameter 表 (TR-069 参数定义库)
type TR069Parameter struct {
	Id            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ParameterName string    `gorm:"column:parameter_name;type:varchar(500)" json:"parameter_name"`
	ParameterType string    `gorm:"column:parameter_type;type:varchar(50)" json:"parameter_type"`
	Description   string    `gorm:"column:description;type:text" json:"description"`
	DefaultValue  string    `gorm:"column:default_value;type:varchar(500)" json:"default_value"`
	IsReadOnly    bool      `gorm:"column:is_read_only" json:"is_read_only"`
	CreateTime    time.Time `gorm:"column:create_time" json:"create_time"`
}

func (TR069Parameter) TableName() string { return "tr069_parameter" }

// ---------- Model Tree DTOs ----------

// ModelTreeNode represents a node in the device parameter tree.
type ModelTreeNode struct {
	Name      string          `json:"name"`
	Path      string          `json:"path"`
	Value     string          `json:"value,omitempty"`
	ParamType string          `json:"paramType,omitempty"`
	Writable  bool            `json:"writable"`
	IsObject  bool            `json:"isObject"`
	Children  []ModelTreeNode `json:"children,omitempty"`
}

// AddObjectRequest is the JSON body for POST /model-tree/:elementId/add-object.
type AddObjectRequest struct {
	ObjectName string `json:"objectName" binding:"required"`
}

// DeleteObjectRequest is the JSON body for POST /model-tree/:elementId/delete-object.
type DeleteObjectRequest struct {
	ObjectName string `json:"objectName" binding:"required"`
}

// BatchDeleteObjectRequest is the JSON body for POST /model-tree/:elementId/batch-delete-object.
type BatchDeleteObjectRequest struct {
	ObjectNames []string `json:"objectNames" binding:"required"`
}

// RefreshParameterRequest is the JSON body for POST /model-tree/:elementId/refresh.
type RefreshParameterRequest struct {
	ParameterPath string `json:"parameterPath" binding:"required"`
}

// ReloadParameterRequest is the JSON body for POST /model-tree/:elementId/reload.
type ReloadParameterRequest struct {
	ParameterPaths []string `json:"parameterPaths" binding:"required"`
}
