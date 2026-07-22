package dashboard

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"nmsappsrv/pkg/apperror"
	redisclient "nmsappsrv/pkg/redis"
)

// Service defines the business-logic contract for Dashboard module.
type Service interface {
	ListCpeOnlineStatistics(ctx context.Context, query *ListCpeOnlineStatisticsQuery) ([]TimeAndDataVO, error)
	ListGNBOnlineStatistics(ctx context.Context, query *ListCpeOnlineStatisticsQuery) ([]TimeAndDataVO, error)
	ListProductTypeAndDeviceCount(ctx context.Context, mode string, tenantId *int) (*ListProductTypeAndDeviceCountVO, error)
	ListBaseStationStatistics(ctx context.Context, tenantId *int, elementIds []int64, startTime, endTime time.Time) (*ListBaseStationStatisticsVO, error)
	ListPDCPTrafficStatistic(ctx context.Context, startTime, endTime string, tenantId *int) ([]ListPDCPTrafficStatisticVO, error)
	ListDeviceOnlineInfo(ctx context.Context, tenantId *int) (*ListDeviceOnlineInfoVO, error)
	StatisticKPIForDevicelop(ctx context.Context, tenantId *int, deviceGroupIds []string, granularity, gmt string, timestamp *int64) (*DashboardKPIStatisticVO, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by the given Repository.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// ListCpeOnlineStatistics returns CPE online statistics aggregated by time buckets from device_statistic table
func (s *service) ListCpeOnlineStatistics(ctx context.Context, query *ListCpeOnlineStatisticsQuery) ([]TimeAndDataVO, error) {
	return s.listOnlineStatistics(ctx, query, "cpe")
}

// ListGNBOnlineStatistics returns GNB online statistics aggregated by time buckets from device_statistic table
func (s *service) ListGNBOnlineStatistics(ctx context.Context, query *ListCpeOnlineStatisticsQuery) ([]TimeAndDataVO, error) {
	return s.listOnlineStatistics(ctx, query, "gnb")
}

// listOnlineStatistics queries device_statistic for devices of a given type, aggregates by time bucket
func (s *service) listOnlineStatistics(ctx context.Context, query *ListCpeOnlineStatisticsQuery, deviceType string) ([]TimeAndDataVO, error) {
	// Default time range: last 24 hours
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if query.StartTime != nil {
		startTime = *query.StartTime
	}
	if query.EndTime != nil {
		endTime = *query.EndTime
	}

	rows, err := s.repo.QueryOnlineStatistics(ctx, query.ElementIds, startTime, endTime, deviceType)
	if err != nil {
		return nil, err
	}

	// Group by time bucket, count online/offline
	type bucketData struct {
		OnlineCount  int `json:"onlineCount"`
		OfflineCount int `json:"offlineCount"`
	}
	buckets := make(map[time.Time]*bucketData)

	for _, row := range rows {
		statTime, _ := row["statistic_time"].(time.Time)
		// Truncate to hour bucket
		bucket := time.Date(statTime.Year(), statTime.Month(), statTime.Day(), statTime.Hour(), 0, 0, 0, statTime.Location())

		if _, exists := buckets[bucket]; !exists {
			buckets[bucket] = &bucketData{}
		}

		online, _ := row["online"].(bool)
		if online {
			buckets[bucket].OnlineCount++
		} else {
			buckets[bucket].OfflineCount++
		}
	}

	// Convert to sorted time-series
	var result []TimeAndDataVO
	for t, data := range buckets {
		bucketTime := t
		result = append(result, TimeAndDataVO{
			Time: &bucketTime,
			Data: data,
		})
	}

	// Sort by time ascending
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Time.After(*result[j].Time) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

// ListProductTypeAndDeviceCount returns device count grouped by product type with online/offline status
func (s *service) ListProductTypeAndDeviceCount(ctx context.Context, mode string, tenantId *int) (*ListProductTypeAndDeviceCountVO, error) {
	if mode == "" {
		return nil, apperror.ErrInvalidInput.WithMessage("mode is required")
	}

	devices, err := s.repo.ListDevicesByMode(ctx, mode, tenantId)
	if err != nil {
		return nil, err
	}

	// Collect element IDs and check online status from Redis
	var elementIds []int64
	type DeviceInfo struct {
		ModelName   string
		ElementId   int64
	}
	var deviceInfos []DeviceInfo

	for _, d := range devices {
		modelName, _ := d["model_name"].(string)
		elementIdFloat, _ := d["ne_neid"].(float64)
		elementId := int64(elementIdFloat)

		deviceInfos = append(deviceInfos, DeviceInfo{
			ModelName: modelName,
			ElementId: elementId,
		})
		elementIds = append(elementIds, elementId)
	}

	// Check online status from Redis
	onlineMap := make(map[int64]bool)
	if len(elementIds) > 0 {
		keys := make([]string, len(elementIds))
		for i, id := range elementIds {
			keys[i] = fmt.Sprintf("online_%d", id)
		}
		values, _ := redisclient.MGet(ctx, keys...)
		for i, val := range values {
			if val != nil && strings.ToLower(val.(string)) == "yes" {
				onlineMap[elementIds[i]] = true
			}
		}
	}

	// Group by product type
	typeCounts := make(map[string]*ProductTypeAndCount)
	for _, info := range deviceInfos {
		productType := info.ModelName
		if productType == "" {
			productType = "unknown"
		}

		if _, exists := typeCounts[productType]; !exists {
			typeCounts[productType] = &ProductTypeAndCount{
				ProductType: productType,
			}
		}

		tc := typeCounts[productType]
		tc.Count++
		if onlineMap[info.ElementId] {
			tc.OnlineCount++
		} else {
			tc.OfflineCount++
		}
	}

	// Convert to slice
	var data []ProductTypeAndCount
	for _, tc := range typeCounts {
		data = append(data, *tc)
	}

	return &ListProductTypeAndDeviceCountVO{Data: data}, nil
}

// ListBaseStationStatistics returns per-base-station PM KPI series (cell availability + PDCP UL/DL)
// keyed by "serialNumber(measObjLdn)", mirroring Java DashboardManagementServiceImpl.listBaseStationStatistics.
// The faithful source is pm_kpi_measurement (parsed PM-file KPIs), not the pre-aggregated dashboard_pm_statistic_data.
func (s *service) ListBaseStationStatistics(ctx context.Context, tenantId *int, elementIds []int64, startTime, endTime time.Time) (*ListBaseStationStatisticsVO, error) {
	if len(elementIds) == 0 {
		return &ListBaseStationStatisticsVO{}, nil
	}

	// serialNumber lookup per element (Java keys series by serialNumber(ldn)).
	devices, err := s.repo.GetDeviceByIds(ctx, elementIds)
	if err != nil {
		return nil, err
	}
	serialByElement := make(map[int64]string, len(devices))
	for _, d := range devices {
		id := toInt64(d["ne_neid"])
		sn, _ := d["serial_number"].(string)
		serialByElement[id] = sn
	}

	kpiNames := []string{"CELL.AvailRatio", "PDCP.RxBytesUl", "PDCP.TxBytesDl"}
	rows, err := s.repo.QueryKpiMeasurements(ctx, elementIds, startTime, endTime, kpiNames)
	if err != nil {
		return nil, err
	}

	cellAvailableRate := make(map[string][]TimeAndDataVO)
	pdcp := make(map[string][]TimeAndDataVO)

	for _, row := range rows {
		elementId := toInt64(row["element_id"])
		kpiName, _ := row["kpi_name"].(string)
		ldn, _ := row["meas_obj_ldn"].(string)
		val := toFloat64(row["measured_value"])
		mt, _ := row["measure_time"].(time.Time)
		if mt.IsZero() {
			continue
		}
		sn := serialByElement[elementId]
		key := sn + "(" + ldn + ")"
		t := mt
		switch kpiName {
		case "CELL.AvailRatio":
			cellAvailableRate[key] = append(cellAvailableRate[key], TimeAndDataVO{Time: &t, Data: val * 100})
		case "PDCP.RxBytesUl":
			pdcp[key+" UL"] = append(pdcp[key+" UL"], TimeAndDataVO{Time: &t, Data: val})
		case "PDCP.TxBytesDl":
			pdcp[key+" DL"] = append(pdcp[key+" DL"], TimeAndDataVO{Time: &t, Data: val})
		}
	}

	// Sort each per-ldn series by time ascending for a stable time-series.
	sortSeriesMap(cellAvailableRate)
	sortSeriesMap(pdcp)

	return &ListBaseStationStatisticsVO{
		CellAvailableRate: cellAvailableRate,
		Pdcp:              pdcp,
	}, nil
}

// ListPDCPTrafficStatistic returns PDCP traffic statistics by PLMN
func (s *service) ListPDCPTrafficStatistic(ctx context.Context, startTime, endTime string, tenantId *int) ([]ListPDCPTrafficStatisticVO, error) {
	results, err := s.repo.ListPDCPTraffic(ctx, startTime, endTime, tenantId)
	if err != nil {
		return nil, err
	}

	var vos []ListPDCPTrafficStatisticVO
	for _, r := range results {
		plmn, _ := r["plmn"].(string)
		dlTraffic, _ := r["SUM(dl_traffic)"].(float64)
		ulTraffic, _ := r["SUM(ul_traffic)"].(float64)

		vos = append(vos, ListPDCPTrafficStatisticVO{
			Plmn: plmn,
			Down: dlTraffic,
			Up:   ulTraffic,
		})
	}

	return vos, nil
}

// ListDeviceOnlineInfo returns counts of online/offline devices by type
func (s *service) ListDeviceOnlineInfo(ctx context.Context, tenantId *int) (*ListDeviceOnlineInfoVO, error) {
	devices, err := s.repo.ListAllDevices(ctx, tenantId)
	if err != nil {
		return nil, err
	}

	// Collect element IDs
	var elementIds []int64
	type DeviceTypeInfo struct {
		ElementId   int64
		DeviceType  string
		Generation  string
	}
	var deviceTypes []DeviceTypeInfo

	for _, d := range devices {
		elementIdFloat, _ := d["ne_neid"].(float64)
		elementId := int64(elementIdFloat)
		deviceType, _ := d["device_type"].(string)
		generation, _ := d["generation"].(string)

		deviceTypes = append(deviceTypes, DeviceTypeInfo{
			ElementId:  elementId,
			DeviceType: deviceType,
			Generation: generation,
		})
		elementIds = append(elementIds, elementId)
	}

	// Check online status from Redis
	onlineMap := make(map[int64]bool)
	if len(elementIds) > 0 {
		keys := make([]string, len(elementIds))
		for i, id := range elementIds {
			keys[i] = fmt.Sprintf("online_%d", id)
		}
		values, _ := redisclient.MGet(ctx, keys...)
		for i, val := range values {
			if val != nil && strings.ToLower(val.(string)) == "yes" {
				onlineMap[elementIds[i]] = true
			}
		}
	}

	vo := &ListDeviceOnlineInfoVO{}
	for _, dt := range deviceTypes {
		online := onlineMap[dt.ElementId]

		if dt.DeviceType == "enb" {
			if dt.Generation == "NR" {
				if online {
					vo.GnbOnlineCount++
				} else {
					vo.GnbOfflineCount++
				}
			} else {
				if online {
					vo.EnbOnlineCount++
				} else {
					vo.EnbOfflineCount++
				}
			}
		} else {
			if online {
				vo.CpeOnlineCount++
			} else {
				vo.CpeOfflineCount++
			}
		}
	}

	return vo, nil
}

// StatisticKPIForDevicelop returns per-bucket CELL availability + MAC UL/DL KPIs for device groups,
// mirroring Java DashboardManagementServiceImpl.statisticKPIForDevicelop and
// PerformanceUtil.dashboardAggregateKpiData. Source is pm_kpi_measurement.
func (s *service) StatisticKPIForDevicelop(ctx context.Context, tenantId *int, deviceGroupIds []string, granularity, gmt string, timestamp *int64) (*DashboardKPIStatisticVO, error) {
	if len(deviceGroupIds) == 0 {
		return nil, apperror.ErrInvalidInput.WithMessage("deviceGroupId is required")
	}

	if granularity != "day" && granularity != "hour" {
		granularity = "day"
	}
	if gmt == "" {
		gmt = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(gmt)
	if err != nil {
		loc = time.FixedZone("Asia/Shanghai", 8*3600)
	}

	endTime := time.Now()
	var startTime time.Time
	if timestamp != nil {
		startTime = time.UnixMilli(*timestamp)
	} else {
		start := endTime.AddDate(0, 0, -6)
		startTime = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	}

	groups, err := s.repo.QueryDeviceGroupsByIds(ctx, deviceGroupIds)
	if err != nil {
		return nil, err
	}

	tenancyNames, err := s.repo.GetTenancyNames(ctx)
	if err != nil {
		return nil, err
	}

	kpiNames := []string{"CELL.AvailRatio", "MAC.RxBytesUl", "MAC.TxBytesDl"}
	axis := generateTimeAxis(startTime, endTime, granularity, loc)

	var deviceGroupVOs []DashboardKPIDeviceGroupVO

	for _, g := range groups {
		groupId, _ := g["id"].(string)
		groupName, _ := g["group_name"].(string)

		var licensePtr *int
		if lid := toInt64(g["tenant_id"]); lid > 0 {
			li := int(lid)
			licensePtr = &li
		}
		tn := tenancyNameFor(tenancyNames, licensePtr)

		elementIds, err := s.repo.QueryGroupElementIdsFiltered(ctx, groupId, licensePtr)
		if err != nil {
			deviceGroupVOs = append(deviceGroupVOs, buildEmptyGroupVO(groupId, groupName, tn, axis, granularity, loc))
			continue
		}

		rows, err := s.repo.QueryKpiMeasurements(ctx, elementIds, startTime, endTime, kpiNames)
		if err != nil {
			deviceGroupVOs = append(deviceGroupVOs, buildEmptyGroupVO(groupId, groupName, tn, axis, granularity, loc))
			continue
		}

		// Bucket measurements by GMT bucket key; drop WPS ldns (mirrors Java filter).
		buckets := make(map[time.Time][]kpiPoint)
		for _, row := range rows {
			ldn, _ := row["meas_obj_ldn"].(string)
			if strings.Contains(strings.ToUpper(ldn), "WPS") {
				continue
			}
			kpiName, _ := row["kpi_name"].(string)
			val := toFloat64(row["measured_value"])
			mt, _ := row["measure_time"].(time.Time)
			if mt.IsZero() {
				continue
			}
			bucket := gmtBucketKey(mt, granularity, loc)
			buckets[bucket] = append(buckets[bucket], kpiPoint{KpiName: kpiName, Value: val})
		}

		var items []DashboardKPIStatisticItemVO
		for _, bucket := range axis {
			st := gmtStatisticTime(bucket, granularity, loc)
			bt := st
			var availSum float64
			var availCount int
			var rxSum, txSum float64
			for _, p := range buckets[bucket] {
				switch p.KpiName {
				case "CELL.AvailRatio":
					availSum += p.Value
					availCount++
				case "MAC.RxBytesUl":
					rxSum += p.Value
				case "MAC.TxBytesDl":
					txSum += p.Value
				}
			}
			var availRatio float64
			if availCount > 0 {
				availRatio = availSum / float64(availCount) * 100
			}
			items = append(items, DashboardKPIStatisticItemVO{
				StatisticTime:  &bt,
				CellAvailRatio: round6(availRatio),
				MacRxBytesUl:   round6(rxSum),
				MacTxBytesDl:   round6(txSum),
			})
		}

		deviceGroupVOs = append(deviceGroupVOs, DashboardKPIDeviceGroupVO{
			GroupId:        groupId,
			GroupName:      groupName,
			TenancyName:    tn,
			StatisticsItem: items,
		})
	}

	if deviceGroupVOs == nil {
		deviceGroupVOs = []DashboardKPIDeviceGroupVO{}
	}

	return &DashboardKPIStatisticVO{
		StatisticStartTime: &startTime,
		StatisticEndTime:   &endTime,
		Granularity:        granularity,
		DeviceGroup:        deviceGroupVOs,
	}, nil
}

// ---------- KPI aggregation helpers (mirror Java PerformanceUtil.dashboardAggregateKpiData) ----------

type kpiPoint struct {
	KpiName string
	Value   float64
}

// toInt64 / toFloat64 coerce gorm map-scan values (numeric columns often arrive as float64) to Go scalars.
func toInt64(v interface{}) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	case float32:
		return int64(x)
	default:
		return 0
	}
}

func toFloat64(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int64:
		return float64(x)
	case int:
		return float64(x)
	default:
		return 0
	}
}

func round6(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

// gmtBucketKey snaps t to the bucket key Java uses: 23:59:59 for day, :59:59 for hour (in loc).
func gmtBucketKey(t time.Time, granularity string, loc *time.Location) time.Time {
	tl := t.In(loc)
	if granularity == "hour" {
		return time.Date(tl.Year(), tl.Month(), tl.Day(), tl.Hour(), 59, 59, 0, loc)
	}
	return time.Date(tl.Year(), tl.Month(), tl.Day(), 23, 59, 59, 0, loc)
}

// gmtStatisticTime normalizes a bucket key to the output statisticTime: midnight for day, top-of-hour for hour.
func gmtStatisticTime(bucket time.Time, granularity string, loc *time.Location) time.Time {
	bl := bucket.In(loc)
	if granularity == "hour" {
		return time.Date(bl.Year(), bl.Month(), bl.Day(), bl.Hour(), 0, 0, 0, loc)
	}
	return time.Date(bl.Year(), bl.Month(), bl.Day(), 0, 0, 0, 0, loc)
}

// nextBucket advances a bucket key by one step (day/hour) keeping the snapped wall-clock in loc.
func nextBucket(b time.Time, granularity string, loc *time.Location) time.Time {
	bl := b.In(loc)
	if granularity == "hour" {
		return time.Date(bl.Year(), bl.Month(), bl.Day(), bl.Hour()+1, 59, 59, 0, loc)
	}
	return time.Date(bl.Year(), bl.Month(), bl.Day()+1, 23, 59, 59, 0, loc)
}

// generateTimeAxis builds the continuous list of bucket keys spanning [startTime, endTime] (inclusive).
func generateTimeAxis(startTime, endTime time.Time, granularity string, loc *time.Location) []time.Time {
	startBucket := gmtBucketKey(startTime, granularity, loc)
	endBucket := gmtBucketKey(endTime, granularity, loc)
	if startBucket.After(endBucket) {
		return nil
	}
	var axis []time.Time
	for b := startBucket; !b.After(endBucket); b = nextBucket(b, granularity, loc) {
		axis = append(axis, b)
	}
	return axis
}

func tenancyNameFor(m map[int]string, tenantId *int) *string {
	if tenantId == nil {
		return nil
	}
	if name, ok := m[*tenantId]; ok {
		return &name
	}
	return nil
}

func sortSeriesMap(m map[string][]TimeAndDataVO) {
	for _, series := range m {
		sort.SliceStable(series, func(i, j int) bool {
			if series[i].Time == nil || series[j].Time == nil {
				return false
			}
			return series[i].Time.Before(*series[j].Time)
		})
	}
}

// buildEmptyGroupVO creates a group VO with zero-valued per-bucket KPI items (used when a group errors or has no data).
func buildEmptyGroupVO(groupId, groupName string, tenancyName *string, axis []time.Time, granularity string, loc *time.Location) DashboardKPIDeviceGroupVO {
	var items []DashboardKPIStatisticItemVO
	for _, bucket := range axis {
		st := gmtStatisticTime(bucket, granularity, loc)
		bt := st
		items = append(items, DashboardKPIStatisticItemVO{StatisticTime: &bt})
	}
	return DashboardKPIDeviceGroupVO{
		GroupId:        groupId,
		GroupName:      groupName,
		TenancyName:    tenancyName,
		StatisticsItem: items,
	}
}
