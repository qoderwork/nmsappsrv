package upgrade

import (
	"encoding/json"

	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository handles database operations for upgrade entities.
type Repository struct {
	*baserepo.BaseRepository[UpgradeFile, int] // embedded generic CRUD for UpgradeFile
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		BaseRepository: baserepo.New[UpgradeFile, int](db, "id"),
		db:             db,
	}
}

// ---------------------------------------------------------------------------
// UpgradeFile
// ---------------------------------------------------------------------------

// FindUpgradeFiles returns a paginated list of upgrade files for the given
// tenancy together with the total count.
func (r *Repository) FindUpgradeFiles(tenancyId int, offset, limit int) ([]UpgradeFile, int64, error) {
	query := r.DB.Model(&UpgradeFile{}).Where("tenancy_id = ?", tenancyId)
	result, err := r.FindPage(query, "upload_time DESC", offset, limit)
	if err != nil {
		logger.Errorf("FindUpgradeFiles error: %v", err)
		return nil, 0, err
	}
	return result.Items, result.Total, nil
}

// ---------------------------------------------------------------------------
// UpgradeTask
// ---------------------------------------------------------------------------

// FindUpgradeTasks returns a paginated list of upgrade tasks for the given
// tenancy together with the total count.
func (r *Repository) FindUpgradeTasks(tenancyId int, filter UpgradeTaskFilter, offset, limit int) ([]UpgradeTask, int64, error) {
	var tasks []UpgradeTask
	var total int64

	query := r.db.Model(&UpgradeTask{}).Where("tenancy_id = ?", tenancyId)

	if filter.SearchText != "" {
		query = query.Where("name LIKE ? OR user LIKE ?", "%"+filter.SearchText+"%", "%"+filter.SearchText+"%")
	}
	if filter.TaskName != "" {
		query = query.Where("name LIKE ?", "%"+filter.TaskName+"%")
	}
	if filter.StartTime != "" {
		query = query.Where("operation_time >= ?", filter.StartTime)
	}
	if filter.EndTime != "" {
		query = query.Where("operation_time <= ?", filter.EndTime)
	}
	if filter.DeviceType != "" {
		query = query.Where("device_type = ?", filter.DeviceType)
	}

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindUpgradeTasks count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("operation_time DESC").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		logger.Errorf("FindUpgradeTasks query error: %v", err)
		return nil, 0, err
	}
	return tasks, total, nil
}

// FindUpgradeTaskByID returns a single upgrade task by its primary key.
func (r *Repository) FindUpgradeTaskByID(id int) (*UpgradeTask, error) {
	var t UpgradeTask
	if err := r.db.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateUpgradeTask inserts a new upgrade task.
func (r *Repository) CreateUpgradeTask(t *UpgradeTask) error {
	return r.db.Create(t).Error
}

// UpdateUpgradeTask saves changes to an existing upgrade task.
func (r *Repository) UpdateUpgradeTask(t *UpgradeTask) error {
	return r.db.Save(t).Error
}

// ---------------------------------------------------------------------------
// UpgradeLog
// ---------------------------------------------------------------------------

// FindUpgradeLogs returns a paginated list of upgrade logs for the given task
// together with the total count.
func (r *Repository) FindUpgradeLogs(taskId int, offset, limit int) ([]UpgradeLog, int64, error) {
	var logs []UpgradeLog
	var total int64

	query := r.db.Model(&UpgradeLog{}).Where("task_id = ?", taskId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindUpgradeLogs count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("creation_time DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		logger.Errorf("FindUpgradeLogs query error: %v", err)
		return nil, 0, err
	}
	return logs, total, nil
}

// CreateUpgradeLog inserts a new upgrade log entry.
func (r *Repository) CreateUpgradeLog(log *UpgradeLog) error {
	return r.db.Create(log).Error
}

// ---------------------------------------------------------------------------
// RebootTask
// ---------------------------------------------------------------------------

// CreateRebootTask inserts a new reboot task.
func (r *Repository) CreateRebootTask(t *RebootTask) error {
	return r.db.Create(t).Error
}

// FindRebootTasks returns a paginated list of reboot tasks for the given
// tenancy together with the total count.
func (r *Repository) FindRebootTasks(tenancyId int, offset, limit int) ([]RebootTask, int64, error) {
	var tasks []RebootTask
	var total int64

	query := r.db.Model(&RebootTask{}).Where("tenancy_id = ?", tenancyId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindRebootTasks count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("operation_time DESC").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		logger.Errorf("FindRebootTasks query error: %v", err)
		return nil, 0, err
	}
	return tasks, total, nil
}

// ---------------------------------------------------------------------------
// RollbackTask
// ---------------------------------------------------------------------------

// CreateRollbackTask inserts a new rollback task.
func (r *Repository) CreateRollbackTask(t *RollbackTask) error {
	return r.db.Create(t).Error
}

// UpdateRollbackTask saves changes to an existing rollback task.
func (r *Repository) UpdateRollbackTask(t *RollbackTask) error {
	return r.db.Save(t).Error
}

// FindRollbackTaskByID returns a single rollback task by its primary key.
func (r *Repository) FindRollbackTaskByID(id int) (*RollbackTask, error) {
	var t RollbackTask
	if err := r.db.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// FindRollbackTasks returns a paginated list of rollback tasks for the given
// tenancy together with the total count.
func (r *Repository) FindRollbackTasks(tenancyId int, offset, limit int) ([]RollbackTask, int64, error) {
	var tasks []RollbackTask
	var total int64

	query := r.db.Model(&RollbackTask{}).Where("tenancy_id = ?", tenancyId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindRollbackTasks count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("operation_time DESC").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		logger.Errorf("FindRollbackTasks query error: %v", err)
		return nil, 0, err
	}
	return tasks, total, nil
}

// ---------------------------------------------------------------------------
// Additional helpers for upgrade dispatch
// ---------------------------------------------------------------------------

// UpdateUpgradeLog saves changes to an existing upgrade log entry.
func (r *Repository) UpdateUpgradeLog(log *UpgradeLog) error {
	return r.db.Save(log).Error
}

// FindElementInfo returns serial_number and device_type for a given element.
func (r *Repository) FindElementInfo(elementId int64) (sn string, deviceType string, err error) {
	var row struct {
		SN         string `gorm:"column:serial_number"`
		DeviceType string `gorm:"column:device_type"`
	}
	err = r.db.Table("cpe_element").
		Select("serial_number, device_type").
		Where("ne_neid = ? AND deleted = 0", elementId).
		Scan(&row).Error
	return row.SN, row.DeviceType, err
}

// ParseElementIds deserializes the JSON element_ids column.
func ParseElementIds(jsonStr string) []int64 {
	var ids []int64
	if jsonStr != "" {
		json.Unmarshal([]byte(jsonStr), &ids)
	}
	return ids
}

// MarshalElementIds serializes element IDs to JSON.
func MarshalElementIds(ids []int64) string {
	b, _ := json.Marshal(ids)
	return string(b)
}
