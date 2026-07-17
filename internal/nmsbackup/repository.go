package nmsbackup

import (
	"context"
	"encoding/json"

	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

const (
	backupRetentionConfigKey = "backup_and_restore_config"
)

// Repository provides data access for NMS backup entities.
// It embeds BaseRepository[NMSBackupAndRevert, int] for standard CRUD on NMSBackupAndRevert (schedule definitions),
// and retains module-specific methods for tasks, logs, and config operations.
type Repository interface {
	// Standard CRUD (promoted from BaseRepository[NMSBackupAndRevert, int])
	Create(entity *NMSBackupAndRevert) error
	Save(entity *NMSBackupAndRevert) error
	FindByID(id int) (*NMSBackupAndRevert, error)
	DeleteByID(id int) error
	DeleteByIDs(ids []int) error
	SoftDelete(id int) error
	UpdateFields(id int, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]NMSBackupAndRevert, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[NMSBackupAndRevert], error)

	// Module-specific queries
	ListSchedules(licenseId int, page, pageSize int) ([]NMSBackupAndRevert, int64, error)
	FindScheduledSchedules() ([]NMSBackupAndRevert, error)
	FindByBackupName(name string) ([]NMSBackupAndRevert, error)
	FindAnyRunning() (*NMSBackupAndRevert, error)
	CreateTask(task *NMSBackupAndRevertTask) error
	GetTaskById(id int) (*NMSBackupAndRevertTask, error)
	UpdateTask(task *NMSBackupAndRevertTask) error
	DeleteTask(id int) error
	CreateLog(log *NMSBackupAndRevertLog) error
	UpdateLog(log *NMSBackupAndRevertLog) error
	GetLogById(id int) (*NMSBackupAndRevertLog, error)
	ListLogs(page, pageSize int) ([]NMSBackupAndRevertLog, int64, error)
	GetRetentionConfig() (*BackupRetentionConfig, error)
	UpdateRetentionConfig(config *BackupRetentionConfig) error
	GetDB() *gorm.DB
}

// repository implements Repository.
type repository struct {
	*baserepo.BaseRepository[NMSBackupAndRevert, int]
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[NMSBackupAndRevert, int](db, "id"),
		db:             db,
	}
}

// ---------------------------------------------------------------------------
// NMSBackupAndRevert – module-specific queries (base provides Create/Save/FindByID/DeleteByID)
// ---------------------------------------------------------------------------

// ListSchedules returns paginated backup schedules for the given license.
func (r *repository) ListSchedules(licenseId int, page, pageSize int) ([]NMSBackupAndRevert, int64, error) {
	var schedules []NMSBackupAndRevert
	var total int64

	query := r.db.Model(&NMSBackupAndRevert{}).Where("license_id = ?", licenseId)
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&schedules).Error
	return schedules, total, err
}

// FindScheduledSchedules returns all backup schedules that are recurring (backup_type=1)
// with a non-nil backup_begin_time.
func (r *repository) FindScheduledSchedules() ([]NMSBackupAndRevert, error) {
	var schedules []NMSBackupAndRevert
	err := r.db.Where("backup_type = ? AND backup_begin_time IS NOT NULL", 1).Find(&schedules).Error
	return schedules, err
}

// FindByBackupName returns schedules whose backup_name matches the given name.
// Mirrors Java nmsBackupAndRevertRepository.findByBackupName(name).
func (r *repository) FindByBackupName(name string) ([]NMSBackupAndRevert, error) {
	var schedules []NMSBackupAndRevert
	err := r.db.Where("backup_name = ?", name).Find(&schedules).Error
	return schedules, err
}

// FindAnyRunning returns the first schedule with backup_status=1 (running),
// or nil if none. Mirrors Java NMSBackupTaskJob's loop checking for any
// schedule with backupStatus==1 before starting a new backup.
func (r *repository) FindAnyRunning() (*NMSBackupAndRevert, error) {
	var schedule NMSBackupAndRevert
	err := r.db.Where("backup_status = ?", 1).First(&schedule).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &schedule, nil
}

// ---------------------------------------------------------------------------
// NMSBackupAndRevertTask – different entity type, kept as module-specific methods
// ---------------------------------------------------------------------------

func (r *repository) CreateTask(task *NMSBackupAndRevertTask) error {
	return r.db.Create(task).Error
}

func (r *repository) GetTaskById(id int) (*NMSBackupAndRevertTask, error) {
	var task NMSBackupAndRevertTask
	err := r.db.Where("id = ?", id).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *repository) UpdateTask(task *NMSBackupAndRevertTask) error {
	return r.db.Save(task).Error
}

func (r *repository) DeleteTask(id int) error {
	return r.db.Where("id = ?", id).Delete(&NMSBackupAndRevertTask{}).Error
}

// ---------------------------------------------------------------------------
// NMSBackupAndRevertLog – different entity type, kept as module-specific methods
// ---------------------------------------------------------------------------

func (r *repository) CreateLog(log *NMSBackupAndRevertLog) error {
	return r.db.Create(log).Error
}

func (r *repository) UpdateLog(log *NMSBackupAndRevertLog) error {
	return r.db.Save(log).Error
}

func (r *repository) GetLogById(id int) (*NMSBackupAndRevertLog, error) {
	var log NMSBackupAndRevertLog
	err := r.db.Where("id = ?", id).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *repository) ListLogs(page, pageSize int) ([]NMSBackupAndRevertLog, int64, error) {
	var logs []NMSBackupAndRevertLog
	var total int64

	query := r.db.Model(&NMSBackupAndRevertLog{})
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&logs).Error
	return logs, total, err
}

// ---------------------------------------------------------------------------
// Retention config operations (via system_config + Redis cache)
// ---------------------------------------------------------------------------

func (r *repository) GetRetentionConfig() (*BackupRetentionConfig, error) {
	ctx := context.Background()

	// Try Redis cache first
	cached, err := redis.Get(ctx, backupRetentionConfigKey)
	if err == nil && cached != "" {
		var config BackupRetentionConfig
		if json.Unmarshal([]byte(cached), &config) == nil {
			return &config, nil
		}
	}

	// Fallback to database
	var value string
	err = r.db.Table("system_config").
		Where("id = ?", backupRetentionConfigKey).
		Pluck("config", &value).Error
	if err != nil {
		return nil, err
	}

	if value == "" {
		// Return defaults (matches Java default: 7 days)
		return &BackupRetentionConfig{
			BackupFileSavedDays: intPtr(7),
		}, nil
	}

	var config BackupRetentionConfig
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		return nil, apperror.Wrap(err, "INTERNAL", 500, "failed to parse retention config")
	}

	// Cache to Redis
	if jsonData, err := json.Marshal(config); err == nil {
		redis.Set(ctx, backupRetentionConfigKey, string(jsonData), 0)
	}

	return &config, nil
}

func (r *repository) UpdateRetentionConfig(config *BackupRetentionConfig) error {
	ctx := context.Background()

	jsonData, err := json.Marshal(config)
	if err != nil {
		return apperror.Wrap(err, "INTERNAL", 500, "failed to marshal retention config")
	}

	// Update database
	var existing string
	err = r.db.Table("system_config").
		Where("id = ?", backupRetentionConfigKey).
		Pluck("config", &existing).Error
	if err != nil {
		return err
	}

	if existing == "" {
		// Insert new record
		err = r.db.Table("system_config").Create(map[string]interface{}{
			"id":     backupRetentionConfigKey,
			"config": string(jsonData),
		}).Error
	} else {
		// Update existing record
		err = r.db.Table("system_config").
			Where("id = ?", backupRetentionConfigKey).
			Update("config", string(jsonData)).Error
	}
	if err != nil {
		return err
	}

	// Update Redis cache
	redis.Set(ctx, backupRetentionConfigKey, string(jsonData), 0)

	logger.Infof("Updated backup retention config: %+v", config)
	return nil
}

func (r *repository) GetDB() *gorm.DB {
	return r.db
}
