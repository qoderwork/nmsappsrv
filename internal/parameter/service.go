package parameter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// Service contains the business logic for parameter management.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ---------------------------------------------------------------------------
// ParameterAttributes
// ---------------------------------------------------------------------------

// GetParameters returns all parameter attributes for the given element.
func (s *Service) GetParameters(elementId int64) ([]ParameterAttributes, error) {
	return s.repo.FindParametersByElementId(elementId)
}

// SetParameter updates a parameter value for the given element and dispatches
// the change to the device via TR-069 SetParameterValues.
func (s *Service) SetParameter(elementId int64, paramName string, value string, username string) error {
	ctx := context.Background()

	// 1. Look up device serial number
	var deviceInfo struct {
		SerialNumber string `gorm:"column:serial_number"`
		DeviceName   string `gorm:"column:device_name"`
	}
	if err := s.repo.db.Table("cpe_element").
		Select("serial_number, device_name").
		Where("ne_neid = ? AND deleted = ?", elementId, false).
		Scan(&deviceInfo).Error; err != nil {
		return fmt.Errorf("device not found: %w", err)
	}
	if deviceInfo.SerialNumber == "" {
		return fmt.Errorf("device %d has no serial number", elementId)
	}

	// 2. Get old value from element_basic_info_parameter (for audit log)
	var oldValue string
	s.repo.db.Table("element_basic_info_parameter").
		Select("param_value").
		Where("element_id = ? AND param_name = ?", elementId, paramName).
		Scan(&oldValue)

	// 3. Create event_log tracking entry (status=1 pending)
	now := time.Now()
	eventLogId, err := s.repo.InsertEventLog("SetParameterValues", elementId, username, 1, "")
	if err != nil {
		return fmt.Errorf("create event_log: %w", err)
	}

	// 4. Build operationParam JSON for tracking
	opParam, _ := json.Marshal([]setParamEntry{{ParamName: paramName, ParamValue: value}})

	// 5. Build SOAP SetParameterValues XML
	headerId := soap.GenerateHeaderID()
	params := []soap.ParameterValueStruct{
		{Name: paramName, Value: value, Type: "xsd:string"},
	}
	soapXml := soap.BuildSetParameterValues(headerId, params, "")

	// 6. Update event_log with tracking data
	trackData, _ := json.Marshal(map[string]interface{}{
		"header_id":      headerId,
		"serial_number":  deviceInfo.SerialNumber,
		"operation_type": "SET_PARAMETER_VALUES",
		"operationParam": string(opParam),
		"event_log_id":   eventLogId,
		"issue_time":     now.Format(time.RFC3339),
	})
	s.repo.db.Table("event_log").Where("id = ?", eventLogId).
		Updates(map[string]interface{}{
			"command_track_data":   string(trackData),
			"command_issue_time":   now,
		})

	// 7. Cache track data in Redis for response processing
	trackKey := fmt.Sprintf("tr069:track:%s", headerId)
	trackJson, _ := json.Marshal(map[string]interface{}{
		"header_id":      headerId,
		"sn":             deviceInfo.SerialNumber,
		"operation_type": "SET_PARAMETER_VALUES",
		"event_log_id":   eventLogId,
	})
	redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

	// 8. Push SOAP XML to device queue
	queueKey := fmt.Sprintf("tr069:queue:%s", deviceInfo.SerialNumber)
	if err := redis.LPush(ctx, queueKey, soapXml); err != nil {
		logger.Errorf("failed to push SPV to device queue %s: %v", deviceInfo.SerialNumber, err)
		// Update event_log as failed
		s.repo.db.Table("event_log").Where("id = ?", eventLogId).
			Update("status", 4) // 4=fail
		return fmt.Errorf("push to device queue: %w", err)
	}
	redis.Expire(ctx, queueKey, 24*time.Hour)

	// 9. Create parameter_log for audit trail (record the intent to change)
	log := &ParameterLog{
		ParameterName: &paramName,
		OldValue:      &oldValue,
		NewValue:      &value,
		ChangeUser:    &username,
		ChangeTime:    &now,
		ElementId:     &elementId,
	}
	if err := s.repo.CreateParameterLog(log); err != nil {
		logger.Errorf("failed to create parameter_log: %v", err)
		// Non-fatal: the command was already dispatched
	}

	logger.Infof("SetParameter dispatched to device %s (neId=%d): %s=%s",
		deviceInfo.SerialNumber, elementId, paramName, value)
	return nil
}

// ---------------------------------------------------------------------------
// ParameterLog
// ---------------------------------------------------------------------------

// ListParameterLogs returns a paginated list of parameter change logs.
func (s *Service) ListParameterLogs(elementId int64, page, pageSize int) ([]ParameterLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindParameterLogs(elementId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// ParameterSet
// ---------------------------------------------------------------------------

// ListParameterSets returns all parameter sets for the given license.
func (s *Service) ListParameterSets(licenseId int) ([]ParameterSet, error) {
	return s.repo.FindParameterSets(licenseId)
}

// CreateParameterSet persists a new parameter set.
func (s *Service) CreateParameterSet(ps *ParameterSet) error {
	return s.repo.CreateParameterSet(ps)
}

// UpdateParameterSet persists changes to an existing parameter set.
func (s *Service) UpdateParameterSet(ps *ParameterSet) error {
	return s.repo.UpdateParameterSet(ps)
}

// DeleteParameterSet removes a parameter set by ID.
func (s *Service) DeleteParameterSet(id string) error {
	return s.repo.DeleteParameterSet(id)
}

// ---------------------------------------------------------------------------
// ParameterTemplate
// ---------------------------------------------------------------------------

// ListParameterTemplates returns all templates for the given tenancy.
func (s *Service) ListParameterTemplates(tenancyId int) ([]ParameterTemplate, error) {
	return s.repo.FindParameterTemplates(tenancyId)
}

// CreateParameterTemplate persists a new parameter template.
func (s *Service) CreateParameterTemplate(t *ParameterTemplate) error {
	return s.repo.CreateParameterTemplate(t)
}

// UpdateParameterTemplate persists changes to an existing parameter template.
func (s *Service) UpdateParameterTemplate(t *ParameterTemplate) error {
	return s.repo.UpdateParameterTemplate(t)
}

// ---------------------------------------------------------------------------
// DeployTemplate
// ---------------------------------------------------------------------------

// DeployTemplateStatus holds the per-device result of a template deployment.
type DeployTemplateStatus struct {
	ElementId    int64  `json:"elementId"`
	SerialNumber string `json:"serialNumber"`
	DeviceName   string `json:"deviceName"`
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	ParamCount   int    `json:"paramCount"`
}

// DeployTemplate deploys a parameter template to the specified target devices.
// It loads the template's parameter paths, reads the desired values from
// element_basic_info_parameter for each device, and sends SPV commands via TR-069.
func (s *Service) DeployTemplate(templateId int64, elementIds []int64, username string) ([]DeployTemplateStatus, error) {
	if len(elementIds) == 0 {
		return nil, fmt.Errorf("no target devices specified")
	}

	// 1. Load template's parameter paths via parameter_template_has_parameter JOIN parameter
	var paramPaths []string
	err := s.repo.db.Raw(`
		SELECT p.path FROM parameter_template_has_parameter pth
		JOIN parameter p ON p.id = pth.parameter_id
		WHERE pth.template_id = ? AND p.path IS NOT NULL AND p.path != ''
	`, templateId).Scan(&paramPaths).Error
	if err != nil {
		return nil, fmt.Errorf("load template parameters: %w", err)
	}
	if len(paramPaths) == 0 {
		return nil, fmt.Errorf("template %d has no parameters", templateId)
	}

	ctx := context.Background()
	now := time.Now()
	var results []DeployTemplateStatus

	for _, elementId := range elementIds {
		status := DeployTemplateStatus{ElementId: elementId}

		// 2. Resolve device SN and name
		var deviceInfo struct {
			SerialNumber string `gorm:"column:serial_number"`
			DeviceName   string `gorm:"column:device_name"`
		}
		if err := s.repo.db.Table("cpe_element").
			Select("serial_number, device_name").
			Where("ne_neid = ? AND deleted = ?", elementId, false).
			Scan(&deviceInfo).Error; err != nil {
			status.Message = fmt.Sprintf("device not found: %v", err)
			results = append(results, status)
			continue
		}
		if deviceInfo.SerialNumber == "" {
			status.Message = "device has no serial number"
			results = append(results, status)
			continue
		}
		status.SerialNumber = deviceInfo.SerialNumber
		status.DeviceName = deviceInfo.DeviceName

		// 3. Read desired parameter values from element_basic_info_parameter
		var paramValues []struct {
			ParamName  string `gorm:"column:param_name"`
			ParamValue string `gorm:"column:param_value"`
		}
		s.repo.db.Table("element_basic_info_parameter").
			Select("param_name, param_value").
			Where("element_id = ? AND param_name IN ?", elementId, paramPaths).
			Scan(&paramValues)

		if len(paramValues) == 0 {
			status.Message = "no parameter values found for device"
			results = append(results, status)
			continue
		}

		// 4. Build SPV entries
		entries := make([]setParamEntry, len(paramValues))
		spvParams := make([]soap.ParameterValueStruct, len(paramValues))
		for i, pv := range paramValues {
			entries[i] = setParamEntry{ParamName: pv.ParamName, ParamValue: pv.ParamValue}
			spvParams[i] = soap.ParameterValueStruct{Name: pv.ParamName, Value: pv.ParamValue, Type: "xsd:string"}
		}
		opParamJSON, _ := json.Marshal(entries)

		// 5. Create event_log (status=1 pending)
		eventLogId, err := s.repo.InsertEventLog("SetParameterValues", elementId, username, 1, "")
		if err != nil {
			status.Message = fmt.Sprintf("create event_log failed: %v", err)
			results = append(results, status)
			continue
		}

		// 6. Build SOAP XML
		headerId := soap.GenerateHeaderID()
		soapXml := soap.BuildSetParameterValues(headerId, spvParams, "")

		// 7. Update event_log with tracking data
		trackData, _ := json.Marshal(map[string]interface{}{
			"header_id":      headerId,
			"serial_number":  deviceInfo.SerialNumber,
			"operation_type": "SET_PARAMETER_VALUES",
			"operationParam": string(opParamJSON),
			"event_log_id":   eventLogId,
			"template_id":    templateId,
			"issue_time":     now.Format(time.RFC3339),
		})
		s.repo.db.Table("event_log").Where("id = ?", eventLogId).
			Updates(map[string]interface{}{
				"command_track_data": string(trackData),
				"command_issue_time": now,
			})

		// 8. Cache track data in Redis
		trackKey := fmt.Sprintf("tr069:track:%s", headerId)
		trackJson, _ := json.Marshal(map[string]interface{}{
			"header_id":      headerId,
			"sn":             deviceInfo.SerialNumber,
			"operation_type": "SET_PARAMETER_VALUES",
			"event_log_id":   eventLogId,
			"template_id":    templateId,
		})
		redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

		// 9. Push SOAP XML to device queue
		queueKey := fmt.Sprintf("tr069:queue:%s", deviceInfo.SerialNumber)
		if err := redis.LPush(ctx, queueKey, soapXml); err != nil {
			status.Message = fmt.Sprintf("push to device queue failed: %v", err)
			s.repo.db.Table("event_log").Where("id = ?", eventLogId).Update("status", 4)
			results = append(results, status)
			continue
		}
		redis.Expire(ctx, queueKey, 24*time.Hour)

		status.Success = true
		status.ParamCount = len(paramValues)
		status.Message = "SPV dispatched successfully"
		results = append(results, status)

		logger.Infof("DeployTemplate: dispatched %d params to device %s (elementId=%d) from template %d",
			len(paramValues), deviceInfo.SerialNumber, elementId, templateId)
	}

	return results, nil
}

// ---------------------------------------------------------------------------
// TriggerBackup (Task 5.8)
// ---------------------------------------------------------------------------

// TriggerBackup triggers a parameter backup for the given device by sending
// GPV for all basic parameter paths. When the GPV response comes back, the
// normal processGetParameterValuesResponse saves the values.
func (s *Service) TriggerBackup(elementId int64, username string) error {
	// 1. Resolve device SN and type
	var deviceInfo struct {
		SerialNumber string `gorm:"column:serial_number"`
		DeviceType   string `gorm:"column:device_type"`
	}
	if err := s.repo.db.Table("cpe_element").
		Select("serial_number, device_type").
		Where("ne_neid = ? AND deleted = ?", elementId, false).
		Scan(&deviceInfo).Error; err != nil {
		return fmt.Errorf("device not found: %w", err)
	}
	if deviceInfo.SerialNumber == "" {
		return fmt.Errorf("device %d has no serial number", elementId)
	}

	// 2. Get basic param paths for the device type
	paramPaths := getBasicParamPathsHelper(deviceInfo.DeviceType)
	if len(paramPaths) == 0 {
		return fmt.Errorf("no basic param paths for device type %s", deviceInfo.DeviceType)
	}

	// 3. Create ParameterBackupLog
	now := time.Now()
	taskId := fmt.Sprintf("backup_%d_%d", elementId, now.UnixMilli())
	backupLog := &ParameterBackupLog{
		TaskId:       &taskId,
		ElementId:    &elementId,
		GenerateTime: func() *int64 { t := now.UnixMilli(); return &t }(),
	}
	if err := s.repo.CreateParameterBackupLog(backupLog); err != nil {
		return fmt.Errorf("create backup log: %w", err)
	}

	// 4. Create event_log for GPV tracking
	eventLogId, err := s.repo.InsertEventLog("GetParameterValues", elementId, username, 1, "")
	if err != nil {
		return fmt.Errorf("create event_log: %w", err)
	}

	// 5. Build SOAP GPV XML
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetParameterValues(headerId, paramPaths)

	// 6. Update event_log with tracking data
	trackData, _ := json.Marshal(map[string]interface{}{
		"header_id":      headerId,
		"serial_number":  deviceInfo.SerialNumber,
		"operation_type": "GET_PARAMETER_VALUES",
		"event_log_id":   eventLogId,
		"backup_task_id": taskId,
		"is_backup":      true,
		"issue_time":     now.Format(time.RFC3339),
	})
	s.repo.db.Table("event_log").Where("id = ?", eventLogId).
		Updates(map[string]interface{}{
			"command_track_data": string(trackData),
			"command_issue_time": now,
		})

	// 7. Cache track data in Redis
	ctx := context.Background()
	trackKey := fmt.Sprintf("tr069:track:%s", headerId)
	trackJson, _ := json.Marshal(map[string]interface{}{
		"header_id":      headerId,
		"sn":             deviceInfo.SerialNumber,
		"operation_type": "GET_PARAMETER_VALUES",
		"event_log_id":   eventLogId,
		"backup_task_id": taskId,
		"is_backup":      true,
	})
	redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

	// 8. Push SOAP XML to device queue
	queueKey := fmt.Sprintf("tr069:queue:%s", deviceInfo.SerialNumber)
	if err := redis.LPush(ctx, queueKey, soapXml); err != nil {
		s.repo.db.Table("event_log").Where("id = ?", eventLogId).Update("status", 4)
		return fmt.Errorf("push to device queue: %w", err)
	}
	redis.Expire(ctx, queueKey, 24*time.Hour)

	logger.Infof("TriggerBackup: GPV dispatched to device %s (elementId=%d) for %d params, taskId=%s",
		deviceInfo.SerialNumber, elementId, len(paramPaths), taskId)
	return nil
}

// ---------------------------------------------------------------------------
// PresetParameters (Task 5.4)
// ---------------------------------------------------------------------------

// PresetParameters sends SPV for a set of preset parameters to a device.
// This is triggered automatically after device registration/onboarding.
func (s *Service) PresetParameters(elementId int64, presets map[string]string) error {
	if len(presets) == 0 {
		return nil
	}

	// 1. Resolve device SN
	var deviceInfo struct {
		SerialNumber string `gorm:"column:serial_number"`
	}
	if err := s.repo.db.Table("cpe_element").
		Select("serial_number").
		Where("ne_neid = ? AND deleted = ?", elementId, false).
		Scan(&deviceInfo).Error; err != nil {
		return fmt.Errorf("device not found: %w", err)
	}
	if deviceInfo.SerialNumber == "" {
		return fmt.Errorf("device %d has no serial number", elementId)
	}

	ctx := context.Background()
	now := time.Now()

	// 2. Build SPV entries
	entries := make([]setParamEntry, 0, len(presets))
	spvParams := make([]soap.ParameterValueStruct, 0, len(presets))
	for paramName, paramValue := range presets {
		entries = append(entries, setParamEntry{ParamName: paramName, ParamValue: paramValue})
		spvParams = append(spvParams, soap.ParameterValueStruct{Name: paramName, Value: paramValue, Type: "xsd:string"})
	}
	opParamJSON, _ := json.Marshal(entries)

	// 3. Create event_log
	eventLogId, err := s.repo.InsertEventLog("SetParameterValues", elementId, "system", 1, "")
	if err != nil {
		return fmt.Errorf("create event_log: %w", err)
	}

	// 4. Build SOAP XML
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildSetParameterValues(headerId, spvParams, "")

	// 5. Update event_log with tracking data
	trackData, _ := json.Marshal(map[string]interface{}{
		"header_id":      headerId,
		"serial_number":  deviceInfo.SerialNumber,
		"operation_type": "SET_PARAMETER_VALUES",
		"operationParam": string(opParamJSON),
		"event_log_id":   eventLogId,
		"is_preset":      true,
		"issue_time":     now.Format(time.RFC3339),
	})
	s.repo.db.Table("event_log").Where("id = ?", eventLogId).
		Updates(map[string]interface{}{
			"command_track_data": string(trackData),
			"command_issue_time": now,
		})

	// 6. Cache track data in Redis
	trackKey := fmt.Sprintf("tr069:track:%s", headerId)
	trackJson, _ := json.Marshal(map[string]interface{}{
		"header_id":      headerId,
		"sn":             deviceInfo.SerialNumber,
		"operation_type": "SET_PARAMETER_VALUES",
		"event_log_id":   eventLogId,
		"is_preset":      true,
	})
	redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

	// 7. Push SOAP XML to device queue
	queueKey := fmt.Sprintf("tr069:queue:%s", deviceInfo.SerialNumber)
	if err := redis.LPush(ctx, queueKey, soapXml); err != nil {
		s.repo.db.Table("event_log").Where("id = ?", eventLogId).Update("status", 4)
		return fmt.Errorf("push to device queue: %w", err)
	}
	redis.Expire(ctx, queueKey, 24*time.Hour)

	logger.Infof("PresetParameters: dispatched %d preset params to device %s (elementId=%d)",
		len(presets), deviceInfo.SerialNumber, elementId)
	return nil
}

// getBasicParamPathsHelper returns basic param paths for a device type.
// This is a local helper to avoid importing the tr069 package directly.
func getBasicParamPathsHelper(deviceType string) []string {
	// Common basic param paths for all device types
	igd := "InternetGatewayDevice"
	common := []string{
		igd + ".DeviceInfo.Manufacturer",
		igd + ".DeviceInfo.ModelName",
		igd + ".DeviceInfo.ProductClass",
		igd + ".DeviceInfo.SerialNumber",
		igd + ".DeviceInfo.HardwareVersion",
		igd + ".DeviceInfo.SoftwareVersion",
		igd + ".DeviceInfo.ProvisioningCode",
		igd + ".DeviceInfo.SpecVersion",
		igd + ".DeviceInfo.UpTime",
		igd + ".ManagementServer.ConnectionRequestURL",
		igd + ".ManagementServer.ConnectionRequestUsername",
		igd + ".ManagementServer.PeriodicInformInterval",
		igd + ".ManagementServer.ParameterKey",
		igd + ".WANDevice.1.WANConnectionDevice.1.WANIPConnection.1.ExternalIPAddress",
		igd + ".WANDevice.1.WANConnectionDevice.1.WANIPConnection.1.MACAddress",
		igd + ".WANDevice.1.WANConnectionDevice.1.WANIPConnection.1.DefaultGateway",
		igd + ".WANDevice.1.WANConnectionDevice.1.WANIPConnection.1.IPAddress",
		igd + ".WANDevice.1.WANConnectionDevice.1.WANIPConnection.1.SubnetMask",
		igd + ".WANDevice.1.WANConnectionDevice.1.WANIPConnection.1.Gateway",
		igd + ".WANDevice.1.WANConnectionDevice.1.WANIPConnection.1.DNSServers",
		igd + ".LANDevice.1.LANEthernetInterfaceConfig.1.MACAddress",
		igd + ".LANDevice.1.LANEthernetInterfaceConfig.1.Status",
	}
	return common
}

// ---------------------------------------------------------------------------
// ParameterBackupLog
// ---------------------------------------------------------------------------

// ListBackupLogs returns all backup logs for the given element.
func (s *Service) ListBackupLogs(elementId int64) ([]ParameterBackupLog, error) {
	return s.repo.FindParameterBackupLogs(elementId)
}

// ---------------------------------------------------------------------------
// Batch Parameter Configuration
// ---------------------------------------------------------------------------

// operationMessage is the JSON payload pushed to Redis operation_queue.
type operationMessage struct {
	EventType      string `json:"eventType"`
	NeNeid         int64  `json:"neNeid"`
	Operation      string `json:"operation"`
	OperationParam string `json:"operationParam"`
	OperationUser  string `json:"operationUser"`
	CommandTrackId int64  `json:"commandTrackId"`
	ExpiredAt      int64  `json:"expiredAt"`
}

// setParamEntry is a single parameter in the operationParam JSON array.
type setParamEntry struct {
	ParamName  string `json:"paramName"`
	ParamValue string `json:"paramValue"`
}

// BatchParameterConfigurationDirect creates a batch parameter configuration task
// and dispatches SetParameterValues commands for each device to Redis.
func (s *Service) BatchParameterConfigurationDirect(req *BatchParameterConfigRequest, username string, tenancyId int) error {
	if len(req.ParamValues) == 0 {
		return fmt.Errorf("paramValues must not be empty")
	}

	// 1. Resolve target device IDs from elementIds and groupIds.
	deviceIds, err := s.resolveDeviceIds(req.ElementIds, req.GroupIds)
	if err != nil {
		return fmt.Errorf("resolve devices: %w", err)
	}
	if len(deviceIds) == 0 {
		return fmt.Errorf("no target devices resolved")
	}

	// 2. Build operationParam JSON.
	entries := make([]setParamEntry, len(req.ParamValues))
	for i, pv := range req.ParamValues {
		entries[i] = setParamEntry{ParamName: pv.ParamKey, ParamValue: pv.ParamValue}
	}
	opParamJSON, _ := json.Marshal(entries)

	// 3. Create batch_configuration_log.
	now := time.Now()
	deviceCount := len(deviceIds)
	taskName := fmt.Sprintf("BatchParameterConfig-%d", now.UnixMilli())
	task := &misc.BatchConfigurationLog{
		Name:          &taskName,
		OperationTime: &now,
		TenancyId:     &tenancyId,
		User:          &username,
		DeviceCount:   &deviceCount,
	}
	if err := s.repo.CreateBatchConfigLog(task); err != nil {
		return fmt.Errorf("create batch config log: %w", err)
	}

	// 4. For each device: blacklist check → EventLog → Redis → DeviceLog.
	expiredAt := now.Add(5 * time.Minute).UnixMilli()
	ctx := context.Background()
	queueName := "operation_queue"

	for _, elementId := range deviceIds {
		// Blacklist check via raw SQL (avoid importing device package).
		var blCount int64
		s.repo.DB().Raw(`
			SELECT COUNT(*) FROM element_black_list
			WHERE serial_number = (SELECT serial_number FROM cpe_element WHERE ne_neid = ?)
		`, elementId).Count(&blCount)
		if blCount > 0 {
			logger.Warnf("device %d is blacklisted, skipping", elementId)
			continue
		}

		// Create EventLog (status=1 = pending).
		eventLogId, err := s.repo.InsertEventLog("SetParameterValues", elementId, username, 1, string(opParamJSON))
		if err != nil {
			logger.Errorf("create event_log for device %d: %v", elementId, err)
			continue
		}

		// Push operation message to Redis.
		msg := operationMessage{
			EventType:      "SetParameterValues",
			NeNeid:         elementId,
			Operation:      "SetParameterValues",
			OperationParam: string(opParamJSON),
			OperationUser:  username,
			CommandTrackId: eventLogId,
			ExpiredAt:      expiredAt,
		}
		msgJSON, _ := json.Marshal(msg)
		if err := redis.LPush(ctx, queueName, string(msgJSON)); err != nil {
			logger.Errorf("push to redis queue for device %d: %v", elementId, err)
		}

		// Create batch_configuration_device_log.
		dataStr := string(opParamJSON)
		deviceLog := &misc.BatchConfigurationDeviceLog{
			TaskId:     &task.Id,
			ElementId:  &elementId,
			Data:       &dataStr,
			EventLogId: &eventLogId,
		}
		if err := s.repo.CreateBatchConfigDeviceLog(deviceLog); err != nil {
			logger.Errorf("create device log for device %d: %v", elementId, err)
		}
	}

	return nil
}

// ListBatchConfigurations returns the paginated task list with progress info.
func (s *Service) ListBatchConfigurations(tenancyId int, page, pageSize int) ([]BatchConfigTaskVo, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	logs, total, err := s.repo.FindBatchConfigLogs(tenancyId, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []BatchConfigTaskVo
	for _, l := range logs {
		vo := BatchConfigTaskVo{
			Id:            l.Id,
			Name:          ptrStr(l.Name),
			OperationUser: ptrStr(l.User),
			OperationTime: ptrTime(l.OperationTime),
			DeviceCount:   ptrInt(l.DeviceCount),
		}
		totalCnt, successCnt, pErr := s.repo.BatchConfigProgress(l.Id)
		if pErr == nil {
			vo.Progress = fmt.Sprintf("%d/%d", successCnt, totalCnt)
		}
		vos = append(vos, vo)
	}
	return vos, total, nil
}

// ListBatchConfigurationDetail returns per-device results for a given task.
func (s *Service) ListBatchConfigurationDetail(taskId int64) ([]BatchConfigTaskDetailVo, error) {
	return s.repo.BatchConfigDetail(taskId)
}

// ---------- helpers ----------

// resolveDeviceIds merges explicit element IDs with IDs resolved from group IDs.
func (s *Service) resolveDeviceIds(elementIds []int64, groupIds []string) ([]int64, error) {
	seen := make(map[int64]struct{})
	var result []int64

	for _, id := range elementIds {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}

	if len(groupIds) > 0 {
		var fromGroups []int64
		if err := s.repo.DB().Raw(`
			SELECT ne_neid FROM cpe_element
			WHERE device_group_id IN (?)
		`, groupIds).Scan(&fromGroups).Error; err != nil {
			return nil, err
		}
		for _, id := range fromGroups {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				result = append(result, id)
			}
		}
	}

	return result, nil
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ptrTime(p *time.Time) string {
	if p == nil {
		return ""
	}
	return p.Format(time.RFC3339)
}

func ptrInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
