package mml

import (
	"context"
	"time"

	"nmsappsrv/internal/mq"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Service contains the business logic for MML operations.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ListMmlSets returns all MML sets for the given license.
func (s *Service) ListMmlSets(licenseId int) ([]MmlSet, error) {
	return s.repo.FindMmlSets(licenseId)
}

// ListMmlCommands returns all commands in the given MML set.
func (s *Service) ListMmlCommands(setId int) ([]MmlCommand, error) {
	return s.repo.FindMmlCommands(setId)
}

// GetMmlCommandParams returns all parameters for the given command.
func (s *Service) GetMmlCommandParams(commandId int) ([]MmlCommandParam, error) {
	return s.repo.FindMmlCommandParams(commandId)
}

// ExecuteMml creates a pending execution result record (status=0) and
// enqueues the MML command to the Redis queue for async dispatch.
func (s *Service) ExecuteMml(elementId int64, command string, uid string, username string, params map[string]interface{}) (*MmlExecuteResult, error) {
	now := time.Now()
	result := &MmlExecuteResult{
		ElementId:     &elementId,
		Command:       &command,
		Uid:           &uid,
		User:          &username,
		Status:        0,
		OperationTime: &now,
		SendTime:      &now,
	}
	if err := s.repo.Create(result); err != nil {
		return nil, err
	}

	// Enqueue MML command to Redis queue for async processing by MML worker
	msg := MMLMessage{
		ElementId: elementId,
		Command:   command,
		Params:    params,
		ResultId:  result.Id,
	}
	if err := mq.Enqueue(context.Background(), mq.MMLQueue, msg); err != nil {
		logger.Errorf("failed to enqueue MML command to queue: %v", err)
		// Result record is already created with status=0; the worker will pick it up
		// once the queue message is retried or manually re-enqueued.
	}

	return result, nil
}

// ListMmlResults returns a paginated list of execution results for an element.
func (s *Service) ListMmlResults(elementId int64, page, pageSize int) ([]MmlExecuteResult, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindMmlExecuteResults(elementId, offset, pageSize)
}

// GetMmlResult returns a single execution result by ID.
func (s *Service) GetMmlResult(id int) (*MmlExecuteResult, error) {
	return s.repo.FindByID(id)
}
