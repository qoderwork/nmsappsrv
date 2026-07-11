package cacert

import (
	"context"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/pkg/baserepo"
)

// Repository handles database operations for CA certificate module.
// It embeds BaseRepository[CaFile, int] for standard CRUD on CaFile,
// and retains module-specific methods for custom queries and other entity types.
type Repository struct {
	*baserepo.BaseRepository[CaFile, int]
	db *gorm.DB
}

// NewRepository creates a new Repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		BaseRepository: baserepo.New[CaFile, int](db, "id"),
		db:             db,
	}
}

// ---------- CaFile ----------

// ListCaFiles returns paginated CA file list ordered by create_time desc
func (r *Repository) ListCaFiles(ctx context.Context, page, pageSize int) ([]CaFile, int64, error) {
	var files []CaFile
	var total int64

	q := r.db.WithContext(ctx).Model(&CaFile{}).Where("del_flag = ? OR del_flag IS NULL", "0")
	q.Count(&total)

	offset := (page - 1) * pageSize
	err := q.Order("create_time DESC").Offset(offset).Limit(pageSize).Find(&files).Error
	return files, total, err
}

// DeleteCaFiles soft-deletes CA files by setting del_flag = 'Y'
func (r *Repository) DeleteCaFiles(ctx context.Context, ids []int) error {
	return r.db.WithContext(ctx).Model(&CaFile{}).
		Where("id IN ?", ids).
		Update("del_flag", "Y").Error
}

// ListAllCaFiles returns all non-deleted CA files (for dropdown)
func (r *Repository) ListAllCaFiles(ctx context.Context) ([]CaFile, error) {
	var files []CaFile
	err := r.db.WithContext(ctx).Model(&CaFile{}).
		Where("del_flag = ? OR del_flag IS NULL", "0").
		Select("id, file_name").
		Order("create_time DESC").
		Find(&files).Error
	return files, err
}

// ---------- CaTask ----------

// ListCaTasks returns paginated CA task list, optionally filtered by tenancyId
func (r *Repository) ListCaTasks(ctx context.Context, page, pageSize int, tenancyId *int) ([]CaTask, int64, error) {
	var tasks []CaTask
	var total int64

	q := r.db.WithContext(ctx).Model(&CaTask{})
	if tenancyId != nil {
		q = q.Where("tenancy_id = ?", *tenancyId)
	}
	q.Count(&total)

	offset := (page - 1) * pageSize
	err := q.Order("create_time DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error
	return tasks, total, err
}

// CreateCaTask inserts a new CA task
func (r *Repository) CreateCaTask(ctx context.Context, task *CaTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

// GetCaTaskByID returns a single CA task by ID
func (r *Repository) GetCaTaskByID(ctx context.Context, id int) (*CaTask, error) {
	var task CaTask
	err := r.db.WithContext(ctx).First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// DeleteCaTasks deletes CA tasks by IDs
func (r *Repository) DeleteCaTasks(ctx context.Context, ids []int) error {
	return r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&CaTask{}).Error
}

// ---------- DeviceSendCaLog ----------

// CreateDeviceSendCaLogs batch-inserts device CA delivery logs
func (r *Repository) CreateDeviceSendCaLogs(ctx context.Context, logs []DeviceSendCaLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

// ListDeviceSendCaLogs returns paginated device CA logs filtered by taskId
func (r *Repository) ListDeviceSendCaLogs(ctx context.Context, taskId int, page, pageSize int) ([]DeviceSendCaLog, int64, error) {
	var logs []DeviceSendCaLog
	var total int64

	q := r.db.WithContext(ctx).Model(&DeviceSendCaLog{}).Where("task_id = ?", taskId)
	q.Count(&total)

	offset := (page - 1) * pageSize
	err := q.Order("id DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

// ---------- helpers ----------

func timeNow() *time.Time {
	t := time.Now()
	return &t
}
