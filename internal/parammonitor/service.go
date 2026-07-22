package parammonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/internal/mq"
	"nmsappsrv/internal/opmsg"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Service defines the business-logic contract for parameter monitoring.
type Service interface {
	AddMonitorConfig(c *gin.Context, req *AddMonitorConfigRequest) error
	DeleteMonitorConfig(req *DeleteMonitorConfigRequest) error
	ViewMonitorConfig(req *ViewMonitorConfigRequest) (*MonitorConfigDetailVo, error)
	UpdateMonitorConfig(req *UpdateMonitorConfigRequest) error
	ListMonitorConfigs(c *gin.Context, req *ListMonitorConfigRequest) ([]MonitorConfigVo, int64, error)
	ToggleMonitorConfig(req *ToggleMonitorConfigRequest) error
	GetRealtimeMonitorData(req *RealtimeMonitorDataRequest) ([]RealtimeMonitorDataVo, error)
	ReloadMonitorParameters(req *ReloadMonitorRequest) error
	BatchQueryDeviceParameters(req *BatchQueryDeviceParamRequest) ([]BatchQueryResultVo, error)
	BatchQueryDeviceParametersLive(req *BatchQueryLiveRequest, username string) ([]BatchQueryLiveResult, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

func NewService(db *gorm.DB) Service {
	return &service{
		repo: NewRepository(db),
	}
}

// countDevices calculates the actual device count based on scope and scope_data.
// scope=1: all devices; scope=2: selected (elementIds or groupIds in scope_data).
func (s *service) countDevices(scope int, scopeData *string) int {
	if scope == 1 {
		var count int64
		s.repo.DB().Table("cpe_element").Where("deleted = ?", false).Count(&count)
		return int(count)
	}

	if scopeData == nil || *scopeData == "" {
		return 0
	}

	var ids []int64
	if err := json.Unmarshal([]byte(*scopeData), &ids); err != nil || len(ids) == 0 {
		return 0
	}

	// Check if IDs are group IDs by looking in device_group table.
	// If matches found, count via group_has_element; otherwise treat as element IDs.
	var groupCount int64
	s.repo.DB().Table("device_group").
		Where("id IN ?", ids).
		Count(&groupCount)

	if groupCount > 0 {
		var elementCount int64
		s.repo.DB().Table("group_has_element").
			Where("group_id IN ?", ids).
			Count(&elementCount)
		return int(elementCount)
	}

	return len(ids)
}

// resolveScopeToElementIds resolves a monitor config's scope to actual element IDs.
// scope=1: all non-deleted devices; scope=2: parse scope_data (handles both elementIds and groupIds).
func (s *service) resolveScopeToElementIds(scope int, scopeData *string) ([]int64, error) {
	if scope == 1 {
		var elementIds []int64
		err := s.repo.DB().Table("cpe_element").
			Where("deleted = ?", false).
			Pluck("ne_neid", &elementIds).Error
		return elementIds, err
	}

	if scopeData == nil || *scopeData == "" {
		return nil, nil
	}

	var ids []int64
	if err := json.Unmarshal([]byte(*scopeData), &ids); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	// Check if IDs are group IDs
	var groupCount int64
	s.repo.DB().Table("device_group").Where("id IN ?", ids).Count(&groupCount)

	if groupCount > 0 {
		var elementIds []int64
		err := s.repo.DB().Table("group_has_element").
			Where("group_id IN ?", ids).
			Pluck("element_id", &elementIds).Error
		return elementIds, err
	}

	return ids, nil
}

func (s *service) AddMonitorConfig(c *gin.Context, req *AddMonitorConfigRequest) error {
	tenantId := middleware.GetTenantId(c)
	now := time.Now()

	config := ParameterMonitorConfig{
		Name:       &req.Name,
		TenantId:  &tenantId,
		Enable:     &req.Enable,
		Scope:      &req.Scope,
		ScopeData:  &req.ScopeData,
		Interval:   &req.Interval,
		CreateTime: &now,
		UpdateTime: &now,
	}

	if err := s.repo.Create(&config); err != nil {
		logger.Errorf("CreateConfig error: %v", err)
		return apperror.Wrap(err, "CREATE_MONITOR_CONFIG_FAILED", 500, "failed to create monitor config")
	}

	if err := s.repo.SetConfigParameters(config.Id, req.ParameterIds); err != nil {
		logger.Errorf("SetConfigParameters error: %v", err)
		return apperror.Wrap(err, "SET_MONITOR_CONFIG_PARAMS_FAILED", 500, "failed to set monitor config parameters")
	}

	return nil
}

func (s *service) DeleteMonitorConfig(req *DeleteMonitorConfigRequest) error {
	// Delete config
	if err := s.repo.DeleteByID(req.Id); err != nil {
		logger.Errorf("DeleteConfig error: %v", err)
		return apperror.Wrap(err, "DELETE_MONITOR_CONFIG_FAILED", 500, "failed to delete monitor config")
	}

	// Delete associations
	if err := s.repo.SetConfigParameters(req.Id, []string{}); err != nil {
		logger.Errorf("Delete associations error: %v", err)
		return apperror.Wrap(err, "DELETE_MONITOR_ASSOCIATIONS_FAILED", 500, "failed to delete monitor config associations")
	}

	return nil
}

func (s *service) ViewMonitorConfig(req *ViewMonitorConfigRequest) (*MonitorConfigDetailVo, error) {
	config, err := s.repo.FindByID(req.Id)
	if err != nil {
		return nil, apperror.Wrap(err, "VIEW_MONITOR_CONFIG_FAILED", 500, "failed to view monitor config")
	}

	paramIds, err := s.repo.GetConfigParameters(req.Id)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_MONITOR_CONFIG_PARAMS_FAILED", 500, "failed to get monitor config parameters")
	}

	paramMap, err := s.repo.GetParameterByIds(paramIds)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_MONITOR_PARAMETERS_FAILED", 500, "failed to get parameters")
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

func (s *service) UpdateMonitorConfig(req *UpdateMonitorConfigRequest) error {
	config, err := s.repo.FindByID(req.Id)
	if err != nil {
		return apperror.Wrap(err, "GET_MONITOR_CONFIG_FAILED", 404, "monitor config not found")
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

	if err := s.repo.Save(config); err != nil {
		return apperror.Wrap(err, "UPDATE_MONITOR_CONFIG_FAILED", 500, "failed to update monitor config")
	}

	if len(req.ParameterIds) > 0 {
		if err := s.repo.SetConfigParameters(req.Id, req.ParameterIds); err != nil {
			return apperror.Wrap(err, "SET_MONITOR_CONFIG_PARAMS_FAILED", 500, "failed to update monitor config parameters")
		}
	}

	return nil
}

func (s *service) ListMonitorConfigs(c *gin.Context, req *ListMonitorConfigRequest) ([]MonitorConfigVo, int64, error) {
	tenantId := middleware.GetTenantId(c)

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	configs, total, err := s.repo.ListConfigs(tenantId, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, apperror.Wrap(err, "LIST_MONITOR_CONFIGS_FAILED", 500, "failed to list monitor configs")
	}

	result := make([]MonitorConfigVo, 0, len(configs))
	for _, config := range configs {
		paramIds, _ := s.repo.GetConfigParameters(config.Id)

		deviceCount := 0
		if config.Scope != nil {
			deviceCount = s.countDevices(*config.Scope, config.ScopeData)
		}

		vo := MonitorConfigVo{
			Id:           config.Id,
			Name:         *config.Name,
			Enable:       *config.Enable,
			Scope:        *config.Scope,
			Interval:     *config.Interval,
			ParameterIds: paramIds,
			DeviceCount:  deviceCount,
			CreateTime:   config.CreateTime.Format("2006-01-02 15:04:05"),
		}
		result = append(result, vo)
	}

	return result, total, nil
}

func (s *service) ToggleMonitorConfig(req *ToggleMonitorConfigRequest) error {
	config, err := s.repo.FindByID(req.Id)
	if err != nil {
		return apperror.Wrap(err, "GET_MONITOR_CONFIG_FAILED", 404, "monitor config not found")
	}

	now := time.Now()
	config.Enable = &req.Enable
	config.UpdateTime = &now

	if err := s.repo.Save(config); err != nil {
		return apperror.Wrap(err, "TOGGLE_MONITOR_CONFIG_FAILED", 500, "failed to toggle monitor config")
	}
	return nil
}

func (s *service) GetRealtimeMonitorData(req *RealtimeMonitorDataRequest) ([]RealtimeMonitorDataVo, error) {
	config, err := s.repo.FindByID(req.ConfigId)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_MONITOR_CONFIG_FAILED", 404, "monitor config not found")
	}

	// Get devices in scope (handles both elementIds and groupIds)
	scope := 0
	if config.Scope != nil {
		scope = *config.Scope
	}
	elementIds, err := s.resolveScopeToElementIds(scope, config.ScopeData)
	if err != nil {
		return nil, apperror.Wrap(err, "RESOLVE_MONITOR_SCOPE_FAILED", 500, "failed to resolve monitor scope")
	}

	// Get parameter IDs for this config
	paramIds, err := s.repo.GetConfigParameters(req.ConfigId)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_MONITOR_CONFIG_PARAMS_FAILED", 500, "failed to get monitor config parameters")
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
		err := s.repo.DB().Table("cpe_element").
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
			err := s.repo.DB().Table("parameter_attributes").
				Where("id = ?", paramId).
				First(&attr).Error
			if err == nil && attr.ParameterName != nil {
				// Read actual parameter value from element_basic_info_parameter
				var paramValue string
				s.repo.DB().Table("element_basic_info_parameter").
					Where("element_id = ? AND param_name = ?", elementId, *attr.ParameterName).
					Pluck("param_value", &paramValue)

				parameters = append(parameters, ParameterValueVo{
					ParameterName: *attr.ParameterName,
					Value:         paramValue,
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

func (s *service) ReloadMonitorParameters(req *ReloadMonitorRequest) error {
	config, err := s.repo.FindByID(req.ConfigId)
	if err != nil {
		return apperror.Wrap(err, "GET_MONITOR_CONFIG_FAILED", 404, "monitor config not found")
	}

	if config.Enable == nil || !*config.Enable {
		return apperror.ErrInvalidInput.WithMessage("monitor config is disabled")
	}

	// Get parameter IDs
	paramIds, err := s.repo.GetConfigParameters(req.ConfigId)
	if err != nil {
		return apperror.Wrap(err, "GET_MONITOR_CONFIG_PARAMS_FAILED", 500, "failed to get monitor config parameters")
	}

	// Determine target devices
	elementIds := req.ElementIds
	if len(elementIds) == 0 {
		// Use all devices in scope
		if config.ScopeData != nil && *config.ScopeData != "" {
			if err := json.Unmarshal([]byte(*config.ScopeData), &elementIds); err != nil {
				return apperror.Wrap(err, "PARSE_SCOPE_DATA_FAILED", 400, "failed to parse scope data")
			}
		}
	}

	// Resolve parameter IDs -> full TR-069 paths (Java GPV needs names, not ids).
	paramMap, err := s.repo.GetParameterByIds(paramIds)
	if err != nil {
		return apperror.Wrap(err, "RESOLVE_MONITOR_PARAMETERS_FAILED", 500, "failed to resolve parameter paths")
	}
	paths := make([]string, 0, len(paramMap))
	for _, p := range paramMap {
		if p != "" {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return apperror.ErrInvalidInput.WithMessage("no resolved parameter paths for monitor config")
	}
	paramJSON, err := json.Marshal(paths)
	if err != nil {
		return apperror.Wrap(err, "MAR_MONITOR_PATHS_FAILED", 500, "failed to marshal parameter paths")
	}

	// Dispatch TR-069 GetParameterValues via the unified operation dispatcher
	// (Java EventType.GET_PARAMETER_VALUES → apiCommandProcessor.processCommand
	// → SendGetParameterValues). The GPV response handler persists returned
	// values into element_basic_info_parameter, which GetRealtimeMonitorData
	// and the threshold checker read.
	ctx := context.Background()
	expiredAt := time.Now().Add(5 * time.Minute).UnixMilli()
	for _, elementId := range elementIds {
		msg := opmsg.Message{
			EventType:      "GetParameterValues",
			NeNeid:         elementId,
			Operation:      "GetParameterValues",
			OperationParam: string(paramJSON),
			OperationUser:  "system",
			ProtocolType:   opmsg.ProtocolTR069,
			ExpiredAt:      expiredAt,
		}
		msgBytes, err := msg.Marshal()
		if err != nil {
			logger.Errorf("marshal opmsg for element %d: %v", elementId, err)
			continue
		}
		if err := redis.LPush(ctx, mq.OperationQueue, string(msgBytes)); err != nil {
			logger.Errorf("Push %s error: %v", mq.OperationQueue, err)
		}
	}

	return nil
}

func (s *service) BatchQueryDeviceParameters(req *BatchQueryDeviceParamRequest) ([]BatchQueryResultVo, error) {
	// Get parameter info
	paramMap, err := s.repo.GetParameterByIds(req.ParameterIds)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_PARAMETER_IDS_FAILED", 500, "failed to get parameter info")
	}

	result := make([]BatchQueryResultVo, 0, len(req.ElementIds))

	for _, elementId := range req.ElementIds {
		// Get device info
		var device struct {
			NeNeid       int64   `gorm:"column:ne_neid"`
			DeviceName   *string `gorm:"column:device_name"`
			SerialNumber *string `gorm:"column:serial_number"`
		}
		err := s.repo.DB().Table("cpe_element").
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
			err := s.repo.DB().Table("parameter_attributes").
				Where("id = ?", paramId).
				First(&attr).Error
			if err == nil && attr.ParameterName != nil {
				// Read actual parameter value from element_basic_info_parameter
				var paramValue string
				s.repo.DB().Table("element_basic_info_parameter").
					Where("element_id = ? AND param_name = ?", elementId, path).
					Pluck("param_value", &paramValue)

				parameters = append(parameters, ParameterValueVo{
					ParameterName: *attr.ParameterName,
					Value:         paramValue,
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
func (s *service) BatchQueryDeviceParametersLive(req *BatchQueryLiveRequest, username string) ([]BatchQueryLiveResult, error) {
	// 1. Resolve parameter IDs to paths
	paramMap, err := s.repo.GetParameterByIds(req.ParameterIds)
	if err != nil {
		return nil, apperror.Wrap(err, "RESOLVE_PARAMETERS_FAILED", 500, "failed to resolve parameters")
	}
	paramPaths := make([]string, 0, len(paramMap))
	for _, path := range paramMap {
		paramPaths = append(paramPaths, path)
	}
	if len(paramPaths) == 0 {
		return nil, apperror.ErrInvalidInput.WithMessage("no valid parameter paths found")
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
		if err := s.repo.DB().Table("cpe_element").
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
			if err := s.repo.DB().Table("event_log").Create(eventLog).Error; err != nil {
				res.Error = fmt.Sprintf("create event_log: %v", err)
				results[idx] = res
				return
			}
			// Retrieve the auto-generated ID
			var eventLogId int64
			s.repo.DB().Raw("SELECT LAST_INSERT_ID()").Scan(&eventLogId)

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
			s.repo.DB().Table("event_log").Where("id = ?", eventLogId).
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
				s.repo.DB().Table("event_log").Where("id = ?", eventLogId).Update("status", 4)
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

// newService creates a Service backed by the given mock Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}
