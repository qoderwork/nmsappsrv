package logcleanup

import (
	"time"

	"gorm.io/gorm"
	"nmsappsrv/pkg/logger"
)

// Repository handles database cleanup operations for log deletion.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// DeleteAlarmLogBefore deletes alarm records older than the given date.
func (r *Repository) DeleteAlarmLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("log_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteEventLogBefore deletes event_log records older than the given date.
func (r *Repository) DeleteEventLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("operation_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteParameterLogBefore deletes parameter_log records older than the given date.
func (r *Repository) DeleteParameterLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("log_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteMmlExecuteResultBefore deletes mml_execute_result records older than the given date.
func (r *Repository) DeleteMmlExecuteResultBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("operation_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteBatchProcessFileSendLogBefore deletes batch_process_file_send_log records older than the given date.
func (r *Repository) DeleteBatchProcessFileSendLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("operation_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteDeviceLogFileLogBefore deletes device_log_file_log records older than the given date.
func (r *Repository) DeleteDeviceLogFileLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("log_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteCaptureFileLogBefore deletes capture_file_log records older than the given date.
func (r *Repository) DeleteCaptureFileLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("upload_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// GetRebootTaskIDsBefore returns reboot_task IDs older than the given date.
func (r *Repository) GetRebootTaskIDsBefore(cutoff time.Time) ([]int, error) {
	var ids []int
	err := r.db.Model(&struct {
		ID            int       `gorm:"primaryKey"`
		OperationTime time.Time `gorm:"column:operation_time"`
	}{}).Where("operation_time < ?", cutoff).Pluck("id", &ids).Error
	return ids, err
}

// DeleteTaskToEventLogByTaskIDs deletes task_to_event_log records by task IDs and task type.
func (r *Repository) DeleteTaskToEventLogByTaskIDs(ids []int, taskType string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Where("task_id IN ? AND task_type = ?", ids, taskType).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteRebootTaskByIDs deletes reboot_task records by IDs.
func (r *Repository) DeleteRebootTaskByIDs(ids []int) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Where("id IN ?", ids).Delete(&struct {
		ID int `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// GetShutdownTaskIDsBefore returns shutdown_task IDs older than the given date.
func (r *Repository) GetShutdownTaskIDsBefore(cutoff time.Time) ([]int, error) {
	var ids []int
	err := r.db.Model(&struct {
		ID            int       `gorm:"primaryKey"`
		OperationTime time.Time `gorm:"column:operation_time"`
	}{}).Where("operation_time < ?", cutoff).Pluck("id", &ids).Error
	return ids, err
}

// DeleteShutdownLogByTaskIDs deletes shutdown_log records by task IDs.
func (r *Repository) DeleteShutdownLogByTaskIDs(ids []int) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Where("task_id IN ?", ids).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteShutdownTaskByIDs deletes shutdown_task records by IDs.
func (r *Repository) DeleteShutdownTaskByIDs(ids []int) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Where("id IN ?", ids).Delete(&struct {
		ID int `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// GetBackupOrRestoreTaskIDsBefore returns backup_or_restore_task IDs older than the given date.
func (r *Repository) GetBackupOrRestoreTaskIDsBefore(cutoff time.Time) ([]int, error) {
	var ids []int
	err := r.db.Model(&struct {
		ID        int       `gorm:"primaryKey"`
		Operation time.Time `gorm:"column:operation_time"`
	}{}).Where("operation_time < ?", cutoff).Pluck("id", &ids).Error
	return ids, err
}

// DeleteBackupOrRestoreTaskByIDs deletes backup_or_restore_task records by IDs.
func (r *Repository) DeleteBackupOrRestoreTaskByIDs(ids []int) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Where("id IN ?", ids).Delete(&struct {
		ID int `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// GetUpgradeTaskIDsBefore returns upgrade_task IDs older than the given date.
func (r *Repository) GetUpgradeTaskIDsBefore(cutoff time.Time) ([]int, error) {
	var ids []int
	err := r.db.Model(&struct {
		ID            int       `gorm:"primaryKey"`
		OperationTime time.Time `gorm:"column:operation_time"`
	}{}).Where("operation_time < ?", cutoff).Pluck("id", &ids).Error
	return ids, err
}

// DeleteUpgradeTaskByIDs deletes upgrade_task records by IDs.
func (r *Repository) DeleteUpgradeTaskByIDs(ids []int) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Where("id IN ?", ids).Delete(&struct {
		ID int `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// GetRollbackTaskIDsBefore returns rollback_task IDs older than the given date.
func (r *Repository) GetRollbackTaskIDsBefore(cutoff time.Time) ([]int, error) {
	var ids []int
	err := r.db.Model(&struct {
		ID            int       `gorm:"primaryKey"`
		OperationTime time.Time `gorm:"column:operation_time"`
	}{}).Where("operation_time < ?", cutoff).Pluck("id", &ids).Error
	return ids, err
}

// DeleteRollbackTaskByIDs deletes rollback_task records by IDs.
func (r *Repository) DeleteRollbackTaskByIDs(ids []int) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Where("id IN ?", ids).Delete(&struct {
		ID int `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// GetBatchConfigurationLogIDsBefore returns batch_configuration_log IDs older than the given date.
func (r *Repository) GetBatchConfigurationLogIDsBefore(cutoff time.Time) ([]int64, error) {
	var ids []int64
	err := r.db.Model(&struct {
		ID            int64     `gorm:"primaryKey"`
		OperationTime time.Time `gorm:"column:operation_time"`
	}{}).Where("operation_time < ?", cutoff).Pluck("id", &ids).Error
	return ids, err
}

// DeleteBatchConfigurationDeviceLogByTaskIDs deletes batch_configuration_device_log records by task IDs.
func (r *Repository) DeleteBatchConfigurationDeviceLogByTaskIDs(ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Where("task_id IN ?", ids).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteBatchConfigurationLogByIDs deletes batch_configuration_log records by IDs.
func (r *Repository) DeleteBatchConfigurationLogByIDs(ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Where("id IN ?", ids).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteEmailNoticeResultBefore deletes email_notice_result records older than the given date.
func (r *Repository) DeleteEmailNoticeResultBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("log_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteEUAndRUBatchUpgradeLogBefore deletes eu_and_ru_batch_upgrade_log records older than the given date.
func (r *Repository) DeleteEUAndRUBatchUpgradeLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("log_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteUpgradeLogBefore deletes upgrade_log records older than the given date.
func (r *Repository) DeleteUpgradeLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("create_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteCoreNetworkOperationLogBefore deletes core_network_operation_log records older than the given date.
func (r *Repository) DeleteCoreNetworkOperationLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("operation_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeletePDCPTrafficBefore deletes pdcp_traffic records older than the given date.
func (r *Repository) DeletePDCPTrafficBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("statistic_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteDashboardPmStatisticDataBefore deletes dashboard_pm_statistic_data records older than the given date.
func (r *Repository) DeleteDashboardPmStatisticDataBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteSystemOperatorLogBefore deletes system_operator_log records older than the given date.
func (r *Repository) DeleteSystemOperatorLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("operation_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteNorthInterfaceLogBefore deletes north_interface_log records older than the given date.
func (r *Repository) DeleteNorthInterfaceLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("operation_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteCbrsLogBefore deletes cbrs_log records older than the given date.
func (r *Repository) DeleteCbrsLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("operation_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteLoginLogBefore deletes login_log records older than the given date.
func (r *Repository) DeleteLoginLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("operation_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeletePMFileLogBefore deletes pm_file_log records older than the given date.
func (r *Repository) DeletePMFileLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("start_time < ?", cutoff).Delete(&struct {
		ID string `gorm:"primaryKey;type:varchar(32)"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteMRFileLogBefore deletes mr_file_log records older than the given date.
func (r *Repository) DeleteMRFileLogBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("collection_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteMRDataBefore deletes mr_data records older than the given date.
func (r *Repository) DeleteMRDataBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("collection_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// DeleteAlarmBefore deletes alarm records older than the given date.
func (r *Repository) DeleteAlarmBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("log_time < ?", cutoff).Delete(&struct {
		ID int64 `gorm:"primaryKey"`
	}{})
	return result.RowsAffected, result.Error
}

// safeDelete wraps a delete operation with error logging.
func safeDelete(name string, fn func() (int64, error)) {
	n, err := fn()
	if err != nil {
		logger.Errorf("logcleanup: failed to delete %s: %v", name, err)
	} else if n > 0 {
		logger.Infof("logcleanup: deleted %d rows from %s", n, name)
	}
}
