package cbsd

import "time"

// CbsdInfo 对应 cbsd_info 表 (UUID主键)
type CbsdInfo struct {
	Id                   string     `gorm:"primaryKey;type:varchar(32)" json:"id"`
	CbsdSerialNumber     *string    `gorm:"column:cbsd_serial_number;type:varchar(255);uniqueIndex:idx_cbsd_unique" json:"cbsd_serial_number"`
	SerialNumber         *string    `gorm:"column:serial_number;type:varchar(255);uniqueIndex:idx_cbsd_unique" json:"serial_number"`
	ElementId            *string    `gorm:"column:element_id;type:varchar(255)" json:"element_id"`
	CbsdCategory         *string    `gorm:"column:cbsd_category;type:varchar(255)" json:"cbsd_category"`
	MeasCapability       *string    `gorm:"column:meas_capability;type:varchar(255)" json:"meas_capability"`
	Latitude             *string    `gorm:"column:latitude;type:varchar(255)" json:"latitude"`
	Longitude            *string    `gorm:"column:longitude;type:varchar(255)" json:"longitude"`
	Height               *string    `gorm:"column:height;type:varchar(255)" json:"height"`
	HeightType           *string    `gorm:"column:height_type;type:varchar(255)" json:"height_type"`
	HorizontalAccuracy   *string    `gorm:"column:horizontal_accuracy;type:varchar(255)" json:"horizontal_accuracy"`
	VerticalAccuracy     *string    `gorm:"column:vertical_accuracy;type:varchar(255)" json:"vertical_accuracy"`
	AntennaGain          *int       `gorm:"column:antenna_gain" json:"antenna_gain"`
	IndoorDeployment     *bool      `gorm:"column:indoor_deployment" json:"indoor_deployment"`
	AntennaBeamwidth     *int       `gorm:"column:antenna_beamwidth" json:"antenna_beamwidth"`
	AntennaAzimuth       *int       `gorm:"column:antenna_azimuth" json:"antenna_azimuth"`
	AntennaDowntilt      *int       `gorm:"column:antenna_downtilt" json:"antenna_downtilt"`
	Vendor               *string    `gorm:"column:vendor;type:varchar(255)" json:"vendor"`
	FirmwareVersion      *string    `gorm:"column:firmware_version;type:varchar(255)" json:"firmware_version"`
	HardwareVersion      *string    `gorm:"column:hardware_version;type:varchar(255)" json:"hardware_version"`
	Model                *string    `gorm:"column:model;type:varchar(255)" json:"model"`
	OperationState       *string    `gorm:"column:operation_state;type:varchar(255)" json:"operation_state"`
	CbsdID               *string    `gorm:"column:cbsd_id;type:varchar(255)" json:"cbsd_id"`
	GrantID              *string    `gorm:"column:grant_id;type:varchar(255)" json:"grant_id"`
	TransmitExpireTime   *string    `gorm:"column:transmit_expire_time;type:varchar(255)" json:"transmit_expire_time"`
	GrantExpireTime      *string    `gorm:"column:grant_expire_time;type:varchar(255)" json:"grant_expire_time"`
	LastGrantTime        *time.Time `gorm:"column:last_grant_time" json:"last_grant_time"`
	Enable               *bool      `gorm:"column:enable" json:"enable"`
	LowFrequency         *int64     `gorm:"column:low_frequency" json:"low_frequency"`
	HighFrequency        *int64     `gorm:"column:high_frequency" json:"high_frequency"`
	MaxEirp              *float64   `gorm:"column:max_eirp" json:"max_eirp"`
	PathLoss             *int       `gorm:"column:path_loss" json:"path_loss"`
	PreferredPower       *int       `gorm:"column:preferred_power" json:"preferred_power"`
	FccId                *string    `gorm:"column:fcc_id;type:varchar(255)" json:"fcc_id"`
	CallSign             *string    `gorm:"column:call_sign;type:varchar(255)" json:"call_sign"`
	PreferredFrequency   *string    `gorm:"column:preferred_frequency;type:varchar(255)" json:"preferred_frequency"`
	MeasReportConfig     *string    `gorm:"column:meas_report_config;type:longtext" json:"meas_report_config"`
	LastRegistrationTime *time.Time `gorm:"column:last_registration_time" json:"last_registration_time"`
	LicenseId            *int       `gorm:"column:license_id;uniqueIndex:idx_cbsd_unique" json:"license_id"`
	PreferredBandwidth   *int       `gorm:"column:preferred_bandwidth" json:"preferred_bandwidth"`
	GroupIds             *string    `gorm:"column:group_ids;type:varchar(255)" json:"group_ids"`
	UpdateTime           *time.Time `gorm:"column:update_time" json:"update_time"`
}

func (CbsdInfo) TableName() string { return "cbsd_info" }

// CBSDCertFileSendTask 对应 cbsdcert_file_send_task 表
type CBSDCertFileSendTask struct {
	Id             int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name           *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	User           *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	OperationTime  *time.Time `gorm:"column:operation_time" json:"operation_time"`
	Status         *int       `gorm:"column:status" json:"status"`
	StartTime      *time.Time `gorm:"column:start_time" json:"start_time"`
	EndTime        *time.Time `gorm:"column:end_time" json:"end_time"`
	ExecuteMode    *int       `gorm:"column:execute_mode" json:"execute_mode"`
	TriggerTime    *time.Time `gorm:"column:trigger_time" json:"trigger_time"`
	TenancyId      *int       `gorm:"column:tenancy_id" json:"tenancy_id"`
	ElementIds     *string    `gorm:"column:element_ids;type:longtext" json:"element_ids"`
	DeviceType     *string    `gorm:"column:device_type;type:varchar(255)" json:"device_type"`
}

func (CBSDCertFileSendTask) TableName() string { return "cbsdcert_file_send_task" }

// SendCBSDCertFileLog 对应 send_cbsd_cert_file_log 表
type SendCBSDCertFileLog struct {
	Id         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	EventLogId *int64     `gorm:"column:event_log_id" json:"event_log_id"`
	Status     *int       `gorm:"column:status" json:"status"`
	TaskId     *int       `gorm:"column:task_id" json:"task_id"`
	ElementId  *int64     `gorm:"column:element_id" json:"element_id"`
	EndTime    *time.Time `gorm:"column:end_time" json:"end_time"`
}

func (SendCBSDCertFileLog) TableName() string { return "send_cbsd_cert_file_log" }

// CbrsLog 对应 cbrs_log 表
type CbrsLog struct {
	Id       int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	LogType  *string    `gorm:"column:log_type;type:varchar(255)" json:"log_type"`
	Status   *string    `gorm:"column:status;type:varchar(255)" json:"status"`
	LogTime  *time.Time `gorm:"column:log_time;index:idx_cbsd_time" json:"log_time"`
	CbsdId   *string    `gorm:"column:cbsd_id;type:varchar(255);index:idx_cbsd_time" json:"cbsd_id"`
	Message  *string    `gorm:"column:message;type:longtext" json:"message"`
}

func (CbrsLog) TableName() string { return "cbrs_log" }

// SasConfig 对应 sas_config 表
type SasConfig struct {
	Id          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	SasName     string    `gorm:"column:sas_name;type:varchar(255)" json:"sas_name"`
	SasUrl      string    `gorm:"column:sas_url;type:varchar(500)" json:"sas_url"`
	CertPath    string    `gorm:"column:cert_path;type:varchar(500)" json:"cert_path"`
	KeyPath     string    `gorm:"column:key_path;type:varchar(500)" json:"key_path"`
	Enabled     bool      `gorm:"column:enabled" json:"enabled"`
	LicenseId   int       `gorm:"column:license_id" json:"license_id"`
	CreateTime  time.Time `gorm:"column:create_time" json:"create_time"`
	UpdateTime  time.Time `gorm:"column:update_time" json:"update_time"`
}

func (SasConfig) TableName() string { return "sas_config" }

// SpectrumInquiryRequest represents a SAS spectrum inquiry request.
type SpectrumInquiryRequest struct {
	LowFrequency   int64   `json:"low_frequency"`
	HighFrequency  int64   `json:"high_frequency"`
}

// GrantRequest represents a SAS grant request.
type GrantRequest struct {
	LowFrequency   int64   `json:"low_frequency"`
	HighFrequency  int64   `json:"high_frequency"`
	MaxEirp        float64 `json:"max_eirp"`
}

// RelinquishmentRequest represents a SAS relinquishment request.
type RelinquishmentRequest struct {
	GrantId  string `json:"grant_id"`
}

// CbsdStatusCountItem holds a status -> count mapping for CBSD statistics.
type CbsdStatusCountItem struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}
