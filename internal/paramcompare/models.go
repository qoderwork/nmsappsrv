package paramcompare

// TemplateValue holds the expected value for a parameter within a template.
// Backed by the parameter_template_value table.
type TemplateValue struct {
	Id          int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateId  int64   `gorm:"column:template_id;not null;index" json:"template_id"`
	ParameterId *string `gorm:"column:parameter_id;type:varchar(32)" json:"parameter_id"`
	ParamPath   *string `gorm:"column:param_path;type:varchar(255)" json:"param_path"`
	ParamValue  *string `gorm:"column:param_value;type:mediumtext" json:"param_value"`
}

func (TemplateValue) TableName() string { return "parameter_template_value" }

// TemplateInfo is a lightweight DTO returned when listing available templates.
type TemplateInfo struct {
	Id          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ParamCount  int    `json:"paramCount"`
}

// DeviceParam is a lightweight DTO for a device's actual parameter value.
type DeviceParam struct {
	ParamName  string `gorm:"column:param_name" json:"param_name"`
	ParamValue string `gorm:"column:param_value" json:"param_value"`
}

// TemplateParam is a lightweight DTO for a template's expected parameter value.
type TemplateParam struct {
	ParamPath  string `gorm:"column:param_path" json:"param_path"`
	ParamValue string `gorm:"column:param_value" json:"param_value"`
}

// ---------------------------------------------------------------------------
// Comparison result DTOs
// ---------------------------------------------------------------------------

// CompareResult summarises the comparison between one device and one template.
type CompareResult struct {
	DeviceID          uint        `json:"device_id"`
	TemplateName      string      `json:"template_name"`
	TotalParams       int         `json:"total_params"`
	MatchCount        int         `json:"match_count"`
	MismatchCount     int         `json:"mismatch_count"`
	MissingInDevice   int         `json:"missing_in_device"`
	MissingInTemplate int         `json:"missing_in_template"`
	Deviations        []Deviation `json:"deviations"`
}

// Deviation describes a single parameter-level difference.
type Deviation struct {
	ParameterName string `json:"parameter_name"`
	ActualValue   string `json:"actual_value"`
	ExpectedValue string `json:"expected_value"`
	Status        string `json:"status"` // "match" | "mismatch" | "missing_in_device" | "missing_in_template"
}

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

// CompareRequest is the JSON body for POST /param-compare/compare.
type CompareRequest struct {
	DeviceID   uint `json:"device_id" binding:"required"`
	TemplateID uint `json:"template_id" binding:"required"`
}

// BatchCompareRequest is the JSON body for POST /param-compare/batch.
type BatchCompareRequest struct {
	DeviceIDs  []uint `json:"device_ids" binding:"required"`
	TemplateID uint   `json:"template_id" binding:"required"`
}
