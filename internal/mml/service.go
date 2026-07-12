package mml

import (
	"context"
	"time"

	"nmsappsrv/internal/mq"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Service defines the business-logic contract for MML operations.
type Service interface {
	ListMmlSets(licenseId int) ([]MmlSet, error)
	ListMmlCommands(setId int) ([]MmlCommand, error)
	GetMmlCommandParams(commandId int) ([]MmlCommandParam, error)
	ExecuteMml(elementId int64, command string, uid string, username string, params map[string]interface{}) (*MmlExecuteResult, error)
	ListMmlResults(elementId int64, page, pageSize int) ([]MmlExecuteResult, int64, error)
	GetMmlResult(id int) (*MmlExecuteResult, error)
	UploadBatchProcessFile(fileName, filePath string, fileSize int64, username string, licenseId int) (*BatchProcessFile, error)
	ListBatchProcessFiles(licenseId int) ([]BatchProcessFile, error)
	SendBatchProcessFile(id int, licenseId int) (*BatchProcessFile, error)
	CheckBatchProcessFile(id int) (*BatchProcessFile, error)
	ListBatchProcessLogs(batchFileId int) ([]BatchProcessLog, error)
	ListBatchExecuteResults(batchFileId int) ([]MmlExecuteResult, error)
	GetBatchProcessFile(id int) (*BatchProcessFile, error)
	DeleteBatchProcessFile(id int) error
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// ListMmlSets returns all MML sets for the given license.
func (s *service) ListMmlSets(licenseId int) ([]MmlSet, error) {
	return s.repo.FindMmlSets(licenseId)
}

// ListMmlCommands returns all commands in the given MML set.
func (s *service) ListMmlCommands(setId int) ([]MmlCommand, error) {
	return s.repo.FindMmlCommands(setId)
}

// GetMmlCommandParams returns all parameters for the given command.
func (s *service) GetMmlCommandParams(commandId int) ([]MmlCommandParam, error) {
	return s.repo.FindMmlCommandParams(commandId)
}

// ExecuteMml creates a pending execution result record (status=0) and
// enqueues the MML command to the Redis queue for async dispatch.
func (s *service) ExecuteMml(elementId int64, command string, uid string, username string, params map[string]interface{}) (*MmlExecuteResult, error) {
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
func (s *service) ListMmlResults(elementId int64, page, pageSize int) ([]MmlExecuteResult, int64, error) {
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
func (s *service) GetMmlResult(id int) (*MmlExecuteResult, error) {
	return s.repo.FindByID(id)
}
