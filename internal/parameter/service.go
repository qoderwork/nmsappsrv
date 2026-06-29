package parameter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/internal/misc"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// Service contains the business logic for parameter management.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ---------------------------------------------------------------------------
// ParameterAttributes
// ---------------------------------------------------------------------------

// GetParameters returns all parameter attributes for the given element.
func (s *Service) GetParameters(elementId int64) ([]ParameterAttributes, error) {
	return s.repo.FindParametersByElementId(elementId)
}

// SetParameter updates a parameter value for the given element and creates a
// change-log entry recording the old and new values.
func (s *Service) SetParameter(elementId int64, paramName string, value string, username string) error {
	pa, err := s.repo.FindParameterAttributes(elementId, paramName)
	if err != nil {
		return err
	}

	// Persist the new value on the parameter_attributes row.
	if err := s.repo.db.Model(&ParameterAttributes{}).
		Where("id = ?", pa.Id).
		Update("value", value).Error; err != nil {
		logger.Errorf("SetParameter update error: %v", err)
		return err
	}

	now := time.Now()
	log := &ParameterLog{
		ParameterName: &paramName,
		NewValue:      &value,
		ChangeUser:    &username,
		ChangeTime:    &now,
		ElementId:     &elementId,
	}
	return s.repo.CreateParameterLog(log)
}

// ---------------------------------------------------------------------------
// ParameterLog
// ---------------------------------------------------------------------------

// ListParameterLogs returns a paginated list of parameter change logs.
func (s *Service) ListParameterLogs(elementId int64, page, pageSize int) ([]ParameterLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindParameterLogs(elementId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// ParameterSet
// ---------------------------------------------------------------------------

// ListParameterSets returns all parameter sets for the given license.
func (s *Service) ListParameterSets(licenseId int) ([]ParameterSet, error) {
	return s.repo.FindParameterSets(licenseId)
}

// CreateParameterSet persists a new parameter set.
func (s *Service) CreateParameterSet(ps *ParameterSet) error {
	return s.repo.CreateParameterSet(ps)
}

// UpdateParameterSet persists changes to an existing parameter set.
func (s *Service) UpdateParameterSet(ps *ParameterSet) error {
	return s.repo.UpdateParameterSet(ps)
}

// DeleteParameterSet removes a parameter set by ID.
func (s *Service) DeleteParameterSet(id string) error {
	return s.repo.DeleteParameterSet(id)
}

// ---------------------------------------------------------------------------
// ParameterTemplate
// ---------------------------------------------------------------------------

// ListParameterTemplates returns all templates for the given tenancy.
func (s *Service) ListParameterTemplates(tenancyId int) ([]ParameterTemplate, error) {
	return s.repo.FindParameterTemplates(tenancyId)
}

// CreateParameterTemplate persists a new parameter template.
func (s *Service) CreateParameterTemplate(t *ParameterTemplate) error {
	return s.repo.CreateParameterTemplate(t)
}

// UpdateParameterTemplate persists changes to an existing parameter template.
func (s *Service) UpdateParameterTemplate(t *ParameterTemplate) error {
	return s.repo.UpdateParameterTemplate(t)
}

// ---------------------------------------------------------------------------
// ParameterBackupLog
// ---------------------------------------------------------------------------

// ListBackupLogs returns all backup logs for the given element.
func (s *Service) ListBackupLogs(elementId int64) ([]ParameterBackupLog, error) {
	return s.repo.FindParameterBackupLogs(elementId)
}

// ---------------------------------------------------------------------------
// Batch Parameter Configuration
// ---------------------------------------------------------------------------

// operationMessage is the JSON payload pushed to Redis operation_queue.
type operationMessage struct {
	EventType      string `json:"eventType"`
	NeNeid         int64  `json:"neNeid"`
	Operation      string `json:"operation"`
	OperationParam string `json:"operationParam"`
	OperationUser  string `json:"operationUser"`
	CommandTrackId int64  `json:"commandTrackId"`
	ExpiredAt      int64  `json:"expiredAt"`
}

// setParamEntry is a single parameter in the operationParam JSON array.
type setParamEntry struct {
	ParamName  string `json:"paramName"`
	ParamValue string `json:"paramValue"`
}

// BatchParameterConfigurationDirect creates a batch parameter configuration task
// and dispatches SetParameterValues commands for each device to Redis.
func (s *Service) BatchParameterConfigurationDirect(req *BatchParameterConfigRequest, username string, tenancyId int) error {
	if len(req.ParamValues) == 0 {
		return fmt.Errorf("paramValues must not be empty")
	}

	// 1. Resolve target device IDs from elementIds and groupIds.
	deviceIds, err := s.resolveDeviceIds(req.ElementIds, req.GroupIds)
	if err != nil {
		return fmt.Errorf("resolve devices: %w", err)
	}
	if len(deviceIds) == 0 {
		return fmt.Errorf("no target devices resolved")
	}

	// 2. Build operationParam JSON.
	entries := make([]setParamEntry, len(req.ParamValues))
	for i, pv := range req.ParamValues {
		entries[i] = setParamEntry{ParamName: pv.ParamKey, ParamValue: pv.ParamValue}
	}
	opParamJSON, _ := json.Marshal(entries)

	// 3. Create batch_configuration_log.
	now := time.Now()
	deviceCount := len(deviceIds)
	taskName := fmt.Sprintf("BatchParameterConfig-%d", now.UnixMilli())
	task := &misc.BatchConfigurationLog{
		Name:          &taskName,
		OperationTime: &now,
		TenancyId:     &tenancyId,
		User:          &username,
		DeviceCount:   &deviceCount,
	}
	if err := s.repo.CreateBatchConfigLog(task); err != nil {
		return fmt.Errorf("create batch config log: %w", err)
	}

	// 4. For each device: blacklist check → EventLog → Redis → DeviceLog.
	expiredAt := now.Add(5 * time.Minute).UnixMilli()
	ctx := context.Background()
	queueName := "operation_queue"

	for _, elementId := range deviceIds {
		// Blacklist check via raw SQL (avoid importing device package).
		var blCount int64
		s.repo.DB().Raw(`
			SELECT COUNT(*) FROM element_black_list
			WHERE serial_number = (SELECT serial_number FROM cpe_element WHERE ne_neid = ?)
		`, elementId).Count(&blCount)
		if blCount > 0 {
			logger.Warnf("device %d is blacklisted, skipping", elementId)
			continue
		}

		// Create EventLog (status=1 = pending).
		eventLogId, err := s.repo.InsertEventLog("SetParameterValues", elementId, username, 1, string(opParamJSON))
		if err != nil {
			logger.Errorf("create event_log for device %d: %v", elementId, err)
			continue
		}

		// Push operation message to Redis.
		msg := operationMessage{
			EventType:      "SetParameterValues",
			NeNeid:         elementId,
			Operation:      "SetParameterValues",
			OperationParam: string(opParamJSON),
			OperationUser:  username,
			CommandTrackId: eventLogId,
			ExpiredAt:      expiredAt,
		}
		msgJSON, _ := json.Marshal(msg)
		if err := redis.LPush(ctx, queueName, string(msgJSON)); err != nil {
			logger.Errorf("push to redis queue for device %d: %v", elementId, err)
		}

		// Create batch_configuration_device_log.
		dataStr := string(opParamJSON)
		deviceLog := &misc.BatchConfigurationDeviceLog{
			TaskId:     &task.Id,
			ElementId:  &elementId,
			Data:       &dataStr,
			EventLogId: &eventLogId,
		}
		if err := s.repo.CreateBatchConfigDeviceLog(deviceLog); err != nil {
			logger.Errorf("create device log for device %d: %v", elementId, err)
		}
	}

	return nil
}

// ListBatchConfigurations returns the paginated task list with progress info.
func (s *Service) ListBatchConfigurations(tenancyId int, page, pageSize int) ([]BatchConfigTaskVo, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	logs, total, err := s.repo.FindBatchConfigLogs(tenancyId, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []BatchConfigTaskVo
	for _, l := range logs {
		vo := BatchConfigTaskVo{
			Id:            l.Id,
			Name:          ptrStr(l.Name),
			OperationUser: ptrStr(l.User),
			OperationTime: ptrTime(l.OperationTime),
			DeviceCount:   ptrInt(l.DeviceCount),
		}
		totalCnt, successCnt, pErr := s.repo.BatchConfigProgress(l.Id)
		if pErr == nil {
			vo.Progress = fmt.Sprintf("%d/%d", successCnt, totalCnt)
		}
		vos = append(vos, vo)
	}
	return vos, total, nil
}

// ListBatchConfigurationDetail returns per-device results for a given task.
func (s *Service) ListBatchConfigurationDetail(taskId int64) ([]BatchConfigTaskDetailVo, error) {
	return s.repo.BatchConfigDetail(taskId)
}

// ---------- helpers ----------

// resolveDeviceIds merges explicit element IDs with IDs resolved from group IDs.
func (s *Service) resolveDeviceIds(elementIds []int64, groupIds []string) ([]int64, error) {
	seen := make(map[int64]struct{})
	var result []int64

	for _, id := range elementIds {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}

	if len(groupIds) > 0 {
		var fromGroups []int64
		if err := s.repo.DB().Raw(`
			SELECT ne_neid FROM cpe_element
			WHERE device_group_id IN (?)
		`, groupIds).Scan(&fromGroups).Error; err != nil {
			return nil, err
		}
		for _, id := range fromGroups {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				result = append(result, id)
			}
		}
	}

	return result, nil
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ptrTime(p *time.Time) string {
	if p == nil {
		return ""
	}
	return p.Format(time.RFC3339)
}

func ptrInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
