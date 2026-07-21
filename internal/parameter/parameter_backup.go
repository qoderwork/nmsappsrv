package parameter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// ---------------------------------------------------------------------------
// TriggerBackup
// ---------------------------------------------------------------------------

// TriggerBackup triggers a parameter backup for the given device by sending
// GPV for all basic parameter paths. When the GPV response comes back, the
// normal processGetParameterValuesResponse saves the values.
func (s *service) TriggerBackup(elementId int64, username string) error {
	// 1. Resolve device SN and type
	var deviceInfo struct {
		SerialNumber string `gorm:"column:serial_number"`
		DeviceType   string `gorm:"column:device_type"`
	}
	if err := s.repo.DB().Table("cpe_element").
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
	s.repo.DB().Table("event_log").Where("id = ?", eventLogId).
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
		s.repo.DB().Table("event_log").Where("id = ?", eventLogId).Update("status", 4)
		return fmt.Errorf("push to device queue: %w", err)
	}
	redis.Expire(ctx, queueKey, 24*time.Hour)

	logger.Infof("TriggerBackup: GPV dispatched to device %s (elementId=%d) for %d params, taskId=%s",
		deviceInfo.SerialNumber, elementId, len(paramPaths), taskId)
	return nil
}

// ---------------------------------------------------------------------------
// ParameterBackupLog
// ---------------------------------------------------------------------------

// ListBackupLogs returns all backup logs for the given element.
func (s *service) ListBackupLogs(elementId int64) ([]ParameterBackupLog, error) {
	return s.repo.FindParameterBackupLogs(elementId)
}

// ListBackupLogsWithPage returns paginated backup logs with optional filtering.
func (s *service) ListBackupLogsWithPage(req *ListParameterBackupLogsRequest) ([]ParameterBackupLogVo, int64, error) {
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	logs, total, err := s.repo.FindParameterBackupLogsWithPage(req.ElementId, req.Keyword, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]ParameterBackupLogVo, 0, len(logs))
	for _, log := range logs {
		vo := ParameterBackupLogVo{
			Id:           log.Id,
			GenerateTime: derefInt64(log.GenerateTime),
		}
		if log.TaskId != nil {
			vo.TaskId = *log.TaskId
		}
		if log.ElementId != nil {
			vo.ElementId = *log.ElementId
		}
		if log.Filename != nil {
			vo.Filename = *log.Filename
		}
		result = append(result, vo)
	}
	return result, total, nil
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
