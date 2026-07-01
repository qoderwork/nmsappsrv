package restapi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/internal/alarm"
	"nmsappsrv/internal/device"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ============================
// Device operations (cpe_element)
// ============================

func (r *Repository) ListDevices(licenseId int, offset, limit int) ([]device.CpeElement, int64, error) {
	var devices []device.CpeElement
	var total int64

	query := r.db.Model(&device.CpeElement{}).Where("license_id = ? AND deleted = ?", licenseId, false)
	query.Count(&total)

	err := query.Offset(offset).Limit(limit).Order("ne_neid ASC").Find(&devices).Error
	return devices, total, err
}

func (r *Repository) GetDeviceById(id int64, licenseId int) (*device.CpeElement, error) {
	var d device.CpeElement
	err := r.db.Where("ne_neid = ? AND license_id = ? AND deleted = ?", id, licenseId, false).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Repository) GetDeviceBySN(sn string, licenseId int) (*device.CpeElement, error) {
	var d device.CpeElement
	err := r.db.Where("serial_number = ? AND license_id = ? AND deleted = ?", sn, licenseId, false).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Repository) CreateDevice(d *device.CpeElement) error {
	return r.db.Create(d).Error
}

func (r *Repository) UpdateDevice(d *device.CpeElement) error {
	return r.db.Save(d).Error
}

func (r *Repository) SoftDeleteDevice(id int64, licenseId int) error {
	return r.db.Model(&device.CpeElement{}).
		Where("ne_neid = ? AND license_id = ?", id, licenseId).
		Update("deleted", true).Error
}

func (r *Repository) CountDevices(licenseId int) (int64, error) {
	var count int64
	err := r.db.Model(&device.CpeElement{}).
		Where("license_id = ? AND deleted = ?", licenseId, false).
		Count(&count).Error
	return count, err
}

// ============================
// Parameter operations (via Redis)
// ============================

func (r *Repository) GetDeviceParams(elementId int64) ([]RestParameterVo, error) {
	ctx := context.Background()
	key := fmt.Sprintf("device:params:%d", elementId)

	data, err := redis.Get(ctx, key)
	if err != nil || data == "" {
		return []RestParameterVo{}, nil
	}

	var params []RestParameterVo
	if err := json.Unmarshal([]byte(data), &params); err != nil {
		logger.Errorf("Failed to unmarshal device params for element %d: %v", elementId, err)
		return []RestParameterVo{}, nil
	}
	return params, nil
}

func (r *Repository) SetDeviceParams(elementId int64, params []RestParameterVo) error {
	ctx := context.Background()
	key := fmt.Sprintf("device:params:%d", elementId)

	data, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	if err := redis.Set(ctx, key, string(data), 0); err != nil {
		return fmt.Errorf("failed to cache parameters: %w", err)
	}

	// Push to operation queue for async device provisioning
	opData, _ := json.Marshal(map[string]interface{}{
		"type":       "set_params",
		"elementId":  elementId,
		"parameters": params,
		"timestamp":  time.Now().Unix(),
	})
	redis.LPush(ctx, "operation_queue", string(opData))

	return nil
}

func (r *Repository) PresetDeviceParams(elementId int64, params []RestParameterVo) (string, error) {
	ctx := context.Background()

	requestId := fmt.Sprintf("preset_%d_%d", elementId, time.Now().UnixNano())

	opData, _ := json.Marshal(map[string]interface{}{
		"type":       "preset_params",
		"elementId":  elementId,
		"parameters": params,
		"requestId":  requestId,
		"timestamp":  time.Now().Unix(),
	})
	redis.LPush(ctx, "operation_queue", string(opData))

	// Track request status
	statusData, _ := json.Marshal(map[string]string{
		"status": "pending",
	})
	redis.Set(ctx, fmt.Sprintf("request:%s", requestId), string(statusData), 0)

	return requestId, nil
}

// ============================
// Alarm operations
// ============================

func (r *Repository) ListAlarms(licenseId int, offset, limit int) ([]alarm.Alarm, int64, error) {
	var alarms []alarm.Alarm
	var total int64

	query := r.db.Model(&alarm.Alarm{}).Where("license_id = ?", licenseId)
	query.Count(&total)

	err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&alarms).Error
	return alarms, total, err
}

func (r *Repository) GetAlarmsByElementIds(elementIds []int64, licenseId int) ([]alarm.Alarm, error) {
	var alarms []alarm.Alarm
	err := r.db.Where("element_id IN ? AND license_id = ?", elementIds, licenseId).
		Order("id DESC").Find(&alarms).Error
	return alarms, err
}

func (r *Repository) ClearAlarms(alarmIds []int64, licenseId int) error {
	clearedStatus := 0
	return r.db.Model(&alarm.Alarm{}).
		Where("id IN ? AND license_id = ?", alarmIds, licenseId).
		Update("alarm_status", &clearedStatus).Error
}

func (r *Repository) SyncAlarms(elementIds []int64, licenseId int) ([]alarm.Alarm, error) {
	var alarms []alarm.Alarm
	err := r.db.Where("element_id IN ? AND license_id = ?", elementIds, licenseId).
		Order("event_time DESC").Find(&alarms).Error
	return alarms, err
}

// ============================
// Upgrade file operations
// ============================

func (r *Repository) CreateUpgradeFile(record map[string]interface{}) (int64, error) {
	err := r.db.Table("upgrade_file").Create(record).Error
	if err != nil {
		return 0, err
	}
	var id int64
	r.db.Table("upgrade_file").Order("id DESC").Limit(1).Pluck("id", &id)
	return id, nil
}

func (r *Repository) ListUpgradeFiles(licenseId int, offset, limit int) ([]map[string]interface{}, int64, error) {
	var files []map[string]interface{}
	var total int64

	query := r.db.Table("upgrade_file").Where("license_id = ?", licenseId)
	query.Count(&total)

	err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&files).Error
	return files, total, err
}

func (r *Repository) DeleteUpgradeFile(id int, licenseId int) error {
	return r.db.Table("upgrade_file").
		Where("id = ? AND license_id = ?", id, licenseId).
		Delete(nil).Error
}

// ============================
// Upgrade task operations
// ============================

func (r *Repository) CreateUpgradeTask(record map[string]interface{}) (int64, error) {
	err := r.db.Table("upgrade_task").Create(record).Error
	if err != nil {
		return 0, err
	}
	var id int64
	r.db.Table("upgrade_task").Order("id DESC").Limit(1).Pluck("id", &id)
	return id, nil
}

func (r *Repository) GetUpgradeTask(id int) (map[string]interface{}, error) {
	var task map[string]interface{}
	err := r.db.Table("upgrade_task").Where("id = ?", id).First(&task).Error
	return task, err
}

func (r *Repository) ListUpgradeTasks(licenseId int, offset, limit int) ([]map[string]interface{}, int64, error) {
	var tasks []map[string]interface{}
	var total int64

	query := r.db.Table("upgrade_task").Where("license_id = ?", licenseId)
	query.Count(&total)

	err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&tasks).Error
	return tasks, total, err
}

// ============================
// Request status operations (via Redis + event_log)
// ============================

func (r *Repository) GetRequestStatus(requestId string) (*RequestStatusVo, error) {
	ctx := context.Background()
	key := fmt.Sprintf("request:%s", requestId)

	data, err := redis.Get(ctx, key)
	if err == nil && data != "" {
		var statusMap map[string]string
		if json.Unmarshal([]byte(data), &statusMap) == nil {
			return &RequestStatusVo{
				RequestId: requestId,
				Status:    statusMap["status"],
				Result:    statusMap["result"],
			}, nil
		}
	}

	// Fallback to event_log table
	var logEntry map[string]interface{}
	err = r.db.Table("event_log").
		Where("request_id = ?", requestId).
		Order("id DESC").First(&logEntry).Error
	if err != nil {
		return &RequestStatusVo{
			RequestId: requestId,
			Status:    "pending",
		}, nil
	}

	status := "completed"
	if logEntry["status"] != nil {
		if s, ok := logEntry["status"].(string); ok {
			status = s
		}
	}

	result := ""
	if logEntry["result"] != nil {
		if r, ok := logEntry["result"].(string); ok {
			result = r
		}
	}

	return &RequestStatusVo{
		RequestId: requestId,
		Status:    status,
		Result:    result,
	}, nil
}

// ============================
// TBG operations
// ============================

func (r *Repository) ListTBGs(licenseId int, offset, limit int) ([]TBGDevice, int64, error) {
	var tbgs []TBGDevice
	var total int64

	query := r.db.Model(&TBGDevice{}).Where("license_id = ?", licenseId)
	query.Count(&total)

	err := query.Offset(offset).Limit(limit).Order("id ASC").Find(&tbgs).Error
	return tbgs, total, err
}

func (r *Repository) GetTBGBySN(sn string) (*TBGDevice, error) {
	var tbg TBGDevice
	err := r.db.Where("serial_number = ?", sn).First(&tbg).Error
	if err != nil {
		return nil, err
	}
	return &tbg, nil
}

func (r *Repository) GetTBGByWanMac(mac string) (*TBGDevice, error) {
	var tbg TBGDevice
	err := r.db.Where("wan_mac_address = ?", mac).First(&tbg).Error
	if err != nil {
		return nil, err
	}
	return &tbg, nil
}

func (r *Repository) CreateTBGs(tbgs []TBGDevice) error {
	return r.db.Create(&tbgs).Error
}

func (r *Repository) UpdateTBG(tbg *TBGDevice) error {
	return r.db.Save(tbg).Error
}

func (r *Repository) DeleteTBGsBySNs(sns []string) error {
	return r.db.Where("serial_number IN ?", sns).Delete(&TBGDevice{}).Error
}

// ============================
// Device Online Status (Task 6.2)
// ============================

// ListAllNonDeletedDevices returns all non-deleted devices with basic info for online status check
func (r *Repository) ListAllNonDeletedDevices(licenseId int) ([]device.CpeElement, error) {
	var devices []device.CpeElement
	err := r.db.Where("license_id = ? AND deleted = ?", licenseId, false).
		Select("ne_neid, serial_number, device_name").
		Find(&devices).Error
	return devices, err
}

// GetDeviceByElementId returns a single non-deleted device by element ID
func (r *Repository) GetDeviceByElementId(elementId int64) (*device.CpeElement, error) {
	var d device.CpeElement
	err := r.db.Where("ne_neid = ? AND deleted = ?", elementId, false).
		Select("ne_neid, serial_number, device_name").
		First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ============================
// ACS Settings (Task 6.3)
// ============================

// GetACSConfig reads ACS config from system_config table
func (r *Repository) GetACSConfig() (string, error) {
	var cfg struct {
		Config *string `gorm:"column:config"`
	}
	err := r.db.Table("system_config").
		Select("config").
		Where("id = ?", "nms_config").
		Scan(&cfg).Error
	if err != nil {
		return "", err
	}
	if cfg.Config == nil {
		return "", nil
	}
	return *cfg.Config, nil
}

// UpdateACSConfig upserts ACS config in system_config table
func (r *Repository) UpdateACSConfig(value string) error {
	var existing struct {
		Id string `gorm:"column:id"`
	}
	err := r.db.Table("system_config").
		Select("id").
		Where("id = ?", "nms_config").
		Scan(&existing).Error

	if err != nil || existing.Id == "" {
		// Insert new
		return r.db.Table("system_config").
			Create(map[string]interface{}{
				"id":     "nms_config",
				"config": value,
			}).Error
	}
	// Update existing
	return r.db.Table("system_config").
		Where("id = ?", "nms_config").
		Update("config", value).Error
}

// ============================
// SNMP Operations (Task 6.4)
// ============================

// ListSnmpOperationLogs returns SNMP operation logs with pagination
func (r *Repository) ListSnmpOperationLogs(offset, limit int) ([]map[string]interface{}, int64, error) {
	var logs []map[string]interface{}
	var total int64

	query := r.db.Table("snmp_operation_log")
	query.Count(&total)

	err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&logs).Error
	return logs, total, err
}
