package misc

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nmsappsrv/internal/device"
	"nmsappsrv/internal/mq"
	"nmsappsrv/internal/opmsg"
	"nmsappsrv/pkg/redis"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Internal helpers & types
// ---------------------------------------------------------------------------

// bsOperationMsg is the TR-069 operation message pushed to the Redis operation queue.
type bsOperationMsg struct {
	EventType      string `json:"eventType"`
	NeNeid         int64  `json:"neNeid"`
	Operation      string `json:"operation"`
	OperationUser  string `json:"operationUser"`
	CommandTrackId int64  `json:"commandTrackId"`
	ExpiredAt      int64  `json:"expiredAt"`
}

// configUploadDir returns the base directory used to store uploaded / backed-up
// configuration files.  It honours the NMS_CONFIG_UPLOAD_DIR env var and falls
// back to a sensible default.
func configUploadDir() string {
	dir := os.Getenv("NMS_CONFIG_UPLOAD_DIR")
	if dir == "" {
		dir = "/data/nms/configs"
	}
	return dir
}

// ---------------------------------------------------------------------------
// Service methods — BaseStation Backup & Restore
// ---------------------------------------------------------------------------

// ListBaseStationBackupInfo returns a paginated list of devices together with
// their latest config-file / backup information.
func (s *service) ListBaseStationBackupInfo(req *ListBaseStationBackupInfoRequest, tenancyId int) ([]BaseStationBackupInfoVo, int64, error) {
	db := s.repo.DB()
	offset := (req.Page - 1) * req.PageSize

	// Base query: non-deleted devices belonging to this license / tenancy.
	baseQuery := db.Model(&device.CpeElement{}).
		Where("license_id = ? AND deleted = ?", tenancyId, false)

	if req.SearchText != "" {
		like := "%" + req.SearchText + "%"
		baseQuery = baseQuery.Where("device_name LIKE ? OR serial_number LIKE ?", like, like)
	}

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count devices: %w", err)
	}

	var elements []device.CpeElement
	if err := baseQuery.Offset(offset).Limit(req.PageSize).Find(&elements).Error; err != nil {
		return nil, 0, fmt.Errorf("query devices: %w", err)
	}

	result := make([]BaseStationBackupInfoVo, 0, len(elements))
	for _, elem := range elements {
		vo := BaseStationBackupInfoVo{
			ElementId:    elem.NeNeid,
			DeviceName:   ptrStr(elem.DeviceName),
			SerialNumber: ptrStr(elem.SerialNumber),
			ConfigFile:   ptrStr(elem.ConfigFile),
		}
		if elem.ConfigFileUploadTime != nil {
			vo.ConfigFileTime = elem.ConfigFileUploadTime.Format(time.RFC3339)
		}

		// Look up the latest backup record from config_upload_log.
		var cul ConfigUploadLog
		err := db.Where("element_id = ? AND license_id = ?", elem.NeNeid, tenancyId).
			Order("upload_time DESC").
			First(&cul).Error
		if err == nil {
			vo.HasBackup = true
			if cul.UploadTime != nil {
				vo.LatestBackupTime = cul.UploadTime.Format(time.RFC3339)
			}
		}

		result = append(result, vo)
	}

	return result, total, nil
}

// ImportConfigFile persists an uploaded configuration file to disk and creates
// a config_upload_log record.
func (s *service) ImportConfigFile(elementId int64, fileName string, fileData []byte, tenancyId int) (*ImportConfigFileResult, error) {
	db := s.repo.DB()

	// Verify the target device exists.
	var elem device.CpeElement
	if err := db.Where("ne_neid = ?", elementId).First(&elem).Error; err != nil {
		return nil, fmt.Errorf("device %d not found: %w", elementId, err)
	}

	// Generate a unique id and persist the file.
	logId := strings.ReplaceAll(uuid.New().String(), "-", "")
	now := time.Now()

	subDir := filepath.Join(configUploadDir(), fmt.Sprintf("%d", elementId))
	if err := os.MkdirAll(subDir, 0755); err != nil {
		return nil, fmt.Errorf("create upload directory: %w", err)
	}

	filePath := filepath.Join(subDir, logId+"_"+fileName)
	if err := os.WriteFile(filePath, fileData, 0644); err != nil {
		return nil, fmt.Errorf("write config file: %w", err)
	}

	// Create the config_upload_log record.
	openStationFile := false
	cul := ConfigUploadLog{
		Id:              logId,
		FileName:        strPtr(fileName),
		ElementId:       int64Ptr(elementId),
		UploadTime:      &now,
		Loc:             strPtr(filePath),
		LicenseId:       intPtr(tenancyId),
		OpenStationFile: &openStationFile,
		DeviceUpload:    true,
	}
	// Atomic: create config upload log + update device config file reference
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&cul).Error; err != nil {
			return fmt.Errorf("create config upload log: %w", err)
		}
		return tx.Model(&device.CpeElement{}).Where("ne_neid = ?", elementId).
			Updates(map[string]interface{}{
				"config_file":            filePath,
				"config_file_upload_time": now,
			}).Error
	}); err != nil {
		return nil, err
	}

	return &ImportConfigFileResult{
		ElementId: elementId,
		FileName:  fileName,
		Success:   true,
	}, nil
}

// ExportConfigFile collects the latest config files for the given devices and
// packs them into a zip archive.  It returns the path to the temporary zip file
// (the caller is responsible for cleanup).
func (s *service) ExportConfigFile(elementIds []int64, tenancyId int) (string, error) {
	db := s.repo.DB()

	// Fetch the latest config_upload_log per device.
	var logs []ConfigUploadLog
	if err := db.Where("element_id IN ? AND license_id = ?", elementIds, tenancyId).
		Order("upload_time DESC").
		Find(&logs).Error; err != nil {
		return "", fmt.Errorf("query config logs: %w", err)
	}

	// De-duplicate: keep only the newest entry per element_id.
	seen := make(map[int64]bool)
	var uniqueLogs []ConfigUploadLog
	for _, l := range logs {
		eid := int64Val(l.ElementId)
		if !seen[eid] && l.Loc != nil && *l.Loc != "" {
			seen[eid] = true
			uniqueLogs = append(uniqueLogs, l)
		}
	}

	// Create a temporary zip file.
	tmpDir, err := os.MkdirTemp("", "bs-export-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	zipPath := filepath.Join(tmpDir, "config_files.zip")

	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("create zip file: %w", err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	for _, l := range uniqueLogs {
		srcPath := ptrStr(l.Loc)
		if _, statErr := os.Stat(srcPath); os.IsNotExist(statErr) {
			continue
		}

		entryName := fmt.Sprintf("%d_%s", int64Val(l.ElementId), filepath.Base(srcPath))
		w, err := zw.Create(entryName)
		if err != nil {
			return "", fmt.Errorf("add zip entry %s: %w", entryName, err)
		}

		src, err := os.Open(srcPath)
		if err != nil {
			return "", fmt.Errorf("open source file %s: %w", srcPath, err)
		}
		if _, err := io.Copy(w, src); err != nil {
			src.Close()
			return "", fmt.Errorf("copy file data for %s: %w", entryName, err)
		}
		src.Close()
	}

	return zipPath, nil
}

// CreateBSBackupTask creates a new backup task and its per-device log entries.
// When ExecuteMode is 1 (immediate) the TR-069 commands are dispatched right away.
func (s *service) CreateBSBackupTask(req *AddBSBackupTaskRequest, username string, tenancyId int) error {
	db := s.repo.DB()

	deviceGroupJSON, err := json.Marshal(req.DeviceGroupIds)
	if err != nil {
		return fmt.Errorf("marshal device group ids: %w", err)
	}

	elementIdStrs := make([]string, 0, len(req.ElementIds))
	for _, id := range req.ElementIds {
		elementIdStrs = append(elementIdStrs, fmt.Sprintf("%d", id))
	}

	now := time.Now()
	var triggerTime *time.Time
	if req.ExecuteMode == 3 && req.TriggerTime != "" {
		t, err := time.Parse(time.RFC3339, req.TriggerTime)
		if err != nil {
			return fmt.Errorf("parse trigger time: %w", err)
		}
		triggerTime = &t
	}

	taskType := "backup"
	task := BackupOrRestoreTask{
		Name:               strPtr(req.Name),
		User:               strPtr(username),
		OperationTime:      &now,
		Status:             intPtr(1), // waiting
		ExecuteMode:        intPtr(req.ExecuteMode),
		TriggerTime:        triggerTime,
		TenancyId:          intPtr(tenancyId),
		TaskType:           strPtr(taskType),
		ExecuteOnAllDevice: boolPtr(req.ExecuteOnAllDevice),
		ElementIds:         strPtr(strings.Join(elementIdStrs, ",")),
		Scope:              strPtr(req.Scope),
		DeviceGroupIds:     strPtr(string(deviceGroupJSON)),
	}

	// Atomic: create task + per-device log entries
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&task).Error; err != nil {
			return fmt.Errorf("create backup task: %w", err)
		}
		return s.createBSDeviceLogs(tx, task.Id, req.ElementIds, taskType)
	}); err != nil {
		return err
	}

	// Immediate execution: dispatch TR-069 commands now.
	if req.ExecuteMode == 1 {
		if err := s.dispatchBSOperation(task.Id, username); err != nil {
			return fmt.Errorf("dispatch backup operation: %w", err)
		}
	}

	return nil
}

// CreateBSRestoreTask creates a new restore task and its per-device log entries.
// When ExecuteMode is 1 (immediate) the TR-069 commands are dispatched right away.
func (s *service) CreateBSRestoreTask(req *AddBSRestoreTaskRequest, username string, tenancyId int) error {
	db := s.repo.DB()

	deviceGroupJSON, err := json.Marshal(req.DeviceGroupIds)
	if err != nil {
		return fmt.Errorf("marshal device group ids: %w", err)
	}

	elementIdStrs := make([]string, 0, len(req.ElementIds))
	for _, id := range req.ElementIds {
		elementIdStrs = append(elementIdStrs, fmt.Sprintf("%d", id))
	}

	now := time.Now()
	var triggerTime *time.Time
	if req.ExecuteMode == 3 && req.TriggerTime != "" {
		t, err := time.Parse(time.RFC3339, req.TriggerTime)
		if err != nil {
			return fmt.Errorf("parse trigger time: %w", err)
		}
		triggerTime = &t
	}

	taskType := "restore"
	task := BackupOrRestoreTask{
		Name:               strPtr(req.Name),
		User:               strPtr(username),
		OperationTime:      &now,
		Status:             intPtr(1), // waiting
		ExecuteMode:        intPtr(req.ExecuteMode),
		TriggerTime:        triggerTime,
		TenancyId:          intPtr(tenancyId),
		TaskType:           strPtr(taskType),
		ExecuteOnAllDevice: boolPtr(req.ExecuteOnAllDevice),
		ElementIds:         strPtr(strings.Join(elementIdStrs, ",")),
		Scope:              strPtr(req.Scope),
		DeviceGroupIds:     strPtr(string(deviceGroupJSON)),
	}

	// Atomic: create task + per-device log entries
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&task).Error; err != nil {
			return fmt.Errorf("create restore task: %w", err)
		}
		return s.createBSDeviceLogs(tx, task.Id, req.ElementIds, taskType)
	}); err != nil {
		return err
	}

	// Immediate execution: dispatch TR-069 commands now.
	if req.ExecuteMode == 1 {
		if err := s.dispatchBSOperation(task.Id, username); err != nil {
			return fmt.Errorf("dispatch restore operation: %w", err)
		}
	}

	return nil
}

// CancelTask sets a backup or restore task's status to 4 (cancelled).
func (s *service) CancelTask(taskId int) error {
	db := s.repo.DB()

	var task BackupOrRestoreTask
	if err := db.Where("id = ?", taskId).First(&task).Error; err != nil {
		return fmt.Errorf("task %d not found: %w", taskId, err)
	}

	status := intVal(task.Status)
	if status == 3 || status == 4 {
		return fmt.Errorf("task %d is already executed or cancelled", taskId)
	}

	now := time.Now()
	if err := db.Model(&BackupOrRestoreTask{}).Where("id = ?", taskId).
		Updates(map[string]interface{}{
			"status":   4,
			"end_time": now,
		}).Error; err != nil {
		return fmt.Errorf("cancel task %d: %w", taskId, err)
	}

	return nil
}

// StartBSBackupRestoreTask transitions an awaiting task (status 1) to executing
// (status 2) and dispatches the TR-069 commands to the target devices.
func (s *service) StartBSBackupRestoreTask(taskId int, username string) error {
	db := s.repo.DB()

	var task BackupOrRestoreTask
	if err := db.Where("id = ?", taskId).First(&task).Error; err != nil {
		return fmt.Errorf("task %d not found: %w", taskId, err)
	}

	if intVal(task.Status) != 1 {
		return fmt.Errorf("task %d is not in waiting state (current status: %d)", taskId, intVal(task.Status))
	}

	now := time.Now()
	if err := db.Model(&BackupOrRestoreTask{}).Where("id = ?", taskId).
		Updates(map[string]interface{}{
			"status":     2,
			"start_time": now,
		}).Error; err != nil {
		return fmt.Errorf("start task %d: %w", taskId, err)
	}

	if err := s.dispatchBSOperation(taskId, username); err != nil {
		return fmt.Errorf("dispatch task %d: %w", taskId, err)
	}

	return nil
}

// ListBSBackupTasks returns a paginated list of backup/restore tasks for the
// given tenancy.  The returned VOs follow the existing BackupRestoreTaskVo
// shape used elsewhere in the misc module.
func (s *service) ListBSBackupTasks(tenancyId int, page, pageSize int) ([]BackupRestoreTaskVo, int64, error) {
	db := s.repo.DB()
	offset := (page - 1) * pageSize

	var total int64
	baseQuery := db.Model(&BackupOrRestoreTask{}).Where("tenancy_id = ?", tenancyId)
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	var tasks []BackupOrRestoreTask
	if err := baseQuery.Order("operation_time DESC").
		Offset(offset).Limit(pageSize).
		Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("query tasks: %w", err)
	}

	result := make([]BackupRestoreTaskVo, 0, len(tasks))
	for _, t := range tasks {
		// Count devices for this task
		var deviceCount int64
		db.Model(&RestoreAndBackUpDeviceLog{}).Where("task_id = ?", t.Id).Count(&deviceCount)
		var successCount int64
		db.Model(&RestoreAndBackUpDeviceLog{}).Where("task_id = ? AND status = ?", t.Id, 3).Count(&successCount)

		vo := BackupRestoreTaskVo{
			Id:              t.Id,
			Name:            ptrStr(t.Name),
			TaskType:        ptrStr(t.TaskType),
			OperationUser:   ptrStr(t.User),
			OperationTime:   timePtrStr(t.OperationTime),
			Status:          intVal(t.Status),
			ExecuteMode:     intVal(t.ExecuteMode),
			DeviceCount:     int(deviceCount),
			Progress:        fmt.Sprintf("%d/%d", successCount, deviceCount),
		}
		result = append(result, vo)
	}

	return result, total, nil
}

// ListDeviceBackupResult returns paginated per-device execution results for a
// given task.
func (s *service) ListDeviceBackupResult(taskId int, page, pageSize int) ([]DeviceBackupResultVo, int64, error) {
	db := s.repo.DB()
	offset := (page - 1) * pageSize

	var total int64
	baseQuery := db.Model(&RestoreAndBackUpDeviceLog{}).Where("task_id = ?", taskId)
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count device logs: %w", err)
	}

	var logs []RestoreAndBackUpDeviceLog
	if err := baseQuery.Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("query device logs: %w", err)
	}

	result := make([]DeviceBackupResultVo, 0, len(logs))
	for _, l := range logs {
		eid := int64Val(l.ElementId)

		// Resolve device name and serial number.
		var elem device.CpeElement
		_ = db.Select("device_name, serial_number").
			Where("ne_neid = ?", eid).
			First(&elem).Error

		vo := DeviceBackupResultVo{
			ElementId:         eid,
			DeviceName:        ptrStr(elem.DeviceName),
			SerialNumber:      ptrStr(elem.SerialNumber),
			Result:            l.Results,
			FailureReason:     ptrStr(l.FailureReason),
			StartTime:         timePtrStr(l.StartTime),
			EndTime:           timePtrStr(l.EndTime),
			ConfigurationFile: ptrStr(l.ConfigurationFile),
		}
		result = append(result, vo)
	}

	return result, total, nil
}

// GetConfigFilePath returns the on-disk path for a configuration file
// identified by its config_upload_log id.
func (s *service) GetConfigFilePath(logId string) (string, error) {
	db := s.repo.DB()

	var cul ConfigUploadLog
	if err := db.Where("id = ?", logId).First(&cul).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("config file record %s not found", logId)
		}
		return "", fmt.Errorf("query config file record: %w", err)
	}

	if cul.Loc == nil || *cul.Loc == "" {
		return "", fmt.Errorf("config file location is empty for record %s", logId)
	}

	filePath := *cul.Loc
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("config file does not exist on disk: %s", filePath)
	}

	return filePath, nil
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

// createBSDeviceLogs inserts a RestoreAndBackUpDeviceLog row for every device
// that belongs to the given task.
func (s *service) createBSDeviceLogs(db *gorm.DB, taskId int, elementIds []int64, taskType string) error {

	for _, eid := range elementIds {
		log := RestoreAndBackUpDeviceLog{
			ElementId: int64Ptr(int64(eid)),
			TaskId:    intPtr(taskId),
			Type:      strPtr(taskType),
		}
		if err := db.Create(&log).Error; err != nil {
			return fmt.Errorf("create device log for element %d: %w", eid, err)
		}
	}
	return nil
}

// dispatchBSOperation sends TR-069 operation messages to the Redis operation
// queue for every pending device log belonging to the given task.
func (s *service) dispatchBSOperation(taskId int, username string) error {
	db := s.repo.DB()
	ctx := context.Background()

	var task BackupOrRestoreTask
	if err := db.Where("id = ?", taskId).First(&task).Error; err != nil {
		return fmt.Errorf("task %d not found for dispatch: %w", taskId, err)
	}

	var logs []RestoreAndBackUpDeviceLog
	if err := db.Where("task_id = ?", taskId).Find(&logs).Error; err != nil {
		return fmt.Errorf("query device logs for dispatch: %w", err)
	}

	taskType := ptrStr(task.TaskType)
	operation := "GetParameterValues"
	if taskType == "restore" {
		operation = "SetParameterValues"
	}

	now := time.Now()

	for _, l := range logs {
		eid := int64Val(l.ElementId)

		// Acquire a per-device lock to prevent concurrent operations.
		lockKey := fmt.Sprintf("bs_operation_lock:%d", eid)
		if !redis.Lock(ctx, lockKey, 5*time.Minute) {
			// Device is locked by another operation; skip and continue.
			continue
		}

		msg := opmsg.Message{
			EventType:      "backup_restore", // legacy label kept for downstream observability
			NeNeid:         eid,
			Operation:      operation, // variable: "SetParameterValues" or other Java-aligned string
			OperationUser:  username,
			CommandTrackId: l.Id,
			ExpiredAt:      now.Add(30 * time.Minute).UnixMilli(),
		}

		msgBytes, err := msg.Marshal()
		if err != nil {
			redis.Unlock(ctx, lockKey)
			continue
		}

		if err := redis.LPush(ctx, mq.OperationQueue, string(msgBytes)); err != nil {
			redis.Unlock(ctx, lockKey)
			continue
		}

		// Update device log with start time.
		db.Model(&RestoreAndBackUpDeviceLog{}).Where("id = ?", l.Id).
			Update("start_time", now)

		redis.Unlock(ctx, lockKey)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Pointer / value helpers (safe dereferencing for GORM nullable fields)
// ---------------------------------------------------------------------------

func intVal(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func int64Val(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func boolVal(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}

func timePtrStr(p *time.Time) string {
	if p == nil {
		return ""
	}
	return p.Format(time.RFC3339)
}

func int64Ptr(i int64) *int64 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}
