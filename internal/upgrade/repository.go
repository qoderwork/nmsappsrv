package upgrade

import (
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository handles database operations for upgrade entities.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ---------------------------------------------------------------------------
// UpgradeFile
// ---------------------------------------------------------------------------

// FindUpgradeFiles returns a paginated list of upgrade files for the given
// tenancy together with the total count.
func (r *Repository) FindUpgradeFiles(tenancyId int, offset, limit int) ([]UpgradeFile, int64, error) {
	var files []UpgradeFile
	var total int64

	query := r.db.Model(&UpgradeFile{}).Where("tenancy_id = ?", tenancyId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindUpgradeFiles count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("upload_time DESC").Offset(offset).Limit(limit).Find(&files).Error; err != nil {
		logger.Errorf("FindUpgradeFiles query error: %v", err)
		return nil, 0, err
	}
	return files, total, nil
}

// CreateUpgradeFile inserts a new upgrade file record.
func (r *Repository) CreateUpgradeFile(f *UpgradeFile) error {
	return r.db.Create(f).Error
}

// DeleteUpgradeFile removes an upgrade file by its primary key.
func (r *Repository) DeleteUpgradeFile(id int) error {
	return r.db.Where("id = ?", id).Delete(&UpgradeFile{}).Error
}

// ---------------------------------------------------------------------------
// UpgradeTask
// ---------------------------------------------------------------------------

// FindUpgradeTasks returns a paginated list of upgrade tasks for the given
// tenancy together with the total count.
func (r *Repository) FindUpgradeTasks(tenancyId int, offset, limit int) ([]UpgradeTask, int64, error) {
	var tasks []UpgradeTask
	var total int64

	query := r.db.Model(&UpgradeTask{}).Where("tenancy_id = ?", tenancyId)

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
