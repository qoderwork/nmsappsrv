package misc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// Service contains the business logic for miscellaneous operations.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ---------------------------------------------------------------------------
// BackupOrRestoreTask
// ---------------------------------------------------------------------------

// ListBackupRestoreTasks returns a paginated list of backup/restore tasks.
func (s *Service) ListBackupRestoreTasks(tenancyId int, page, pageSize int) ([]BackupOrRestoreTask, int64, error) {
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
func (s *Service) CreateBackupRestoreTask(t *BackupOrRestoreTask) error {
	return s.repo.CreateBackupOrRestoreTask(t)
}

// ---------------------------------------------------------------------------
// BatchConfigurationLog
// ---------------------------------------------------------------------------

// ListBatchConfigLogs returns a paginated list of batch configuration logs.
func (s *Service) ListBatchConfigLogs(tenancyId int, page, pageSize int) ([]BatchConfigurationLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindBatchConfigLogs(tenancyId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// MRData
// ---------------------------------------------------------------------------

// ListMRData returns a paginated list of MR data records.
func (s *Service) ListMRData(elementId int64, page, pageSize int) ([]MRData, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindMRData(elementId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// ZTPLog
// ---------------------------------------------------------------------------

// ListZTPLogs returns all ZTP logs for the given element.
func (s *Service) ListZTPLogs(elementId int64) ([]ZTPLog, error) {
	return s.repo.FindZTPLogs(elementId)
}

// ---------------------------------------------------------------------------
// NorthReport
// ---------------------------------------------------------------------------

// ListNorthReports returns all north reports for the given license.
func (s *Service) ListNorthReports(licenseId int) ([]NorthReport, error) {
	return s.repo.FindNorthReports(licenseId)
}

// CreateNorthReport persists a new north report.
func (s *Service) CreateNorthReport(r *NorthReport) error {
	return s.repo.CreateNorthReport(r)
}

// UpdateNorthReport persists changes to an existing north report.
func (s *Service) UpdateNorthReport(r *NorthReport) error {
	return s.repo.UpdateNorthReport(r)
}

// DeleteNorthReport removes a north report by ID.
func (s *Service) DeleteNorthReport(id int) error {
	return s.repo.DeleteNorthReport(id)
}

// ---------------------------------------------------------------------------
// Radius
// ---------------------------------------------------------------------------

// ListRadius returns all RADIUS configurations for the given tenancy.
func (s *Service) ListRadius(tenancyId int) ([]Radius, error) {
	return s.repo.FindRadius(tenancyId)
}

// SaveRadius inserts or updates a RADIUS configuration.
func (s *Service) SaveRadius(r *Radius) error {
	return s.repo.SaveRadius(r)
}

// DeleteRadius removes a RADIUS configuration by ID.
func (s *Service) DeleteRadius(id int) error {
	return s.repo.DeleteRadius(id)
}

// ---------------------------------------------------------------------------
// SystemOperatorLog
// ---------------------------------------------------------------------------

// ListOperatorLogs returns a paginated list of operator logs.
func (s *Service) ListOperatorLogs(tenancyId int, page, pageSize int) ([]SystemOperatorLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindOperatorLogs(tenancyId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// UploadFile
// ---------------------------------------------------------------------------

// ListUploadFiles returns a paginated list of uploaded files.
func (s *Service) ListUploadFiles(page, pageSize int) ([]UploadFile, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindUploadFiles(offset, pageSize)
}

// CreateUploadFile persists a new upload file record.
func (s *Service) CreateUploadFile(f *UploadFile) error {
	return s.repo.CreateUploadFile(f)
}

// DeleteUploadFile removes an upload file by ID.
func (s *Service) DeleteUploadFile(id string) error {
	return s.repo.DeleteUploadFile(id)
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
func (s *Service) BatchAddObject(req *BatchAddObjectRequest, username string, tenancyId int) error {
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
	queueName := "operation_queue" // default; could be read from config

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
		msg := operationMessage{
			EventType:      "AddObject",
			NeNeid:         elementId,
			Operation:      "AddObject",
			OperationParam: objPath,
			OperationUser:  username,
			CommandTrackId: eventLogId,
			ExpiredAt:      expiredAt,
		}
		msgJSON, _ := json.Marshal(msg)
		if err := redis.LPush(ctx, queueName, string(msgJSON)); err != nil {
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
func (s *Service) ListBatchAddObjectTasks(tenancyId int, page, pageSize int) ([]BatchAddObjectTaskVo, int64, error) {
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
func (s *Service) ListBatchAddObjectTaskDetail(taskId int) ([]BatchAddObjectTaskDetailVo, error) {
	return s.repo.BatchAddObjectTaskDetail(taskId)
}

// ---------------------------------------------------------------------------
// Batch Backup / Restore
// ---------------------------------------------------------------------------

// CreateBackupTask creates a batch backup task and dispatches commands for
// immediate execution (mode 1) or saves it for later trigger (mode 2/3).
func (s *Service) CreateBackupTask(req *BackupRestoreRequest, username string, tenancyId int) error {
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
func (s *Service) CreateRestoreTask(req *BackupRestoreRequest, username string, tenancyId int) error {
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
func (s *Service) StartBackupRestoreTask(taskId int, username string) error {
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
func (s *Service) CancelBackupRestoreTask(taskId int) error {
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
func (s *Service) ListBackupRestoreTasksVo(tenancyId int, page, pageSize int) ([]BackupRestoreTaskVo, int64, error) {
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
func (s *Service) ListBackupRestoreTaskDetail(taskId int) ([]BackupRestoreTaskDetailVo, error) {
	return s.repo.BatchBackupRestoreDetail(taskId)
}

// ---------- internal dispatch ----------

// dispatchBackupRestore resolves target devices and dispatches TR-069 commands.
func (s *Service) dispatchBackupRestore(task *BackupOrRestoreTask, username string, operation string) {
	deviceIds := s.resolveBackupDeviceIds(task)
	if len(deviceIds) == 0 {
		logger.Warnf("no devices resolved for task %d", task.Id)
		return
	}

	now := time.Now()
	expiredAt := now.Add(10 * time.Minute).UnixMilli()
	ctx := context.Background()
	queueName := "operation_queue"
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
		msg := operationMessage{
			EventType:      operation,
			NeNeid:         elementId,
			Operation:      operation,
			OperationParam: opParam,
			OperationUser:  username,
			CommandTrackId: eventLogId,
			ExpiredAt:      expiredAt,
		}
		msgJSON, _ := json.Marshal(msg)
		if err := redis.LPush(ctx, queueName, string(msgJSON)); err != nil {
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
func (s *Service) resolveBackupDeviceIds(task *BackupOrRestoreTask) []int64 {
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

// ---------------------------------------------------------------------------
// ZTP
// ---------------------------------------------------------------------------

// GetZTPSetting loads and parses the ZTP configuration from system_config.
func (s *Service) GetZTPSetting() (*ZTPSetting, error) {
	val, err := s.repo.GetSystemConfigValue("ztp_config")
	if err != nil {
		// No config yet — return defaults.
		return &ZTPSetting{}, nil
	}
	var setting ZTPSetting
	if err := json.Unmarshal([]byte(val), &setting); err != nil {
		return nil, fmt.Errorf("invalid ztp_config: %w", err)
	}
	return &setting, nil
}

// SaveZTPSetting persists the ZTP configuration to system_config.
func (s *Service) SaveZTPSetting(setting *ZTPSetting) error {
	data, err := json.Marshal(setting)
	if err != nil {
		return fmt.Errorf("marshal ztp_config: %w", err)
	}
	return s.repo.SaveSystemConfigValue("ztp_config", string(data))
}

// ListZTPResults returns paginated ZTP provisioning results.
func (s *Service) ListZTPResults(req *ListZTPResultsRequest) ([]ZTPResultVo, int64, error) {
	return s.repo.FindZTPResults(req)
}

// ListZTPRetryLogs returns retry logs for a device.
func (s *Service) ListZTPRetryLogs(elementId int64) ([]ZTPRetryLogVo, error) {
	return s.repo.FindZTPRetryLogs(elementId)
}

// ListHistoryZTPFiles returns paginated ZTP file history.
func (s *Service) ListHistoryZTPFiles(elementId int64, page, pageSize int) ([]HistoryZTPFileVo, int64, error) {
	return s.repo.FindHistoryZTPFiles(elementId, page, pageSize)
}

// SetZTPStatus enables or disables ZTP for the given devices.
func (s *Service) SetZTPStatus(req *SetZTPStatusRequest) error {
	if req.Status == "enable" {
		// Reset aos_file_name to nil and read_to_ztp to 0 so the ZTP thread picks them up.
		return s.repo.ClearDeviceAOSFile(req.ElementIds)
	}
	// "disable": just clear the read_to_ztp flag.
	return s.repo.DB().Table("cpe_element").
		Where("ne_neid IN (?)", req.ElementIds).
		Update("read_to_ztp", 0).Error
}

// BatchReZTP triggers re-provisioning for a batch of devices.
func (s *Service) BatchReZTP(req *BatchReZTPRequest) error {
	elementIds := s.resolveReZTPDeviceIds(req)
	if len(elementIds) == 0 {
		return fmt.Errorf("no devices resolved for re-ZTP")
	}

	// Clean up old ZTP data for these devices.
	_ = s.repo.DeleteZTPLogsByElementIds(elementIds)
	_ = s.repo.DeleteZTPFileSendLogsByElementIds(elementIds)
	for _, id := range elementIds {
		_ = s.repo.DeleteGnbIdUsedByElementId(id)
	}

	// Reset device AOS file so the ZTP thread picks them up again.
	return s.repo.ClearDeviceAOSFile(elementIds)
}

// DeleteZTPFiles deletes ZTP files and related data for the given devices.
func (s *Service) DeleteZTPFiles(req *DeleteZTPFileRequest) error {
	_ = s.repo.DeleteZTPLogsByElementIds(req.ElementIds)
	_ = s.repo.DeleteZTPFileSendLogsByElementIds(req.ElementIds)
	for _, id := range req.ElementIds {
		_ = s.repo.DeleteGnbIdUsedByElementId(id)
	}
	return s.repo.ClearDeviceAOSFile(req.ElementIds)
}

// resolveReZTPDeviceIds extracts device IDs from the batch re-ZTP request.
func (s *Service) resolveReZTPDeviceIds(req *BatchReZTPRequest) []int64 {
	seen := make(map[int64]struct{})
	var result []int64

	addId := func(id int64) {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}

	for _, id := range req.ElementIds {
		addId(id)
	}

	if req.Scope == "deviceGroup" && len(req.DeviceGroupIds) > 0 {
		var fromGroups []int64
		s.repo.DB().Raw(`SELECT ne_neid FROM cpe_element WHERE device_group_id IN (?)`, req.DeviceGroupIds).Scan(&fromGroups)
		for _, id := range fromGroups {
			addId(id)
		}
	}

	if req.Scope == "market" && len(req.Markets) > 0 {
		var fromMarkets []int64
		s.repo.DB().Raw(`SELECT ne_neid FROM cpe_element WHERE market IN (?)`, req.Markets).Scan(&fromMarkets)
		for _, id := range fromMarkets {
			addId(id)
		}
	}

	return result
}

// ---------- helpers ----------

func strPtr(s string) *string  { return &s }
func intPtr(i int) *int        { return &i }
func ptrStr(p *string) string  { if p == nil { return "" }; return *p }
func ptrInt(p *int) int        { if p == nil { return 0 }; return *p }
func ptrTime(p *time.Time) string {
	if p == nil {
		return ""
	}
	return p.Format(time.RFC3339)
}
func ptrTimePtr(p *time.Time) *time.Time { return p }
