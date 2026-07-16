package dashboard

import "time"

// TimeAndDataVO represents a time-series data point
type TimeAndDataVO struct {
	Time *time.Time `json:"time"`
	Data interface{} `json:"data"`
}

// ListProductTypeAndDeviceCountVO response for product type and device count
type ListProductTypeAndDeviceCountVO struct {
	Data []ProductTypeAndCount `json:"data"`
}

// ProductTypeAndCount represents count info for a product type
type ProductTypeAndCount struct {
	ProductType  string `json:"productType"`
	Count        int    `json:"count"`
	OnlineCount  int    `json:"onlineCount"`
	OfflineCount int    `json:"offlineCount"`
}

// ListBaseStationStatisticsVO response for base station statistics
type ListBaseStationStatisticsVO struct {
	CellAvailableRate map[string][]TimeAndDataVO `json:"cellAvailableRate"`
	Pdcp              map[string][]TimeAndDataVO `json:"pdcp"`
}

// ListPDCPTrafficStatisticVO response for PDCP traffic statistics
type ListPDCPTrafficStatisticVO struct {
	Plmn string  `json:"plmn"`
	Down float64 `json:"down"`
	Up   float64 `json:"up"`
}

// ListDeviceOnlineInfoVO response for device online info
type ListDeviceOnlineInfoVO struct {
	GnbOnlineCount  int `json:"gnbOnlineCount"`
	GnbOfflineCount int `json:"gnbOfflineCount"`
	EnbOnlineCount  int `json:"enbOnlineCount"`
	EnbOfflineCount int `json:"enbOfflineCount"`
	CpeOnlineCount  int `json:"cpeOnlineCount"`
	CpeOfflineCount int `json:"cpeOfflineCount"`
}

// DashboardKPIStatisticVO response for KPI statistics
type DashboardKPIStatisticVO struct {
	StatisticStartTime *time.Time                `json:"statisticStartTime"`
	StatisticEndTime   *time.Time                `json:"statisticEndTime"`
	Granularity        string                    `json:"granularity"`
	DeviceGroup        []DashboardKPIDeviceGroupVO `json:"deviceGroup"`
}

// DashboardKPIDeviceGroupVO represents KPI data for a device group
type DashboardKPIDeviceGroupVO struct {
	GroupId       string                      `json:"groupId"`
	GroupName     string                      `json:"groupName"`
	TenancyName   *string                     `json:"tenancyName"`
	StatisticsItem []DashboardKPIStatisticItemVO `json:"statisticsItem"`
}

// DashboardKPIStatisticItemVO represents a single per-bucket KPI statistic item,
// mirroring Java's DashboardKPIStatisticItemVO (scalar KPIs per time bucket).
type DashboardKPIStatisticItemVO struct {
	StatisticTime  *time.Time `json:"statisticTime"`
	CellAvailRatio float64    `json:"cellAvailRatio"`
	MacRxBytesUl   float64    `json:"macRxBytesUl"`
	MacTxBytesDl   float64    `json:"macTxBytesDl"`
}

// ---------- DTOs ----------

// ListCpeOnlineStatisticsQuery query for CPE/GNB online statistics
type ListCpeOnlineStatisticsQuery struct {
	ElementIds []int64    `json:"elementIds"`
	StartTime  *time.Time `json:"startTime"`
	EndTime    *time.Time `json:"endTime"`
}

// ListProductTypeAndDeviceCountQuery query for product type count
type ListProductTypeAndDeviceCountQuery struct {
	Mode string `json:"mode"` // "CPE", "eNB", "gNB"
}

// StatisticKPIForDevicelopQuery query for KPI statistics
type StatisticKPIForDevicelopQuery struct {
	DeviceGroupId []string `json:"deviceGroupId"`
	Granularity   string   `json:"granularity"` // "day" or "hour"
	Gmt           string   `json:"gmt"`
	Timestamp     *int64   `json:"timestamp"`
}
