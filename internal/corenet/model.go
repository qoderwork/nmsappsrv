package corenet

import "time"

// CoreNetwork 对应 core_network 表
type CoreNetwork struct {
	Id              int     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name            *string `gorm:"column:name;type:varchar(255)" json:"name"`
	ElementId       *int64  `gorm:"column:element_id" json:"element_id"`
	TenancyId       *int    `gorm:"column:tenancy_id" json:"tenancy_id"`
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
