package eventlog

import (
	"gorm.io/gorm"
)

// Service contains the business logic for event-log management.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ---------------------------------------------------------------------------
// EventLog
// ---------------------------------------------------------------------------

// ListEventLogs returns a paginated list of event logs filtered by element
// and optional event type.
func (s *Service) ListEventLogs(elementId int64, eventType string, page, pageSize int) ([]EventLog, int64, error) {
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
func (s *Service) GetEventLog(id int64) (*EventLog, error) {
	return s.repo.FindByID(id)
}

// ---------------------------------------------------------------------------
// TaskToEventLog
// ---------------------------------------------------------------------------

// ListTaskEventLogs returns all event-log associations for the given task.
func (s *Service) ListTaskEventLogs(taskId int, taskType string) ([]TaskToEventLog, error) {
	return s.repo.FindTaskEventLogs(taskId, taskType)
}
