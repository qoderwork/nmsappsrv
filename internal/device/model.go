package device

import "time"

// CpeElement 对应 cpe_element 表 (Java: NeElement)
type CpeElement struct {
	NeNeid                  int64      `gorm:"primaryKey;autoIncrement;column:ne_neid" json:"ne_neid"`
	NeAreaid                *int       `gorm:"column:ne_areaid" json:"ne_areaid"`
	SerialNumber            *string    `gorm:"column:serial_number;type:varchar(255)" json:"serial_number"`
	DeviceName              *string    `gorm:"column:device_name;type:varchar(255)" json:"device_name"`
	InstallationLocation    *string    `gorm:"column:installation_location;type:varchar(255)" json:"installation_location"`
	SoftwareVersion         *string    `gorm:"column:software_version;type:varchar(255)" json:"software_version"`
	DeviceIp                *string    `gorm:"column:device_ip;type:varchar(255)" json:"device_ip"`
	SiteId                  *string    `gorm:"column:site_id;type:varchar(255)" json:"site_id"`
	HardwareVersion         *string    `gorm:"column:hardware_version;type:varchar(255)" json:"hardware_version"`
	CreationTime            *time.Time `gorm:"column:creation_time" json:"creation_time"`
	Product                 *string    `gorm:"column:product;type:varchar(255)" json:"product"`
	MaintainerInfoId        *string    `gorm:"column:maintainer_info_id;type:varchar(255)" json:"maintainer_info_id"`
	LoadedBasicInfo         bool       `gorm:"column:loaded_basic_info;default:false" json:"loaded_basic_info"`
	IsInitialized           bool       `gorm:"column:is_initialized;default:false" json:"is_initialized"`
	IsNewVersion            bool       `gorm:"column:is_new_version;default:true" json:"is_new_version"`
	OpenStationConfigStatus *string    `gorm:"column:open_station_config_status;type:varchar(255)" json:"open_station_config_status"`
	Manufacturer            *string    `gorm:"column:manufacturer;type:varchar(255)" json:"manufacturer"`
	Oui                     *string    `gorm:"column:oui;type:varchar(255)" json:"oui"`
	LastLogCollectionTime   *time.Time `gorm:"column:last_log_collection_time" json:"last_log_collection_time"`
	CbsdCertFile            *string    `gorm:"column:cbsd_cert_file;type:varchar(255)" json:"cbsd_cert_file"`
	CbsdCertFileUploadTime  *time.Time `gorm:"column:cbsd_cert_file_upload_time" json:"cbsd_cert_file_upload_time"`
	ConfigFile              *string    `gorm:"column:config_file;type:varchar(255)" json:"config_file"`
	ConfigFileUploadTime    *time.Time `gorm:"column:config_file_upload_time" json:"config_file_upload_time"`
	Generation              *string    `gorm:"column:generation;type:varchar(255)" json:"generation"`
	Longitude               *string    `gorm:"column:longitude;type:varchar(255)" json:"longitude"`
	Latitude                *string    `gorm:"column:latitude;type:varchar(255)" json:"latitude"`
	Ip                      *string    `gorm:"column:ip;type:varchar(255)" json:"ip"`
	AosFileName             *string    `gorm:"column:aos_file_name;type:varchar(255)" json:"aos_file_name"`
	ReadyToZTP              *bool      `gorm:"column:read_to_ztp" json:"ready_to_ztp"`
	WifiOrGpsInfo           *string    `gorm:"column:wifi_or_gps_info;type:longtext" json:"wifi_or_gps_info"`
	InitialId               *int64     `gorm:"column:initial_id" json:"initial_id"`
	Status                  *string    `gorm:"column:status;type:varchar(255)" json:"status"`
	Port                    *int       `gorm:"column:port" json:"port"`
	E911Data                *string    `gorm:"column:e911_data;type:varchar(255)" json:"e911_data"`
	ZtpParameters           *string    `gorm:"column:ztp_parameters;type:longtext" json:"ztp_parameters"`
	Mac                     *string    `gorm:"column:mac;type:varchar(255)" json:"mac"`
	FullSoftwareVersion     *string    `gorm:"column:full_software_version;type:varchar(255)" json:"full_software_version"`
	TargetVersion           *string    `gorm:"column:target_version;type:varchar(255)" json:"target_version"`
	StmVersion              *string    `gorm:"column:stm_version;type:varchar(255)" json:"stm_version"`
	TargetHardwareVersion   *string    `gorm:"column:target_hardware_version;type:varchar(255)" json:"target_hardware_version"`
	Market                  *string    `gorm:"column:market;type:varchar(255)" json:"market"`
	PsapId                  *string    `gorm:"column:psap_id;type:varchar(255)" json:"psap_id"`
	FirmwareVersion         *string    `gorm:"column:firmware_version;type:varchar(255)" json:"firmware_version"`
	RootNode                *string    `gorm:"column:root_node;type:varchar(255)" json:"root_node"`
	TenantId               *int       `gorm:"column:license_id" json:"license_id"`
	DeviceType              *string    `gorm:"column:device_type;type:varchar(255)" json:"device_type"`
	Deleted                 bool       `gorm:"column:deleted;default:false" json:"deleted"`
	ModelName               *string    `gorm:"column:model_name;type:varchar(255)" json:"model_name"`
	CoonReqUrl              *string    `gorm:"column:coon_req_url;type:varchar(255)" json:"coon_req_url"`
	InDiagnostics           *bool      `gorm:"column:in_diagnostics" json:"in_diagnostics"`
	LowResource             *bool      `gorm:"column:low_resource" json:"low_resource"`
	ConnectionRequestUsername *string  `gorm:"column:connection_request_username;type:varchar(255)" json:"connection_request_username"`
	ConnectionRequestPassword *string  `gorm:"column:connection_request_password;type:varchar(255)" json:"connection_request_password"`
}

func (CpeElement) TableName() string { return "cpe_element" }

// DeviceGroup 对应 device_group 表
type DeviceGroup struct {
	Id           string       `gorm:"primaryKey;type:varchar(32)" json:"id"`
	GroupName    *string      `gorm:"column:group_name;type:varchar(255);uniqueIndex:idx_license_group" json:"group_name"`
	Description  *string      `gorm:"column:description;type:varchar(255)" json:"description"`
	CreationTime *time.Time   `gorm:"column:creation_time" json:"creation_time"`
	TenantId    *int         `gorm:"column:license_id;uniqueIndex:idx_license_group" json:"license_id"`
	DefaultGroup bool         `gorm:"column:default_group;default:false" json:"default_group"`
	Elements     []CpeElement `gorm:"many2many:group_has_element" json:"elements,omitempty"`
}

func (DeviceGroup) TableName() string { return "device_group" }

// GroupHasElement 对应 group_has_element 关联表
type GroupHasElement struct {
	GroupId   string `gorm:"primaryKey;column:group_id;type:varchar(32)" json:"group_id"`
	ElementId int64  `gorm:"primaryKey;column:element_id" json:"element_id"`
}

func (GroupHasElement) TableName() string { return "group_has_element" }

// ElementBasicInfoParameter 对应 element_basic_info_parameter 表
type ElementBasicInfoParameter struct {
	ParamId    int64      `gorm:"primaryKey;autoIncrement;column:param_id" json:"param_id"`
	ElementId  *int64     `gorm:"column:element_id;index" json:"element_id"`
	ParamName  *string    `gorm:"column:param_name;type:varchar(255);index" json:"param_name"`
	ParamValue *string    `gorm:"column:param_value;type:mediumtext" json:"param_value"`
	UpdateTime *time.Time `gorm:"column:update_time" json:"update_time"`
}

func (ElementBasicInfoParameter) TableName() string { return "element_basic_info_parameter" }

// ElementBlackList 对应 element_black_list 表
type ElementBlackList struct {
	Id         int        `gorm:"primaryKey;autoIncrement" json:"id"`
	SN         *string    `gorm:"column:sn;type:varchar(255);uniqueIndex" json:"sn"`
	TenantId  *int       `gorm:"column:license_id;uniqueIndex:idx_license_sn" json:"license_id"`
	DeviceType *string    `gorm:"column:device_type;type:varchar(255)" json:"device_type"`
	AddTime    *time.Time `gorm:"column:add_time" json:"add_time"`
	Username   *string    `gorm:"column:username;type:varchar(255)" json:"username"`
	Reason     *string    `gorm:"column:reason;type:varchar(255)" json:"reason"`
}

func (ElementBlackList) TableName() string { return "element_black_list" }

// DeviceStatistic 对应 device_statistic 表
type DeviceStatistic struct {
	Id            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId     *int64     `gorm:"column:element_id;index:idx_element_time" json:"element_id"`
	Online        *bool      `gorm:"column:online" json:"online"`
	Active        *bool      `gorm:"column:active" json:"active"`
	AmfActive     *bool      `gorm:"column:amf_active" json:"amf_active"`
	StatisticTime *time.Time `gorm:"column:statistic_time;index:idx_element_time" json:"statistic_time"`
}

func (DeviceStatistic) TableName() string { return "device_statistic" }

// CpeStatisticRecord 对应 cpe_statistic_record 表
type CpeStatisticRecord struct {
	Id           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId    *int64     `gorm:"column:element_id" json:"element_id"`
	Rsrp1        *string    `gorm:"column:rsrp1;type:varchar(255)" json:"rsrp1"`
	Rsrp2        *string    `gorm:"column:rsrp2;type:varchar(255)" json:"rsrp2"`
	Cinr1        *string    `gorm:"column:cinr1;type:varchar(255)" json:"cinr1"`
	Cinr2        *string    `gorm:"column:cinr2;type:varchar(255)" json:"cinr2"`
	Sinr         *string    `gorm:"column:sinr;type:varchar(255)" json:"sinr"`
	DlThroughput *string    `gorm:"column:dl_throughput;type:varchar(255)" json:"dl_throughput"`
	UlThroughput *string    `gorm:"column:ul_throughput;type:varchar(255)" json:"ul_throughput"`
	DlMcs        *string    `gorm:"column:dl_mcs;type:varchar(255)" json:"dl_mcs"`
	UlMcs        *string    `gorm:"column:ul_mcs;type:varchar(255)" json:"ul_mcs"`
	CreateTime   *time.Time `gorm:"column:create_time" json:"create_time"`
}

func (CpeStatisticRecord) TableName() string { return "cpe_statistic_record" }

// DeviceUeNumberRecord 对应 device_ue_number_record 表
type DeviceUeNumberRecord struct {
	Id         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId  *int64     `gorm:"column:element_id" json:"element_id"`
	UeNumber   *int       `gorm:"column:ue_number" json:"ue_number"`
	RecordTime *time.Time `gorm:"column:record_time;index" json:"record_time"`
}

func (DeviceUeNumberRecord) TableName() string { return "device_ue_number_record" }

// ---------------------------------------------------------------------------
// Device Import DTOs
// ---------------------------------------------------------------------------

// ImportDeviceRow represents one row parsed from the import Excel file.
// Columns: Serial Number | Device Name | Location | Longitude | Latitude | Operation
type ImportDeviceRow struct {
	SerialNumber string `json:"serialNumber"`
	DeviceName   string `json:"deviceName"`
	Location     string `json:"location"`
	Longitude    string `json:"longitude"`
	Latitude     string `json:"latitude"`
	Operation    string `json:"operation"` // "Add", "Modify", "Delete" (case-insensitive)
}

// ImportDeviceRequest is the query parameters for the import endpoint.
type ImportDeviceRequest struct {
	DeviceGroupId string `form:"deviceGroupId"` // optional group to assign imported devices to
	DeviceType    string `form:"deviceType"`    // "gnb", "enb", "cpe"
}

// ImportDeviceResult summarises the outcome of an import operation.
type ImportDeviceResult struct {
	AddedCount    int      `json:"addedCount"`
	ModifiedCount int      `json:"modifiedCount"`
	DeletedCount  int      `json:"deletedCount"`
	FailedCount   int      `json:"failedCount"`
	AddedIds      []int64  `json:"addedIds"`
	Errors        []string `json:"errors,omitempty"`
}
