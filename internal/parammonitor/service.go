package parammonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct {
	repo *Repository
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		repo: NewRepository(db),
	}
}

func (s *Service) AddMonitorConfig(c *gin.Context, req *AddMonitorConfigRequest) error {
	licenseId := middleware.GetLicenseId(c)
	now := time.Now()

	config := ParameterMonitorConfig{
		Name:       &req.Name,
		LicenseId:  &licenseId,
		Enable:     &req.Enable,
		Scope:      &req.Scope,
		ScopeData:  &req.ScopeData,
		Interval:   &req.Interval,
		CreateTime: &now,
		UpdateTime: &now,
	}

	if err := s.repo.CreateConfig(&config); err != nil {
		logger.Errorf("CreateConfig error: %v", err)
		return err
	}

	if err := s.repo.SetConfigParameters(config.Id, req.ParameterIds); err != nil {
		logger.Errorf("SetConfigParameters error: %v", err)
		return err
	}

	return nil
}

func (s *Service) DeleteMonitorConfig(req *DeleteMonitorConfigRequest) error {
	// Delete config
	if err := s.repo.DeleteConfig(req.Id); err != nil {
		logger.Errorf("DeleteConfig error: %v", err)
		return err
	}

	// Delete associations
	if err := s.repo.SetConfigParameters(req.Id, []string{}); err != nil {
		logger.Errorf("Delete associations error: %v", err)
		return err
	}

	return nil
}

func (s *Service) ViewMonitorConfig(req *ViewMonitorConfigRequest) (*MonitorConfigDetailVo, error) {
	config, err := s.repo.GetConfig(req.Id)
	if err != nil {
		return nil, err
	}

	paramIds, err := s.repo.GetConfigParameters(req.Id)
	if err != nil {
		return nil, err
	}

	paramMap, err := s.repo.GetParameterByIds(paramIds)
	if err != nil {
		return nil, err
	}

	parameters := make([]ParameterVo, 0, len(paramIds))
	for _, id := range paramIds {
		path := paramMap[id]
		parameters = append(parameters, ParameterVo{
			Id:   id,
			Path: path,
			Name: path,
		})
	}

	vo := MonitorConfigDetailVo{
		Id:         config.Id,
		Name:       *config.Name,
		Enable:     *config.Enable,
		Scope:      *config.Scope,
		ScopeData:  *config.ScopeData,
		Interval:   *config.Interval,
		Parameters: parameters,
	}

	return &vo, nil
}

func (s *Service) UpdateMonitorConfig(req *UpdateMonitorConfigRequest) error {
	config, err := s.repo.GetConfig(req.Id)
	if err != nil {
		return err
	}

	now := time.Now()
	config.UpdateTime = &now

	if req.Name != "" {
		config.Name = &req.Name
	}
	if req.Enable != nil {
		config.Enable = req.Enable
	}
	if req.Scope != nil {
		config.Scope = req.Scope
	}
	if req.ScopeData != nil {
		config.ScopeData = req.ScopeData
	}
	if req.Interval != nil {
		config.Interval = req.Interval
	}

	if err := s.repo.UpdateConfig(config); err != nil {
		return err
	}

	if len(req.ParameterIds) > 0 {
		if err := s.repo.SetConfigParameters(req.Id, req.ParameterIds); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) ListMonitorConfigs(c *gin.Context, req *ListMonitorConfigRequest) ([]MonitorConfigVo, int64, error) {
	licenseId := middleware.GetLicenseId(c)

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	configs, total, err := s.repo.ListConfigs(licenseId, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]MonitorConfigVo, 0, len(configs))
	for _, config := range configs {
		paramIds, _ := s.repo.GetConfigParameters(config.Id)

		vo := MonitorConfigVo{
			Id:           config.Id,
			Name:         *config.Name,
			Enable:       *config.Enable,
			Scope:        *config.Scope,
			Interval:     *config.Interval,
			ParameterIds: paramIds,
			DeviceCount:  0, // TODO: calculate device count based on scope
			CreateTime:   config.CreateTime.Format("2006-01-02 15:04:05"),
		}
		result = append(result, vo)
	}

	return result, total, nil
}

func (s *Service) ToggleMonitorConfig(req *ToggleMonitorConfigRequest) error {
	config, err := s.repo.GetConfig(req.Id)
	if err != nil {
		return err
	}

	now := time.Now()
	config.Enable = &req.Enable
	config.UpdateTime = &now

	return s.repo.UpdateConfig(config)
}

func (s *Service) GetRealtimeMonitorData(req *RealtimeMonitorDataRequest) ([]RealtimeMonitorDataVo, error) {
	config, err := s.repo.GetConfig(req.ConfigId)
	if err != nil {
		return nil, err
	}

	// Get devices in scope
	var elementIds []int64
	if config.ScopeData != nil && *config.ScopeData != "" {
		if err := json.Unmarshal([]byte(*config.ScopeData), &elementIds); err != nil {
			logger.Errorf("Unmarshal scope_data error: %v", err)
			return nil, err
		}
	}

	// Get parameter IDs for this config
	paramIds, err := s.repo.GetConfigParameters(req.ConfigId)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	result := make([]RealtimeMonitorDataVo, 0, len(elementIds))

	for _, elementId := range elementIds {
		// Check device online status
		onlineStr, _ := redis.Get(ctx, fmt.Sprintf("online_%d", elementId))
		online := onlineStr == "true"

		// Get device info
		var device struct {
			NeNeid       int64   `gorm:"column:ne_neid"`
			DeviceName   *string `gorm:"column:device_name"`
			SerialNumber *string `gorm:"column:serial_number"`
		}
		err := s.repo.db.Table("cpe_element").
			Where("ne_neid = ? AND deleted = ?", elementId, false).
			First(&device).Error
		if err != nil {
			logger.Warnf("Device %d not found", elementId)
			continue
		}

		// Get parameter values from parameter_attributes
		parameters := make([]ParameterValueVo, 0, len(paramIds))
		for _, paramId := range paramIds {
			var attr struct {
				ParameterName *string `gorm:"column:parameter_name"`
			}
			err := s.repo.db.Table("parameter_attributes").
				Where("id = ?", paramId).
				First(&attr).Error
			if err == nil && attr.ParameterName != nil {
				// TODO: Read actual parameter value from device cache or real-time query
				parameters = append(parameters, ParameterValueVo{
					ParameterName: *attr.ParameterName,
					Value:         "", // Placeholder - would come from device
				})
			}
		}

		vo := RealtimeMonitorDataVo{
			ElementId:    elementId,
			DeviceName:   *device.DeviceName,
			SerialNumber: *device.SerialNumber,
			Online:       online,
			Parameters:   parameters,
		}
		result = append(result, vo)
	}

	return result, nil
}

func (s *Service) ReloadMonitorParameters(req *ReloadMonitorRequest) error {
	config, err := s.repo.GetConfig(req.ConfigId)
	if err != nil {
		return err
	}

	if config.Enable == nil || !*config.Enable {
		return fmt.Errorf("monitor config is disabled")
	}

	// Get parameter IDs
	paramIds, err := s.repo.GetConfigParameters(req.ConfigId)
	if err != nil {
		return err
	}

	// Determine target devices
	elementIds := req.ElementIds
	if len(elementIds) == 0 {
		// Use all devices in scope
		if config.ScopeData != nil && *config.ScopeData != "" {
			if err := json.Unmarshal([]byte(*config.ScopeData), &elementIds); err != nil {
				return err
			}
		}
	}

	ctx := context.Background()
	for _, elementId := range elementIds {
		// Build TR-069 GetParameterValues command
		cmd := map[string]interface{}{
			"eventType":    "getParameterValues",
			"elementId":    elementId,
			"parameterIds": paramIds,
		}
		cmdBytes, _ := json.Marshal(cmd)

		// Push to Redis operation queue
		if err := redis.LPush(ctx, "operation_queue", string(cmdBytes)); err != nil {
			logger.Errorf("Push operation_queue error: %v", err)
		}
	}

	return nil
}

func (s *Service) BatchQueryDeviceParameters(req *BatchQueryDeviceParamRequest) ([]BatchQueryResultVo, error) {
	// Get parameter info
	paramMap, err := s.repo.GetParameterByIds(req.ParameterIds)
	if err != nil {
		return nil, err
	}

	result := make([]BatchQueryResultVo, 0, len(req.ElementIds))

	for _, elementId := range req.ElementIds {
		// Get device info
		var device struct {
			NeNeid       int64   `gorm:"column:ne_neid"`
			DeviceName   *string `gorm:"column:device_name"`
			SerialNumber *string `gorm:"column:serial_number"`
		}
		err := s.repo.db.Table("cpe_element").
			Where("ne_neid = ? AND deleted = ?", elementId, false).
			First(&device).Error
		if err != nil {
			logger.Warnf("Device %d not found", elementId)
			continue
		}

		// Get parameter values
		parameters := make([]ParameterValueVo, 0, len(req.ParameterIds))
		for _, paramId := range req.ParameterIds {
			path := paramMap[paramId]

			var attr struct {
				ParameterName *string `gorm:"column:parameter_name"`
			}
			err := s.repo.db.Table("parameter_attributes").
				Where("element_id = ? AND id = ?", elementId, paramId).
				First(&attr).Error
			if err == nil && attr.ParameterName != nil {
				parameters = append(parameters, ParameterValueVo{
					ParameterName: *attr.ParameterName,
					Value:         path, // Placeholder
				})
			}
		}

		vo := BatchQueryResultVo{
			ElementId:    elementId,
			DeviceName:   *device.DeviceName,
			SerialNumber: *device.SerialNumber,
			Parameters:   parameters,
		}
		result = append(result, vo)
	}

	return result, nil
}

// BatchQueryDeviceParametersLive sends live GetParameterValues commands to multiple
// devices concurrently via TR-069. It resolves parameter IDs to paths, builds SOAP
// GPV XML for each device, creates event_log tracking entries, and pushes to device
// queues. Returns dispatch status per device (actual values arrive asynchronously).
func (s *Service) BatchQueryDeviceParametersLive(req *BatchQueryLiveRequest, username string) ([]BatchQueryLiveResult, error) {
	// 1. Resolve parameter IDs to paths
	paramMap, err := s.repo.GetParameterByIds(req.ParameterIds)
	if err != nil {
		return nil, fmt.Errorf("resolve parameters: %w", err)
	}
	paramPaths := make([]string, 0, len(paramMap))
	for _, path := range paramMap {
		paramPaths = append(paramPaths, path)
	}
	if len(paramPaths) == 0 {
		return nil, fmt.Errorf("no valid parameter paths found")
	}

	// 2. Resolve device info for all element IDs
	type deviceInfo struct {
		ElementId    int64
		SerialNumber string
		DeviceName   string
	}
	devices := make([]deviceInfo, 0, len(req.ElementIds))
	for _, elementId := range req.ElementIds {
		var d struct {
			NeNeid       int64   `gorm:"column:ne_neid"`
			SerialNumber *string `gorm:"column:serial_number"`
			DeviceName   *string `gorm:"column:device_name"`
		}
		if err := s.repo.db.Table("cpe_element").
			Where("ne_neid = ? AND deleted = ?", elementId, false).
			Scan(&d).Error; err != nil {
			logger.Warnf("BatchQueryLive: device %d not found: %v", elementId)
			continue
		}
		if d.SerialNumber == nil || *d.SerialNumber == "" {
			logger.Warnf("BatchQueryLive: device %d has no serial number", elementId)
			continue
		}
		name := ""
		if d.DeviceName != nil {
			name = *d.DeviceName
		}
		devices = append(devices, deviceInfo{
			ElementId:    elementId,
			SerialNumber: *d.SerialNumber,
			DeviceName:   name,
		})
	}

	// 3. Concurrently dispatch GPV to each device
	results := make([]BatchQueryLiveResult, len(devices))
	var wg sync.WaitGroup
	wg.Add(len(devices))

	for i, dev := range devices {
		go func(idx int, d deviceInfo) {
			defer wg.Done()
			res := BatchQueryLiveResult{
				ElementId:    d.ElementId,
				SerialNumber: d.SerialNumber,
				DeviceName:   d.DeviceName,
			}

			// Create event_log entry (status=1 pending)
			now := time.Now()
			eventLog := map[string]interface{}{
				"event_type":         "GetParameterValues",
				"element_id":         d.ElementId,
				"username":           username,
				"status":             1,
				"command_issue_time": now,
				"create_time":        now,
			}
			if err := s.repo.db.Table("event_log").Create(eventLog).Error; err != nil {
				res.Error = fmt.Sprintf("create event_log: %v", err)
				results[idx] = res
				return
			}
			// Retrieve the auto-generated ID
			var eventLogId int64
			s.repo.db.Raw("SELECT LAST_INSERT_ID()").Scan(&eventLogId)

			// Build SOAP GPV XML
			headerId := soap.GenerateHeaderID()
			soapXml := soap.BuildGetParameterValues(headerId, paramPaths)

			// Update event_log with tracking data
			trackData, _ := json.Marshal(map[string]interface{}{
				"header_id":      headerId,
				"serial_number":  d.SerialNumber,
				"operation_type": "GET_PARAMETER_VALUES",
				"event_log_id":   eventLogId,
				"param_names":    paramPaths,
				"issue_time":     now.Format(time.RFC3339),
			})
			s.repo.db.Table("event_log").Where("id = ?", eventLogId).
				Updates(map[string]interface{}{
					"command_track_data": string(trackData),
				})

			// Cache track data in Redis for response correlation
			ctx := context.Background()
			trackKey := fmt.Sprintf("tr069:track:%s", headerId)
			trackJson, _ := json.Marshal(map[string]interface{}{
				"header_id":      headerId,
				"sn":             d.SerialNumber,
				"operation_type": "GET_PARAMETER_VALUES",
				"event_log_id":   eventLogId,
				"is_live_query":  true,
			})
			redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

			// Push SOAP XML to device queue
			queueKey := fmt.Sprintf("tr069:queue:%s", d.SerialNumber)
			if err := redis.LPush(ctx, queueKey, soapXml); err != nil {
				s.repo.db.Table("event_log").Where("id = ?", eventLogId).Update("status", 4)
				res.Error = fmt.Sprintf("push to queue: %v", err)
				results[idx] = res
				return
			}

			res.Dispatched = true
			res.EventLogId = eventLogId
			results[idx] = res
		}(i, dev)
	}

	wg.Wait()
	return results, nil
}
