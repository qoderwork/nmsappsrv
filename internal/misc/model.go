package misc

import (
	"fmt"
	"time"
)

// --- Batch operation tables ---

// BatchAddObjectTask 对应 batch_add_object_task 表
type BatchAddObjectTask struct {
	Id        int        `gorm:"primaryKey;autoIncrement" json:"id"`
	User      *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	Time      *time.Time `gorm:"column:time" json:"time"`
	TenantId *int       `gorm:"column:tenant_id" json:"tenant_id"`
}

func (BatchAddObjectTask) TableName() string { return "batch_add_object_task" }

// BatchAddObjectTaskLog 对应 batch_add_object_task_log 表
type BatchAddObjectTaskLog struct {
	Id         int64 `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId     *int  `gorm:"column:task_id" json:"task_id"`
	EventLogId *int64 `gorm:"column:event_log_id" json:"event_log_id"`
}

func (BatchAddObjectTaskLog) TableName() string { return "batch_add_object_task_log" }

// ---------- BatchAddObject DTOs ----------

// BatchAddObjectRequest is the JSON body for POST /batch-add-object.
type BatchAddObjectRequest struct {
	Ids        []int64 `json:"ids" binding:"required"`        // device element IDs
	Type       string  `json:"type" binding:"required"`       // object type key
	AmfNumber  *int    `json:"amfNumber,omitempty"`           // parent AMF index
	SliceNumber *int   `json:"sliceNumber,omitempty"`         // parent slice index
	TaNumber   *int    `json:"taNumber,omitempty"`            // parent TA index
	PlmnNumber *int    `json:"plmnNumber,omitempty"`          // parent PLMN index
}

// BatchAddObjectTaskVo is the response item for the task list endpoint.
type BatchAddObjectTaskVo struct {
	Id           int    `json:"id"`
	Name         string `json:"name"`          // object type label
	OperationUser string `json:"operationUser"`
	OperationTime string `json:"operationTime"`
	Progress     string `json:"progress"`      // e.g. "3/5"
	TenancyName  string `json:"tenancyName"`
}

// BatchAddObjectTaskDetailVo is the response item for the task detail endpoint.
type BatchAddObjectTaskDetailVo struct {
	DeviceName   string `json:"deviceName"`
	SerialNumber string `json:"serialNumber"`
	Result       *int   `json:"result"`       // 1=success, 2=fail
	FaultInfo    string `json:"faultInfo"`
	ElementId    int64  `json:"elementId"`
	TenancyName  string `json:"tenancyName"`
}

// ---------- TR069 AddObject path mapping ----------

// tr069ObjectPaths maps the request type key to the TR-069 object path prefix.
var tr069ObjectPaths = map[string]string{
	"NR AMF":            "Device.Services.FAPService.1.FAPControl.NR.AMFPoolConfigParam.",
	"GUAMI":             "Device.Services.FAPService.1.FAPControl.NR.AMFPoolConfigParam.",
	"AFM_PLMN":          "Device.Services.FAPService.1.FAPControl.NR.AMFPoolConfigParam.",
	"slice":             "Device.Services.FAPService.1.CellConfig.1.NR.NGC.Slice.SliceList.",
	"SNSSAI":            "Device.Services.FAPService.1.CellConfig.1.NR.NGC.Slice.SliceList.",
	"5QI_PLMN":          "Device.Services.FAPService.1.CellConfig.1.NR.NGC.Slice.SliceList.",
	"QoS":               "Device.Services.FAPService.1.CellConfig.1.NR.NGC.QoS.",
	"Route":             "Device.Ethernet.IpRoute.",
	"DNS_IPV4":          "Device.Ethernet.DNS.IPv4Address.",
	"DNS_IPV6":          "Device.Ethernet.DNS.IPv6Address.",
	"XN":                "Device.Services.FAPService.1.FAPControl.NR.XnIpAddrMapInfo.",
	"NeighborList_LTE":  "Device.Services.FAPService.1.RAN.NeighborList.LTECell.",
	"NeighborList_NR":   "Device.Services.FAPService.1.RAN.NeighborList.NRCell.",
	"DRX":               "Device.Services.FAPService.1.RAN.Drx.",
	"TA":                "Device.Services.FAPService.1.NR.CN.TA.",
	"TA_PLMN":           "Device.Services.FAPService.1.NR.CN.TA.",
	"TA_Slice":          "Device.Services.FAPService.1.NR.CN.TA.",
	"VoNR":              "Device.Services.FAPService.1.VoNR.VoNRList.",
}

// BuildTR069ObjectPath builds the full TR-069 object name for the given type and indices.
func BuildTR069ObjectPath(objType string, amfNum, sliceNum, taNum, plmnNum *int) string {
	base, ok := tr069ObjectPaths[objType]
	if !ok {
		return ""
	}
	switch objType {
	case "GUAMI":
		if amfNum != nil {
			return base + fmt.Sprintf("%d.GUAMI.", *amfNum)
		}
	case "AFM_PLMN":
		if amfNum != nil {
			return base + fmt.Sprintf("%d.SupportPLMNList.", *amfNum)
		}
	case "SNSSAI":
		if sliceNum != nil {
			return base + fmt.Sprintf("%d.SNSSAIList.", *sliceNum)
		}
	case "5QI_PLMN":
		return base + "Default5QI.PLMN."
	case "TA_PLMN":
		if taNum != nil {
			return base + fmt.Sprintf("%d.PLMNList.", *taNum)
		}
	case "TA_Slice":
		if taNum != nil && plmnNum != nil {
			return base + fmt.Sprintf("%d.PLMNList.%d.SliceList.", *taNum, *plmnNum)
		}
	}
	return base
}

// BatchConfigurationLog 对应 batch_configuration_log 表
type BatchConfigurationLog struct {
	Id            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name          *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	OperationTime *time.Time `gorm:"column:operation_time" json:"operation_time"`
	TenantId     *int       `gorm:"column:tenant_id" json:"tenant_id"`
	User          *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	DeviceCount   *int       `gorm:"column:device_count" json:"device_count"`
}

func (BatchConfigurationLog) TableName() string { return "batch_configuration_log" }

// BatchConfigurationDeviceLog 对应 batch_configuration_device_log 表
type BatchConfigurationDeviceLog struct {
	Id         int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId     *int64  `gorm:"column:task_id" json:"task_id"`
	ElementId  *int64  `gorm:"column:element_id" json:"element_id"`
	Data       *string `gorm:"column:data;type:longtext" json:"data"`
	EventLogId *int64  `gorm:"column:event_log_id" json:"event_log_id"`
}

func (BatchConfigurationDeviceLog) TableName() string { return "batch_configuration_device_log" }

// BatchProcessFile 对应 batch_process_file 表
type BatchProcessFile struct {
	Id          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	FileName    *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	UploadTime  *time.Time `gorm:"column:upload_time" json:"upload_time"`
	UploadUser  *string    `gorm:"column:upload_user;type:varchar(255)" json:"upload_user"`
	TenantId   *int       `gorm:"column:tenant_id" json:"tenant_id"`
}

func (BatchProcessFile) TableName() string { return "batch_process_file" }

// BatchProcessFileSendLog 对应 batch_process_file_send_log 表
type BatchProcessFileSendLog struct {
	Id             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId      *int64     `gorm:"column:element_id" json:"element_id"`
	CommandTrackId *int64     `gorm:"column:command_track_id" json:"command_track_id"`
	SendTime       *time.Time `gorm:"column:send_time" json:"send_time"`
	DownloadTime   *time.Time `gorm:"column:download_time" json:"download_time"`
	TenantId      *int       `gorm:"column:tenant_id" json:"tenant_id"`
	Status         *int       `gorm:"column:status" json:"status"`
	FaultInfo      *string    `gorm:"column:fault_info;type:text" json:"fault_info"`
	FileName       *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	CheckFile      *bool      `gorm:"column:check_file" json:"check_file"`
}

func (BatchProcessFileSendLog) TableName() string { return "batch_process_file_send_log" }

// BackupOrRestoreTask 对应 backup_or_restore_task 表
type BackupOrRestoreTask struct {
	Id                  int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name                *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	User                *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	OperationTime       *time.Time `gorm:"column:operation_time" json:"operation_time"`
	Status              *int       `gorm:"column:status" json:"status"`
	StartTime           *time.Time `gorm:"column:start_time" json:"start_time"`
	EndTime             *time.Time `gorm:"column:end_time" json:"end_time"`
	ExecuteMode         *int       `gorm:"column:execute_mode" json:"execute_mode"`
	TriggerTime         *time.Time `gorm:"column:trigger_time" json:"trigger_time"`
	TenantId           *int       `gorm:"column:tenant_id" json:"tenant_id"`
	TaskType            *string    `gorm:"column:task_type;type:varchar(255)" json:"task_type"`
	ExecuteOnAllDevice  *bool      `gorm:"column:execute_on_all_device" json:"execute_on_all_device"`
	ElementIds          *string    `gorm:"column:element_ids;type:text" json:"element_ids"`
	Scope               *string    `gorm:"column:scope;type:varchar(255)" json:"scope"`
	DeviceGroupIds      *string    `gorm:"column:device_group_ids;type:longtext" json:"device_group_ids"`
}

func (BackupOrRestoreTask) TableName() string { return "backup_or_restore_task" }

// RestoreAndBackUpDeviceLog 对应 restore_and_back_up_device_log 表
type RestoreAndBackUpDeviceLog struct {
	Id                int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId         *int64     `gorm:"column:element_id" json:"element_id"`
	EventLogId        *int64     `gorm:"column:event_log_id" json:"event_log_id"`
	StartTime         *time.Time `gorm:"column:start_time" json:"start_time"`
	EndTime           *time.Time `gorm:"column:end_time" json:"end_time"`
	Results           *int       `gorm:"column:results" json:"results"`
	FailureReason     *string    `gorm:"column:failure_reason;type:text" json:"failure_reason"`
	TaskId            *int       `gorm:"column:task_id" json:"task_id"`
	Type              *string    `gorm:"column:type;type:varchar(255)" json:"type"`
	ConfigurationFile *string    `gorm:"column:configuration_file;type:varchar(255)" json:"configuration_file"`
}

func (RestoreAndBackUpDeviceLog) TableName() string { return "restore_and_back_up_device_log" }

// ---------- Batch Backup/Restore DTOs ----------

// BackupRestoreRequest is the JSON body for POST /batch-backup and /batch-restore.
type BackupRestoreRequest struct {
	Name               string  `json:"name" binding:"required"`
	ExecuteMode        int     `json:"executeMode" binding:"required"` // 1=immediate, 2=manual, 3=scheduled
	TriggerTime        *string `json:"triggerTime,omitempty"`          // RFC3339, required for mode 3
	ElementIds         []int64 `json:"elementIds"`
	ExecuteOnAllDevice bool    `json:"executeOnAllDevice"`
	Scope              string  `json:"scope"`              // "element" or "deviceGroup"
	DeviceGroupIds     []string `json:"deviceGroupIds"`
}

// BackupRestoreTaskVo is the response item for the task list endpoint.
type BackupRestoreTaskVo struct {
	Id             int    `json:"id"`
	Name           string `json:"name"`
	TaskType       string `json:"taskType"` // "backup" or "restore"
	OperationUser  string `json:"operationUser"`
	OperationTime  string `json:"operationTime"`
	Status         int    `json:"status"` // 1=waiting, 2=executing, 3=executed, 4=cancelled
	ExecuteMode    int    `json:"executeMode"`
	DeviceCount    int    `json:"deviceCount"`
	Progress       string `json:"progress"` // "success/total"
	Result         *int   `json:"result"`   // nil=pending, 1=all success, 2=has failure
}

// BackupRestoreTaskDetailVo is the response item for per-device results.
type BackupRestoreTaskDetailVo struct {
	DeviceName        string `json:"deviceName"`
	SerialNumber      string `json:"serialNumber"`
	ElementId         int64  `json:"elementId"`
	Result            *int   `json:"result"`       // null=pending, 1=success, 2=failure
	FailureReason     string `json:"failureReason"`
	StartTime         string `json:"startTime"`
	EndTime           string `json:"endTime"`
	ConfigurationFile string `json:"configurationFile"`
}

// --- MR tables ---

// MRData 对应 mr_data 表
type MRData struct {
	Id             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId      *int64     `gorm:"column:element_id;index:idx_elem_start_end" json:"element_id"`
	CellId         *string    `gorm:"column:cell_id;type:varchar(255)" json:"cell_id"`
	UeId           *string    `gorm:"column:ue_id;type:varchar(255)" json:"ue_id"`
	AmfId          *string    `gorm:"column:amf_id;type:varchar(255)" json:"amf_id"`
	EventTime      *time.Time `gorm:"column:event_time" json:"event_time"`
	StartTime      *time.Time `gorm:"column:start_time;index:idx_elem_start_end" json:"start_time"`
	EndTime        *time.Time `gorm:"column:end_time;index:idx_elem_start_end" json:"end_time"`
	EventType      *string    `gorm:"column:event_type;type:varchar(255)" json:"event_type"`
	NRScArfcn      *string    `gorm:"column:nr_sc_arfcn;type:varchar(255)" json:"nr_sc_arfcn"`
	NRScPci        *string    `gorm:"column:nr_sc_pci;type:varchar(255)" json:"nr_sc_pci"`
	NRScSSRSRP     *string    `gorm:"column:nr_sc_ssrsrp;type:varchar(255)" json:"nr_sc_ssrsrp"`
	NRScSSRSRQ     *string    `gorm:"column:nr_sc_ssrsrq;type:varchar(255)" json:"nr_sc_ssrsrq"`
	NRScSSSINR     *string    `gorm:"column:nr_sc_sssinr;type:varchar(255)" json:"nr_sc_sssinr"`
	NRScTadv       *string    `gorm:"column:nr_sc_tadv;type:varchar(255)" json:"nr_sc_tadv"`
	NRScPHR        *string    `gorm:"column:nr_sc_phr;type:varchar(255)" json:"nr_sc_phr"`
	HAOA           *string    `gorm:"column:h_aoa;type:varchar(255)" json:"h_aoa"`
	VAOA           *string    `gorm:"column:v_aoa;type:varchar(255)" json:"v_aoa"`
	NRUEPlrUL      *string    `gorm:"column:nrue_plr_ul;type:varchar(255)" json:"nrue_plr_ul"`
	NRUEPlrDL      *string    `gorm:"column:nrue_plr_dl;type:varchar(255)" json:"nrue_plr_dl"`
	NRNcArfcn      *string    `gorm:"column:nr_nc_arfcn;type:varchar(255)" json:"nr_nc_arfcn"`
	NRNcPci        *string    `gorm:"column:nr_nc_pci;type:varchar(255)" json:"nr_nc_pci"`
	NRNcSSRSRP     *string    `gorm:"column:nr_nc_ssrsrp;type:varchar(255)" json:"nr_nc_ssrsrp"`
	NRNcSSRSRQ     *string    `gorm:"column:nr_nc_ssrsrq;type:varchar(255)" json:"nr_nc_ssrsrq"`
	NRNcSSSINR     *string    `gorm:"column:nr_nc_sssinr;type:varchar(255)" json:"nr_nc_sssinr"`
	LteNcEarfcn    *string    `gorm:"column:lte_nc_earfcn;type:varchar(255)" json:"lte_nc_earfcn"`
	LteNcPci       *string    `gorm:"column:lte_nc_pci;type:varchar(255)" json:"lte_nc_pci"`
	LteNcRSRP      *string    `gorm:"column:lte_nc_rsrp;type:varchar(255)" json:"lte_nc_rsrp"`
	LteNcRSRQ      *string    `gorm:"column:lte_nc_rsrq;type:varchar(255)" json:"lte_nc_rsrq"`
	PLMN           *string    `gorm:"column:plmn;type:varchar(255)" json:"plmn"`
	NRScSSBIndexId *string    `gorm:"column:nr_sc_ssb_index_id;type:varchar(255)" json:"nr_sc_ssb_index_id"`
	NRNcSSBIndexId *string    `gorm:"column:nr_nc_ssb_index_id;type:varchar(255)" json:"nr_nc_ssb_index_id"`
	Longitude      *string    `gorm:"column:longitude;type:varchar(255)" json:"longitude"`
	Latitude       *string    `gorm:"column:latitude;type:varchar(255)" json:"latitude"`
}

func (MRData) TableName() string { return "mr_data" }

// MRFileLog 对应 mr_file_upload_log 表
type MRFileLog struct {
	Id         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId  *int64     `gorm:"column:element_id;index" json:"element_id"`
	FileName   *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	UploadTime *time.Time `gorm:"column:upload_time;index" json:"upload_time"`
}

func (MRFileLog) TableName() string { return "mr_file_upload_log" }

// MRUploadTask 对应 mr_upload_task 表
type MRUploadTask struct {
	Id         int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name       *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	Period     *int       `gorm:"column:period" json:"period"`
	StartTime  *time.Time `gorm:"column:start_time" json:"start_time"`
	EndTime    *time.Time `gorm:"column:end_time" json:"end_time"`
	TenantId  *int       `gorm:"column:tenant_id" json:"tenant_id"`
	UpdateTime *time.Time `gorm:"column:update_time" json:"update_time"`
}

func (MRUploadTask) TableName() string { return "mr_upload_task" }

// MRUploadTaskHasElement 对应 mr_upload_task_has_element 表
type MRUploadTaskHasElement struct {
	Id        int    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId    *int   `gorm:"column:task_id" json:"task_id"`
	ElementId *int64 `gorm:"column:element_id" json:"element_id"`
}

func (MRUploadTaskHasElement) TableName() string { return "mr_upload_task_has_element" }

// --- ZTP tables ---

// ZTPLog 对应 ztp_log 表
type ZTPLog struct {
	Id         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId  *int64     `gorm:"column:element_id" json:"element_id"`
	Progress   *int       `gorm:"column:progress" json:"progress"`
	Done       *bool      `gorm:"column:done" json:"done"`
	Info       *string    `gorm:"column:info;type:longtext" json:"info"`
	StartTime  *time.Time `gorm:"column:start_time" json:"start_time"`
	EndTime    *time.Time `gorm:"column:end_time" json:"end_time"`
	EventLogId *int64     `gorm:"column:event_log_id" json:"event_log_id"`
	HasFault   *bool      `gorm:"column:has_fault" json:"has_fault"`
}

func (ZTPLog) TableName() string { return "ztp_log" }

// ZTPRetryLog 对应 ztp_retry_log 表
type ZTPRetryLog struct {
	Id        int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId *int64     `gorm:"column:element_id" json:"element_id"`
	RetryTime *time.Time `gorm:"column:retry_time" json:"retry_time"`
	Info      *string    `gorm:"column:info;type:varchar(255)" json:"info"`
}

func (ZTPRetryLog) TableName() string { return "ztp_retry_log" }

// ZTPFileSendLog 对应 ztp_file_send_log 表
type ZTPFileSendLog struct {
	Id           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId    *int64     `gorm:"column:element_id" json:"element_id"`
	FileName     *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	GenerateTime *time.Time `gorm:"column:generate_time" json:"generate_time"`
}

func (ZTPFileSendLog) TableName() string { return "ztp_file_send_log" }

// ZTPGnbIdUsed 对应 ztp_gnbid_used 表 (UUID主键)
type ZTPGnbIdUsed struct {
	Id        string  `gorm:"primaryKey;type:varchar(32)" json:"id"`
	ElementId *int64  `gorm:"column:element_id" json:"element_id"`
	Market    *string `gorm:"column:market;type:varchar(255)" json:"market"`
	GnbId     *int    `gorm:"column:gnb_id;index" json:"gnb_id"`
}

func (ZTPGnbIdUsed) TableName() string { return "ztp_gnbid_used" }

// ZTPTACUsed 对应 ztp_tac_used 表 (UUID主键)
type ZTPTACUsed struct {
	Id             string  `gorm:"primaryKey;type:varchar(32)" json:"id"`
	Market         *string `gorm:"column:market;type:varchar(255)" json:"market"`
	CurrentUsedTac *int    `gorm:"column:current_used_tac" json:"current_used_tac"`
}

func (ZTPTACUsed) TableName() string { return "ztp_tac_used" }

// ---------- ZTP Setting (stored in system_config as JSON) ----------

// PTPSetting mirrors Java ZTPSettingDTO.ptpSetting (PTP clock config applied
// to the device when mode ∈ {MANUAL, WIFI}).
type PTPSetting struct {
	ClockInterfaceName *string `json:"clockInterfaceName"`
	ClockDomainNumber  *int    `json:"clockDomainNumber"`
	ClockSyncMode      *string `json:"clockSyncMode"`
}

// SpectrumSpatialSetting mirrors Java ZTPSettingDTO.spectrumSpatialSetting.
// Drives the reverse-geocode (PSAP + expected location) and geocode calls.
type SpectrumSpatialSetting struct {
	URL              *string `json:"url"`
	PsapURL          *string `json:"psapUrl"`          // present in DTO; not used by the thread
	GeoCodeURL       *string `json:"geoCodeUrl"`
	ReverseGeoCodeURL *string `json:"reverseGeoCodeUrl"`
	DeleteRetryTimes *int    `json:"deleteRetryTimes"`
}

// ExternalEndpointSetting is the shared shape for MSAG / BMC (old) / GMLC /
// LMF instances: a URL plus Basic-auth credentials and a retry counter.
type ExternalEndpointSetting struct {
	URL              *string `json:"url"`
	Username         *string `json:"username"`
	Password         *string `json:"password"`
	DeleteRetryTimes *int    `json:"deleteRetryTimes"`
}

// TPlatformSetting mirrors Java ZTPSettingDTO.tPlatformSetting (OAuth2
// client_credentials + PoP token, alert notifications only).
type TPlatformSetting struct {
	URL        *string `json:"url"`
	ClientID   *string `json:"clientId"`
	Secret     *string `json:"secret"`
	RetryTimes *int    `json:"retryTimes"`
}

// ZTPSetting represents the ZTP configuration stored in system_config table.
//
// It carries both the original flat top-level fields (consumed by the SPV
// worker in internal/tr069) and the nested per-system settings mirrored from
// Java's ZTPSettingDTO. The external-registration orchestrator
// (internal/ztp) reads the nested settings; FromZTPSetting in the external
// package prefers a nested value and falls back to its flat equivalent so
// legacy configs that only populate the flat URLs keep working.
type ZTPSetting struct {
	GnbIdStart          *int     `json:"gnbIdStart"`
	GnbIdEnd            *int     `json:"gnbIdEnd"`
	TacStart            *int     `json:"tacStart"` // TAC range; Java reads it from CORE_NR_FEMTO.csv. Exposed here until core-file parsing lands.
	TacEnd              *int     `json:"tacEnd"`
	GoogleAPIKey        *string  `json:"googleAPIKey"`
	RadiusThreshold     *float64 `json:"radiusThreshold"`
	ZTPTimeoutTime      *int     `json:"ztpTimeoutTime"` // minutes
	PTPEnable           *bool    `json:"ptpEnable"`
	SpectrumSpatialURL  *string  `json:"spectrumSpatialUrl"`
	MSAGUrl             *string  `json:"msagUrl"`
	BMCUrl              *string  `json:"bmcUrl"`
	BMCNewApi           *bool    `json:"bmcNewApi"`
	LMFUrls             []string `json:"lmfUrls"`
	GMLCUrl             *string  `json:"gmlcUrl"`
	TPlatformUrl        *string  `json:"tPlatformUrl"`
	WifiPositioning     *bool    `json:"wifiPositioning"`
	SFTPUsername        *string  `json:"sftpUsername"`
	SFTPPassword        *string  `json:"sftpPassword"`

	// Nested per-system settings (mirror Java ZTPSettingDTO). Preferred by
	// the external orchestrator; fall back to flat URLs when nil.
	PTP              *PTPSetting              `json:"ptp"`
	SpectrumSpatial  *SpectrumSpatialSetting  `json:"spectrumSpatial"`
	MSAG             *ExternalEndpointSetting `json:"msag"`
	BMC              *ExternalEndpointSetting `json:"bmc"`
	NewBMC           *ExternalEndpointSetting `json:"newBmc"`
	LMF              *ExternalEndpointSetting `json:"lmf"`
	LMF2             *ExternalEndpointSetting `json:"lmf2"`
	LMF3             *ExternalEndpointSetting `json:"lmf3"`
	LMF4             *ExternalEndpointSetting `json:"lmf4"`
	GMLC             *ExternalEndpointSetting `json:"gmlc"`
	TPlatform        *TPlatformSetting        `json:"tPlatform"`
}

// SystemConfig 对应 system_config 表 (generic key-value config store)
type SystemConfig struct {
	Id     string  `gorm:"primaryKey;column:id;type:varchar(255)" json:"id"`
	Config *string `gorm:"column:config;type:longtext" json:"config"`
}

func (SystemConfig) TableName() string { return "system_config" }

// ---------- ZTP DTOs ----------

// ListZTPResultsRequest is the JSON body for POST /ztp/results.
type ListZTPResultsRequest struct {
	SearchText    string `json:"searchText"`
	Progress      *int   `json:"progress"`
	Result        *int   `json:"result"`
	SerialNumbers string `json:"serialNumbers"`
	Succeed       *bool  `json:"succeed"`
	DeviceGroupId string `json:"deviceGroupId"`
	Page          int    `json:"page"`
	PageSize      int    `json:"pageSize"`
}

// ZTPResultVo is the response item for ZTP results listing.
type ZTPResultVo struct {
	ElementId       int64  `json:"elementId"`
	DeviceName      string `json:"deviceName"`
	SerialNumber    string `json:"serialNumber"`
	Progress        *int   `json:"progress"`
	Info            string `json:"info"`
	Result          string `json:"result"`
	ZTPFileName     string `json:"ztpFileName"`
	TenancyName     string `json:"tenancyName"`
	StartTime       string `json:"startTime"`
	EndTime         string `json:"endTime"`
	Status          string `json:"status"`
	CurrentProgress int    `json:"currentProgress"`
	Mode            string `json:"mode"`
	MAC             string `json:"mac"`
}

// ZTPRetryLogVo is the response item for retry log listing.
type ZTPRetryLogVo struct {
	OperationDate string `json:"operationDate"`
	Message       string `json:"message"`
}

// HistoryZTPFileVo is the response item for history ZTP file listing.
type HistoryZTPFileVo struct {
	Id           int64  `json:"id"`
	ElementId    int64  `json:"elementId"`
	NeName       string `json:"neName"`
	FileName     string `json:"fileName"`
	GenerateTime string `json:"generateTime"`
}

// SetZTPStatusRequest is the JSON body for POST /ztp/status.
type SetZTPStatusRequest struct {
	ElementIds []int64 `json:"elementIds" binding:"required"`
	Status     string  `json:"status" binding:"required"` // "enable" or "disable"
}

// BatchReZTPRequest is the JSON body for POST /ztp/batch-reztp.
type BatchReZTPRequest struct {
	Scope          string  `json:"scope"` // "element", "deviceGroup", "market"
	ElementIds     []int64 `json:"elementIds"`
	DeviceGroupIds []string `json:"deviceGroupIds"`
	Markets        []string `json:"markets"`
}

// DeleteZTPFileRequest is the JSON body for POST /ztp/delete-files.
type DeleteZTPFileRequest struct {
	ElementIds []int64 `json:"elementIds" binding:"required"`
}

// --- North interface tables ---

// NorthReport 对应 north_report 表
type NorthReport struct {
	Id         int        `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskName   *string    `gorm:"column:task_name;type:varchar(255)" json:"task_name"`
	TaskType   *string    `gorm:"column:task_type;type:varchar(255)" json:"task_type"`
	ServerUrl  *string    `gorm:"column:server_url;type:varchar(255)" json:"server_url"`
	TaskState  *bool      `gorm:"column:task_state" json:"task_state"`
	Deleted    *int       `gorm:"column:deleted" json:"deleted"`
	CreateTime *time.Time `gorm:"column:create_time" json:"create_time"`
	UpdateTime *time.Time `gorm:"column:update_time" json:"update_time"`
	TenantId  *int       `gorm:"column:tenant_id" json:"tenant_id"`
}

func (NorthReport) TableName() string { return "north_report" }

// NorthInterfaceLog 对应 north_interface_log 表
type NorthInterfaceLog struct {
	Id             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	LogName        *string    `gorm:"column:log_name;type:varchar(255)" json:"log_name"`
	User           *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	OperationTime  *time.Time `gorm:"column:operation_time" json:"operation_time"`
	Result         *int       `gorm:"column:result" json:"result"`
	RequestData    *string    `gorm:"column:request_data;type:longtext" json:"request_data"`
	ElementId      *int64     `gorm:"column:element_id" json:"element_id"`
	PresetTaskId   *int       `gorm:"column:preset_task_id" json:"preset_task_id"`
	Info           *string    `gorm:"column:info;type:text" json:"info"`
	TenantId      *int       `gorm:"column:tenant_id" json:"tenant_id"`
}

func (NorthInterfaceLog) TableName() string { return "north_interface_log" }

// --- Other tables ---

// Radius 对应 radius 表
type Radius struct {
	Id           int     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         *string `gorm:"column:name;type:varchar(255);uniqueIndex" json:"name"`
	Host         *string `gorm:"column:host;type:varchar(255)" json:"host"`
	Port         *int    `gorm:"column:port" json:"port"`
	AuthProtocol *string `gorm:"column:auth_protocol;type:varchar(255)" json:"auth_protocol"`
	TenantId    *int    `gorm:"column:tenant_id" json:"tenant_id"`
	SharedSecret *string `gorm:"column:shared_secret;type:varchar(255)" json:"shared_secret"`
}

func (Radius) TableName() string { return "radius" }

// UploadFile 对应 upload_file 表
type UploadFile struct {
	FileId           string     `gorm:"primaryKey;type:varchar(32)" json:"file_id"`
	FilePath         *string    `gorm:"column:file_path;type:varchar(255)" json:"file_path"`
	FileSize         *string    `gorm:"column:file_size;type:varchar(255)" json:"file_size"`
	FileSuffix       *string    `gorm:"column:file_suffix;type:varchar(255)" json:"file_suffix"`
	FileName         *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	FileMd5          *string    `gorm:"column:file_md5;type:varchar(255)" json:"file_md5"`
	FileStatus       *int       `gorm:"column:file_status" json:"file_status"`
	CreateTime       *time.Time `gorm:"column:create_time" json:"create_time"`
	UpdateTime       *time.Time `gorm:"column:update_time" json:"update_time"`
	Username         *string    `gorm:"column:username;type:varchar(255)" json:"username"`
	OriginalFileName *string    `gorm:"column:original_file_name;type:varchar(255)" json:"original_file_name"`
}

func (UploadFile) TableName() string { return "upload_file" }

// EmailNoticeResult 对应 email_notice_result 表
type EmailNoticeResult struct {
	Id              int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	AlarmTemplateId *int       `gorm:"column:alarm_template_id" json:"alarm_template_id"`
	EmailSubject    *string    `gorm:"column:email_subject;type:longtext" json:"email_subject"`
	EmailRecipient  *string    `gorm:"column:email_recipient;type:varchar(255)" json:"email_recipient"`
	PostTime        *time.Time `gorm:"column:post_time" json:"post_time"`
	Result          *int       `gorm:"column:result" json:"result"`
	FailureReason   *string    `gorm:"column:failure_reason;type:longtext" json:"failure_reason"`
}

func (EmailNoticeResult) TableName() string { return "email_notice_result" }

// SSHLabel 对应 ssh_label 表
type SSHLabel struct {
	Id        int     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      *string `gorm:"column:name;type:varchar(255)" json:"name"`
	Content   *string `gorm:"column:content;type:text" json:"content"`
	TenantId *int    `gorm:"column:tenant_id" json:"tenant_id"`
}

func (SSHLabel) TableName() string { return "ssh_label" }

// SystemOperatorLog 对应 system_operator_log 表
type SystemOperatorLog struct {
	Id               int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Username         *string    `gorm:"column:username;type:varchar(255)" json:"username"`
	IpAddress        *string    `gorm:"column:ip_address;type:varchar(255)" json:"ip_address"`
	LogName          *string    `gorm:"column:log_name;type:varchar(255)" json:"log_name"`
	RecordDetail     *string    `gorm:"column:record_detail;type:longtext" json:"record_detail"`
	Results          *int       `gorm:"column:results" json:"results"`
	FailureReason    *string    `gorm:"column:failure_reason;type:text" json:"failure_reason"`
	OperationStartTime *time.Time `gorm:"column:operation_start_time" json:"operation_start_time"`
	OperationEndTime *time.Time `gorm:"column:operation_end_time" json:"operation_end_time"`
	TenantId        *int       `gorm:"column:tenant_id" json:"tenant_id"`
}

func (SystemOperatorLog) TableName() string { return "system_operator_log" }

// ConfigUploadLog 对应 config_upload_log 表 (UUID主键)
type ConfigUploadLog struct {
	Id              string     `gorm:"primaryKey;type:varchar(32)" json:"id"`
	FileName        *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	ElementId       *int64     `gorm:"column:element_id" json:"element_id"`
	UploadTime      *time.Time `gorm:"column:upload_time" json:"upload_time"`
	Loc             *string    `gorm:"column:loc;type:varchar(255)" json:"loc"`
	TenantId       *int       `gorm:"column:tenant_id" json:"tenant_id"`
	OpenStationFile *bool      `gorm:"column:open_station_file" json:"open_station_file"`
	DeviceUpload    bool       `gorm:"column:device_upload;default:true" json:"device_upload"`
}

func (ConfigUploadLog) TableName() string { return "config_upload_log" }

// PresetParametersTask 对应 preset_parameters_task 表
type PresetParametersTask struct {
	Id         int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	Task       *string `gorm:"column:task;type:varchar(255)" json:"task"`
	ElementId  *int64  `gorm:"column:element_id" json:"element_id"`
	Parameters *string `gorm:"column:parameters;type:longtext" json:"parameters"`
	EventLogId *int64  `gorm:"column:event_log_id" json:"event_log_id"`
	Status     *int    `gorm:"column:status" json:"status"`
	InSet      *bool   `gorm:"column:in_set" json:"in_set"`
}

func (PresetParametersTask) TableName() string { return "preset_parameters_task" }

// ErrorInfo 对应 error_info 表
type ErrorInfo struct {
	Id        int     `gorm:"primaryKey;autoIncrement" json:"id"`
	ErrorCode *string `gorm:"column:error_code;type:varchar(255)" json:"error_code"`
	ErrorInfo *string `gorm:"column:error_info;type:longtext" json:"error_info"`
	TenantId *int    `gorm:"column:tenant_id" json:"tenant_id"`
}

func (ErrorInfo) TableName() string { return "error_info" }

// RemoteUpload 对应 remote_upload 表 (UUID主键)
type RemoteUpload struct {
	Id        string  `gorm:"primaryKey;type:varchar(32)" json:"id"`
	Name      *string `gorm:"column:name;type:varchar(255)" json:"name"`
	Loc       *string `gorm:"column:loc;type:varchar(255)" json:"loc"`
	Version   *string `gorm:"column:version;type:varchar(255)" json:"version"`
	TenantId *int    `gorm:"column:tenant_id" json:"tenant_id"`
}

func (RemoteUpload) TableName() string { return "remote_upload" }

// CallTraceFileLog 对应 call_trace_file_log 表 (UUID主键)
type CallTraceFileLog struct {
	Id             string     `gorm:"primaryKey;type:varchar(32)" json:"id"`
	NeId           *int64     `gorm:"column:ne_id;index:idx_ne_time" json:"ne_id"`
	FileName       *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	CollectionTime *time.Time `gorm:"column:collection_time;index:idx_ne_time" json:"collection_time"`
}

func (CallTraceFileLog) TableName() string { return "call_trace_file_log" }

// MACPMFileLog 对应 mac_pm_file_log 表
type MACPMFileLog struct {
	Id         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	FileName   *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	UploadTime *time.Time `gorm:"column:upload_time" json:"upload_time"`
	ElementId  *int64     `gorm:"column:element_id" json:"element_id"`
}

func (MACPMFileLog) TableName() string { return "mac_pm_file_log" }

// RPCMethod 对应 rpc_method 表 (UUID主键)
type RPCMethod struct {
	Id         string  `gorm:"primaryKey;type:varchar(32)" json:"id"`
	NeNeid     *int64  `gorm:"column:ne_neid" json:"ne_neid"`
	MethodName *string `gorm:"column:method_name;type:varchar(255)" json:"method_name"`
}

func (RPCMethod) TableName() string { return "rpc_method" }

// --- AOS Management tables ---

// TBG 对应 tbg 表 (Tunnel Border Gateway)
type TBG struct {
	Id         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name       *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	IP         *string    `gorm:"column:ip;type:varchar(255)" json:"ip"`
	Port       *int       `gorm:"column:port" json:"port"`
	TenantId  *int       `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime *time.Time `gorm:"column:create_time" json:"create_time"`
	UpdateTime *time.Time `gorm:"column:update_time" json:"update_time"`
}

func (TBG) TableName() string { return "tbg" }

// PSAPID 对应 psap_id 表
type PSAPID struct {
	Id         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	PsapId     *string    `gorm:"column:psap_id;type:varchar(255)" json:"psap_id"`
	Name       *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	Address    *string    `gorm:"column:address;type:varchar(255)" json:"address"`
	Latitude   *float64   `gorm:"column:latitude" json:"latitude"`
	Longitude  *float64   `gorm:"column:longitude" json:"longitude"`
	TenantId  *int       `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime *time.Time `gorm:"column:create_time" json:"create_time"`
}

func (PSAPID) TableName() string { return "psap_id" }

// PSAPIDSyncLog 对应 psap_id_sync_log 表
type PSAPIDSyncLog struct {
	Id         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Operator   *string    `gorm:"column:operator;type:varchar(255)" json:"operator"`
	Status     *int       `gorm:"column:status" json:"status"`
	Detail     *string    `gorm:"column:detail;type:text" json:"detail"`
	CreateTime *time.Time `gorm:"column:create_time" json:"create_time"`
}

func (PSAPIDSyncLog) TableName() string { return "psap_id_sync_log" }

// SpatialFileMarket 对应 spatial_file_market 表
type SpatialFileMarket struct {
	Id        int     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      *string `gorm:"column:name;type:varchar(255)" json:"name"`
	Code      *string `gorm:"column:code;type:varchar(255)" json:"code"`
	TenantId *int    `gorm:"column:tenant_id" json:"tenant_id"`
}

func (SpatialFileMarket) TableName() string { return "spatial_file_market" }

// ---------- AOS Management DTOs ----------

// ListTBGRequest is the JSON body for POST /tbg/list.
type ListTBGRequest struct {
	Name      string `json:"name"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
}

// AddTBGRequest is the JSON body for POST /tbg/add.
type AddTBGRequest struct {
	Name string `json:"name" binding:"required"`
	IP   string `json:"ip" binding:"required"`
	Port int    `json:"port" binding:"required"`
}

// ModifyTBGRequest is the JSON body for POST /tbg/modify.
type ModifyTBGRequest struct {
	Id   int64  `json:"id" binding:"required"`
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port *int   `json:"port"`
}

// DeleteTBGRequest is the JSON body for POST /tbg/delete.
type DeleteTBGRequest struct {
	Ids []int64 `json:"ids" binding:"required"`
}

// ListPSAPIDRequest is the JSON body for POST /psap-id/list.
type ListPSAPIDRequest struct {
	PsapId   string `json:"psapId"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

// SyncPSAPIDRequest is the JSON body for POST /psap-id/sync.
type SyncPSAPIDRequest struct {
	TenantId int `json:"tenantId"`
}

// MarketCoordinateRequest is the JSON body for POST /spatial-file/market-coordinates.
type MarketCoordinateRequest struct {
	MarketId int `json:"marketId" binding:"required"`
}
