package eventlog

import (
	"gorm.io/gorm"
)

// Service defines the business-logic contract for event-log management.
type Service interface {
	ListEventLogs(elementId int64, eventType string, page, pageSize int) ([]EventLog, int64, error)
	GetEventLog(id int64) (*EventLog, error)
	ListTaskEventLogs(taskId int, taskType string) ([]TaskToEventLog, error)
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

// ---------------------------------------------------------------------------
// EventLog
// ---------------------------------------------------------------------------

// ListEventLogs returns a paginated list of event logs filtered by element
// and optional event type.
func (s *service) ListEventLogs(elementId int64, eventType string, page, pageSize int) ([]EventLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindEventLogs(elementId, eventType, offset, pageSize)
}

// GetEventLog returns a single event log by ID.
func (s *service) GetEventLog(id int64) (*EventLog, error) {
	return s.repo.FindByID(id)
}

// ---------------------------------------------------------------------------
// TaskToEventLog
// ---------------------------------------------------------------------------

// ListTaskEventLogs returns all event-log associations for the given task.
func (s *service) ListTaskEventLogs(taskId int, taskType string) ([]TaskToEventLog, error) {
	return s.repo.FindTaskEventLogs(taskId, taskType)
}
