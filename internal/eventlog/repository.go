package eventlog

import (
	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for event-log entities.
// It embeds BaseRepository[EventLog, int64] for standard CRUD on EventLog,
// and retains module-specific methods for custom queries and other entity types.
type Repository interface {
	// Generic CRUD delegated to BaseRepository[EventLog, int64].
	Create(entity *EventLog) error
	Save(entity *EventLog) error
	FindByID(id int64) (*EventLog, error)
	DeleteByID(id int64) error
	DeleteByIDs(ids []int64) error
	SoftDelete(id int64) error
	UpdateFields(id int64, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]EventLog, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[EventLog], error)

	// Module-specific methods.
	FindEventLogs(elementId int64, eventType string, offset, limit int) ([]EventLog, int64, error)
	FindTaskEventLogs(taskId int, taskType string) ([]TaskToEventLog, error)
	CreateTaskToEventLog(t *TaskToEventLog) error
}

// repository is the concrete GORM-backed implementation of Repository.
type repository struct {
	*baserepo.BaseRepository[EventLog, int64]
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[EventLog, int64](db, "id"),
		db:             db,
	}
}

// ---------------------------------------------------------------------------
// EventLog
// ---------------------------------------------------------------------------

// FindEventLogs returns a paginated list of event logs filtered by element
// and (optionally) event type, together with the total count.
func (r *repository) FindEventLogs(elementId int64, eventType string, offset, limit int) ([]EventLog, int64, error) {
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

// ---------------------------------------------------------------------------
// TaskToEventLog
// ---------------------------------------------------------------------------

// FindTaskEventLogs returns all task-to-event-log associations for the given
// task ID and task type.
func (r *repository) FindTaskEventLogs(taskId int, taskType string) ([]TaskToEventLog, error) {
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
func (r *repository) CreateTaskToEventLog(t *TaskToEventLog) error {
	return r.db.Create(t).Error
}
