package upgrade

import (
	"encoding/json"

	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for upgrade entities.
type Repository interface {
	// Generic CRUD provided by the embedded BaseRepository[UpgradeFile, int].
	Create(entity *UpgradeFile) error
	Save(entity *UpgradeFile) error
	FindByID(id int) (*UpgradeFile, error)
	DeleteByID(id int) error
	DeleteByIDs(ids []int) error
	SoftDelete(id int) error
	UpdateFields(id int, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]UpgradeFile, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[UpgradeFile], error)
	RawDB() *gorm.DB

	// Custom methods.
	FindUpgradeFiles(tenantId int, offset, limit int) ([]UpgradeFile, int64, error)
	FindUpgradeTasks(tenantId int, filter UpgradeTaskFilter, offset, limit int) ([]UpgradeTask, int64, error)
	FindUpgradeTaskByID(id int) (*UpgradeTask, error)
	CreateUpgradeTask(t *UpgradeTask) error
	UpdateUpgradeTask(t *UpgradeTask) error
	FindUpgradeLogs(taskId int, offset, limit int) ([]UpgradeLog, int64, error)
	CreateUpgradeLog(log *UpgradeLog) error
	CreateRebootTask(t *RebootTask) error
	FindRebootTasks(tenantId int, offset, limit int) ([]RebootTask, int64, error)
	CreateRollbackTask(t *RollbackTask) error
	UpdateRollbackTask(t *RollbackTask) error
	FindRollbackTaskByID(id int) (*RollbackTask, error)
	FindRollbackTasks(tenantId int, offset, limit int) ([]RollbackTask, int64, error)
	UpdateUpgradeLog(log *UpgradeLog) error
	FindElementInfo(elementId int64) (sn string, deviceType string, err error)
	CountUpgradeLogsBySuccess(taskId int, success bool) (int64, error)

	// Task lifecycle
	CancelUpgradeTask(id int) error
	CancelRollbackTask(id int) error

	// Results
	FindUpgradeLogsByTaskId(taskId int, offset, limit int) ([]UpgradeLog, int64, error)
	FindUpgradeLogsByTaskIdDetail(taskId int, offset, limit int) ([]UpgradeLog, int64, error)

	// Statistics
	CountUpgradeTaskStatusCounts(tenantId int) ([]StatusCountItem, error)
	CountUpgradeDeviceResultCounts(taskId int) ([]DeviceResultCountItem, error)

	// AutoUpgradeTask CRUD
	FindAutoUpgradeTasks(offset, limit int) ([]AutoUpgradeTask, int64, error)
	FindAutoUpgradeTaskByID(id int64) (*AutoUpgradeTask, error)
	CreateAutoUpgradeTask(t *AutoUpgradeTask) error
	UpdateAutoUpgradeTask(t *AutoUpgradeTask) error
	DeleteAutoUpgradeTask(id int64) error

	// UpgradeFile update
	UpdateUpgradeFile(f *UpgradeFile) error
	FindUpgradeFileByID(id int) (*UpgradeFile, error)
}

// repository is the concrete GORM-backed implementation of Repository.
type repository struct {
	*baserepo.BaseRepository[UpgradeFile, int] // embedded generic CRUD for UpgradeFile
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[UpgradeFile, int](db, "id"),
		db:             db,
	}
}

// RawDB returns the underlying *gorm.DB for ad-hoc queries outside the
// pre-defined repository contract.
func (r *repository) RawDB() *gorm.DB {
	return r.db
}

// ---------------------------------------------------------------------------
// UpgradeFile
// ---------------------------------------------------------------------------

// FindUpgradeFiles returns a paginated list of upgrade files for the given
// tenancy together with the total count.
func (r *repository) FindUpgradeFiles(tenantId int, offset, limit int) ([]UpgradeFile, int64, error) {
	query := r.DB.Model(&UpgradeFile{}).Where("tenant_id = ?", tenantId)
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
func (r *repository) FindUpgradeTasks(tenantId int, filter UpgradeTaskFilter, offset, limit int) ([]UpgradeTask, int64, error) {
	var tasks []UpgradeTask
	var total int64

	query := r.db.Model(&UpgradeTask{}).Where("tenant_id = ?", tenantId)

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
func (r *repository) FindUpgradeTaskByID(id int) (*UpgradeTask, error) {
	var t UpgradeTask
	if err := r.db.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateUpgradeTask inserts a new upgrade task.
func (r *repository) CreateUpgradeTask(t *UpgradeTask) error {
	return r.db.Create(t).Error
}

// UpdateUpgradeTask saves changes to an existing upgrade task.
func (r *repository) UpdateUpgradeTask(t *UpgradeTask) error {
	return r.db.Save(t).Error
}

// ---------------------------------------------------------------------------
// UpgradeLog
// ---------------------------------------------------------------------------

// FindUpgradeLogs returns a paginated list of upgrade logs for the given task
// together with the total count.
func (r *repository) FindUpgradeLogs(taskId int, offset, limit int) ([]UpgradeLog, int64, error) {
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
func (r *repository) CreateUpgradeLog(log *UpgradeLog) error {
	return r.db.Create(log).Error
}

// ---------------------------------------------------------------------------
// RebootTask
// ---------------------------------------------------------------------------

// CreateRebootTask inserts a new reboot task.
func (r *repository) CreateRebootTask(t *RebootTask) error {
	return r.db.Create(t).Error
}

// FindRebootTasks returns a paginated list of reboot tasks for the given
// tenancy together with the total count.
func (r *repository) FindRebootTasks(tenantId int, offset, limit int) ([]RebootTask, int64, error) {
	var tasks []RebootTask
	var total int64

	query := r.db.Model(&RebootTask{}).Where("tenant_id = ?", tenantId)

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
func (r *repository) CreateRollbackTask(t *RollbackTask) error {
	return r.db.Create(t).Error
}

// UpdateRollbackTask saves changes to an existing rollback task.
func (r *repository) UpdateRollbackTask(t *RollbackTask) error {
	return r.db.Save(t).Error
}

// FindRollbackTaskByID returns a single rollback task by its primary key.
func (r *repository) FindRollbackTaskByID(id int) (*RollbackTask, error) {
	var t RollbackTask
	if err := r.db.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// FindRollbackTasks returns a paginated list of rollback tasks for the given
// tenancy together with the total count.
func (r *repository) FindRollbackTasks(tenantId int, offset, limit int) ([]RollbackTask, int64, error) {
	var tasks []RollbackTask
	var total int64

	query := r.db.Model(&RollbackTask{}).Where("tenant_id = ?", tenantId)

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
func (r *repository) UpdateUpgradeLog(log *UpgradeLog) error {
	return r.db.Save(log).Error
}

// FindElementInfo returns serial_number and device_type for a given element.
func (r *repository) FindElementInfo(elementId int64) (sn string, deviceType string, err error) {
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

// CountUpgradeLogsBySuccess returns the number of upgrade logs for a task
// filtered by the success flag.
func (r *repository) CountUpgradeLogsBySuccess(taskId int, success bool) (int64, error) {
	var count int64
	if err := r.db.Model(&UpgradeLog{}).Where("task_id = ? AND success = ?", taskId, success).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
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

// ---------------------------------------------------------------------------
// Task lifecycle: Cancel
// ---------------------------------------------------------------------------

// CancelUpgradeTask sets an upgrade task status to cancelled (status=4, aligned with Java UpgradeTask status enum: 1=Waiting,2=Executing,3=Executed,4=Cancelled).
func (r *repository) CancelUpgradeTask(id int) error {
	return r.db.Model(&UpgradeTask{}).Where("id = ? AND status IN (1, 2)", id).
		Update("status", 4).Error
}

// CancelRollbackTask sets a rollback task status to cancelled (status=4, see CancelUpgradeTask for enum).
func (r *repository) CancelRollbackTask(id int) error {
	return r.db.Model(&RollbackTask{}).Where("id = ? AND status IN (1, 2)", id).
		Update("status", 4).Error
}

// ---------------------------------------------------------------------------
// Results
// ---------------------------------------------------------------------------

// FindUpgradeLogsByTaskId returns paginated upgrade logs for a given task.
func (r *repository) FindUpgradeLogsByTaskId(taskId int, offset, limit int) ([]UpgradeLog, int64, error) {
	var logs []UpgradeLog
	var total int64

	query := r.db.Model(&UpgradeLog{}).Where("task_id = ?", taskId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindUpgradeLogsByTaskId count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("creation_time DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		logger.Errorf("FindUpgradeLogsByTaskId query error: %v", err)
		return nil, 0, err
	}
	return logs, total, nil
}

// FindUpgradeLogsByTaskIdDetail returns paginated detailed upgrade logs for a task.
func (r *repository) FindUpgradeLogsByTaskIdDetail(taskId int, offset, limit int) ([]UpgradeLog, int64, error) {
	var logs []UpgradeLog
	var total int64

	query := r.db.Model(&UpgradeLog{}).Where("task_id = ?", taskId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindUpgradeLogsByTaskIdDetail count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("creation_time DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		logger.Errorf("FindUpgradeLogsByTaskIdDetail query error: %v", err)
		return nil, 0, err
	}
	return logs, total, nil
}

// ---------------------------------------------------------------------------
// Statistics
// ---------------------------------------------------------------------------

// CountUpgradeTaskStatusCounts returns per-status counts of upgrade tasks for a tenancy.
func (r *repository) CountUpgradeTaskStatusCounts(tenantId int) ([]StatusCountItem, error) {
	var results []StatusCountItem
	type row struct {
		Status int   `gorm:"column:status"`
		Cnt    int64 `gorm:"column:cnt"`
	}
	var rows []row
	err := r.db.Model(&UpgradeTask{}).
		Select("status, COUNT(*) as cnt").
		Where("tenant_id = ?", tenantId).
		Group("status").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		results = append(results, StatusCountItem{Status: r.Status, Count: r.Cnt})
	}
	return results, nil
}

// CountUpgradeDeviceResultCounts returns per-result counts of upgrade logs for a task.
func (r *repository) CountUpgradeDeviceResultCounts(taskId int) ([]DeviceResultCountItem, error) {
	var results []DeviceResultCountItem

	var successCount, failCount, pendingCount int64
	r.db.Model(&UpgradeLog{}).Where("task_id = ? AND is_done = ? AND success = ?", taskId, true, true).Count(&successCount)
	r.db.Model(&UpgradeLog{}).Where("task_id = ? AND is_done = ? AND success = ?", taskId, true, false).Count(&failCount)
	r.db.Model(&UpgradeLog{}).Where("task_id = ? AND is_done = ?", taskId, false).Count(&pendingCount)

	results = append(results,
		DeviceResultCountItem{Result: "success", Count: successCount},
		DeviceResultCountItem{Result: "fail", Count: failCount},
		DeviceResultCountItem{Result: "pending", Count: pendingCount},
	)
	return results, nil
}

// ---------------------------------------------------------------------------
// AutoUpgradeTask CRUD
// ---------------------------------------------------------------------------

// FindAutoUpgradeTasks returns a paginated list of auto-upgrade tasks.
func (r *repository) FindAutoUpgradeTasks(offset, limit int) ([]AutoUpgradeTask, int64, error) {
	var tasks []AutoUpgradeTask
	var total int64

	query := r.db.Model(&AutoUpgradeTask{})

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindAutoUpgradeTasks count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		logger.Errorf("FindAutoUpgradeTasks query error: %v", err)
		return nil, 0, err
	}
	return tasks, total, nil
}

// FindAutoUpgradeTaskByID returns a single auto-upgrade task by ID.
func (r *repository) FindAutoUpgradeTaskByID(id int64) (*AutoUpgradeTask, error) {
	var t AutoUpgradeTask
	if err := r.db.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateAutoUpgradeTask inserts a new auto-upgrade task.
func (r *repository) CreateAutoUpgradeTask(t *AutoUpgradeTask) error {
	return r.db.Create(t).Error
}

// UpdateAutoUpgradeTask saves changes to an existing auto-upgrade task.
func (r *repository) UpdateAutoUpgradeTask(t *AutoUpgradeTask) error {
	return r.db.Save(t).Error
}

// DeleteAutoUpgradeTask removes an auto-upgrade task by ID.
func (r *repository) DeleteAutoUpgradeTask(id int64) error {
	return r.db.Where("id = ?", id).Delete(&AutoUpgradeTask{}).Error
}

// ---------------------------------------------------------------------------
// UpgradeFile update
// ---------------------------------------------------------------------------

// UpdateUpgradeFile saves changes to an existing upgrade file.
func (r *repository) UpdateUpgradeFile(f *UpgradeFile) error {
	return r.db.Save(f).Error
}

// FindUpgradeFileByID returns a single upgrade file by ID.
func (r *repository) FindUpgradeFileByID(id int) (*UpgradeFile, error) {
	return r.FindByID(id)
}
