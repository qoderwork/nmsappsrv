package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"nmsappsrv/internal/device"
	"nmsappsrv/internal/mq"
	"nmsappsrv/internal/opmsg"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// ShutdownService provides business logic for shutdown management.
type ShutdownService struct {
	repo *ShutdownRepository
}

// NewShutdownService creates a new ShutdownService.
func NewShutdownService(db *gorm.DB) *ShutdownService {
	return &ShutdownService{
		repo: NewShutdownRepository(db),
	}
}

// CreateShutdownTask creates a new shutdown task and dispatches commands if immediate.
func (s *ShutdownService) CreateShutdownTask(req *AddShutdownTaskRequest, username string, tenantId int) (int, error) {
	elementIdsJSON, err := json.Marshal(req.ElementIds)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal element IDs: %w", err)
	}

	now := time.Now()
	task := ShutdownMyTask{
		Name:          &req.Name,
		User:          &username,
		OperationTime: &now,
		Status:        intPtr(1), // Waiting
		ExecuteMode:   &req.ExecuteMode,
		TenantId:     &tenantId,
		ElementIds:    strPtr(string(elementIdsJSON)),
	}

	// Handle trigger time for scheduled mode
	if req.ExecuteMode == 3 && req.TriggerTime != "" {
		triggerTime, err := time.Parse(time.RFC3339, req.TriggerTime)
		if err != nil {
			return 0, fmt.Errorf("invalid trigger time format: %w", err)
		}
		task.TriggerTime = &triggerTime
	}

	// Handle awaiting start mode
	if req.ExecuteMode == 2 {
		task.StartTime = &now
	}

	if err := s.repo.db.Create(&task).Error; err != nil {
		return 0, fmt.Errorf("failed to create shutdown task: %w", err)
	}

	// If immediate execution, dispatch shutdown commands
	if req.ExecuteMode == 1 {
		task.Status = intPtr(2) // Executing
		s.repo.db.Save(&task)

		for _, elementId := range req.ElementIds {
			log := ShutdownLog{
				TaskId:    &task.Id,
				ElementId: &elementId,
				Status:    intPtr(1), // Pending
				Time:      &now,
			}
			s.repo.db.Create(&log)

			msg := opmsg.Message{
				EventType:      "Shutdown", // Java EventType.SHUTDOWN
				NeNeid:         elementId,
				Operation:      "Shutdown",
				OperationUser:  username,
				CommandTrackId: log.Id,
				ExpiredAt:      time.Now().Add(30 * time.Minute).UnixMilli(),
			}
			msgBytes, _ := msg.Marshal()
			if err := redis.LPush(context.Background(), mq.OperationQueue, string(msgBytes)); err != nil {
				logger.Errorf("Failed to push shutdown command to Redis: %v", err)
			}
		}
	}

	return task.Id, nil
}

// ListShutdownTasks returns a paginated list of shutdown tasks for a tenancy.
func (s *ShutdownService) ListShutdownTasks(page, pageSize, tenantId int) ([]ShutdownTaskVo, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	var tasks []ShutdownMyTask
	var total int64

	query := s.repo.db.Where("tenant_id = ?", tenantId)
	query.Model(&ShutdownMyTask{}).Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to query shutdown tasks: %w", err)
	}

	var result []ShutdownTaskVo
	for _, task := range tasks {
		vo := ShutdownTaskVo{
			Id:            task.Id,
			Name:          derefString(task.Name),
			OperationUser: derefString(task.User),
			OperationTime: formatTime(task.OperationTime),
			Status:        derefInt(task.Status),
			ExecuteMode:   derefInt(task.ExecuteMode),
		}

		// Compute device count and progress from shutdown logs
		var totalLogs int64
		var successLogs int64
		s.repo.db.Model(&ShutdownLog{}).Where("task_id = ?", task.Id).Count(&totalLogs)
		s.repo.db.Model(&ShutdownLog{}).Where("task_id = ? AND status = ?", task.Id, 3).Count(&successLogs)

		vo.DeviceCount = int(totalLogs)
		vo.Progress = fmt.Sprintf("%d/%d", successLogs, totalLogs)

		result = append(result, vo)
	}

	return result, total, nil
}

// ViewShutdownTask returns the detail of a shutdown task with device information.
func (s *ShutdownService) ViewShutdownTask(taskId int) (*ViewShutdownTaskVo, error) {
	var task ShutdownMyTask
	if err := s.repo.db.Where("id = ?", taskId).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to query task: %w", err)
	}

	vo := &ViewShutdownTaskVo{
		Id:            task.Id,
		Name:          derefString(task.Name),
		OperationUser: derefString(task.User),
		OperationTime: formatTime(task.OperationTime),
		Status:        derefInt(task.Status),
		ExecuteMode:   derefInt(task.ExecuteMode),
		TriggerTime:   formatTime(task.TriggerTime),
	}

	// Parse element IDs from JSON
	var elementIds []int64
	if task.ElementIds != nil && *task.ElementIds != "" {
		if err := json.Unmarshal([]byte(*task.ElementIds), &elementIds); err != nil {
			return nil, fmt.Errorf("failed to parse element IDs: %w", err)
		}
	}

	// Query device information
	if len(elementIds) > 0 {
		var devices []ShutdownDeviceVo
		var elements []device.CpeElement
		if err := s.repo.db.Where("ne_neid IN ?", elementIds).Find(&elements).Error; err != nil {
			return nil, fmt.Errorf("failed to query devices: %w", err)
		}

		for _, elem := range elements {
			devices = append(devices, ShutdownDeviceVo{
				ElementId:    elem.NeNeid,
				DeviceName:   derefString(elem.DeviceName),
				SerialNumber: derefString(elem.SerialNumber),
			})
		}
		vo.Devices = devices
	}

	return vo, nil
}

// DeleteShutdownTask deletes a shutdown task and its associated logs.
func (s *ShutdownService) DeleteShutdownTask(taskId int) error {
	return s.repo.db.Transaction(func(tx *gorm.DB) error {
		// Delete associated logs
		if err := tx.Where("task_id = ?", taskId).Delete(&ShutdownLog{}).Error; err != nil {
			return fmt.Errorf("failed to delete shutdown logs: %w", err)
		}

		// Delete task
		if err := tx.Where("id = ?", taskId).Delete(&ShutdownMyTask{}).Error; err != nil {
			return fmt.Errorf("failed to delete shutdown task: %w", err)
		}

		return nil
	})
}

// ListShutdownResults returns the per-device shutdown results for a task.
func (s *ShutdownService) ListShutdownResults(taskId, page, pageSize int) ([]ShutdownResultVo, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	var logs []ShutdownLog
	var total int64

	query := s.repo.db.Where("task_id = ?", taskId)
	query.Model(&ShutdownLog{}).Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to query shutdown logs: %w", err)
	}

	var result []ShutdownResultVo
	for _, log := range logs {
		vo := ShutdownResultVo{
			ElementId: derefInt64(log.ElementId),
			Status:    derefInt(log.Status),
			Time:      formatTime(log.Time),
		}

		// Query device information
		if log.ElementId != nil {
			var element device.CpeElement
			if err := s.repo.db.Where("ne_neid = ?", *log.ElementId).First(&element).Error; err == nil {
				vo.DeviceName = derefString(element.DeviceName)
				vo.SerialNumber = derefString(element.SerialNumber)
			}
		}

		result = append(result, vo)
	}

	return result, total, nil
}

// ShutdownRepository provides database operations for shutdown management.
type ShutdownRepository struct {
	db *gorm.DB
}

// NewShutdownRepository creates a new ShutdownRepository.
func NewShutdownRepository(db *gorm.DB) *ShutdownRepository {
	return &ShutdownRepository{db: db}
}

// Helper functions

func intPtr(i int) *int {
	return &i
}

func strPtr(s string) *string {
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func derefInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
