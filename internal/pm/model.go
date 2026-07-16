package pm

import "time"

// PerformanceKpi 对应 performance_kpi 表
type PerformanceKpi struct {
	Id                     string     `gorm:"primaryKey;type:varchar(32)" json:"id"`
	KpiName                *string    `gorm:"column:kpi_name;type:varchar(255)" json:"kpi_name"`
	KpiNameTranslation     *string    `gorm:"column:kpi_name_translation;type:varchar(255)" json:"kpi_name_translation"`
	Kpi                    *string    `gorm:"column:kpi;type:varchar(255)" json:"kpi"`
	Unit                   *string    `gorm:"column:unit;type:varchar(255)" json:"unit"`
	UnitTranslation        *string    `gorm:"column:unit_translation;type:varchar(255)" json:"unit_translation"`
	StatisticType          *string    `gorm:"column:statistic_type;type:varchar(255)" json:"statistic_type"`
	Description            *string    `gorm:"column:description;type:longtext" json:"description"`
	DescriptionTranslation *string    `gorm:"column:description_translation;type:longtext" json:"description_translation"`
	TriggerPoint           *string    `gorm:"column:trigger_point;type:longtext" json:"trigger_point"`
	TriggerPointTranslation *string   `gorm:"column:trigger_point_translation;type:longtext" json:"trigger_point_translation"`
	TenancyId              *int       `gorm:"column:tenancy_id" json:"tenancy_id"`
	KpiSetId               *int       `gorm:"column:kpi_set_id" json:"kpi_set_id"`
	IdFormula              *string    `gorm:"column:id_formula;type:varchar(255)" json:"id_formula"`
	UpdateTime             *time.Time `gorm:"column:update_time" json:"update_time"`
	UpdateUser             *string    `gorm:"column:update_user;type:varchar(255)" json:"update_user"`
	Type                   *int       `gorm:"column:type" json:"type"`
	DefaultKpi             *bool      `gorm:"column:default_kpi" json:"default_kpi"`
}

func (PerformanceKpi) TableName() string { return "performance_kpi" }

// PerformanceKpiSet 对应 performance_kpi_set 表
type PerformanceKpiSet struct {
	Id          int     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        *string `gorm:"column:name;type:varchar(255)" json:"name"`
	StandardSet *bool   `gorm:"column:standard_set" json:"standard_set"`
	TenancyId   *int    `gorm:"column:tenancy_id" json:"tenancy_id"`
	DefaultSet  *bool   `gorm:"column:default_set" json:"default_set"`
}

func (PerformanceKpiSet) TableName() string { return "performance_kpi_set" }

// PerformanceKpiTemplate 对应 performance_kpi_template 表
type PerformanceKpiTemplate struct {
	Id              int        `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateName    *string    `gorm:"column:template_name;type:varchar(255)" json:"template_name"`
	UpdateTime      *time.Time `gorm:"column:update_time" json:"update_time"`
	User            *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	Description     *string    `gorm:"column:description;type:varchar(255)" json:"description"`
	TenancyId       *int       `gorm:"column:tenancy_id" json:"tenancy_id"`
	DefaultTemplate *bool      `gorm:"column:default_template" json:"default_template"`
}

func (PerformanceKpiTemplate) TableName() string { return "performance_kpi_template" }

// PerformanceKpiTemplateHasElement 对应 performance_kpi_template_has_element 表
type PerformanceKpiTemplateHasElement struct {
	Id         int    `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateId *int   `gorm:"column:template_id" json:"template_id"`
	ElementId  *int64 `gorm:"column:element_id" json:"element_id"`
}

func (PerformanceKpiTemplateHasElement) TableName() string { return "performance_kpi_template_has_element" }

// PerformanceKpiTemplateHasKpi 对应 performance_kpi_template_has_kpi 表
type PerformanceKpiTemplateHasKpi struct {
	Id         int     `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateId *int    `gorm:"column:template_id" json:"template_id"`
	KpiId      *string `gorm:"column:kpi_id;type:varchar(32)" json:"kpi_id"`
	Sort       *int    `gorm:"column:sort" json:"sort"`
}

func (PerformanceKpiTemplateHasKpi) TableName() string { return "performance_kpi_template_has_kpi" }

// PMFileLog 对应 pm_file_log 表 (UUID主键)
type PMFileLog struct {
	Id             string     `gorm:"primaryKey;type:varchar(32)" json:"id"`
	NeId           *int64     `gorm:"column:ne_id;index:idx_ne_start" json:"ne_id"`
	FileName       *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	CollectionTime *time.Time `gorm:"column:collection_time" json:"collection_time"`
	StartTime      *time.Time `gorm:"column:start_time;index:idx_ne_start" json:"start_time"`
	TenancyId      *int       `gorm:"column:tenancy_id;index:idx_tenancy_start" json:"tenancy_id"`
}

func (PMFileLog) TableName() string { return "pm_file_log" }

// KpiAlarmTemplate 对应 kpi_alarm_template 表
type KpiAlarmTemplate struct {
	Id          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        *string    `gorm:"column:name;type:varchar(255)" json:"name"`
	Description *string    `gorm:"column:description;type:varchar(255)" json:"description"`
	Enable      *bool      `gorm:"column:enable" json:"enable"`
	ScopeMode   *int       `gorm:"column:scope_mode" json:"scope_mode"`
	TenancyId   *int       `gorm:"column:tenancy_id;uniqueIndex:idx_tenancy_name" json:"tenancy_id"`
	UpdateTime  *time.Time `gorm:"column:update_time" json:"update_time"`
	User        *string    `gorm:"column:user;type:varchar(255)" json:"user"`
}

func (KpiAlarmTemplate) TableName() string { return "kpi_alarm_template" }

// KpiAlarmTemplateHasElement 对应 kpi_alarm_template_has_element 表
type KpiAlarmTemplateHasElement struct {
	Id         int    `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateId *int   `gorm:"column:template_id" json:"template_id"`
	ElementId  *int64 `gorm:"column:element_id" json:"element_id"`
}

func (KpiAlarmTemplateHasElement) TableName() string { return "kpi_alarm_template_has_element" }

// KpiAlarmTemplateHasGroup 对应 kpi_alarm_template_has_group 表
type KpiAlarmTemplateHasGroup struct {
	Id         int     `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateId *int    `gorm:"column:template_id" json:"template_id"`
	GroupId    *string `gorm:"column:group_id;type:varchar(32)" json:"group_id"`
}

func (KpiAlarmTemplateHasGroup) TableName() string { return "kpi_alarm_template_has_group" }

// KpiAlarmTemplateHasKpiThreshold 对应 kpi_alarm_template_has_kpi_threshold 表
type KpiAlarmTemplateHasKpiThreshold struct {
	Id          int      `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateId  *int     `gorm:"column:template_id" json:"template_id"`
	KpiId       *string  `gorm:"column:kpi_id;type:varchar(32)" json:"kpi_id"`
	Threshold   *float64 `gorm:"column:threshold" json:"threshold"`
	TriggerMode *int     `gorm:"column:trigger_mode" json:"trigger_mode"`
}

func (KpiAlarmTemplateHasKpiThreshold) TableName() string { return "kpi_alarm_template_has_kpi_threshold" }

// DashboardPmStatisticData 对应 dashboard_pm_statistic_data 表
type DashboardPmStatisticData struct {
	Id              int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Time            *time.Time `gorm:"column:time" json:"time"`
	PdcpUlRate      *float64   `gorm:"column:pdcp_ul_rate" json:"pdcp_ul_rate"`
	PdcpDlRate      *float64   `gorm:"column:pdcp_dl_rate" json:"pdcp_dl_rate"`
	CellAvailableRate *float64 `gorm:"column:cell_available_rate" json:"cell_available_rate"`
	TenancyId       *int       `gorm:"column:tenancy_id" json:"tenancy_id"`
}

func (DashboardPmStatisticData) TableName() string { return "dashboard_pm_statistic_data" }

// PDCPTraffic 对应 pdcp_traffic 表
type PDCPTraffic struct {
	Id            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	TenancyId     *int       `gorm:"column:tenancy_id" json:"tenancy_id"`
	UlTraffic     *float64   `gorm:"column:ul_traffic" json:"ul_traffic"`
	DlTraffic     *float64   `gorm:"column:dl_traffic" json:"dl_traffic"`
	StatisticTime *time.Time `gorm:"column:statistic_time" json:"statistic_time"`
	Plmn          *string    `gorm:"column:plmn;type:varchar(255)" json:"plmn"`
}

func (PDCPTraffic) TableName() string { return "pdcp_traffic" }

// ---------- Dashboard DTOs ----------

// DeviceOnlineInfoVO 设备在线信息统计 (gNB/eNB/CPE 各自在线/离线数)
type DeviceOnlineInfoVO struct {
	GnbTotal    int64 `json:"gnbTotal"`
	GnbOnline   int64 `json:"gnbOnline"`
	GnbOffline  int64 `json:"gnbOffline"`
	EnbTotal    int64 `json:"enbTotal"`
	EnbOnline   int64 `json:"enbOnline"`
	EnbOffline  int64 `json:"enbOffline"`
	CpeTotal    int64 `json:"cpeTotal"`
	CpeOnline   int64 `json:"cpeOnline"`
	CpeOffline  int64 `json:"cpeOffline"`
}

// ProductTypeAndCount 按产品型号统计设备数量及在线情况
type ProductTypeAndCount struct {
	ProductType string `json:"productType"`
	Count       int64  `json:"count"`
	OnlineCount int64  `json:"onlineCount"`
	OfflineCount int64 `json:"offlineCount"`
}

// elementRow 用于从 cpe_element 表查询的中间结构
type elementRow struct {
	NeNeid     int64   `gorm:"column:ne_neid"`
	DeviceType *string `gorm:"column:device_type"`
	Generation *string `gorm:"column:generation"`
	ModelName  *string `gorm:"column:model_name"`
}

// MeasDeviceVo is the API response item for listKPIMeas (paginated eNB
// devices eligible for KPI measurement). Mirrors the Java response shape.
type MeasDeviceVo struct {
	NeNeid      int64   `json:"neNeid"`
	DeviceName  *string `json:"deviceName"`
	SerialNumber *string `json:"serialNumber"`
	DeviceType  *string `json:"deviceType"`
	RootNode    *string `json:"rootNode"`
	// MeasSwitch is the current state of FAP.PerfMgmt.Config.1.Enable
	// for the device (true if a recent value is "1"). nil if unknown.
	MeasSwitch *bool `json:"measSwitch"`
}
