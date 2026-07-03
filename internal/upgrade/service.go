package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// Service contains the business logic for upgrade management.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ---------------------------------------------------------------------------
// UpgradeFile
// ---------------------------------------------------------------------------

// ListUpgradeFiles returns a paginated list of upgrade files.
func (s *Service) ListUpgradeFiles(tenancyId int, page, pageSize int) ([]UpgradeFile, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindUpgradeFiles(tenancyId, offset, pageSize)
}

// UploadUpgradeFile persists a new upgrade file record.
func (s *Service) UploadUpgradeFile(f *UpgradeFile) error {
	return s.repo.CreateUpgradeFile(f)
}

// DeleteUpgradeFile removes an upgrade file by ID.
func (s *Service) DeleteUpgradeFile(id int) error {
	return s.repo.DeleteUpgradeFile(id)
}

// ---------------------------------------------------------------------------
// UpgradeTask
// ---------------------------------------------------------------------------

// ListUpgradeTasks returns a paginated list of upgrade task VOs with computed fields.
func (s *Service) ListUpgradeTasks(tenancyId int, filter UpgradeTaskFilter, page, pageSize int) ([]UpgradeTaskVo, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	tasks, total, err := s.repo.FindUpgradeTasks(tenancyId, filter, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	// Convert to VOs with computed fields
	vos := make([]UpgradeTaskVo, 0, len(tasks))
	for _, t := range tasks {
		vo := UpgradeTaskVo{
			Id:          t.Id,
			Status:      t.Status,
			ExecuteMode: t.ExecuteMode,
		}
		if t.Name != nil {
			vo.Name = *t.Name
		}
		if t.User != nil {
			vo.User = *t.User
		}
		if t.OperationTime != nil {
			vo.OperationTime = t.OperationTime.Format("2006-01-02 15:04:05")
		}
		if t.StartTime != nil {
			vo.StartTime = t.StartTime.Format("2006-01-02 15:04:05")
		}
		if t.EndTime != nil {
			vo.EndTime = t.EndTime.Format("2006-01-02 15:04:05")
		}
		if t.DeviceType != nil {
			vo.DeviceType = *t.DeviceType
		}
		if t.UpgradeType != nil {
			vo.UpgradeType = *t.UpgradeType
		}

		// Compute device count from elementIds
		elementIds := ParseElementIds(derefString(t.ElementIds))
		vo.DeviceCount = len(elementIds)

		// Lookup version from UpgradeFile
		if t.UpgradeFileId != nil && *t.UpgradeFileId > 0 {
			if f, err := s.repo.FindUpgradeFileByID(*t.UpgradeFileId); err == nil && f.Version != nil {
				vo.Version = *f.Version
			}
		}

		// Compute progress from UpgradeLog counts
		var successCount, failCount int64
		s.repo.db.Model(&UpgradeLog{}).Where("task_id = ? AND success = ?", t.Id, true).Count(&successCount)
		s.repo.db.Model(&UpgradeLog{}).Where("task_id = ? AND success = ?", t.Id, false).Count(&failCount)
		vo.SuccessCount = int(successCount)
		vo.FailCount = int(failCount)
		vo.Progress = fmt.Sprintf("%d/%d", successCount+failCount, vo.DeviceCount)

		vos = append(vos, vo)
	}

	return vos, total, nil
}

// GetUpgradeTask returns a single upgrade task by ID.
func (s *Service) GetUpgradeTask(id int) (*UpgradeTask, error) {
	return s.repo.FindUpgradeTaskByID(id)
}

// CreateUpgradeTask persists a new upgrade task, creates per-device UpgradeLog
// records, and pushes dispatch messages to the upgrade queue.
func (s *Service) CreateUpgradeTask(t *UpgradeTask) error {
	if err := s.repo.CreateUpgradeTask(t); err != nil {
		return err
	}

	// Parse element IDs
	elementIds := ParseElementIds(derefString(t.ElementIds))
	if len(elementIds) == 0 {
		logger.Warnf("CreateUpgradeTask: task %d has no element IDs", t.Id)
		return nil
	}

	// Set status to executing if immediate mode
	now := time.Now()
	if derefInt(t.ExecuteMode) == 1 {
		status := 2 // Executing
		t.Status = &status
		t.StartTime = &now
		s.repo.UpdateUpgradeTask(t)
	} else {
		status := 1 // Waiting
		t.Status = &status
		s.repo.UpdateUpgradeTask(t)
	}

	// Only dispatch for immediate execution
	if derefInt(t.ExecuteMode) != 1 {
		return nil
	}

	upgradeFileId := derefInt(t.UpgradeFileId)
	concurrentNumber := derefInt(t.ConcurrentNumber)
	if concurrentNumber < 1 {
		concurrentNumber = 1
	}

	ctx := context.Background()
	sem := make(chan struct{}, concurrentNumber)

	for _, eid := range elementIds {
		sem <- struct{}{} // acquire concurrency slot

		// Create UpgradeLog for each device
		logUuid := uuid.New().String()
		logUuid = replaceHyphens(logUuid)

		isUpgrade := true
		creationTime := now
		logEntry := &UpgradeLog{
			Id:           logUuid,
			NeId:         int64Ptr(eid),
			CreationTime: &creationTime,
			TaskId:       intPtr(t.Id),
			Upgrade:      &isUpgrade,
			IsDone:       boolPtrVal(false),
			IsDownloaded: boolPtrVal(false),
			Success:      boolPtrVal(false),
			RetryTimes:   intPtr(0),
			TenancyId:    t.TenancyId,
			DeviceType:   t.DeviceType,
			UpgradeType:  t.UpgradeType,
		}
		if err := s.repo.CreateUpgradeLog(logEntry); err != nil {
			logger.Errorf("CreateUpgradeTask: create log for element %d: %v", eid, err)
			<-sem
			continue
		}

		// Push message to upgrade queue
		msg := UpgradeMessage{
			TaskId:        t.Id,
			ElementId:     eid,
			UpgradeFileId: upgradeFileId,
			OperationType: "UPGRADE",
			LogUuid:       logUuid,
		}
		msgJSON, err := json.Marshal(msg)
		if err != nil {
			logger.Errorf("CreateUpgradeTask: marshal message for element %d: %v", eid, err)
			<-sem
			continue
		}

		if err := redis.LPush(ctx, "queue:upgrade", string(msgJSON)); err != nil {
			logger.Errorf("CreateUpgradeTask: push to queue for element %d: %v", eid, err)
			<-sem
			continue
		}

		<-sem // release concurrency slot
	}

	return nil
}

// ---------------------------------------------------------------------------
// UpgradeLog
// ---------------------------------------------------------------------------

// ListUpgradeLogs returns a paginated list of upgrade logs for the given task.
func (s *Service) ListUpgradeLogs(taskId int, page, pageSize int) ([]UpgradeLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindUpgradeLogs(taskId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// RebootTask
// ---------------------------------------------------------------------------

// CreateRebootTask persists a new reboot task.
func (s *Service) CreateRebootTask(t *RebootTask) error {
	return s.repo.CreateRebootTask(t)
}

// ListRebootTasks returns a paginated list of reboot tasks.
func (s *Service) ListRebootTasks(tenancyId int, page, pageSize int) ([]RebootTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindRebootTasks(tenancyId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// RollbackTask
// ---------------------------------------------------------------------------

// CreateRollbackTask persists a new rollback task, creates per-device UpgradeLog
// records, and pushes dispatch messages to the upgrade queue.
func (s *Service) CreateRollbackTask(t *RollbackTask) error {
	if err := s.repo.CreateRollbackTask(t); err != nil {
		return err
	}

	// Parse element IDs
	elementIds := ParseElementIds(derefString(t.ElementIds))
	if len(elementIds) == 0 {
		logger.Warnf("CreateRollbackTask: task %d has no element IDs", t.Id)
		return nil
	}

	// Set status
	now := time.Now()
	if derefInt(t.ExecuteMode) == 1 {
		status := 2 // Executing
		t.Status = &status
		t.StartTime = &now
	} else {
		status := 1 // Waiting
		t.Status = &status
	}
	s.repo.UpdateRollbackTask(t)

	// Only dispatch for immediate execution
	if derefInt(t.ExecuteMode) != 1 {
		return nil
	}

	ctx := context.Background()

	for _, eid := range elementIds {
		// Create UpgradeLog for each device (rollback uses same log table)
		logUuid := uuid.New().String()
		logUuid = replaceHyphens(logUuid)

		isUpgrade := false
		creationTime := now
		logEntry := &UpgradeLog{
			Id:           logUuid,
			NeId:         int64Ptr(eid),
			CreationTime: &creationTime,
			TaskId:       intPtr(t.Id),
			Upgrade:      &isUpgrade,
			IsDone:       boolPtrVal(false),
			IsDownloaded: boolPtrVal(false),
			Success:      boolPtrVal(false),
			RetryTimes:   intPtr(0),
			TenancyId:    t.TenancyId,
		}
		if err := s.repo.CreateUpgradeLog(logEntry); err != nil {
			logger.Errorf("CreateRollbackTask: create log for element %d: %v", eid, err)
			continue
		}

		// Push message to upgrade queue
		// For rollback, upgrade_file_id is 0 (will be resolved from task context)
		msg := UpgradeMessage{
			TaskId:        t.Id,
			ElementId:     eid,
			UpgradeFileId: 0,
			OperationType: "ROLLBACK",
			LogUuid:       logUuid,
		}
		msgJSON, err := json.Marshal(msg)
		if err != nil {
			logger.Errorf("CreateRollbackTask: marshal message for element %d: %v", eid, err)
			continue
		}

		if err := redis.LPush(ctx, "queue:upgrade", string(msgJSON)); err != nil {
			logger.Errorf("CreateRollbackTask: push to queue for element %d: %v", eid, err)
		}
	}

	return nil
}

// ListRollbackTasks returns a paginated list of rollback tasks.
func (s *Service) ListRollbackTasks(tenancyId int, page, pageSize int) ([]RollbackTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindRollbackTasks(tenancyId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func int64Ptr(i int64) *int64 {
	return &i
}

func boolPtrVal(b bool) *bool {
	return &b
}

// replaceHyphens removes hyphens from a UUID to get a 32-char string.
func replaceHyphens(s string) string {
	result := make([]byte, 0, 32)
	for i := 0; i < len(s); i++ {
		if s[i] != '-' {
			result = append(result, s[i])
		}
	}
	return string(result)
}

// StartUpgradeTask manually starts a waiting upgrade task.
func (s *Service) StartUpgradeTask(id int) error {
	task, err := s.repo.FindUpgradeTaskByID(id)
	if err != nil {
		return err
	}
	if derefInt(task.Status) != 1 {
		return fmt.Errorf("task already started or completed")
	}

	ctx := context.Background()
	lockKey := fmt.Sprintf("upgrade_task_start_%d", id)
	if !redis.Lock(ctx, lockKey, 60*time.Second) {
		return fmt.Errorf("task is being started by another request")
	}
	defer redis.Unlock(ctx, lockKey)

	now := time.Now()
	status := 2
	task.Status = &status
	task.StartTime = &now
	s.repo.UpdateUpgradeTask(task)

	// Dispatch
	elementIds := ParseElementIds(derefString(task.ElementIds))
	upgradeFileId := derefInt(task.UpgradeFileId)
	concurrentNumber := derefInt(task.ConcurrentNumber)
	if concurrentNumber < 1 {
		concurrentNumber = 1
	}

	sem := make(chan struct{}, concurrentNumber)
	for _, eid := range elementIds {
		sem <- struct{}{}

		logUuid := replaceHyphens(uuid.New().String())
		isUpgrade := true
		logEntry := &UpgradeLog{
			Id:           logUuid,
			NeId:         int64Ptr(eid),
			CreationTime: &now,
			TaskId:       intPtr(task.Id),
			Upgrade:      &isUpgrade,
			IsDone:       boolPtrVal(false),
			IsDownloaded: boolPtrVal(false),
			Success:      boolPtrVal(false),
			RetryTimes:   intPtr(0),
			TenancyId:    task.TenancyId,
			DeviceType:   task.DeviceType,
			UpgradeType:  task.UpgradeType,
		}
		if err := s.repo.CreateUpgradeLog(logEntry); err != nil {
			logger.Errorf("StartUpgradeTask: create log for element %d: %v", eid, err)
			<-sem
			continue
		}

		msg := UpgradeMessage{
			TaskId:        task.Id,
			ElementId:     eid,
			UpgradeFileId: upgradeFileId,
			OperationType: "UPGRADE",
			LogUuid:       logUuid,
		}
		msgJSON, _ := json.Marshal(msg)
		if err := redis.LPush(ctx, "queue:upgrade", string(msgJSON)); err != nil {
			logger.Errorf("StartUpgradeTask: push to queue for element %d: %v", eid, err)
		}

		<-sem
	}

	return nil
}

// StartRollbackTask manually starts a waiting rollback task.
func (s *Service) StartRollbackTask(id int) error {
	task, err := s.repo.FindRollbackTaskByID(id)
	if err != nil {
		return err
	}
	if derefInt(task.Status) != 1 {
		return fmt.Errorf("task already started or completed")
	}

	ctx := context.Background()
	lockKey := fmt.Sprintf("rollback_task_start_%d", id)
	if !redis.Lock(ctx, lockKey, 60*time.Second) {
		return fmt.Errorf("task is being started by another request")
	}
	defer redis.Unlock(ctx, lockKey)

	now := time.Now()
	status := 2
	task.Status = &status
	task.StartTime = &now
	s.repo.UpdateRollbackTask(task)

	elementIds := ParseElementIds(derefString(task.ElementIds))
	for _, eid := range elementIds {
		logUuid := replaceHyphens(uuid.New().String())
		isUpgrade := false
		logEntry := &UpgradeLog{
			Id:           logUuid,
			NeId:         int64Ptr(eid),
			CreationTime: &now,
			TaskId:       intPtr(task.Id),
			Upgrade:      &isUpgrade,
			IsDone:       boolPtrVal(false),
			IsDownloaded: boolPtrVal(false),
			Success:      boolPtrVal(false),
			RetryTimes:   intPtr(0),
			TenancyId:    task.TenancyId,
		}
		if err := s.repo.CreateUpgradeLog(logEntry); err != nil {
			logger.Errorf("StartRollbackTask: create log for element %d: %v", eid, err)
			continue
		}

		msg := UpgradeMessage{
			TaskId:        task.Id,
			ElementId:     eid,
			UpgradeFileId: 0,
			OperationType: "ROLLBACK",
			LogUuid:       logUuid,
		}
		msgJSON, _ := json.Marshal(msg)
		if err := redis.LPush(ctx, "queue:upgrade", string(msgJSON)); err != nil {
			logger.Errorf("StartRollbackTask: push to queue for element %d: %v", eid, err)
		}
	}

	return nil
}
