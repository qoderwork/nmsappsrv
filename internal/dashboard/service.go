package dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nmsappsrv/pkg/apperror"
	redisclient "nmsappsrv/pkg/redis"
)

// Service defines the business-logic contract for Dashboard module.
type Service interface {
	ListCpeOnlineStatistics(ctx context.Context, query *ListCpeOnlineStatisticsQuery) ([]TimeAndDataVO, error)
	ListGNBOnlineStatistics(ctx context.Context, query *ListCpeOnlineStatisticsQuery) ([]TimeAndDataVO, error)
	ListProductTypeAndDeviceCount(ctx context.Context, mode string, tenancyId *int) (*ListProductTypeAndDeviceCountVO, error)
	ListBaseStationStatistics(ctx context.Context, tenancyId *int, elementIds []int64, startTime, endTime time.Time) (*ListBaseStationStatisticsVO, error)
	ListPDCPTrafficStatistic(ctx context.Context, startTime, endTime string, tenancyId *int) ([]ListPDCPTrafficStatisticVO, error)
	ListDeviceOnlineInfo(ctx context.Context, tenancyId *int) (*ListDeviceOnlineInfoVO, error)
	StatisticKPIForDevicelop(ctx context.Context, tenancyId *int, deviceGroupIds []string, granularity, gmt string, timestamp *int64) (*DashboardKPIStatisticVO, error)
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
func (s *service) ListProductTypeAndDeviceCount(ctx context.Context, mode string, tenancyId *int) (*ListProductTypeAndDeviceCountVO, error) {
	if mode == "" {
		return nil, apperror.ErrInvalidInput.WithMessage("mode is required")
	}

	devices, err := s.repo.ListDevicesByMode(ctx, mode, tenancyId)
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

// ListBaseStationStatistics returns base station statistics from dashboard_pm_statistic_data
func (s *service) ListBaseStationStatistics(ctx context.Context, tenancyId *int, elementIds []int64, startTime, endTime time.Time) (*ListBaseStationStatisticsVO, error) {
	rows, err := s.repo.QueryDashboardPmData(ctx, tenancyId, startTime, endTime)
	if err != nil {
		return nil, err
	}

	cellAvailableRate := make(map[string][]TimeAndDataVO)
	pdcp := make(map[string][]TimeAndDataVO)

	// Use "all" as the default PLMN key since dashboard_pm_statistic_data has no PLMN column
	const plmnKey = "all"

	var cellSeries []TimeAndDataVO
	var pdcpSeries []TimeAndDataVO

	for _, row := range rows {
		rowTime, _ := row["time"].(time.Time)
		t := rowTime

		// CellAvailableRate
		if car, ok := row["cell_available_rate"]; ok && car != nil {
			carVal, _ := car.(float64)
			cellSeries = append(cellSeries, TimeAndDataVO{
				Time: &t,
				Data: carVal,
			})
		}

		// Pdcp (UL + DL)
		ulVal := 0.0
		dlVal := 0.0
		if ul, ok := row["pdcp_ul_rate"]; ok && ul != nil {
			ulVal, _ = ul.(float64)
		}
		if dl, ok := row["pdcp_dl_rate"]; ok && dl != nil {
			dlVal, _ = dl.(float64)
		}
		pdcpSeries = append(pdcpSeries, TimeAndDataVO{
			Time: &t,
			Data: PdcpData{UlRate: ulVal, DlRate: dlVal},
		})
	}

	if len(cellSeries) > 0 {
		cellAvailableRate[plmnKey] = cellSeries
	}
	if len(pdcpSeries) > 0 {
		pdcp[plmnKey] = pdcpSeries
	}

	return &ListBaseStationStatisticsVO{
		CellAvailableRate: cellAvailableRate,
		Pdcp:              pdcp,
	}, nil
}

// ListPDCPTrafficStatistic returns PDCP traffic statistics by PLMN
func (s *service) ListPDCPTrafficStatistic(ctx context.Context, startTime, endTime string, tenancyId *int) ([]ListPDCPTrafficStatisticVO, error) {
	results, err := s.repo.ListPDCPTraffic(ctx, startTime, endTime, tenancyId)
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
func (s *service) ListDeviceOnlineInfo(ctx context.Context, tenancyId *int) (*ListDeviceOnlineInfoVO, error) {
	devices, err := s.repo.ListAllDevices(ctx, tenancyId)
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

// StatisticKPIForDevicelop returns KPI statistics for device groups
func (s *service) StatisticKPIForDevicelop(ctx context.Context, tenancyId *int, deviceGroupIds []string, granularity, gmt string, timestamp *int64) (*DashboardKPIStatisticVO, error) {
	if len(deviceGroupIds) == 0 {
		return nil, apperror.ErrInvalidInput.WithMessage("deviceGroupId is required")
	}

	// Default granularity
	if granularity != "day" && granularity != "hour" {
		granularity = "day"
	}

	// Default timezone
	if gmt == "" {
		gmt = "Asia/Shanghai"
	}

	// Calculate time range
	var startTime time.Time
	endTime := time.Now()

	if timestamp != nil {
		startTime = time.UnixMilli(*timestamp)
	} else {
		// Last 7 days
		startTime = endTime.AddDate(0, 0, -6)
		startTime = time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, startTime.Location())
	}

	// Query device groups
	groups, err := s.repo.QueryDeviceGroupsByIds(ctx, deviceGroupIds)
	if err != nil {
		return nil, err
	}

	var deviceGroupVOs []DashboardKPIDeviceGroupVO

	for _, g := range groups {
		groupId, _ := g["id"].(string)
		groupName, _ := g["group_name"].(string)

		// Get element IDs for this group
		elementIds, err := s.repo.QueryGroupElementIds(ctx, groupId)
		if err != nil {
			continue
		}

		totalDevices := len(elementIds)

		// Query device_statistic for these elements in the time range
		statRows, err := s.repo.QueryDeviceStatisticForGroup(ctx, elementIds, startTime, endTime)
		if err != nil {
			// If query fails, still include the group with zero-valued items
			deviceGroupVOs = append(deviceGroupVOs, buildEmptyGroupVO(groupId, groupName, totalDevices, &startTime, &endTime, granularity))
			continue
		}

		// Aggregate online/offline counts by time bucket
		type bucketCounts struct {
			online  int
			offline int
		}
		buckets := make(map[time.Time]*bucketCounts)

		for _, row := range statRows {
			statTime, _ := row["statistic_time"].(time.Time)
			var bucket time.Time
			if granularity == "hour" {
				bucket = time.Date(statTime.Year(), statTime.Month(), statTime.Day(), statTime.Hour(), 0, 0, 0, statTime.Location())
			} else {
				bucket = time.Date(statTime.Year(), statTime.Month(), statTime.Day(), 0, 0, 0, 0, statTime.Location())
			}

			if _, exists := buckets[bucket]; !exists {
				buckets[bucket] = &bucketCounts{}
			}

			online, _ := row["online"].(bool)
			if online {
				buckets[bucket].online++
			} else {
				buckets[bucket].offline++
			}
		}

		// Build sorted time series for DeviceOnline and DeviceOffline
		var onlineSeries []TimeAndDataVO
		var offlineSeries []TimeAndDataVO
		var times []time.Time
		for t := range buckets {
			times = append(times, t)
		}
		// Sort times ascending
		for i := 0; i < len(times); i++ {
			for j := i + 1; j < len(times); j++ {
				if times[i].After(times[j]) {
					times[i], times[j] = times[j], times[i]
				}
			}
		}

		for _, t := range times {
			bc := buckets[t]
			bt := t
			onlineSeries = append(onlineSeries, TimeAndDataVO{Time: &bt, Data: bc.online})
			offlineSeries = append(offlineSeries, TimeAndDataVO{Time: &bt, Data: bc.offline})
		}

		// Build DeviceTotal series (constant = totalDevices for each time point)
		var totalSeries []TimeAndDataVO
		for _, t := range times {
			bt := t
			totalSeries = append(totalSeries, TimeAndDataVO{Time: &bt, Data: totalDevices})
		}

		groupVO := DashboardKPIDeviceGroupVO{
			GroupId:   groupId,
			GroupName: groupName,
			StatisticsItem: []DashboardKPIStatisticItemVO{
				{KpiName: "DeviceOnline", Data: onlineSeries},
				{KpiName: "DeviceOffline", Data: offlineSeries},
				{KpiName: "DeviceTotal", Data: totalSeries},
			},
		}

		deviceGroupVOs = append(deviceGroupVOs, groupVO)
	}

	// If no groups found, return empty slice (groups requested but not found in DB)
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

// buildEmptyGroupVO creates a group VO with zero-valued KPI items
func buildEmptyGroupVO(groupId, groupName string, totalDevices int, startTime, endTime *time.Time, granularity string) DashboardKPIDeviceGroupVO {
	return DashboardKPIDeviceGroupVO{
		GroupId:   groupId,
		GroupName: groupName,
		StatisticsItem: []DashboardKPIStatisticItemVO{
			{KpiName: "DeviceOnline", Data: []TimeAndDataVO{}},
			{KpiName: "DeviceOffline", Data: []TimeAndDataVO{}},
			{KpiName: "DeviceTotal", Data: []TimeAndDataVO{}},
		},
	}
}
