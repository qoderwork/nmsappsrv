package eventlog

import (
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository handles database operations for event-log entities.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ---------------------------------------------------------------------------
// EventLog
// ---------------------------------------------------------------------------

// FindEventLogs returns a paginated list of event logs filtered by element
// and (optionally) event type, together with the total count.
func (r *Repository) FindEventLogs(elementId int64, eventType string, offset, limit int) ([]EventLog, int64, error) {
	var logs []EventLog
	var total int64

	query := r.db.Model(&EventLog{}).Where("element_id = ?", elementId)
	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindEventLogs count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("operation_time DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		logger.Errorf("FindEventLogs query error: %v", err)
		return nil, 0, err
	}
	return logs, total, nil
}

// FindEventLogByID returns a single event log by its primary key.
func (r *Repository) FindEventLogByID(id int64) (*EventLog, error) {
	var log EventLog
	if err := r.db.Where("id = ?", id).First(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

// CreateEventLog inserts a new event log entry.
func (r *Repository) CreateEventLog(log *EventLog) error {
	return r.db.Create(log).Error
}

// UpdateEventLog saves changes to an existing event log entry.
func (r *Repository) UpdateEventLog(log *EventLog) error {
	return r.db.Save(log).Error
}

// ---------------------------------------------------------------------------
// TaskToEventLog
// ---------------------------------------------------------------------------

// FindTaskEventLogs returns all task-to-event-log associations for the given
// task ID and task type.
func (r *Repository) FindTaskEventLogs(taskId int, taskType string) ([]TaskToEventLog, error) {
	var rels []TaskToEventLog
	query := r.db.Where("task_id = ?", taskId)
	if taskType != "" {
		query = query.Where("task_type = ?", taskType)
	}
	if err := query.Find(&rels).Error; err != nil {
		return nil, err
	}
	return rels, nil
}

// CreateTaskToEventLog inserts a new task-to-event-log association.
func (r *Repository) CreateTaskToEventLog(t *TaskToEventLog) error {
	return r.db.Create(t).Error
}
