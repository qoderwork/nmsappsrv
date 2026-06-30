package restapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nmsappsrv/internal/device"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/snmp"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"github.com/gin-gonic/gin"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ============================
// Device operations
// ============================

func (s *Service) ListDevices(c *gin.Context, offset, limit int) ([]RestDeviceVo, int64, error) {
	licenseId := middleware.GetLicenseId(c)

	devices, total, err := s.repo.ListDevices(licenseId, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	var result []RestDeviceVo
	for _, d := range devices {
		vo := RestDeviceVo{
			Id:              d.NeNeid,
			SerialNumber:    derefStr(d.SerialNumber),
			DeviceName:      derefStr(d.DeviceName),
			DeviceType:      derefStr(d.DeviceType),
			Product:         derefStr(d.Product),
			SoftwareVersion: derefStr(d.SoftwareVersion),
			Manufacturer:    derefStr(d.Manufacturer),
			LicenseId:       derefIntPtr(d.LicenseId),
		}
		// Determine status from device state
		if d.Deleted {
			vo.Status = "deleted"
		} else {
			vo.Status = "online"
		}
		result = append(result, vo)
	}

	return result, total, nil
}

func (s *Service) GetDevice(c *gin.Context, id int64) (*RestDeviceVo, error) {
	licenseId := middleware.GetLicenseId(c)

	d, err := s.repo.GetDeviceById(id, licenseId)
	if err != nil {
		return nil, fmt.Errorf("device not found")
	}

	vo := &RestDeviceVo{
		Id:              d.NeNeid,
		SerialNumber:    derefStr(d.SerialNumber),
		DeviceName:      derefStr(d.DeviceName),
		DeviceType:      derefStr(d.DeviceType),
		Product:         derefStr(d.Product),
		SoftwareVersion: derefStr(d.SoftwareVersion),
		Manufacturer:    derefStr(d.Manufacturer),
		LicenseId:       derefIntPtr(d.LicenseId),
		Status:          "online",
	}

	return vo, nil
}

func (s *Service) AddDevice(c *gin.Context, req *AddRestDeviceRequest) (*RestDeviceVo, error) {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)

	// Check for duplicate serial number
	existing, _ := s.repo.GetDeviceBySN(req.SerialNumber, licenseId)
	if existing != nil {
		return nil, fmt.Errorf("device with serial number %s already exists", req.SerialNumber)
	}

	// Check device limit (default max 10000)
	count, err := s.repo.CountDevices(licenseId)
	if err != nil {
		return nil, fmt.Errorf("failed to check device count")
	}
	if count >= 10000 {
		return nil, fmt.Errorf("device limit reached (max 10000)")
	}

	sn := req.SerialNumber
	d := &device.CpeElement{
		SerialNumber: &sn,
		LicenseId:    &licenseId,
	}
	if req.DeviceName != "" {
		d.DeviceName = &req.DeviceName
	}
	if req.DeviceType != "" {
		d.DeviceType = &req.DeviceType
	}
	if req.Product != "" {
		d.Product = &req.Product
	}

	if err := s.repo.CreateDevice(d); err != nil {
		logger.Errorf("Failed to create device: %v", err)
		return nil, fmt.Errorf("failed to create device")
	}

	logger.Infof("Device added: SN=%s by user %s", req.SerialNumber, username)

	vo := &RestDeviceVo{
		Id:           d.NeNeid,
		SerialNumber: req.SerialNumber,
		DeviceName:   req.DeviceName,
		DeviceType:   req.DeviceType,
		Product:      req.Product,
		LicenseId:    licenseId,
		Status:       "online",
	}
	return vo, nil
}

func (s *Service) ModifyDeviceById(c *gin.Context, id int64, req *ModifyRestDeviceRequest) error {
	licenseId := middleware.GetLicenseId(c)

	d, err := s.repo.GetDeviceById(id, licenseId)
	if err != nil {
		return fmt.Errorf("device not found")
	}

	if req.DeviceName != nil {
		d.DeviceName = req.DeviceName
	}
	if req.DeviceType != nil {
		d.DeviceType = req.DeviceType
	}
	if req.Product != nil {
		d.Product = req.Product
	}

	if err := s.repo.UpdateDevice(d); err != nil {
		logger.Errorf("Failed to modify device %d: %v", id, err)
		return fmt.Errorf("failed to modify device")
	}

	return nil
}

func (s *Service) ModifyDeviceBySN(c *gin.Context, req *ModifyRestDeviceBySNRequest) error {
	licenseId := middleware.GetLicenseId(c)

	d, err := s.repo.GetDeviceBySN(req.SerialNumber, licenseId)
	if err != nil {
		return fmt.Errorf("device with serial number %s not found", req.SerialNumber)
	}

	if req.DeviceName != nil {
		d.DeviceName = req.DeviceName
	}
	if req.DeviceType != nil {
		d.DeviceType = req.DeviceType
	}

	if err := s.repo.UpdateDevice(d); err != nil {
		logger.Errorf("Failed to modify device by SN %s: %v", req.SerialNumber, err)
		return fmt.Errorf("failed to modify device")
	}

	return nil
}

func (s *Service) DeleteDevice(c *gin.Context, id int64) error {
	licenseId := middleware.GetLicenseId(c)

	if err := s.repo.SoftDeleteDevice(id, licenseId); err != nil {
		logger.Errorf("Failed to delete device %d: %v", id, err)
		return fmt.Errorf("failed to delete device")
	}

	return nil
}

// ============================
// Parameter operations
// ============================

func (s *Service) GetDeviceParams(c *gin.Context, elementId int64) ([]RestParameterVo, error) {
	licenseId := middleware.GetLicenseId(c)

	// Verify device exists and belongs to this license
	_, err := s.repo.GetDeviceById(elementId, licenseId)
	if err != nil {
		return nil, fmt.Errorf("device not found")
	}

	params, err := s.repo.GetDeviceParams(elementId)
	if err != nil {
		return nil, fmt.Errorf("failed to get device parameters")
	}

	return params, nil
}

func (s *Service) SetDeviceParams(c *gin.Context, elementId int64, req *SetRestParameterRequest) error {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)

	// Verify device exists
	_, err := s.repo.GetDeviceById(elementId, licenseId)
	if err != nil {
		return fmt.Errorf("device not found")
	}

	if err := s.repo.SetDeviceParams(elementId, req.Parameters); err != nil {
		logger.Errorf("Failed to set device params for element %d: %v", elementId, err)
		return fmt.Errorf("failed to set device parameters")
	}

	logger.Infof("Device params set for element %d by user %s", elementId, username)
	return nil
}

func (s *Service) PresetDeviceParams(c *gin.Context, elementId int64, req *PresetParameterRequest) (*RequestStatusVo, error) {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)

	// Verify device exists
	_, err := s.repo.GetDeviceById(elementId, licenseId)
	if err != nil {
		return nil, fmt.Errorf("device not found")
	}

	// Create preset parameters task record
	task := &misc.PresetParametersTask{
		ElementId: &elementId,
	}
	_ = task // Task record creation would use its own repository in production

	requestId, err := s.repo.PresetDeviceParams(elementId, req.Parameters)
	if err != nil {
		logger.Errorf("Failed to preset device params for element %d: %v", elementId, err)
		return nil, fmt.Errorf("failed to preset device parameters")
	}

	logger.Infof("Preset params queued for element %d, requestId=%s, by user %s", elementId, requestId, username)

	return &RequestStatusVo{
		RequestId: requestId,
		Status:    "pending",
	}, nil
}

// ============================
// Alarm operations
// ============================

func (s *Service) ListAlarms(c *gin.Context, offset, limit int) ([]RestAlarmVo, int64, error) {
	licenseId := middleware.GetLicenseId(c)

	alarms, total, err := s.repo.ListAlarms(licenseId, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	var result []RestAlarmVo
	for _, a := range alarms {
		vo := RestAlarmVo{
			Id:              a.Id,
			Severity:        derefStr(a.Severity),
			AlarmIdentifier: derefStr(a.AlarmIdentifier),
			ProbableCause:   derefStr(a.ProbableCause),
			AlarmStatus:     derefIntPtr(a.AlarmStatus),
			EventType:       derefStr(a.EventType),
			ElementId:       derefInt64Ptr(a.ElementId),
			EventTime:       formatTime(a.EventTime),
		}
		result = append(result, vo)
	}

	return result, total, nil
}

func (s *Service) SyncAlarms(c *gin.Context, req *SyncAlarmRequest) ([]RestAlarmVo, error) {
	licenseId := middleware.GetLicenseId(c)

	alarms, err := s.repo.SyncAlarms(req.ElementIds, licenseId)
	if err != nil {
		return nil, fmt.Errorf("failed to sync alarms")
	}

	var result []RestAlarmVo
	for _, a := range alarms {
		vo := RestAlarmVo{
			Id:              a.Id,
			Severity:        derefStr(a.Severity),
			AlarmIdentifier: derefStr(a.AlarmIdentifier),
			ProbableCause:   derefStr(a.ProbableCause),
			AlarmStatus:     derefIntPtr(a.AlarmStatus),
			EventType:       derefStr(a.EventType),
			ElementId:       derefInt64Ptr(a.ElementId),
			EventTime:       formatTime(a.EventTime),
		}
		result = append(result, vo)
	}

	return result, nil
}

func (s *Service) ClearAlarms(c *gin.Context, req *ClearAlarmRequest) error {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)

	if err := s.repo.ClearAlarms(req.AlarmIds, licenseId); err != nil {
		logger.Errorf("Failed to clear alarms: %v", err)
		return fmt.Errorf("failed to clear alarms")
	}

	logger.Infof("Cleared %d alarms by user %s", len(req.AlarmIds), username)
	return nil
}

// ============================
// Upgrade file operations
// ============================

func (s *Service) UploadUpgradeFile(c *gin.Context, fileName string, filePath string, fileSize int64) (*RestUpgradeFileVo, error) {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)
	now := time.Now()

	record := map[string]interface{}{
		"file_name":   fileName,
		"file_path":   filePath,
		"file_size":   fileSize,
		"license_id":  licenseId,
		"user":        username,
		"upload_time": now,
		"create_time": now,
	}

	id, err := s.repo.CreateUpgradeFile(record)
	if err != nil {
		logger.Errorf("Failed to create upgrade file record: %v", err)
		return nil, fmt.Errorf("failed to upload upgrade file")
	}

	logger.Infof("Upgrade file uploaded: %s by user %s", fileName, username)

	return &RestUpgradeFileVo{
		Id:         int(id),
		FileName:   fileName,
		FileSize:   fileSize,
		UploadTime: now.Format("2006-01-02 15:04:05"),
	}, nil
}

func (s *Service) ListUpgradeFiles(c *gin.Context, offset, limit int) ([]RestUpgradeFileVo, int64, error) {
	licenseId := middleware.GetLicenseId(c)

	files, total, err := s.repo.ListUpgradeFiles(licenseId, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	var result []RestUpgradeFileVo
	for _, f := range files {
		vo := RestUpgradeFileVo{}
		if v, ok := f["id"].(int64); ok {
			vo.Id = int(v)
		}
		if v, ok := f["file_name"].(string); ok {
			vo.FileName = v
		}
		if v, ok := f["version"].(string); ok {
			vo.Version = v
		}
		if v, ok := f["device_type"].(string); ok {
			vo.DeviceType = v
		}
		if v, ok := f["file_size"].(int64); ok {
			vo.FileSize = v
		}
		if v, ok := f["upload_time"].(time.Time); ok {
			vo.UploadTime = v.Format("2006-01-02 15:04:05")
		}
		result = append(result, vo)
	}

	return result, total, nil
}

func (s *Service) DeleteUpgradeFile(c *gin.Context, id int) error {
	licenseId := middleware.GetLicenseId(c)

	if err := s.repo.DeleteUpgradeFile(id, licenseId); err != nil {
		logger.Errorf("Failed to delete upgrade file %d: %v", id, err)
		return fmt.Errorf("failed to delete upgrade file")
	}

	return nil
}

// ============================
// Upgrade task operations
// ============================

func (s *Service) CreateUpgradeTask(c *gin.Context, req *RestUpgradeTaskRequest) (*RestUpgradeTaskVo, error) {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)
	now := time.Now()

	elementIdJSON, _ := json.Marshal(req.ElementIds)

	record := map[string]interface{}{
		"name":            req.Name,
		"upgrade_file_id": req.UpgradeFileId,
		"element_ids":     string(elementIdJSON),
		"status":          1, // pending
		"license_id":      licenseId,
		"user":            username,
		"create_time":     now,
		"update_time":     now,
	}

	id, err := s.repo.CreateUpgradeTask(record)
	if err != nil {
		logger.Errorf("Failed to create upgrade task: %v", err)
		return nil, fmt.Errorf("failed to create upgrade task")
	}

	logger.Infof("Upgrade task %d created by user %s for %d devices", id, username, len(req.ElementIds))

	return &RestUpgradeTaskVo{
		Id:       int(id),
		Name:     req.Name,
		Status:   1,
		Progress: "0/" + fmt.Sprintf("%d", len(req.ElementIds)),
	}, nil
}

func (s *Service) GetUpgradeTask(c *gin.Context, id int) (*RestUpgradeTaskVo, error) {
	task, err := s.repo.GetUpgradeTask(id)
	if err != nil {
		return nil, fmt.Errorf("upgrade task not found")
	}

	vo := &RestUpgradeTaskVo{}
	if v, ok := task["id"].(int64); ok {
		vo.Id = int(v)
	}
	if v, ok := task["name"].(string); ok {
		vo.Name = v
	}
	if v, ok := task["status"].(int64); ok {
		vo.Status = int(v)
	}

	return vo, nil
}

func (s *Service) ListUpgradeTasks(c *gin.Context, offset, limit int) ([]RestUpgradeTaskVo, int64, error) {
	licenseId := middleware.GetLicenseId(c)

	tasks, total, err := s.repo.ListUpgradeTasks(licenseId, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	var result []RestUpgradeTaskVo
	for _, t := range tasks {
		vo := RestUpgradeTaskVo{}
		if v, ok := t["id"].(int64); ok {
			vo.Id = int(v)
		}
		if v, ok := t["name"].(string); ok {
			vo.Name = v
		}
		if v, ok := t["status"].(int64); ok {
			vo.Status = int(v)
		}
		result = append(result, vo)
	}

	return result, total, nil
}

// ============================
// Request status operations
// ============================

func (s *Service) GetRequestStatus(requestId string) (*RequestStatusVo, error) {
	status, err := s.repo.GetRequestStatus(requestId)
	if err != nil {
		return nil, fmt.Errorf("request not found")
	}
	return status, nil
}

// ============================
// TBG (femtocell) operations
// ============================

func (s *Service) ListTBGs(c *gin.Context, offset, limit int) ([]TBGVo, int64, error) {
	licenseId := middleware.GetLicenseId(c)

	tbgs, total, err := s.repo.ListTBGs(licenseId, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	var result []TBGVo
	for _, t := range tbgs {
		vo := TBGVo{
			Id:            t.Id,
			SerialNumber:  derefStr(t.SerialNumber),
			Band:          derefStr(t.Band),
			Address:       derefStr(t.Address),
			WanMacAddress: derefStr(t.WanMacAddress),
		}
		result = append(result, vo)
	}

	return result, total, nil
}

func (s *Service) GetTBGBySN(sn string) (*TBGVo, error) {
	tbg, err := s.repo.GetTBGBySN(sn)
	if err != nil {
		return nil, fmt.Errorf("TBG device not found")
	}

	return &TBGVo{
		Id:            tbg.Id,
		SerialNumber:  derefStr(tbg.SerialNumber),
		Band:          derefStr(tbg.Band),
		Address:       derefStr(tbg.Address),
		WanMacAddress: derefStr(tbg.WanMacAddress),
	}, nil
}

func (s *Service) GetTBGByWanMac(mac string) (*TBGVo, error) {
	tbg, err := s.repo.GetTBGByWanMac(mac)
	if err != nil {
		return nil, fmt.Errorf("TBG device not found")
	}

	return &TBGVo{
		Id:            tbg.Id,
		SerialNumber:  derefStr(tbg.SerialNumber),
		Band:          derefStr(tbg.Band),
		Address:       derefStr(tbg.Address),
		WanMacAddress: derefStr(tbg.WanMacAddress),
	}, nil
}

func (s *Service) AddTBGs(c *gin.Context, reqs []AddTBGRequest) ([]TBGVo, error) {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)

	if len(reqs) > 100 {
		return nil, fmt.Errorf("batch size exceeds maximum of 100")
	}

	var tbgs []TBGDevice
	for _, req := range reqs {
		// Check for duplicate SN
		existing, _ := s.repo.GetTBGBySN(req.SerialNumber)
		if existing != nil {
			return nil, fmt.Errorf("TBG with serial number %s already exists", req.SerialNumber)
		}

		sn := req.SerialNumber
		tbg := TBGDevice{
			SerialNumber: &sn,
			LicenseId:    &licenseId,
		}
		if req.Band != "" {
			tbg.Band = &req.Band
		}
		if req.Address != "" {
			tbg.Address = &req.Address
		}
		if req.WanMacAddress != "" {
			tbg.WanMacAddress = &req.WanMacAddress
		}
		tbgs = append(tbgs, tbg)
	}

	if err := s.repo.CreateTBGs(tbgs); err != nil {
		logger.Errorf("Failed to create TBG devices: %v", err)
		return nil, fmt.Errorf("failed to create TBG devices")
	}

	logger.Infof("Created %d TBG devices by user %s", len(tbgs), username)

	var result []TBGVo
	for _, t := range tbgs {
		vo := TBGVo{
			Id:            t.Id,
			SerialNumber:  derefStr(t.SerialNumber),
			Band:          derefStr(t.Band),
			Address:       derefStr(t.Address),
			WanMacAddress: derefStr(t.WanMacAddress),
		}
		result = append(result, vo)
	}

	return result, nil
}

func (s *Service) ModifyTBGs(c *gin.Context, reqs []ModifyTBGRequest) error {
	username := middleware.GetUsername(c)

	for _, req := range reqs {
		tbg, err := s.repo.GetTBGBySN(req.SerialNumber)
		if err != nil {
			return fmt.Errorf("TBG with serial number %s not found", req.SerialNumber)
		}

		if req.Band != nil {
			tbg.Band = req.Band
		}
		if req.Address != nil {
			tbg.Address = req.Address
		}
		if req.WanMacAddress != nil {
			tbg.WanMacAddress = req.WanMacAddress
		}

		if err := s.repo.UpdateTBG(tbg); err != nil {
			logger.Errorf("Failed to update TBG %s: %v", req.SerialNumber, err)
			return fmt.Errorf("failed to update TBG device %s", req.SerialNumber)
		}
	}

	logger.Infof("Modified %d TBG devices by user %s", len(reqs), username)
	return nil
}

func (s *Service) DeleteTBGs(c *gin.Context, req *DeleteTBGRequest) error {
	username := middleware.GetUsername(c)

	if len(req.SerialNumbers) > 100 {
		return fmt.Errorf("batch size exceeds maximum of 100")
	}

	if err := s.repo.DeleteTBGsBySNs(req.SerialNumbers); err != nil {
		logger.Errorf("Failed to delete TBG devices: %v", err)
		return fmt.Errorf("failed to delete TBG devices")
	}

	logger.Infof("Deleted %d TBG devices by user %s", len(req.SerialNumbers), username)
	return nil
}

// ============================
// Device Online Status (Task 6.2)
// ============================

// ListDeviceOnlineStatus returns real-time online status for all devices
func (s *Service) ListDeviceOnlineStatus(c *gin.Context) ([]DeviceOnlineStatusVo, error) {
	licenseId := middleware.GetLicenseId(c)

	devices, err := s.repo.ListAllNonDeletedDevices(licenseId)
	if err != nil {
		logger.Errorf("Failed to list devices for online status: %v", err)
		return nil, fmt.Errorf("failed to list devices")
	}

	if len(devices) == 0 {
		return []DeviceOnlineStatusVo{}, nil
	}

	// Build Redis keys for online status check
	keys := make([]string, len(devices))
	for i, d := range devices {
		keys[i] = fmt.Sprintf("online_%d", d.NeNeid)
	}

	// Batch check online status from Redis
	ctx := c.Request.Context()
	values, _ := redis.MGet(ctx, keys...)

	var result []DeviceOnlineStatusVo
	for i, d := range devices {
		online := false
		if i < len(values) && values[i] != nil {
			if val, ok := values[i].(string); ok && strings.ToLower(val) == "yes" {
				online = true
			}
		}

		vo := DeviceOnlineStatusVo{
			ElementId:    d.NeNeid,
			SerialNumber: derefStr(d.SerialNumber),
			DeviceName:   derefStr(d.DeviceName),
			Online:       online,
		}
		result = append(result, vo)
	}

	return result, nil
}

// GetDeviceOnlineStatus returns real-time online status for a single device
func (s *Service) GetDeviceOnlineStatus(c *gin.Context, elementId int64) (*DeviceOnlineStatusVo, error) {
	d, err := s.repo.GetDeviceByElementId(elementId)
	if err != nil {
		return nil, fmt.Errorf("device not found")
	}

	// Check Redis for online status
	ctx := c.Request.Context()
	key := fmt.Sprintf("online_%d", elementId)
	val, _ := redis.Get(ctx, key)

	online := strings.ToLower(val) == "yes"

	return &DeviceOnlineStatusVo{
		ElementId:    d.NeNeid,
		SerialNumber: derefStr(d.SerialNumber),
		DeviceName:   derefStr(d.DeviceName),
		Online:       online,
	}, nil
}

// ============================
// ACS Settings (Task 6.3)
// ============================

// GetACSSettings returns the ACS configuration via REST API
func (s *Service) GetACSSettings(c *gin.Context) (*RestACSConfigVo, error) {
	configJSON, err := s.repo.GetACSConfig()
	if err != nil {
		logger.Errorf("Failed to get ACS config: %v", err)
		return nil, fmt.Errorf("failed to get ACS config")
	}

	if configJSON == "" {
		return &RestACSConfigVo{}, nil
	}

	// Parse the full ACS config from DB
	var fullCfg struct {
		AcsUrl            *string `json:"acsUrl"`
		AcsUsername       *string `json:"acsUsername"`
		AcsPassword       *string `json:"acsPassword"`
		ConnectionTimeout *int    `json:"connectionTimeout"`
		InformInterval    *int    `json:"informInterval"`
		UdpPort           *int    `json:"udpPort"`
		TR069Enabled      *bool   `json:"tr069Enabled"`
	}
	if err := json.Unmarshal([]byte(configJSON), &fullCfg); err != nil {
		logger.Errorf("Failed to unmarshal ACS config: %v", err)
		return nil, fmt.Errorf("failed to parse ACS config")
	}

	// Return without password for security
	return &RestACSConfigVo{
		AcsUrl:            fullCfg.AcsUrl,
		AcsUsername:       fullCfg.AcsUsername,
		ConnectionTimeout: fullCfg.ConnectionTimeout,
		InformInterval:    fullCfg.InformInterval,
		UdpPort:           fullCfg.UdpPort,
		TR069Enabled:      fullCfg.TR069Enabled,
	}, nil
}

// UpdateACSSettings updates the ACS configuration via REST API
func (s *Service) UpdateACSSettings(c *gin.Context, req *RestUpdateACSConfigRequest) error {
	// Get existing config
	configJSON, err := s.repo.GetACSConfig()
	if err != nil {
		logger.Errorf("Failed to get existing ACS config: %v", err)
		return fmt.Errorf("failed to get ACS config")
	}

	// Parse existing config
	var existing struct {
		AcsUrl            *string `json:"acsUrl"`
		AcsUsername       *string `json:"acsUsername"`
		AcsPassword       *string `json:"acsPassword"`
		ConnectionTimeout *int    `json:"connectionTimeout"`
		InformInterval    *int    `json:"informInterval"`
		UdpPort           *int    `json:"udpPort"`
		TR069Enabled      *bool   `json:"tr069Enabled"`
	}
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &existing); err != nil {
			logger.Errorf("Failed to unmarshal existing ACS config: %v", err)
			return fmt.Errorf("failed to parse existing ACS config")
		}
	}

	// Merge with request
	if req.AcsUrl != nil {
		existing.AcsUrl = req.AcsUrl
	}
	if req.AcsUsername != nil {
		existing.AcsUsername = req.AcsUsername
	}
	if req.AcsPassword != nil {
		existing.AcsPassword = req.AcsPassword
	}
	if req.ConnectionTimeout != nil {
		existing.ConnectionTimeout = req.ConnectionTimeout
	}
	if req.InformInterval != nil {
		existing.InformInterval = req.InformInterval
	}
	if req.UdpPort != nil {
		existing.UdpPort = req.UdpPort
	}
	if req.TR069Enabled != nil {
		existing.TR069Enabled = req.TR069Enabled
	}

	// Marshal updated config
	data, err := json.Marshal(existing)
	if err != nil {
		return fmt.Errorf("failed to marshal ACS config")
	}

	// Save to DB
	if err := s.repo.UpdateACSConfig(string(data)); err != nil {
		logger.Errorf("Failed to update ACS config: %v", err)
		return fmt.Errorf("failed to update ACS config")
	}

	// Also update Redis cache
	ctx := c.Request.Context()
	redis.Set(ctx, "nms_config", string(data), 0)

	return nil
}

// ============================
// SNMP Operations (Task 6.4)
// ============================

// SnmpGet queues an SNMP GET operation to the Redis SNMP queue
func (s *Service) SnmpGet(c *gin.Context, req *SnmpGetRequest) error {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)

	// Verify device exists and get connection info
	dev, err := s.repo.GetDeviceById(req.ElementId, licenseId)
	if err != nil {
		return fmt.Errorf("device not found")
	}

	if dev.DeviceIp == nil || *dev.DeviceIp == "" {
		return fmt.Errorf("device has no IP address configured")
	}

	// Build SNMP parameters from OIDs
	var payload []snmp.SnmpParameter
	for _, oid := range req.OIDs {
		payload = append(payload, snmp.SnmpParameter{
			OID:  oid,
			Type: "string",
		})
	}

	// Build SNMP message
	msg := snmp.SnmpMessage{
		OperationType: snmp.OperationGet,
		ConnectionInfo: snmp.SnmpConnectionInfo{
			IP:        *dev.DeviceIp,
			Port:      161,
			Version:   2,
			Community: "public",
		},
		Payload: payload,
	}

	// Marshal and push to Redis SNMP queue
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal SNMP message")
	}

	ctx := c.Request.Context()
	if err := redis.LPush(ctx, snmp.SnmpQueueName, string(msgJSON)); err != nil {
		logger.Errorf("Failed to push SNMP GET to queue: %v", err)
		return fmt.Errorf("failed to queue SNMP GET operation")
	}

	logger.Infof("SNMP GET queued for device %d (%d OIDs) by user %s", req.ElementId, len(req.OIDs), username)
	return nil
}

// SnmpSet queues an SNMP SET operation to the Redis SNMP queue
func (s *Service) SnmpSet(c *gin.Context, req *SnmpSetRequest) error {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)

	// Verify device exists and get connection info
	dev, err := s.repo.GetDeviceById(req.ElementId, licenseId)
	if err != nil {
		return fmt.Errorf("device not found")
	}

	if dev.DeviceIp == nil || *dev.DeviceIp == "" {
		return fmt.Errorf("device has no IP address configured")
	}

	// Build SNMP parameters
	var payload []snmp.SnmpParameter
	for _, p := range req.Parameters {
		payload = append(payload, snmp.SnmpParameter{
			OID:   p.OID,
			Type:  p.Type,
			Value: p.Value,
		})
	}

	// Build SNMP message
	msg := snmp.SnmpMessage{
		OperationType: snmp.OperationSet,
		ConnectionInfo: snmp.SnmpConnectionInfo{
			IP:        *dev.DeviceIp,
			Port:      161,
			Version:   2,
			Community: "public",
		},
		Payload: payload,
	}

	// Marshal and push to Redis SNMP queue
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal SNMP message")
	}

	ctx := c.Request.Context()
	if err := redis.LPush(ctx, snmp.SnmpQueueName, string(msgJSON)); err != nil {
		logger.Errorf("Failed to push SNMP SET to queue: %v", err)
		return fmt.Errorf("failed to queue SNMP SET operation")
	}

	logger.Infof("SNMP SET queued for device %d (%d params) by user %s", req.ElementId, len(req.Parameters), username)
	return nil
}

// ListSnmpOperationLogs returns SNMP operation logs with pagination
func (s *Service) ListSnmpOperationLogs(c *gin.Context, offset, limit int) ([]SnmpOperationLogVo, int64, error) {
	logs, total, err := s.repo.ListSnmpOperationLogs(offset, limit)
	if err != nil {
		logger.Errorf("Failed to list SNMP operation logs: %v", err)
		return nil, 0, fmt.Errorf("failed to list SNMP operation logs")
	}

	var result []SnmpOperationLogVo
	for _, l := range logs {
		vo := SnmpOperationLogVo{}
		if v, ok := l["id"].(int64); ok {
			vo.Id = v
		}
		if v, ok := l["element_id"].(int64); ok {
			vo.ElementId = &v
		}
		if v, ok := l["operation"].(string); ok {
			vo.Operation = v
		}
		if v, ok := l["oid"].(string); ok {
			vo.OID = v
		}
		if v, ok := l["value"].(string); ok {
			vo.Value = v
		}
		if v, ok := l["status"].(string); ok {
			vo.Status = v
		}
		if v, ok := l["error_msg"].(string); ok {
			vo.ErrorMsg = v
		}
		if v, ok := l["operator"].(string); ok {
			vo.Operator = v
		}
		if v, ok := l["operate_time"].(time.Time); ok {
			vo.OperateTime = v.Format("2006-01-02T15:04:05Z07:00")
		}
		result = append(result, vo)
	}

	return result, total, nil
}

// ============================
// Helper functions
// ============================

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefIntPtr(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func derefInt64Ptr(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02T15:04:05Z07:00")
}

// generateLinkHeader generates RFC 5988 Link headers for offset-based pagination
func generateLinkHeader(baseUrl string, offset, limit, total int) string {
	var links []string

	// next
	if offset+limit < total {
		nextOffset := offset + limit
		links = append(links, fmt.Sprintf("<%s?offset=%d&limit=%d>; rel=\"next\"", baseUrl, nextOffset, limit))
	}

	// prev
	if offset > 0 {
		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		links = append(links, fmt.Sprintf("<%s?offset=%d&limit=%d>; rel=\"prev\"", baseUrl, prevOffset, limit))
	}

	// first
	links = append(links, fmt.Sprintf("<%s?offset=0&limit=%d>; rel=\"first\"", baseUrl, limit))

	// last
	lastOffset := 0
	if total > 0 {
		lastOffset = ((total - 1) / limit) * limit
	}
	links = append(links, fmt.Sprintf("<%s?offset=%d&limit=%d>; rel=\"last\"", baseUrl, lastOffset, limit))

	return strings.Join(links, ", ")
}

