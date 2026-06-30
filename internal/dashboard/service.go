package dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"

	redisclient "nmsappsrv/pkg/redis"
)

// Service handles business logic for Dashboard module
type Service struct {
	repo *Repository
}

// NewService creates a new Service
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ListCpeOnlineStatistics returns CPE online statistics aggregated by time buckets from device_statistic table
func (s *Service) ListCpeOnlineStatistics(ctx context.Context, query *ListCpeOnlineStatisticsQuery) ([]TimeAndDataVO, error) {
	return s.listOnlineStatistics(ctx, query, "cpe")
}

// ListGNBOnlineStatistics returns GNB online statistics aggregated by time buckets from device_statistic table
func (s *Service) ListGNBOnlineStatistics(ctx context.Context, query *ListCpeOnlineStatisticsQuery) ([]TimeAndDataVO, error) {
	return s.listOnlineStatistics(ctx, query, "gnb")
}

// listOnlineStatistics queries device_statistic for devices of a given type, aggregates by time bucket
func (s *Service) listOnlineStatistics(ctx context.Context, query *ListCpeOnlineStatisticsQuery, deviceType string) ([]TimeAndDataVO, error) {
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
func (s *Service) ListProductTypeAndDeviceCount(ctx context.Context, mode string, tenancyId *int) (*ListProductTypeAndDeviceCountVO, error) {
	if mode == "" {
		return nil, fmt.Errorf("mode is required")
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

// ListBaseStationStatistics returns base station statistics (simplified - returns empty maps)
func (s *Service) ListBaseStationStatistics(ctx context.Context, elementIds []int64, startTime, endTime time.Time) (*ListBaseStationStatisticsVO, error) {
	// This requires PM file processing which is complex
	// Return empty structure matching Java behavior when no PM data
	return &ListBaseStationStatisticsVO{
		CellAvailableRate: make(map[string][]TimeAndDataVO),
		Pdcp:              make(map[string][]TimeAndDataVO),
	}, nil
}

// ListPDCPTrafficStatistic returns PDCP traffic statistics by PLMN
func (s *Service) ListPDCPTrafficStatistic(ctx context.Context, startTime, endTime string, tenancyId *int) ([]ListPDCPTrafficStatisticVO, error) {
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
func (s *Service) ListDeviceOnlineInfo(ctx context.Context, tenancyId *int) (*ListDeviceOnlineInfoVO, error) {
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

// StatisticKPIForDevicelop returns KPI statistics for device groups (simplified stub)
func (s *Service) StatisticKPIForDevicelop(ctx context.Context, deviceGroupIds []string, granularity, gmt string, timestamp *int64) (*DashboardKPIStatisticVO, error) {
	if len(deviceGroupIds) == 0 {
		return nil, fmt.Errorf("deviceGroupId is required")
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

	return &DashboardKPIStatisticVO{
		StatisticStartTime: &startTime,
		StatisticEndTime:   &endTime,
		Granularity:        granularity,
		DeviceGroup:        []DashboardKPIDeviceGroupVO{}, // Empty as PM processing is not implemented
	}, nil
}
