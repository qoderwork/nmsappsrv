package restapi

// --- Device DTOs ---

type RestDeviceVo struct {
	Id              int64  `json:"id"`
	SerialNumber    string `json:"serialNumber"`
	DeviceName      string `json:"deviceName"`
	DeviceType      string `json:"deviceType"`
	Product         string `json:"product"`
	SoftwareVersion string `json:"softwareVersion"`
	Manufacturer    string `json:"manufacturer"`
	Status          string `json:"status"`
	TenantId       int    `json:"tenantId"`
}

type AddRestDeviceRequest struct {
	SerialNumber string `json:"serialNumber" binding:"required"`
	DeviceName   string `json:"deviceName"`
	DeviceType   string `json:"deviceType"`
	Product      string `json:"product"`
}

type ModifyRestDeviceRequest struct {
	DeviceName  *string `json:"deviceName"`
	DeviceType  *string `json:"deviceType"`
	Product     *string `json:"product"`
}

type ModifyRestDeviceBySNRequest struct {
	SerialNumber string  `json:"serialNumber" binding:"required"`
	DeviceName   *string `json:"deviceName"`
	DeviceType   *string `json:"deviceType"`
}

// --- Parameter DTOs ---

type RestParameterVo struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SetRestParameterRequest struct {
	Parameters []RestParameterVo `json:"parameters" binding:"required"`
}

type PresetParameterRequest struct {
	Parameters []RestParameterVo `json:"parameters" binding:"required"`
}

// --- Alarm DTOs ---

type RestAlarmVo struct {
	Id              int64  `json:"id"`
	Severity        string `json:"severity"`
	AlarmIdentifier string `json:"alarmIdentifier"`
	ProbableCause   string `json:"probableCause"`
	AlarmStatus     int    `json:"alarmStatus"`
	EventType       string `json:"eventType"`
	ElementId       int64  `json:"elementId"`
	EventTime       string `json:"eventTime"`
}

type SyncAlarmRequest struct {
	ElementIds []int64 `json:"elementIds" binding:"required"`
}

type ClearAlarmRequest struct {
	AlarmIds []int64 `json:"alarmIds" binding:"required"`
}

// --- Upgrade DTOs ---

type RestUpgradeFileVo struct {
	Id         int    `json:"id"`
	FileName   string `json:"fileName"`
	Version    string `json:"version"`
	DeviceType string `json:"deviceType"`
	FileSize   int64  `json:"fileSize"`
	UploadTime string `json:"uploadTime"`
}

type RestUpgradeTaskRequest struct {
	Name          string  `json:"name" binding:"required"`
	ElementIds    []int64 `json:"elementIds" binding:"required"`
	UpgradeFileId int     `json:"upgradeFileId" binding:"required"`
}

type RestUpgradeTaskVo struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Status   int    `json:"status"`
	Progress string `json:"progress"`
}

// --- Request status ---

type RequestStatusVo struct {
	RequestId string `json:"requestId"`
	Status    string `json:"status"` // "pending", "completed", "failed"
	Result    string `json:"result,omitempty"`
}

// --- TBG (femtocell) DTOs ---

type TBGDevice struct {
	Id            int     `gorm:"primaryKey;autoIncrement" json:"id"`
	SerialNumber  *string `gorm:"column:serial_number;type:varchar(255);uniqueIndex" json:"serial_number"`
	Band          *string `gorm:"column:band;type:varchar(255)" json:"band"`
	Address       *string `gorm:"column:address;type:varchar(255)" json:"address"`
	WanMacAddress   *string `gorm:"column:wan_mac_address;type:varchar(255)" json:"wan_mac_address"`
	RadiusThreshold *int    `gorm:"column:radius_threshold" json:"radius_threshold"`
	EnableGeofence  *bool   `gorm:"column:enable_geofence" json:"enable_geofence"`
	TenantId       *int    `gorm:"column:tenant_id" json:"tenant_id"`
}

func (TBGDevice) TableName() string { return "tbg" }

type AddTBGRequest struct {
	SerialNumber  string `json:"serialNumber" binding:"required"`
	Band          string `json:"band"`
	Address       string `json:"address"`
	WanMacAddress string `json:"wanMacAddress"`
}

type ModifyTBGRequest struct {
	SerialNumber  string  `json:"serialNumber" binding:"required"`
	Band          *string `json:"band"`
	Address       *string `json:"address"`
	WanMacAddress *string `json:"wanMacAddress"`
}

type DeleteTBGRequest struct {
	SerialNumbers []string `json:"serialNumbers" binding:"required"`
}

type TBGVo struct {
	Id            int    `json:"id"`
	SerialNumber  string `json:"serialNumber"`
	Band          string `json:"band"`
	Address       string `json:"address"`
	WanMacAddress string `json:"wanMacAddress"`
}

// --- Device Online Status DTOs (Task 6.2) ---

// DeviceOnlineStatusVo represents real-time online status of a device
type DeviceOnlineStatusVo struct {
	ElementId  int64  `json:"elementId"`
	SerialNumber string `json:"serialNumber"`
	DeviceName string `json:"deviceName"`
	Online     bool   `json:"online"`
	LastSeen   string `json:"lastSeen,omitempty"`
}

// --- ACS Settings DTOs (Task 6.3) ---

// RestACSConfigVo represents the ACS configuration returned by the REST API
type RestACSConfigVo struct {
	AcsUrl            *string `json:"acsUrl"`
	AcsUsername       *string `json:"acsUsername"`
	ConnectionTimeout *int    `json:"connectionTimeout"`
	InformInterval    *int    `json:"informInterval"`
	UdpPort           *int    `json:"udpPort"`
	TR069Enabled      *bool   `json:"tr069Enabled"`
}

// RestUpdateACSConfigRequest represents the request to update ACS configuration via REST API
type RestUpdateACSConfigRequest struct {
	AcsUrl            *string `json:"acsUrl"`
	AcsUsername       *string `json:"acsUsername"`
	AcsPassword       *string `json:"acsPassword"`
	ConnectionTimeout *int    `json:"connectionTimeout"`
	InformInterval    *int    `json:"informInterval"`
	UdpPort           *int    `json:"udpPort"`
	TR069Enabled      *bool   `json:"tr069Enabled"`
}

// --- SNMP Operation DTOs (Task 6.4) ---

// SnmpGetRequest represents a request to perform an SNMP GET operation
type SnmpGetRequest struct {
	ElementId int64    `json:"elementId" binding:"required"`
	OIDs      []string `json:"oids" binding:"required"`
}

// SnmpSetRequest represents a request to perform an SNMP SET operation
type SnmpSetRequest struct {
	ElementId  int64              `json:"elementId" binding:"required"`
	Parameters []SnmpParameterVo  `json:"parameters" binding:"required"`
}

// SnmpParameterVo represents a single SNMP variable binding for SET operations
type SnmpParameterVo struct {
	OID   string `json:"oid" binding:"required"`
	Type  string `json:"type" binding:"required"`
	Value string `json:"value" binding:"required"`
}

// SnmpOperationLogVo represents an SNMP operation log entry
type SnmpOperationLogVo struct {
	Id          int64  `json:"id"`
	ElementId   *int64 `json:"elementId"`
	Operation   string `json:"operation"`
	OID         string `json:"oid"`
	Value       string `json:"value"`
	Status      string `json:"status"`
	ErrorMsg    string `json:"errorMsg"`
	Operator    string `json:"operator"`
	OperateTime string `json:"operateTime"`
}
