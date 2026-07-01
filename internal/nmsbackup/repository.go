package nmsbackup

import (
	"context"
	"encoding/json"
	"fmt"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

const (
	backupRetentionConfigKey = "nms_backup_retention"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// --- Task operations ---

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

func (r *Repository) ListTasks(licenseId int, page, pageSize int) ([]NMSBackupAndRevertTask, int64, error) {
	var tasks []NMSBackupAndRevertTask
	var total int64

	query := r.db.Model(&NMSBackupAndRevertTask{}).Where("license_id = ?", licenseId)
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&tasks).Error
	return tasks, total, err
}

// FindScheduledTasks returns all backup tasks that are scheduled (execute_mode=2)
// with a non-empty cron_expr.
func (r *Repository) FindScheduledTasks() ([]NMSBackupAndRevertTask, error) {
	var tasks []NMSBackupAndRevertTask
	err := r.db.Where("execute_mode = ? AND cron_expr IS NOT NULL AND cron_expr != ''", 2).Find(&tasks).Error
	return tasks, err
}

// --- Backup record operations ---

func (r *Repository) CreateBackupRecord(record *NMSBackupAndRevert) error {
	return r.db.Create(record).Error
}

func (r *Repository) GetBackupById(id int) (*NMSBackupAndRevert, error) {
	var record NMSBackupAndRevert
	err := r.db.Where("id = ?", id).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *Repository) DeleteBackupRecordsByTaskId(taskId int) error {
	return r.db.Where("task_id = ?", taskId).Delete(&NMSBackupAndRevert{}).Error
}

// --- Log operations ---

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

func (r *Repository) ListLogs(taskId int, page, pageSize int) ([]NMSBackupAndRevertLog, int64, error) {
	var logs []NMSBackupAndRevertLog
	var total int64

	query := r.db.Model(&NMSBackupAndRevertLog{})
	if taskId > 0 {
		query = query.Where("task_id = ?", taskId)
	}
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
		// Return defaults
		return &BackupRetentionConfig{
			MaxBackupCount: intPtr(10),
			RetentionDays:  intPtr(30),
			AutoCleanup:    boolPtr(false),
		}, nil
	}

	var config BackupRetentionConfig
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		return nil, fmt.Errorf("failed to parse retention config: %w", err)
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
		return fmt.Errorf("failed to marshal retention config: %w", err)
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

// --- Helper functions ---

func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}
