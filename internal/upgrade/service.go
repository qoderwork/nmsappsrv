package upgrade

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// Service defines the business-logic contract for upgrade management.
type Service interface {
	ListUpgradeFiles(tenancyId int, page, pageSize int) ([]UpgradeFile, int64, error)
	UploadUpgradeFile(f *UpgradeFile) error
	DeleteUpgradeFile(id int) error
	ListUpgradeTasks(tenancyId int, filter UpgradeTaskFilter, page, pageSize int) ([]UpgradeTaskVo, int64, error)
	GetUpgradeTask(id int) (*UpgradeTask, error)
	CreateUpgradeTask(t *UpgradeTask) error
	ListUpgradeLogs(taskId int, page, pageSize int) ([]UpgradeLog, int64, error)
	CreateRebootTask(t *RebootTask) error
	ListRebootTasks(tenancyId int, page, pageSize int) ([]RebootTask, int64, error)
	CreateRollbackTask(t *RollbackTask) error
	ListRollbackTasks(tenancyId int, page, pageSize int) ([]RollbackTask, int64, error)
	StartUpgradeTask(id int) error
	StartRollbackTask(id int) error

	// Task lifecycle
	CancelUpgradeTask(id int) error
	CancelRollbackTask(id int) error

	// Results
	ListUpgradeResults(taskId int, page, pageSize int) ([]UpgradeResultVo, int64, error)
	ListUpgradeResultDetail(taskId int, page, pageSize int) ([]UpgradeResultDetailVo, int64, error)
	ListRollbackResults(taskId int, page, pageSize int) ([]UpgradeResultVo, int64, error)

	// Statistics
	ListUpgradeTaskStatusCount(tenancyId int) ([]StatusCountItem, error)
	ListUpgradeDeviceResultCount(taskId int) ([]DeviceResultCountItem, error)

	// AutoUpgradeTask CRUD
	ListAutoUpgradeTasks(page, pageSize int) ([]AutoUpgradeTask, int64, error)
	AddAutoUpgradeTask(t *AutoUpgradeTask) error
	ModifyAutoUpgradeTask(t *AutoUpgradeTask) error
	DeleteAutoUpgradeTask(id int64) error

	// File operations
	DownloadUpgradeFile(id int) (*UpgradeFile, error)
	ViewUpgradeFile(id int) (*UpgradeFile, error)
	UpdateUpgradeFile(f *UpgradeFile) error
	UploadUpgradeFileByPiecemeal(req *PiecemealUploadRequest, chunkData []byte, tenancyId int, user string) error
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// newService builds a Service from an injected Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// ---------------------------------------------------------------------------
// UpgradeFile
// ---------------------------------------------------------------------------

// ListUpgradeFiles returns a paginated list of upgrade files.
func (s *service) ListUpgradeFiles(tenancyId int, page, pageSize int) ([]UpgradeFile, int64, error) {
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
func (s *service) UploadUpgradeFile(f *UpgradeFile) error {
	return s.repo.Create(f)
}

// DeleteUpgradeFile removes an upgrade file by ID.
func (s *service) DeleteUpgradeFile(id int) error {
	return s.repo.DeleteByID(id)
}

// ---------------------------------------------------------------------------
// UpgradeTask
// ---------------------------------------------------------------------------

// ListUpgradeTasks returns a paginated list of upgrade task VOs with computed fields.
func (s *service) ListUpgradeTasks(tenancyId int, filter UpgradeTaskFilter, page, pageSize int) ([]UpgradeTaskVo, int64, error) {
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
			if f, err := s.repo.FindByID(*t.UpgradeFileId); err == nil && f.Version != nil {
				vo.Version = *f.Version
			}
		}

		// Compute progress from UpgradeLog counts
		var successCount, failCount int64
		successCount, _ = s.repo.CountUpgradeLogsBySuccess(t.Id, true)
		failCount, _ = s.repo.CountUpgradeLogsBySuccess(t.Id, false)
		vo.SuccessCount = int(successCount)
		vo.FailCount = int(failCount)
		vo.Progress = fmt.Sprintf("%d/%d", successCount+failCount, vo.DeviceCount)

		vos = append(vos, vo)
	}

	return vos, total, nil
}

// GetUpgradeTask returns a single upgrade task by ID.
func (s *service) GetUpgradeTask(id int) (*UpgradeTask, error) {
	return s.repo.FindUpgradeTaskByID(id)
}

// CreateUpgradeTask persists a new upgrade task, creates per-device UpgradeLog
// records, and pushes dispatch messages to the upgrade queue.
func (s *service) CreateUpgradeTask(t *UpgradeTask) error {
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
func (s *service) ListUpgradeLogs(taskId int, page, pageSize int) ([]UpgradeLog, int64, error) {
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
func (s *service) CreateRebootTask(t *RebootTask) error {
	return s.repo.CreateRebootTask(t)
}

// ListRebootTasks returns a paginated list of reboot tasks.
func (s *service) ListRebootTasks(tenancyId int, page, pageSize int) ([]RebootTask, int64, error) {
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
func (s *service) CreateRollbackTask(t *RollbackTask) error {
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
func (s *service) ListRollbackTasks(tenancyId int, page, pageSize int) ([]RollbackTask, int64, error) {
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
func (s *service) StartUpgradeTask(id int) error {
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
func (s *service) StartRollbackTask(id int) error {
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

// ---------------------------------------------------------------------------
// Task lifecycle: Cancel
// ---------------------------------------------------------------------------

// CancelUpgradeTask cancels a waiting or executing upgrade task.
func (s *service) CancelUpgradeTask(id int) error {
	task, err := s.repo.FindUpgradeTaskByID(id)
	if err != nil {
		return err
	}
	status := derefInt(task.Status)
	if status != 1 && status != 2 {
		return fmt.Errorf("task cannot be cancelled in current status")
	}

	ctx := context.Background()
	lockKey := fmt.Sprintf("upgrade_task_cancel_%d", id)
	if !redis.Lock(ctx, lockKey, 30*time.Second) {
		return fmt.Errorf("task is being operated by another request")
	}
	defer redis.Unlock(ctx, lockKey)

	return s.repo.CancelUpgradeTask(id)
}

// CancelRollbackTask cancels a waiting or executing rollback task.
func (s *service) CancelRollbackTask(id int) error {
	task, err := s.repo.FindRollbackTaskByID(id)
	if err != nil {
		return err
	}
	status := derefInt(task.Status)
	if status != 1 && status != 2 {
		return fmt.Errorf("task cannot be cancelled in current status")
	}

	ctx := context.Background()
	lockKey := fmt.Sprintf("rollback_task_cancel_%d", id)
	if !redis.Lock(ctx, lockKey, 30*time.Second) {
		return fmt.Errorf("task is being operated by another request")
	}
	defer redis.Unlock(ctx, lockKey)

	return s.repo.CancelRollbackTask(id)
}

// ---------------------------------------------------------------------------
// Results
// ---------------------------------------------------------------------------

// ListUpgradeResults returns paginated upgrade results for a task.
func (s *service) ListUpgradeResults(taskId int, page, pageSize int) ([]UpgradeResultVo, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	logs, total, err := s.repo.FindUpgradeLogsByTaskId(taskId, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	vos := make([]UpgradeResultVo, 0, len(logs))
	for _, log := range logs {
		vo := UpgradeResultVo{
			TaskId:  derefInt(log.TaskId),
			Success: derefBool(log.Success),
			Message: derefString(log.Message),
		}
		if log.NeId != nil {
			vo.ElementId = *log.NeId
		}
		if log.OldVersion != nil {
			vo.OldVersion = *log.OldVersion
		}
		if log.NewVersion != nil {
			vo.NewVersion = *log.NewVersion
		}
		if log.CreationTime != nil {
			vo.CreationTime = log.CreationTime.Format("2006-01-02 15:04:05")
		}
		if log.DoneTime != nil {
			vo.DoneTime = log.DoneTime.Format("2006-01-02 15:04:05")
		}
		if derefBool(log.IsDone) {
			if derefBool(log.Success) {
				vo.Status = 3 // Success
			} else {
				vo.Status = 4 // Failed
			}
		} else {
			vo.Status = 2 // In progress
		}
		vos = append(vos, vo)
	}
	return vos, total, nil
}

// ListUpgradeResultDetail returns paginated detailed upgrade results for a task.
func (s *service) ListUpgradeResultDetail(taskId int, page, pageSize int) ([]UpgradeResultDetailVo, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	logs, total, err := s.repo.FindUpgradeLogsByTaskIdDetail(taskId, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	vos := make([]UpgradeResultDetailVo, 0, len(logs))
	for _, log := range logs {
		vo := UpgradeResultDetailVo{
			UpgradeResultVo: UpgradeResultVo{
				TaskId:  derefInt(log.TaskId),
				Success: derefBool(log.Success),
				Message: derefString(log.Message),
			},
			IsDownloaded: derefBool(log.IsDownloaded),
			UpgradeType:  derefString(log.UpgradeType),
			RetryTimes:   derefInt(log.RetryTimes),
		}
		if log.NeId != nil {
			vo.ElementId = *log.NeId
		}
		if log.OldVersion != nil {
			vo.OldVersion = *log.OldVersion
		}
		if log.NewVersion != nil {
			vo.NewVersion = *log.NewVersion
		}
		if log.CreationTime != nil {
			vo.CreationTime = log.CreationTime.Format("2006-01-02 15:04:05")
		}
		if log.DoneTime != nil {
			vo.DoneTime = log.DoneTime.Format("2006-01-02 15:04:05")
		}
		if log.DownloadedTime != nil {
			vo.DownloadedTime = log.DownloadedTime.Format("2006-01-02 15:04:05")
		}
		if derefBool(log.IsDone) {
			if derefBool(log.Success) {
				vo.Status = 3
			} else {
				vo.Status = 4
			}
		} else {
			vo.Status = 2
		}
		vos = append(vos, vo)
	}
	return vos, total, nil
}

// ListRollbackResults returns paginated rollback results for a task.
func (s *service) ListRollbackResults(taskId int, page, pageSize int) ([]UpgradeResultVo, int64, error) {
	return s.ListUpgradeResults(taskId, page, pageSize)
}

// ---------------------------------------------------------------------------
// Statistics
// ---------------------------------------------------------------------------

// ListUpgradeTaskStatusCount returns per-status counts of upgrade tasks.
func (s *service) ListUpgradeTaskStatusCount(tenancyId int) ([]StatusCountItem, error) {
	return s.repo.CountUpgradeTaskStatusCounts(tenancyId)
}

// ListUpgradeDeviceResultCount returns per-result counts for a task.
func (s *service) ListUpgradeDeviceResultCount(taskId int) ([]DeviceResultCountItem, error) {
	return s.repo.CountUpgradeDeviceResultCounts(taskId)
}

// ---------------------------------------------------------------------------
// AutoUpgradeTask CRUD
// ---------------------------------------------------------------------------

// ListAutoUpgradeTasks returns a paginated list of auto-upgrade tasks.
func (s *service) ListAutoUpgradeTasks(page, pageSize int) ([]AutoUpgradeTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindAutoUpgradeTasks(offset, pageSize)
}

// AddAutoUpgradeTask persists a new auto-upgrade task.
func (s *service) AddAutoUpgradeTask(t *AutoUpgradeTask) error {
	now := time.Now()
	t.CreateTime = &now
	t.UpdateTime = &now
	return s.repo.CreateAutoUpgradeTask(t)
}

// ModifyAutoUpgradeTask updates an existing auto-upgrade task.
func (s *service) ModifyAutoUpgradeTask(t *AutoUpgradeTask) error {
	existing, err := s.repo.FindAutoUpgradeTaskByID(int64(t.Id))
	if err != nil {
		return err
	}
	t.CreateTime = existing.CreateTime
	now := time.Now()
	t.UpdateTime = &now
	return s.repo.UpdateAutoUpgradeTask(t)
}

// DeleteAutoUpgradeTask removes an auto-upgrade task.
func (s *service) DeleteAutoUpgradeTask(id int64) error {
	return s.repo.DeleteAutoUpgradeTask(id)
}

// ---------------------------------------------------------------------------
// File operations
// ---------------------------------------------------------------------------

// DownloadUpgradeFile returns the upgrade file record for download.
func (s *service) DownloadUpgradeFile(id int) (*UpgradeFile, error) {
	return s.repo.FindUpgradeFileByID(id)
}

// ViewUpgradeFile returns the upgrade file record for viewing.
func (s *service) ViewUpgradeFile(id int) (*UpgradeFile, error) {
	return s.repo.FindUpgradeFileByID(id)
}

// UpdateUpgradeFile persists changes to an existing upgrade file.
func (s *service) UpdateUpgradeFile(f *UpgradeFile) error {
	return s.repo.UpdateUpgradeFile(f)
}

// UploadUpgradeFileByPiecemeal handles chunked file upload.
// Each chunk is cached in Redis; once every chunk has arrived the chunks are
// reassembled, persisted to local storage, and the absolute path is recorded so
// the worker can hand a device-reachable URL to TR-069 Download.
func (s *service) UploadUpgradeFileByPiecemeal(req *PiecemealUploadRequest, chunkData []byte, tenancyId int, user string) error {
	ctx := context.Background()

	// Store chunk in Redis with a key based on uploadId and chunk index
	chunkKey := fmt.Sprintf("upgrade_chunk:%s:%d", req.UploadId, req.ChunkIndex)
	if err := redis.Set(ctx, chunkKey, chunkData, 2*time.Hour); err != nil {
		return fmt.Errorf("failed to store chunk: %w", err)
	}

	// Track uploaded chunks
	metaKey := fmt.Sprintf("upgrade_chunk_meta:%s", req.UploadId)
	if err := redis.HSet(ctx, metaKey, fmt.Sprintf("chunk_%d", req.ChunkIndex), "1"); err != nil {
		return fmt.Errorf("failed to track chunk: %w", err)
	}
	redis.Expire(ctx, metaKey, 2*time.Hour)

	// Check if all chunks are uploaded
	chunks := redis.HGetAll(ctx, metaKey)
	if len(chunks) >= req.TotalChunks {
		// All chunks received - assemble and persist to local storage.
		var buf bytes.Buffer
		for i := 0; i < req.TotalChunks; i++ {
			ck := fmt.Sprintf("upgrade_chunk:%s:%d", req.UploadId, i)
			data, err := redis.Get(ctx, ck)
			if err != nil {
				return fmt.Errorf("missing chunk %d: %w", i, err)
			}
			if _, err := buf.Write([]byte(data)); err != nil {
				return fmt.Errorf("failed to assemble chunk %d: %w", i, err)
			}
		}

		storedPath, err := saveUpgradeFile(tenancyId, req.FileName, buf.Bytes())
		if err != nil {
			return fmt.Errorf("failed to persist assembled file: %w", err)
		}

		now := time.Now()
		file := &UpgradeFile{
			FileName:         &req.FileName,
			FilePath:         &storedPath,
			Version:          &req.Version,
			DeviceType:       &req.DeviceType,
			FileSize:         &req.TotalSize,
			UploadTime:       &now,
			ProductType:      &req.ProductType,
			OriginalFileName: &req.FileName,
			TenancyId:        &tenancyId,
			User:             &user,
		}
		if err := s.repo.Create(file); err != nil {
			return fmt.Errorf("failed to create upgrade file record: %w", err)
		}

		// Clean up chunk keys
		for i := 0; i < req.TotalChunks; i++ {
			redis.Del(ctx, fmt.Sprintf("upgrade_chunk:%s:%d", req.UploadId, i))
		}
		redis.Del(ctx, metaKey)

		logger.Infof("piecemeal upload completed for file %s (id=%d)", req.FileName, file.Id)
	}

	return nil
}

// derefBool safely dereferences a *bool, returning false if nil.
func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
