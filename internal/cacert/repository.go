package cacert

import (
	"context"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/pkg/baserepo"
)

// Repository defines the data-access contract for CA certificate module.
type Repository interface {
	Create(entity *CaFile) error
	Save(entity *CaFile) error
	FindByID(id int) (*CaFile, error)
	DeleteByID(id int) error
	DeleteByIDs(ids []int) error
	SoftDelete(id int) error
	UpdateFields(id int, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]CaFile, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[CaFile], error)
	ListCaFiles(ctx context.Context, page, pageSize int) ([]CaFile, int64, error)
	DeleteCaFiles(ctx context.Context, ids []int) error
	ListAllCaFiles(ctx context.Context) ([]CaFile, error)
	ListCaTasks(ctx context.Context, page, pageSize int, tenantId *int) ([]CaTask, int64, error)
	CreateCaTask(ctx context.Context, task *CaTask) error
	GetCaTaskByID(ctx context.Context, id int) (*CaTask, error)
	DeleteCaTasks(ctx context.Context, ids []int) error
	CreateDeviceSendCaLogs(ctx context.Context, logs []DeviceSendCaLog) error
	ListDeviceSendCaLogs(ctx context.Context, taskId int, page, pageSize int) ([]DeviceSendCaLog, int64, error)
	// DB exposes the underlying *gorm.DB for module-specific queries used by
	// the service layer helpers that are not part of the generic contract.
	DB() *gorm.DB
}

// repository is the concrete GORM-backed implementation of Repository.
// It embeds BaseRepository[CaFile, int] for standard CRUD on CaFile,
// and retains module-specific methods for custom queries and other entity types.
type repository struct {
	*baserepo.BaseRepository[CaFile, int]
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[CaFile, int](db, "id"),
		db:             db,
	}
}

// DB returns the underlying *gorm.DB handle.
func (r *repository) DB() *gorm.DB {
	return r.db
}

// ---------- CaFile ----------

// ListCaFiles returns paginated CA file list ordered by create_time desc
func (r *repository) ListCaFiles(ctx context.Context, page, pageSize int) ([]CaFile, int64, error) {
	var files []CaFile
	var total int64

	q := r.db.WithContext(ctx).Model(&CaFile{}).Where("del_flag = ? OR del_flag IS NULL", "0")
	q.Count(&total)

	offset := (page - 1) * pageSize
	err := q.Order("create_time DESC").Offset(offset).Limit(pageSize).Find(&files).Error
	return files, total, err
}

// DeleteCaFiles soft-deletes CA files by setting del_flag = 'Y'
func (r *repository) DeleteCaFiles(ctx context.Context, ids []int) error {
	return r.db.WithContext(ctx).Model(&CaFile{}).
		Where("id IN ?", ids).
		Update("del_flag", "Y").Error
}

// ListAllCaFiles returns all non-deleted CA files (for dropdown)
func (r *repository) ListAllCaFiles(ctx context.Context) ([]CaFile, error) {
	var files []CaFile
	err := r.db.WithContext(ctx).Model(&CaFile{}).
		Where("del_flag = ? OR del_flag IS NULL", "0").
		Select("id, file_name").
		Order("create_time DESC").
		Find(&files).Error
	return files, err
}

// ---------- CaTask ----------

// ListCaTasks returns paginated CA task list, optionally filtered by tenantId
func (r *repository) ListCaTasks(ctx context.Context, page, pageSize int, tenantId *int) ([]CaTask, int64, error) {
	var tasks []CaTask
	var total int64

	q := r.db.WithContext(ctx).Model(&CaTask{})
	if tenantId != nil {
		q = q.Where("tenant_id = ?", *tenantId)
	}
	q.Count(&total)

	offset := (page - 1) * pageSize
	err := q.Order("create_time DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error
	return tasks, total, err
}

// CreateCaTask inserts a new CA task
func (r *repository) CreateCaTask(ctx context.Context, task *CaTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

// GetCaTaskByID returns a single CA task by ID
func (r *repository) GetCaTaskByID(ctx context.Context, id int) (*CaTask, error) {
	var task CaTask
	err := r.db.WithContext(ctx).First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// DeleteCaTasks deletes CA tasks by IDs
func (r *repository) DeleteCaTasks(ctx context.Context, ids []int) error {
	return r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&CaTask{}).Error
}

// ---------- DeviceSendCaLog ----------

// CreateDeviceSendCaLogs batch-inserts device CA delivery logs
func (r *repository) CreateDeviceSendCaLogs(ctx context.Context, logs []DeviceSendCaLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

// ListDeviceSendCaLogs returns paginated device CA logs filtered by taskId
func (r *repository) ListDeviceSendCaLogs(ctx context.Context, taskId int, page, pageSize int) ([]DeviceSendCaLog, int64, error) {
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
