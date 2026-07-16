package misc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/internal/mq"
	"nmsappsrv/internal/opmsg"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// ---------------------------------------------------------------------------
// BackupOrRestoreTask
// ---------------------------------------------------------------------------

// ListBackupRestoreTasks returns a paginated list of backup/restore tasks.
func (s *service) ListBackupRestoreTasks(tenancyId int, page, pageSize int) ([]BackupOrRestoreTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindBackupOrRestoreTasks(tenancyId, offset, pageSize)
}

// CreateBackupRestoreTask persists a new backup/restore task.
func (s *service) CreateBackupRestoreTask(t *BackupOrRestoreTask) error {
	return s.repo.CreateBackupOrRestoreTask(t)
}

// ---------------------------------------------------------------------------
// BatchAddObject
// ---------------------------------------------------------------------------

// operationMessage is the JSON payload pushed to Redis operation_queue.
type operationMessage struct {
	EventType      string `json:"eventType"`
	NeNeid         int64  `json:"neNeid"`
	Operation      string `json:"operation"`
	OperationParam string `json:"operationParam"`
	OperationUser  string `json:"operationUser"`
	CommandTrackId int64  `json:"commandTrackId"`
	ExpiredAt      int64  `json:"expiredAt"` // unix milliseconds
}

// BatchAddObject creates a batch-add-object task and dispatches AddObject
// commands for each device to the Redis operation queue.
func (s *service) BatchAddObject(req *BatchAddObjectRequest, username string, tenancyId int) error {
	if len(req.Ids) == 0 {
		return fmt.Errorf("device ids must not be empty")
	}
	objPath := BuildTR069ObjectPath(req.Type, req.AmfNumber, req.SliceNumber, req.TaNumber, req.PlmnNumber)
	if objPath == "" {
		return fmt.Errorf("unknown object type: %s", req.Type)
	}

	// 1. Create the task record.
	now := time.Now()
	task := &BatchAddObjectTask{
		User:      &username,
		Time:      &now,
		TenancyId: &tenancyId,
	}
	if err := s.repo.CreateBatchAddObjectTask(task); err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	// 2. For each device: check blacklist → create EventLog → push to Redis → save TaskLog.
	expiredAt := now.Add(5 * time.Minute).UnixMilli()
	ctx := context.Background()

	for _, elementId := range req.Ids {
		// Blacklist check via raw table query (avoid importing device package).
		var blCount int64
		s.repo.DB().Raw(`
			SELECT COUNT(*) FROM element_black_list
			WHERE serial_number = (SELECT serial_number FROM cpe_element WHERE ne_neid = ?)
		`, elementId).Count(&blCount)
		if blCount > 0 {
			logger.Warnf("device %d is blacklisted, skipping", elementId)
			continue
		}

		// Create EventLog (status=1 means pending).
		eventLogId, err := s.repo.InsertEventLog("AddObject", elementId, username, 1, objPath)
		if err != nil {
			logger.Errorf("create event_log for device %d: %v", elementId, err)
			continue
		}

		// Build and push operation message to Redis.
		msg := opmsg.Message{
			EventType:      "AddObject", // Java EventType.ADD_OBJECT
			NeNeid:         elementId,
			Operation:      "AddObject",
			OperationParam: objPath,
			OperationUser:  username,
			CommandTrackId: eventLogId,
			ExpiredAt:      expiredAt,
		}
		msgJSON, _ := msg.Marshal()
		if err := redis.LPush(ctx, mq.OperationQueue, string(msgJSON)); err != nil {
			logger.Errorf("push to redis queue for device %d: %v", elementId, err)
		}

		// Link task → event_log.
		taskLog := &BatchAddObjectTaskLog{
			TaskId:     &task.Id,
			EventLogId: &eventLogId,
		}
		if err := s.repo.CreateBatchAddObjectTaskLog(taskLog); err != nil {
			logger.Errorf("create task_log for device %d: %v", elementId, err)
		}
	}

	return nil
}

// ListBatchAddObjectTasks returns the paginated task list with progress info.
func (s *service) ListBatchAddObjectTasks(tenancyId int, page, pageSize int) ([]BatchAddObjectTaskVo, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	tasks, total, err := s.repo.FindBatchAddObjectTasks(tenancyId, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []BatchAddObjectTaskVo
	for _, t := range tasks {
		vo := BatchAddObjectTaskVo{
			Id:            t.Id,
			OperationUser: ptrStr(t.User),
			OperationTime: ptrTime(t.Time),
		}
		totalCnt, successCnt, pErr := s.repo.BatchAddObjectTaskProgress(t.Id)
		if pErr == nil {
			vo.Progress = fmt.Sprintf("%d/%d", successCnt, totalCnt)
		}
		vos = append(vos, vo)
	}
	return vos, total, nil
}

// ListBatchAddObjectTaskDetail returns per-device results for a given task.
func (s *service) ListBatchAddObjectTaskDetail(taskId int) ([]BatchAddObjectTaskDetailVo, error) {
	return s.repo.BatchAddObjectTaskDetail(taskId)
}

// ---------------------------------------------------------------------------
// Batch Backup / Restore
// ---------------------------------------------------------------------------

// CreateBackupTask creates a batch backup task and dispatches commands for
// immediate execution (mode 1) or saves it for later trigger (mode 2/3).
func (s *service) CreateBackupTask(req *BackupRestoreRequest, username string, tenancyId int) error {
	if req.Name == "" {
		return fmt.Errorf("task name is required")
	}
	if req.ExecuteMode < 1 || req.ExecuteMode > 3 {
		return fmt.Errorf("invalid execute mode: %d", req.ExecuteMode)
	}
	if s.repo.CheckDuplicateBackupRestoreTaskName(req.Name, tenancyId) {
		return fmt.Errorf("task name already exists: %s", req.Name)
	}

	now := time.Now()
	taskType := "backup"
	status := 1 // waiting
	if req.ExecuteMode == 1 {
		status = 2 // executing
	}

	elementIdsJSON, _ := json.Marshal(req.ElementIds)
	groupIdsJSON, _ := json.Marshal(req.DeviceGroupIds)

	task := &BackupOrRestoreTask{
		Name:               &req.Name,
		User:               &username,
		OperationTime:      &now,
		Status:             &status,
		ExecuteMode:        &req.ExecuteMode,
		TenancyId:          &tenancyId,
		TaskType:           &taskType,
		ExecuteOnAllDevice: &req.ExecuteOnAllDevice,
		ElementIds:         strPtr(string(elementIdsJSON)),
		Scope:              strPtr(req.Scope),
		DeviceGroupIds:     strPtr(string(groupIdsJSON)),
	}
	if req.TriggerTime != nil {
		if tt, err := time.Parse(time.RFC3339, *req.TriggerTime); err == nil {
			task.TriggerTime = &tt
		}
	}
	if status == 2 {
		task.StartTime = &now
	}

	if err := s.repo.CreateBackupOrRestoreTask(task); err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	// Mode 1: dispatch immediately.
	if req.ExecuteMode == 1 {
		s.dispatchBackupRestore(task, username, "Backup")
	}

	return nil
}

// CreateRestoreTask creates a batch restore task and dispatches commands for
// immediate execution (mode 1) or saves it for later trigger (mode 2/3).
func (s *service) CreateRestoreTask(req *BackupRestoreRequest, username string, tenancyId int) error {
	if req.Name == "" {
		return fmt.Errorf("task name is required")
	}
	if req.ExecuteMode < 1 || req.ExecuteMode > 3 {
		return fmt.Errorf("invalid execute mode: %d", req.ExecuteMode)
	}
	if s.repo.CheckDuplicateBackupRestoreTaskName(req.Name, tenancyId) {
		return fmt.Errorf("task name already exists: %s", req.Name)
	}

	now := time.Now()
	taskType := "restore"
	status := 1
	if req.ExecuteMode == 1 {
		status = 2
	}

	elementIdsJSON, _ := json.Marshal(req.ElementIds)
	groupIdsJSON, _ := json.Marshal(req.DeviceGroupIds)

	task := &BackupOrRestoreTask{
		Name:               &req.Name,
		User:               &username,
		OperationTime:      &now,
		Status:             &status,
		ExecuteMode:        &req.ExecuteMode,
		TenancyId:          &tenancyId,
		TaskType:           &taskType,
		ExecuteOnAllDevice: &req.ExecuteOnAllDevice,
		ElementIds:         strPtr(string(elementIdsJSON)),
		Scope:              strPtr(req.Scope),
		DeviceGroupIds:     strPtr(string(groupIdsJSON)),
	}
	if req.TriggerTime != nil {
		if tt, err := time.Parse(time.RFC3339, *req.TriggerTime); err == nil {
			task.TriggerTime = &tt
		}
	}
	if status == 2 {
		task.StartTime = &now
	}

	if err := s.repo.CreateBackupOrRestoreTask(task); err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	if req.ExecuteMode == 1 {
		s.dispatchBackupRestore(task, username, "Restore")
	}

	return nil
}

// StartBackupRestoreTask manually triggers a pending task (executeMode=2).
func (s *service) StartBackupRestoreTask(taskId int, username string) error {
	task, err := s.repo.FindBackupRestoreTaskById(taskId)
	if err != nil {
		return fmt.Errorf("task not found: %d", taskId)
	}
	if task.Status == nil || *task.Status != 1 {
		return fmt.Errorf("task %d is not in waiting status", taskId)
	}

	// Update status to executing.
	now := time.Now()
	statusExec := 2
	task.Status = &statusExec
	task.StartTime = &now
	if err := s.repo.UpdateBackupRestoreTask(task); err != nil {
		return fmt.Errorf("update task status: %w", err)
	}

	taskType := "Backup"
	if task.TaskType != nil && *task.TaskType == "restore" {
		taskType = "Restore"
	}
	s.dispatchBackupRestore(task, username, taskType)

	// Mark as executed.
	statusDone := 3
	task.Status = &statusDone
	task.EndTime = ptrTimePtr(&now)
	_ = s.repo.UpdateBackupRestoreTask(task)

	return nil
}

// CancelBackupRestoreTask cancels a pending or scheduled task.
func (s *service) CancelBackupRestoreTask(taskId int) error {
	task, err := s.repo.FindBackupRestoreTaskById(taskId)
	if err != nil {
		return fmt.Errorf("task not found: %d", taskId)
	}
	if task.Status == nil || (*task.Status != 1 && *task.Status != 2) {
		return fmt.Errorf("task %d cannot be cancelled in current status", taskId)
	}

	statusCancelled := 4
	task.Status = &statusCancelled
	return s.repo.UpdateBackupRestoreTask(task)
}

// ListBackupRestoreTasksVo returns the task list with progress and result info.
func (s *service) ListBackupRestoreTasksVo(tenancyId int, page, pageSize int) ([]BackupRestoreTaskVo, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	tasks, total, err := s.repo.FindBackupOrRestoreTasks(tenancyId, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []BackupRestoreTaskVo
	for _, t := range tasks {
		vo := BackupRestoreTaskVo{
			Id:            t.Id,
			Name:          ptrStr(t.Name),
			TaskType:      ptrStr(t.TaskType),
			OperationUser: ptrStr(t.User),
			OperationTime: ptrTime(t.OperationTime),
			ExecuteMode:   ptrInt(t.ExecuteMode),
		}
		if t.Status != nil {
			vo.Status = *t.Status
		}

		totalCnt, successCnt, pErr := s.repo.BatchBackupRestoreProgress(t.Id)
		if pErr == nil {
			vo.DeviceCount = int(totalCnt)
			vo.Progress = fmt.Sprintf("%d/%d", successCnt, totalCnt)
		}

		result, rErr := s.repo.BatchBackupRestoreResult(t.Id)
		if rErr == nil {
			vo.Result = result
		}

		vos = append(vos, vo)
	}
	return vos, total, nil
}

// ListBackupRestoreTaskDetail returns per-device results for a task.
func (s *service) ListBackupRestoreTaskDetail(taskId int) ([]BackupRestoreTaskDetailVo, error) {
	return s.repo.BatchBackupRestoreDetail(taskId)
}

// ---------- internal dispatch ----------

// dispatchBackupRestore resolves target devices and dispatches TR-069 commands.
func (s *service) dispatchBackupRestore(task *BackupOrRestoreTask, username string, operation string) {
	deviceIds := s.resolveBackupDeviceIds(task)
	if len(deviceIds) == 0 {
		logger.Warnf("no devices resolved for task %d", task.Id)
		return
	}

	now := time.Now()
	expiredAt := now.Add(10 * time.Minute).UnixMilli()
	ctx := context.Background()
	taskType := ptrStr(task.TaskType)

	for _, elementId := range deviceIds {
		// Blacklist check.
		var blCount int64
		s.repo.DB().Raw(`
			SELECT COUNT(*) FROM element_black_list
			WHERE serial_number = (SELECT serial_number FROM cpe_element WHERE ne_neid = ?)
		`, elementId).Count(&blCount)
		if blCount > 0 {
			logger.Warnf("device %d is blacklisted, skipping", elementId)
			failureMsg := "Device is blacklisted"
			dl := &RestoreAndBackUpDeviceLog{
				ElementId:     &elementId,
				TaskId:        &task.Id,
				Type:          &taskType,
				StartTime:     &now,
				EndTime:       &now,
				Results:       intPtr(2),
				FailureReason: &failureMsg,
			}
			s.repo.CreateDeviceLog(dl)
			continue
		}

		// Build operation param based on operation type.
		var opParam string
		if operation == "Backup" {
			opParam = fmt.Sprintf(`{"url":"/api/acs-file-server/upload/config/0/%d/"}`, elementId)
		} else {
			opParam = fmt.Sprintf(`{"url":"/api/acs-file-server/configFile?elementId=%d"}`, elementId)
		}

		// Create EventLog (status=1 = pending).
		eventLogId, err := s.repo.InsertEventLog(operation, elementId, username, 1, opParam)
		if err != nil {
			logger.Errorf("create event_log for device %d: %v", elementId, err)
			continue
		}

		// Push to Redis.
		msg := opmsg.Message{
			EventType:      operation, // Java EventType — variable (Backup / BackupDaily / Restore / etc.)
			NeNeid:         elementId,
			Operation:      operation,
			OperationParam: opParam,
			OperationUser:  username,
			CommandTrackId: eventLogId,
			ExpiredAt:      expiredAt,
		}
		msgJSON, _ := msg.Marshal()
		if err := redis.LPush(ctx, mq.OperationQueue, string(msgJSON)); err != nil {
			logger.Errorf("push to redis queue for device %d: %v", elementId, err)
		}

		// Create device log.
		dl := &RestoreAndBackUpDeviceLog{
			ElementId:  &elementId,
			EventLogId: &eventLogId,
			TaskId:     &task.Id,
			Type:       &taskType,
			StartTime:  &now,
		}
		s.repo.CreateDeviceLog(dl)
	}
}

// resolveBackupDeviceIds extracts device IDs from the task configuration.
func (s *service) resolveBackupDeviceIds(task *BackupOrRestoreTask) []int64 {
	seen := make(map[int64]struct{})
	var result []int64

	addId := func(id int64) {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}

	// From elementIds JSON.
	if task.ElementIds != nil && *task.ElementIds != "" && *task.ElementIds != "null" {
		var ids []int64
		if err := json.Unmarshal([]byte(*task.ElementIds), &ids); err == nil {
			for _, id := range ids {
				addId(id)
			}
		}
	}

	// From deviceGroupIds JSON.
	if task.Scope != nil && *task.Scope == "deviceGroup" && task.DeviceGroupIds != nil && *task.DeviceGroupIds != "" {
		var groupIds []string
		if err := json.Unmarshal([]byte(*task.DeviceGroupIds), &groupIds); err == nil && len(groupIds) > 0 {
			var fromGroups []int64
			s.repo.DB().Raw(`SELECT ne_neid FROM cpe_element WHERE device_group_id IN (?)`, groupIds).Scan(&fromGroups)
			for _, id := range fromGroups {
				addId(id)
			}
		}
	}

	// ExecuteOnAllDevice: if no devices resolved yet and flag is set, get all.
	if (task.ExecuteOnAllDevice == nil || *task.ExecuteOnAllDevice) && len(result) == 0 {
		var allIds []int64
		s.repo.DB().Raw(`SELECT ne_neid FROM cpe_element`).Scan(&allIds)
		for _, id := range allIds {
			addId(id)
		}
	}

	return result
}
