package corenet

import "time"

// CoreNetwork 对应 core_network 表
type CoreNetwork struct {
	Id              int     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name            *string `gorm:"column:name;type:varchar(255)" json:"name"`
	ElementId       *int64  `gorm:"column:element_id" json:"element_id"`
	TenantId       *int    `gorm:"column:tenant_id" json:"tenant_id"`
	Deleted         *bool   `gorm:"column:deleted" json:"deleted"`
	InstallLocation *string `gorm:"column:install_location;type:varchar(255)" json:"install_location"`
	Ip              *string `gorm:"column:ip;type:varchar(255)" json:"ip"`
	Port            *int    `gorm:"column:port" json:"port"`
}

func (CoreNetwork) TableName() string { return "core_network" }

// CoreNetworkKpi 对应 core_network_kpi 表
type CoreNetworkKpi struct {
	Id           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	StartTime    *time.Time `gorm:"column:start_time" json:"start_time"`
	EndTime      *time.Time `gorm:"column:end_time" json:"end_time"`
	N6UlTraffic  *float64   `gorm:"column:n6_ul_traffic" json:"n6_ul_traffic"`
	N6DlTraffic  *float64   `gorm:"column:n6_dl_traffic" json:"n6_dl_traffic"`
	SgiUlTraffic *float64   `gorm:"column:sgi_ul_traffic" json:"sgi_ul_traffic"`
	SgiDlTraffic *float64   `gorm:"column:sgi_dl_traffic" json:"sgi_dl_traffic"`
	RmUid        *string    `gorm:"column:rm_uid;type:varchar(255)" json:"rm_uid"`
}

func (CoreNetworkKpi) TableName() string { return "core_network_kpi" }

// CoreNetworkOperationLog 对应 core_network_operation_log 表
type CoreNetworkOperationLog struct {
	Id             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	LogType        *string    `gorm:"column:log_type;type:varchar(255)" json:"log_type"`
	User           *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	OperationTime  *time.Time `gorm:"column:operation_time" json:"operation_time"`
	ResponseTime   *time.Time `gorm:"column:response_time" json:"response_time"`
	Result         *int       `gorm:"column:result" json:"result"`
	EventLogId     *int64     `gorm:"column:event_log_id" json:"event_log_id"`
	CoreNetworkId  *int       `gorm:"column:core_network_id" json:"core_network_id"`
	Info           *string    `gorm:"column:info;type:longtext" json:"info"`
	RequestId      *string    `gorm:"column:request_id;type:varchar(255)" json:"request_id"`
}

func (CoreNetworkOperationLog) TableName() string { return "core_network_operation_log" }

// CoreNetworkStatisticData 对应 core_network_statistic_data 表
type CoreNetworkStatisticData struct {
	Id             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ImsUeNumber    *int       `gorm:"column:ims_ue_number" json:"ims_ue_number"`
	SmfUeNumber    *int       `gorm:"column:smf_ue_number" json:"smf_ue_number"`
	StatisticTime  *time.Time `gorm:"column:statistic_time" json:"statistic_time"`
	CoreNetworkId  *int       `gorm:"column:core_network_id;index:idx_cn_time" json:"core_network_id"`
}

func (CoreNetworkStatisticData) TableName() string { return "core_network_statistic_data" }

// CoreNetworkAlarmVo is the response item for getCoreNetworkAlarms
// (mirrors Java GetCoreNetworkAlarmsVO). The Go side does not have a
// dedicated corenet_alarm table; the core_network_id link is not yet
// modelled. For now we surface recent license-scoped alarms as a
// proxy -- a future migration can add the proper join.
type CoreNetworkAlarmVo struct {
	Id              int64      `json:"id"`
	Severity        *string    `json:"severity"`
	AlarmIdentifier *string    `json:"alarmIdentifier"`
	ProbableCause   *string    `json:"probableCause"`
	EventTime       *time.Time `json:"eventTime"`
	AlarmStatus     *int       `json:"alarmStatus"`
}

// UeInfo is the response item for getUeInfos (mirrors Java ImsiDetailDTO).
// The Go side has no UE table; the endpoint returns an empty list until
// the schema lands.
type UeInfo struct {
	Imsi      string `json:"imsi"`
	Imei      string `json:"imei"`
	Msisdn    string `json:"msisdn"`
	State     string `json:"state"`
	StartTime string `json:"startTime"`
}

// UeListVo is the response item for listUEList (mirrors Java
// ListUEListVO). No Go schema yet; returns empty list.
type UeListVo struct {
	Imsi     string `json:"imsi"`
	Msisdn   string `json:"msisdn"`
	Category string `json:"category"`
}

// UeNumberStatisticVo is the response for listUENumberStatistic
// (mirrors Java ListUENumberStatisticVO). Aggregated UE counts.
type UeNumberStatisticVo struct {
	Total       int64            `json:"total"`
	ByCategory  map[string]int64 `json:"byCategory"`
	ByState     map[string]int64 `json:"byState"`
	GeneratedAt string           `json:"generatedAt"`
}

// CoreNetworkUserInfoVo is the response for getCoreNetworkUserInfo /
// getBuiltInCoreNetworkUserInfo (mirrors Java
// GetCoreNetworkUpfUserCountVO).
type CoreNetworkUserInfoVo struct {
	TotalUsers   int64            `json:"totalUsers"`
	ActiveUsers  int64            `json:"activeUsers"`
	IdleUsers    int64            `json:"idleUsers"`
	ByCoreNet    map[string]int64 `json:"byCoreNet"`
	GeneratedAt  string           `json:"generatedAt"`
}

// CoreNetworkUpfTrafficVo is the response for getCoreNetworkUpfTraffic /
// getBuiltInCoreNetworkUpfTraffic (mirrors Java
// GetCoreNetworkUpfTrafficVO).
type CoreNetworkUpfTrafficVo struct {
	UplinkBps   float64 `json:"uplinkBps"`
	DownlinkBps float64 `json:"downlinkBps"`
	TotalBytes  int64   `json:"totalBytes"`
	GeneratedAt string  `json:"generatedAt"`
}

// KpiReportRow is one row of the KPI report (mirrors Java's
// kpiReport/{index} response). Index selects the report kind.
type KpiReportRow struct {
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics"`
}
