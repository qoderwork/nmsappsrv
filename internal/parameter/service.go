package parameter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// Service defines the business-logic contract for parameter management.
type Service interface {
	GetParameters(elementId int64) (*DeviceParameterDetailVo, error)
	SetParameter(elementId int64, paramName string, value string, username string) error
	BatchSetParameter(elementId int64, records []SetParameterRecord, username string) error
	ListParameterLogs(elementId int64, keyword string, page, pageSize int) ([]ParameterLog, int64, error)
	PresetParameters(elementId int64, presets map[string]string) error
	ListParameterSets(tenantId int) ([]ParameterSet, error)
	CreateParameterSet(ps *ParameterSet) error
	UpdateParameterSet(ps *ParameterSet) error
	DeleteParameterSet(id string) error
	ListParameterTemplates(tenantId int) ([]ParameterTemplate, error)
	CreateParameterTemplate(req *ParameterTemplateRequest) error
	UpdateParameterTemplate(req *ParameterTemplateRequest) error
	GetParameterTemplate(id int64) (*ParameterTemplateDetailVo, error)
	DeleteParameterTemplate(id int64) error
	DeployTemplate(templateId int64, elementIds []int64, username string) ([]DeployTemplateStatus, error)
	ListDeployTemplateLogs(templateId int64, page, pageSize int) ([]DeployTemplateLogVo, int64, error)
	TriggerBackup(elementId int64, username string) error
	ListBackupLogs(elementId int64) ([]ParameterBackupLog, error)
	ListBackupLogsWithPage(req *ListParameterBackupLogsRequest) ([]ParameterBackupLogVo, int64, error)
	BatchParameterConfigurationDirect(req *BatchParameterConfigRequest, username string, tenantId int) error
	BatchParameterConfiguration(excelBytes []byte, username string, tenantId int) error
	ListBatchConfigurations(tenantId int, page, pageSize int) ([]BatchConfigTaskVo, int64, error)
	ListBatchConfigurationDetail(taskId int64) ([]BatchConfigTaskDetailVo, error)

	// TR-069 Parameter Definition CRUD.
	AddTR069Parameter(param *TR069Parameter) error
	ListTR069Parameters(page, pageSize int) ([]TR069Parameter, int64, error)
	ViewTR069Parameter(id int64) (*TR069Parameter, error)
	UpdateTR069Parameter(param *TR069Parameter) error
	DeleteTR069Parameter(id int64) error

	// Model Tree operations.
	GetModelTree(elementId int64) (*ModelTreeNode, error)
	RefreshParameter(elementId int64, paramPath string, username string) error
	ReloadParameter(elementId int64, paramPaths []string, username string) error
	AddObject(elementId int64, objectName string, username string) error
	DeleteObject(elementId int64, objectName string, username string) error
	BatchDeleteObject(elementId int64, objectNames []string, username string) error
	DeleteObjectAfterNeedReboot(elementId int64, objectNames []string, username string) error

	// Export Parameter Template.
	ExportParameterTemplate(templateId int64) ([]byte, string, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// ---------------------------------------------------------------------------
// Shared types
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

// ---------------------------------------------------------------------------
// ParameterAttributes
// ---------------------------------------------------------------------------

// GetParameters returns enriched parameter data for the given element, including
// device metadata and full parameter definitions with current values.
func (s *service) GetParameters(elementId int64) (*DeviceParameterDetailVo, error) {
	// 1. Get device metadata
	var deviceInfo struct {
		SerialNumber string `gorm:"column:serial_number"`
		DeviceName   string `gorm:"column:device_name"`
		DeviceType   string `gorm:"column:device_type"`
	}
	if err := s.repo.DB().Table("cpe_element").
		Select("serial_number, device_name, device_type").
		Where("ne_neid = ? AND deleted = ?", elementId, false).
		Scan(&deviceInfo).Error; err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// 2. Check online status from Redis
	online := false
	if deviceInfo.SerialNumber != "" {
		ctx := context.Background()
		val, err := redis.Get(ctx, fmt.Sprintf("device:online:%s", deviceInfo.SerialNumber))
		if err == nil && val == "true" {
			online = true
		}
	}

	// 3. Get enriched parameters
	params, err := s.repo.FindParameterVosByElementId(elementId)
	if err != nil {
		return nil, fmt.Errorf("get parameters: %w", err)
	}

	return &DeviceParameterDetailVo{
		ElementId:    elementId,
		SerialNumber: deviceInfo.SerialNumber,
		DeviceName:   deviceInfo.DeviceName,
		DeviceType:   deviceInfo.DeviceType,
		Online:       online,
		Parameters:   params,
	}, nil
}

// SetParameter updates a parameter value for the given element and dispatches
// the change to the device via TR-069 SetParameterValues.
func (s *service) SetParameter(elementId int64, paramName string, value string, username string) error {
	ctx := context.Background()

	// 1. Look up device serial number
	var deviceInfo struct {
		SerialNumber string `gorm:"column:serial_number"`
		DeviceName   string `gorm:"column:device_name"`
	}
	if err := s.repo.DB().Table("cpe_element").
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
	s.repo.DB().Table("element_basic_info_parameter").
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
	s.repo.DB().Table("event_log").Where("id = ?", eventLogId).
		Updates(map[string]interface{}{
			"command_track_data": string(trackData),
			"command_issue_time": now,
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
		s.repo.DB().Table("event_log").Where("id = ?", eventLogId).
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

// BatchSetParameter sets multiple parameters atomically on a single device.
// Aligns with Java batch SPV: sends a single SetParameterValues RPC with all parameters.
func (s *service) BatchSetParameter(elementId int64, records []SetParameterRecord, username string) error {
	ctx := context.Background()

	// 1. Look up device serial number
	var deviceInfo struct {
		SerialNumber string `gorm:"column:serial_number"`
		DeviceName   string `gorm:"column:device_name"`
	}
	if err := s.repo.DB().Table("cpe_element").
		Select("serial_number, device_name").
		Where("ne_neid = ? AND deleted = ?", elementId, false).
		Scan(&deviceInfo).Error; err != nil {
		return fmt.Errorf("device not found: %w", err)
	}
	if deviceInfo.SerialNumber == "" {
		return fmt.Errorf("device %d has no serial number", elementId)
	}

	// 2. Build parameter list for SPV
	paramList := make([]soap.ParameterValueStruct, 0, len(records))
	entries := make([]setParamEntry, 0, len(records))
	for _, r := range records {
		paramList = append(paramList, soap.ParameterValueStruct{
			Name:  r.ParamName,
			Value: r.Value,
			Type:  "xsd:string",
		})
		entries = append(entries, setParamEntry{ParamName: r.ParamName, ParamValue: r.Value})
	}

	// 3. Create event_log tracking entry (status=1 pending)
	now := time.Now()
	eventLogId, err := s.repo.InsertEventLog("SetParameterValues", elementId, username, 1, "")
	if err != nil {
		return fmt.Errorf("create event_log: %w", err)
	}

	// 4. Build SOAP SetParameterValues XML
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildSetParameterValues(headerId, paramList, "")

	// 5. Update event_log with tracking data
	opParamJSON, _ := json.Marshal(entries)
	trackData, _ := json.Marshal(map[string]interface{}{
		"header_id":      headerId,
		"serial_number":  deviceInfo.SerialNumber,
		"operation_type": "SET_PARAMETER_VALUES",
		"operationParam": string(opParamJSON),
		"event_log_id":   eventLogId,
		"batch":          true,
		"param_count":    len(records),
		"issue_time":     now.Format(time.RFC3339),
	})
	s.repo.DB().Table("event_log").Where("id = ?", eventLogId).
		Updates(map[string]interface{}{
			"command_track_data": string(trackData),
			"command_issue_time": now,
		})

	// 6. Cache track data in Redis for response processing
	trackKey := fmt.Sprintf("tr069:track:%s", headerId)
	trackJson, _ := json.Marshal(map[string]interface{}{
		"header_id":      headerId,
		"sn":             deviceInfo.SerialNumber,
		"operation_type": "SET_PARAMETER_VALUES",
		"event_log_id":   eventLogId,
	})
	redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

	// 7. Push SOAP XML to device queue
	queueKey := fmt.Sprintf("tr069:queue:%s", deviceInfo.SerialNumber)
	if err := redis.LPush(ctx, queueKey, soapXml); err != nil {
		logger.Errorf("failed to push batch SPV to device queue %s: %v", deviceInfo.SerialNumber, err)
		s.repo.DB().Table("event_log").Where("id = ?", eventLogId).Update("status", 4) // 4=fail
		return fmt.Errorf("push to device queue: %w", err)
	}
	redis.Expire(ctx, queueKey, 24*time.Hour)

	// 8. Create parameter_log for each parameter (audit trail)
	for _, r := range records {
		paramName := r.ParamName
		log := &ParameterLog{
			ParameterName: &paramName,
			NewValue:      &r.Value,
			ChangeUser:    &username,
			ChangeTime:    &now,
			ElementId:     &elementId,
		}
		if err := s.repo.CreateParameterLogWithID(log); err != nil {
			logger.Errorf("failed to create parameter_log for %s: %v", paramName, err)
			// Non-fatal: the command was already dispatched
		}
	}

	logger.Infof("batch SPV: sent %d params to device %s (elementId=%d)", len(records), deviceInfo.SerialNumber, elementId)
	return nil
}

// ---------------------------------------------------------------------------
// ParameterLog
// ---------------------------------------------------------------------------

// ListParameterLogs returns a paginated list of parameter change logs.
// When elementId is 0, logs for all devices under the tenant are returned.
// keyword matches device name or serial number.
func (s *service) ListParameterLogs(elementId int64, keyword string, page, pageSize int) ([]ParameterLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindParameterLogs(elementId, keyword, offset, pageSize)
}

// ---------------------------------------------------------------------------
// PresetParameters
// ---------------------------------------------------------------------------

// PresetParameters sends SPV for a set of preset parameters to a device.
// This is triggered automatically after device registration/onboarding.
func (s *service) PresetParameters(elementId int64, presets map[string]string) error {
	if len(presets) == 0 {
		return nil
	}

	// 1. Resolve device SN
	var deviceInfo struct {
		SerialNumber string `gorm:"column:serial_number"`
	}
	if err := s.repo.DB().Table("cpe_element").
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
	s.repo.DB().Table("event_log").Where("id = ?", eventLogId).
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
		s.repo.DB().Table("event_log").Where("id = ?", eventLogId).Update("status", 4)
		return fmt.Errorf("push to device queue: %w", err)
	}
	redis.Expire(ctx, queueKey, 24*time.Hour)

	logger.Infof("PresetParameters: dispatched %d preset params to device %s (elementId=%d)",
		len(presets), deviceInfo.SerialNumber, elementId)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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

// newService creates a Service backed by the given mock Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}
