package reboot

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// Service defines the business-logic contract for reboot management.
type Service interface {
	AddRebootTask(req *AddRebootTaskRequest, tenancyId int, username string) (int, error)
	DeleteRebootTask(id int) error
	StartRebootTask(id int, username string) error
	CancelRebootTask(id int) error
	ListTasks(tenancyId int, query ListRebootTaskQuery) ([]RebootTaskVO, int64, error)
	ListTaskResults(query ListRebootTaskResultQuery) ([]RebootTaskResultVO, int64, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a new reboot service.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// AddRebootTask creates a new reboot task and dispatches commands if immediate.
func (s *service) AddRebootTask(req *AddRebootTaskRequest, tenancyId int, username string) (int, error) {
	if s.repo.TaskNameExists(tenancyId, req.Name) {
		return 0, fmt.Errorf("task name already exists")
	}

	// Resolve element IDs
	elementIds := req.ElementIds
	if req.Scope == "deviceGroup" && len(req.DeviceGroupIds) > 0 {
		groupIds, err := s.repo.FindElementIdsByGroup(req.DeviceGroupIds)
		if err != nil {
			return 0, fmt.Errorf("resolve device groups: %w", err)
		}
		elementIds = append(elementIds, groupIds...)
	}
	if len(elementIds) == 0 {
		return 0, fmt.Errorf("no devices selected")
	}

	now := time.Now()
	task := &RebootTask{
		Name:           req.Name,
		User:           username,
		OperationTime:  now,
		ExecuteMode:    req.ExecuteMode,
		TenancyId:      tenancyId,
		ElementIds:     MarshalElementIds(elementIds),
		DeviceType:     req.DeviceType,
		Scope:          req.Scope,
		DeviceGroupIds: MarshalGroupIds(req.DeviceGroupIds),
		SoftReboot:     req.SoftReboot,
	}

	if req.ExecuteMode == 1 {
		task.Status = 2 // Executing
		task.StartTime = &now
	} else {
		task.Status = 1 // Waiting
	}

	if req.ExecuteMode == 3 && req.TriggerTime != nil {
		if t, err := time.Parse(time.RFC3339, *req.TriggerTime); err == nil {
			task.TriggerTime = &t
		}
	}

	if err := s.repo.Create(task); err != nil {
		return 0, err
	}

	// Immediate dispatch
	if req.ExecuteMode == 1 {
		s.dispatchReboot(task, elementIds, username)
	}

	return task.Id, nil
}

// DeleteRebootTask removes a reboot task.
func (s *service) DeleteRebootTask(id int) error {
	return s.repo.DeleteByID(id)
}

// StartRebootTask manually starts a waiting task.
func (s *service) StartRebootTask(id int, username string) error {
	task, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if task.Status != 1 {
		return fmt.Errorf("task already started or completed")
	}

	// Distributed lock
	ctx := context.Background()
	lockKey := fmt.Sprintf("reboot_task_start_%d", id)
	if !redis.Lock(ctx, lockKey, 60*time.Second) {
		return fmt.Errorf("task is being started by another request")
	}
	defer redis.Unlock(ctx, lockKey)

	now := time.Now()
	task.Status = 2
	task.StartTime = &now
	s.repo.Save(task)

	elementIds := ParseElementIds(task.ElementIds)
	s.dispatchReboot(task, elementIds, username)
	return nil
}

// CancelRebootTask cancels a waiting task.
func (s *service) CancelRebootTask(id int) error {
	task, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	task.Status = 4 // Cancelled
	return s.repo.Save(task)
}

// ListTasks returns paginated reboot tasks.
func (s *service) ListTasks(tenancyId int, query ListRebootTaskQuery) ([]RebootTaskVO, int64, error) {
	return s.repo.ListTasks(tenancyId, query)
}

// ListTaskResults returns per-device results for a task.
func (s *service) ListTaskResults(query ListRebootTaskResultQuery) ([]RebootTaskResultVO, int64, error) {
	return s.repo.ListTaskResults(query)
}

// ---------- dispatch ----------

func (s *service) dispatchReboot(task *RebootTask, elementIds []int64, username string) {
	eventType := "Reboot"
	operation := "Reboot"
	taskType := "reboot"
	if task.SoftReboot {
		eventType = "SoftReboot"
		operation = "SoftReboot"
		taskType = "softReboot"
	}

	for _, eid := range elementIds {
		sn, _, err := s.repo.FindElementInfo(eid)
		if err != nil {
			logger.Errorf("reboot: find element %d: %v", eid, err)
			continue
		}

		// Blacklist check
		ctx := context.Background()
		blKey := fmt.Sprintf("black_list_%s%s", task.DeviceType, sn)
		blVal, _ := redis.Get(ctx, blKey)
		if blVal == "y" {
			logger.Infof("reboot: device %s is blacklisted, skipping", sn)
			continue
		}

		// Upgrade conflict check
		if s.repo.IsDeviceInUpgrade(eid) {
			elId, err := s.repo.InsertEventLog(eventType, eid, username, 5, "Device is in upgrade")
			if err == nil {
				s.repo.CreateTaskToEventLog(task.Id, elId, taskType)
			}
			continue
		}

		// Create event_log (pending)
		elId, err := s.repo.InsertEventLog(eventType, eid, username, 1, "")
		if err != nil {
			logger.Errorf("reboot: create event_log for %d: %v", eid, err)
			continue
		}
		s.repo.CreateTaskToEventLog(task.Id, elId, taskType)

		// Push to operation_queue
		now := time.Now()
		msg := operationMessage{
			EventType:      eventType,
			NeNeid:         eid,
			Operation:      operation,
			OperationParam: "",
			OperationUser:  username,
			CommandTrackId: elId,
			ExpiredAt:      now.Add(5 * time.Minute).UnixMilli(),
		}
		msgJSON, _ := json.Marshal(msg)
		if err := redis.LPush(ctx, "operation_queue", string(msgJSON)); err != nil {
			logger.Errorf("reboot: push to queue for %d: %v", eid, err)
		}
	}
}

// newService creates a Service backed by the given mock Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}
