package cacert

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"nmsappsrv/internal/config"
	"nmsappsrv/pkg/apperror"
	redisclient "nmsappsrv/pkg/redis"
)

// Service handles business logic for CA certificate module
type Service struct {
	repo *Repository
}

// NewService creates a new Service
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ---------- CaFile operations ----------

// ListCaFiles returns paginated CA file list
func (s *Service) ListCaFiles(ctx context.Context, page, pageSize int) ([]map[string]interface{}, int64, error) {
	files, total, err := s.repo.ListCaFiles(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]map[string]interface{}, len(files))
	for i, f := range files {
		result[i] = map[string]interface{}{
			"id":          f.Id,
			"fileName":    strOrEmpty(f.FileName),
			"url":         strOrEmpty(f.URL),
			"delFlag":     strOrEmpty(f.DelFlag),
			"createBy":    strOrEmpty(f.CreateBy),
			"createTime":  formatTime(f.CreateTime),
			"description": strOrEmpty(f.Description),
		}
	}
	return result, total, nil
}

// DeleteCaFiles soft-deletes CA files by IDs
func (s *Service) DeleteCaFiles(ctx context.Context, ids []int) error {
	return s.repo.DeleteCaFiles(ctx, ids)
}

// ListAllCaFiles returns all non-deleted CA files (for dropdown)
func (s *Service) ListAllCaFiles(ctx context.Context) ([]map[string]interface{}, error) {
	files, err := s.repo.ListAllCaFiles(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, len(files))
	for i, f := range files {
		result[i] = map[string]interface{}{
			"id":       f.Id,
			"fileName": strOrEmpty(f.FileName),
		}
	}
	return result, nil
}

// GetCaFileByID returns a single CA file by ID
func (s *Service) GetCaFileByID(ctx context.Context, id int) (*CaFile, error) {
	return s.repo.GetCaFileByID(ctx, id)
}

// CreateCaFileRecord creates a CA file record after upload
func (s *Service) CreateCaFileRecord(ctx context.Context, fileName, url, description, createBy string) error {
	now := time.Now()
	file := &CaFile{
		FileName:    &fileName,
		URL:         &url,
		DelFlag:     strPtr("0"),
		CreateBy:    &createBy,
		CreateTime:  &now,
		Description: &description,
	}
	return s.repo.CreateCaFile(ctx, file)
}

// GetCaFilePath returns the configured CA file storage path
func (s *Service) GetCaFilePath() string {
	if config.Cfg != nil && config.Cfg.TR069.FileServerIp != "" {
		// In production, this would be from a dedicated caFilePath config
		// For now return a default path
		return filepath.Join(os.TempDir(), "ca_files")
	}
	return filepath.Join(os.TempDir(), "ca_files")
}

// ---------- CaTask operations ----------

// SaveCaTask creates a CA deployment task and dispatches TR-069 commands to devices
func (s *Service) SaveCaTask(ctx context.Context, taskName string, caFileId int, scope string, deviceIds []int64, groupIds []string, username string) error {
	// Validate CA file exists
	caFile, err := s.repo.GetCaFileByID(ctx, caFileId)
	if err != nil {
		return apperror.ErrNotFound.WithMessage("CA file not found")
	}

	// Create task record
	now := time.Now()
	task := &CaTask{
		TaskName:   &taskName,
		CaFileId:   &caFileId,
		Status:     strPtr("0"), // pending
		CreateBy:   &username,
		CreateTime: &now,
	}
	if err := s.repo.CreateCaTask(ctx, task); err != nil {
		return apperror.Wrap(err, "CA_TASK_CREATE_FAILED", 500, "failed to create CA task")
	}

	// Build device list based on scope
	var targetDevices []int64

	if scope == "2" {
		// Individual devices
		targetDevices = deviceIds
	} else if scope == "1" {
		// Device groups - query devices in each group
		for _, groupId := range groupIds {
			devices, err := s.getDevicesInGroup(ctx, groupId)
			if err != nil {
				continue
			}
			targetDevices = append(targetDevices, devices...)
		}
	}

	// Dispatch TR-069 commands for each device
	var logs []DeviceSendCaLog
	fileName := strOrEmpty(caFile.FileName)
	filePath := filepath.Join(s.GetCaFilePath(), fileName)

	for _, deviceId := range targetDevices {
		// Check if file exists before dispatching
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			// File doesn't exist, skip this device
			continue
		}

		// Create event_log entry to get commandTrackId
		eventLogId, err := s.createEventLog(ctx, "UpdateCertificate", deviceId, username)
		if err != nil {
			continue
		}

		// Build TR-069 download operation message
		opMsg := s.buildCaOperationMessage(deviceId, caFileId, fileName, filePath, username, eventLogId)

		// Push to Redis operation_queue
		msgJSON, _ := json.Marshal(opMsg)
		if err := redisclient.LPush(ctx, "queue:operation", string(msgJSON)); err != nil {
			// Log error but continue with other devices
			continue
		}

		// Create device send log
		log := DeviceSendCaLog{
			DeviceId:   &deviceId,
			Result:     intPtr(1), // dispatched
			Scope:      &scope,
			TaskId:     &task.Id,
			EventLogId: &eventLogId,
		}
		if scope == "1" {
			// For group scope, we'd need to track which group this device belongs to
			// Simplified: leave DeviceGroupId empty
		}
		logs = append(logs, log)
	}

	// Batch insert device logs
	if len(logs) > 0 {
		s.repo.CreateDeviceSendCaLogs(ctx, logs)
	}

	return nil
}

// ListCaTasks returns paginated CA task list
func (s *Service) ListCaTasks(ctx context.Context, page, pageSize int, tenancyId *int) ([]map[string]interface{}, int64, error) {
	tasks, total, err := s.repo.ListCaTasks(ctx, page, pageSize, tenancyId)
	if err != nil {
		return nil, 0, err
	}

	result := make([]map[string]interface{}, len(tasks))
	for i, t := range tasks {
		result[i] = map[string]interface{}{
			"id":         t.Id,
			"taskName":   strOrEmpty(t.TaskName),
			"status":     strOrEmpty(t.Status),
			"caFileId":   t.CaFileId,
			"tenancyId":  t.TenancyId,
			"createTime": formatTime(t.CreateTime),
		}
	}
	return result, total, nil
}

// GetCaTaskDetail returns a single CA task detail
func (s *Service) GetCaTaskDetail(ctx context.Context, id int) (map[string]interface{}, error) {
	task, err := s.repo.GetCaTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":         task.Id,
		"taskName":   strOrEmpty(task.TaskName),
		"status":     strOrEmpty(task.Status),
		"caFileId":   task.CaFileId,
		"createTime": formatTime(task.CreateTime),
	}, nil
}

// DeleteCaTasks deletes CA tasks by IDs
func (s *Service) DeleteCaTasks(ctx context.Context, ids []int) error {
	return s.repo.DeleteCaTasks(ctx, ids)
}

// ---------- DeviceSendCaLog operations ----------

// ListDeviceSendCaLogs returns paginated device CA delivery logs
func (s *Service) ListDeviceSendCaLogs(ctx context.Context, taskId int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	logs, total, err := s.repo.ListDeviceSendCaLogs(ctx, taskId, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]map[string]interface{}, len(logs))
	for i, log := range logs {
		// Enrich with device info
		deviceName, serialNumber := s.getDeviceInfo(ctx, *log.DeviceId)

		result[i] = map[string]interface{}{
			"id":           log.Id,
			"result":       log.Result,
			"serialNumber": serialNumber,
			"deviceName":   deviceName,
			"info":         strOrEmpty(log.Info),
		}
	}
	return result, total, nil
}

// ---------- Internal helpers ----------

// getDevicesInGroup returns device IDs belonging to a group
func (s *Service) getDevicesInGroup(ctx context.Context, groupId string) ([]int64, error) {
	// Query group_has_element table for devices in this group
	type GroupElement struct {
		ElementId int64 `gorm:"column:element_id"`
	}
	var elements []GroupElement
	err := s.repo.db.WithContext(ctx).
		Table("group_has_element").
		Where("group_id = ?", groupId).
		Select("element_id").
		Find(&elements).Error
	if err != nil {
		return nil, err
	}

	deviceIds := make([]int64, len(elements))
	for i, e := range elements {
		deviceIds[i] = e.ElementId
	}
	return deviceIds, nil
}

// createEventLog creates an event_log entry and returns the auto-generated ID
func (s *Service) createEventLog(ctx context.Context, eventType string, deviceId int64, user string) (int64, error) {
	row := struct {
		Id               int64     `gorm:"primaryKey;autoIncrement"`
		EventType        string    `gorm:"column:event_type;type:varchar(255)"`
		OperationTime    time.Time `gorm:"column:operation_time"`
		User             string    `gorm:"column:user;type:varchar(255)"`
		ElementId        int64     `gorm:"column:element_id"`
		Status           int       `gorm:"column:status"`
		CommandTrackData string    `gorm:"column:command_track_data;type:longtext"`
	}{
		EventType:     eventType,
		OperationTime: time.Now(),
		User:          user,
		ElementId:     deviceId,
		Status:        1, // pending
	}
	if err := s.repo.db.WithContext(ctx).Table("event_log").Create(&row).Error; err != nil {
		return 0, err
	}
	return row.Id, nil
}

// buildCaOperationMessage builds the TR-069 operation message for CA file download
func (s *Service) buildCaOperationMessage(deviceId int64, fileId int, fileName, filePath, username string, eventLogId int64) map[string]interface{} {
	// Get file server IP from config
	fileServerIp := "http://localhost" // default; should come from config
	if config.Cfg != nil && config.Cfg.TR069.FileServerIp != "" {
		fileServerIp = config.Cfg.TR069.FileServerIp
	}

	// Calculate file size
	fileSize := int64(0)
	if info, err := os.Stat(filePath); err == nil {
		fileSize = info.Size()
	}

	// Build URL for file download
	downloadURL := fmt.Sprintf("%s/api/acs-file-server/ca/downloadFile/%d", fileServerIp, fileId)

	// Build operation param (TR069DownloadDTO equivalent)
	opParam := map[string]interface{}{
		"type":           "CA_FILE",
		"url":            downloadURL,
		"targetFileName": fileName,
		"fileSize":       fileSize,
	}
	opParamJSON, _ := json.Marshal(opParam)

	now := time.Now()
	return map[string]interface{}{
		"eventType":      "UpdateCertificate",
		"neNeid":         deviceId,
		"operation":      "UpdateCertificate",
		"operationParam": string(opParamJSON),
		"operationUser":  username,
		"commandTrackId": eventLogId,
		"expiredAt":      now.Add(5 * time.Minute).UnixMilli(),
	}
}

// getDeviceInfo retrieves device name and serial number from cpe_element table
func (s *Service) getDeviceInfo(ctx context.Context, deviceId int64) (string, string) {
	type DeviceInfo struct {
		DeviceName   *string `gorm:"column:device_name"`
		SerialNumber *string `gorm:"column:serial_number"`
	}
	var info DeviceInfo
	err := s.repo.db.WithContext(ctx).
		Table("cpe_element").
		Where("ne_neid = ?", deviceId).
		Select("device_name, serial_number").
		First(&info).Error
	if err != nil {
		return "", ""
	}
	return strOrEmpty(info.DeviceName), strOrEmpty(info.SerialNumber)
}

// ---------- Utility functions ----------

func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
