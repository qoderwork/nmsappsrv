package nmsbackup

import (
	"context"
	"encoding/json"

	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

const (
	backupRetentionConfigKey = "backup_and_restore_config"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// --- Schedule definition operations (nms_backup_and_revert) ---

func (r *Repository) CreateSchedule(schedule *NMSBackupAndRevert) error {
	return r.db.Create(schedule).Error
}

func (r *Repository) GetScheduleById(id int) (*NMSBackupAndRevert, error) {
	var schedule NMSBackupAndRevert
	err := r.db.Where("id = ?", id).First(&schedule).Error
	if err != nil {
		return nil, err
	}
	return &schedule, nil
}

func (r *Repository) UpdateSchedule(schedule *NMSBackupAndRevert) error {
	return r.db.Save(schedule).Error
}

func (r *Repository) DeleteSchedule(id int) error {
	return r.db.Where("id = ?", id).Delete(&NMSBackupAndRevert{}).Error
}

func (r *Repository) ListSchedules(licenseId int, page, pageSize int) ([]NMSBackupAndRevert, int64, error) {
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
func (r *Repository) FindScheduledSchedules() ([]NMSBackupAndRevert, error) {
	var schedules []NMSBackupAndRevert
	err := r.db.Where("backup_type = ? AND backup_begin_time IS NOT NULL", 1).Find(&schedules).Error
	return schedules, err
}

// --- Task operations (nms_backup_and_revert_task) ---

func (r *Repository) CreateTask(task *NMSBackupAndRevertTask) error {
	return r.db.Create(task).Error
}

func (r *Repository) GetTaskById(id int) (*NMSBackupAndRevertTask, error) {
	var task NMSBackupAndRevertTask
	err := r.db.Where("id = ?", id).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *Repository) UpdateTask(task *NMSBackupAndRevertTask) error {
	return r.db.Save(task).Error
}

func (r *Repository) DeleteTask(id int) error {
	return r.db.Where("id = ?", id).Delete(&NMSBackupAndRevertTask{}).Error
}

// --- Log operations (nms_backup_and_revert_log) ---

func (r *Repository) CreateLog(log *NMSBackupAndRevertLog) error {
	return r.db.Create(log).Error
}

func (r *Repository) UpdateLog(log *NMSBackupAndRevertLog) error {
	return r.db.Save(log).Error
}

func (r *Repository) GetLogById(id int) (*NMSBackupAndRevertLog, error) {
	var log NMSBackupAndRevertLog
	err := r.db.Where("id = ?", id).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *Repository) ListLogs(page, pageSize int) ([]NMSBackupAndRevertLog, int64, error) {
	var logs []NMSBackupAndRevertLog
	var total int64

	query := r.db.Model(&NMSBackupAndRevertLog{})
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&logs).Error
	return logs, total, err
}

// --- Retention config operations (via system_config + Redis cache) ---

func (r *Repository) GetRetentionConfig() (*BackupRetentionConfig, error) {
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

func (r *Repository) UpdateRetentionConfig(config *BackupRetentionConfig) error {
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

func (r *Repository) GetDB() *gorm.DB {
	return r.db
}
