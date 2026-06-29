package misc

import (
	"time"

	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository handles database operations for miscellaneous entities.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// FindBackupOrRestoreTasks returns a paginated list of backup/restore tasks.
func (r *Repository) FindBackupOrRestoreTasks(tenancyId int, offset, limit int) ([]BackupOrRestoreTask, int64, error) {
	var tasks []BackupOrRestoreTask
	var total int64

	query := r.db.Model(&BackupOrRestoreTask{}).Where("tenancy_id = ?", tenancyId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindBackupOrRestoreTasks count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		logger.Errorf("FindBackupOrRestoreTasks query error: %v", err)
		return nil, 0, err
	}
	return tasks, total, nil
}

// CreateBackupOrRestoreTask inserts a new backup/restore task.
func (r *Repository) CreateBackupOrRestoreTask(t *BackupOrRestoreTask) error {
	return r.db.Create(t).Error
}

// FindBatchConfigLogs returns a paginated list of batch configuration logs.
func (r *Repository) FindBatchConfigLogs(tenancyId int, offset, limit int) ([]BatchConfigurationLog, int64, error) {
	var logs []BatchConfigurationLog
	var total int64

	query := r.db.Model(&BatchConfigurationLog{}).Where("tenancy_id = ?", tenancyId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindBatchConfigLogs count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		logger.Errorf("FindBatchConfigLogs query error: %v", err)
		return nil, 0, err
	}
	return logs, total, nil
}

// FindMRData returns a paginated list of MR data records for the given element.
func (r *Repository) FindMRData(elementId int64, offset, limit int) ([]MRData, int64, error) {
	var data []MRData
	var total int64

	query := r.db.Model(&MRData{}).Where("element_id = ?", elementId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindMRData count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&data).Error; err != nil {
		logger.Errorf("FindMRData query error: %v", err)
		return nil, 0, err
	}
	return data, total, nil
}

// FindZTPLogs returns all ZTP logs for the given element.
func (r *Repository) FindZTPLogs(elementId int64) ([]ZTPLog, error) {
	var logs []ZTPLog
	if err := r.db.Where("element_id = ?", elementId).Order("id DESC").Find(&logs).Error; err != nil {
		logger.Errorf("FindZTPLogs error: %v", err)
		return nil, err
	}
	return logs, nil
}

// CreateZTPLog inserts a new ZTP log record.
func (r *Repository) CreateZTPLog(log *ZTPLog) error {
	return r.db.Create(log).Error
}

// FindNorthReports returns all north reports for the given license.
func (r *Repository) FindNorthReports(licenseId int) ([]NorthReport, error) {
	var reports []NorthReport
	if err := r.db.Where("license_id = ? AND (deleted = ? OR deleted IS NULL)", licenseId, 0).Find(&reports).Error; err != nil {
		logger.Errorf("FindNorthReports error: %v", err)
		return nil, err
	}
	return reports, nil
}

// CreateNorthReport inserts a new north report.
func (r *Repository) CreateNorthReport(report *NorthReport) error {
	return r.db.Create(report).Error
}

// UpdateNorthReport saves changes to an existing north report.
func (r *Repository) UpdateNorthReport(report *NorthReport) error {
	return r.db.Save(report).Error
}

// DeleteNorthReport removes a north report by ID.
func (r *Repository) DeleteNorthReport(id int) error {
	return r.db.Where("id = ?", id).Delete(&NorthReport{}).Error
}

// FindRadius returns all RADIUS configurations for the given tenancy.
func (r *Repository) FindRadius(tenancyId int) ([]Radius, error) {
	var list []Radius
	if err := r.db.Where("tenancy_id = ?", tenancyId).Find(&list).Error; err != nil {
		logger.Errorf("FindRadius error: %v", err)
		return nil, err
	}
	return list, nil
}

// SaveRadius inserts or updates a RADIUS configuration.
func (r *Repository) SaveRadius(rad *Radius) error {
	if rad.Id == 0 {
		return r.db.Create(rad).Error
	}
	return r.db.Save(rad).Error
}

// DeleteRadius removes a RADIUS configuration by ID.
func (r *Repository) DeleteRadius(id int) error {
	return r.db.Where("id = ?", id).Delete(&Radius{}).Error
}

// CreateOperatorLog inserts a new operator log record.
func (r *Repository) CreateOperatorLog(log *SystemOperatorLog) error {
	return r.db.Create(log).Error
}

// FindOperatorLogs returns a paginated list of operator logs.
func (r *Repository) FindOperatorLogs(tenancyId int, offset, limit int) ([]SystemOperatorLog, int64, error) {
	var logs []SystemOperatorLog
	var total int64

	query := r.db.Model(&SystemOperatorLog{}).Where("tenancy_id = ?", tenancyId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindOperatorLogs count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		logger.Errorf("FindOperatorLogs query error: %v", err)
		return nil, 0, err
	}
	return logs, total, nil
}

// FindUploadFiles returns a paginated list of uploaded files.
func (r *Repository) FindUploadFiles(offset, limit int) ([]UploadFile, int64, error) {
	var files []UploadFile
	var total int64

	query := r.db.Model(&UploadFile{})

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindUploadFiles count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("create_time DESC").Offset(offset).Limit(limit).Find(&files).Error; err != nil {
		logger.Errorf("FindUploadFiles query error: %v", err)
		return nil, 0, err
	}
	return files, total, nil
}

// CreateUploadFile inserts a new upload file record.
func (r *Repository) CreateUploadFile(f *UploadFile) error {
	return r.db.Create(f).Error
}

// DeleteUploadFile removes an upload file by ID.
func (r *Repository) DeleteUploadFile(id string) error {
	return r.db.Where("file_id = ?", id).Delete(&UploadFile{}).Error
}

// FindErrorInfos returns all error info records for the given tenancy.
func (r *Repository) FindErrorInfos(tenancyId int) ([]ErrorInfo, error) {
	var infos []ErrorInfo
	if err := r.db.Where("tenancy_id = ?", tenancyId).Find(&infos).Error; err != nil {
		logger.Errorf("FindErrorInfos error: %v", err)
		return nil, err
	}
	return infos, nil
}

// CreateErrorInfo inserts a new error info record.
func (r *Repository) CreateErrorInfo(e *ErrorInfo) error {
	return r.db.Create(e).Error
}

// ---------------------------------------------------------------------------
// BatchAddObjectTask
// ---------------------------------------------------------------------------

// CreateBatchAddObjectTask inserts a new batch-add-object task.
func (r *Repository) CreateBatchAddObjectTask(t *BatchAddObjectTask) error {
	return r.db.Create(t).Error
}

// FindBatchAddObjectTasks returns a paginated list of tasks for the given tenancy.
func (r *Repository) FindBatchAddObjectTasks(tenancyId int, offset, limit int) ([]BatchAddObjectTask, int64, error) {
	var tasks []BatchAddObjectTask
	var total int64

	query := r.db.Model(&BatchAddObjectTask{}).Where("tenancy_id = ?", tenancyId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindBatchAddObjectTasks count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		logger.Errorf("FindBatchAddObjectTasks query error: %v", err)
		return nil, 0, err
	}
	return tasks, total, nil
}

// CreateBatchAddObjectTaskLog inserts a task-to-eventlog link.
func (r *Repository) CreateBatchAddObjectTaskLog(log *BatchAddObjectTaskLog) error {
	return r.db.Create(log).Error
}

// BatchAddObjectTaskProgress returns (total, success_count) for a given task.
func (r *Repository) BatchAddObjectTaskProgress(taskId int) (total int64, success int64, err error) {
	type row struct {
		Total   int64
		Success int64
	}
	var res row
	err = r.db.Raw(`
		SELECT COUNT(*) AS total,
		       SUM(CASE WHEN el.status = 3 THEN 1 ELSE 0 END) AS success
		FROM batch_add_object_task_log tl
		JOIN event_log el ON el.id = tl.event_log_id
		WHERE tl.task_id = ?
	`, taskId).Scan(&res).Error
	return res.Total, res.Success, err
}

// BatchAddObjectTaskDetail returns per-device results for a task.
func (r *Repository) BatchAddObjectTaskDetail(taskId int) ([]BatchAddObjectTaskDetailVo, error) {
	var list []BatchAddObjectTaskDetailVo
	err := r.db.Raw(`
		SELECT ce.device_name  AS device_name,
		       ce.serial_number AS serial_number,
		       el.status       AS result,
		       IFNULL(el.fault_info, '') AS fault_info,
		       ce.ne_neid      AS element_id,
		       ''              AS tenancy_name
		FROM batch_add_object_task_log tl
		JOIN event_log el ON el.id = tl.event_log_id
		JOIN cpe_element ce ON ce.ne_neid = el.element_id
		WHERE tl.task_id = ?
		ORDER BY tl.id
	`, taskId).Scan(&list).Error
	if err != nil {
		logger.Errorf("BatchAddObjectTaskDetail error: %v", err)
		return nil, err
	}
	return list, nil
}

// InsertEventLog creates an event_log row and returns its auto-generated ID.
// This avoids a circular import with the eventlog package.
func (r *Repository) InsertEventLog(eventType string, elementId int64, user string, status int, commandTrackData string) (int64, error) {
	row := struct {
		Id               int64  `gorm:"primaryKey;autoIncrement"`
		EventType        string `gorm:"column:event_type;type:varchar(255)"`
		OperationTime    time.Time `gorm:"column:operation_time"`
		User             string `gorm:"column:user;type:varchar(255)"`
		ElementId        int64  `gorm:"column:element_id"`
		Status           int    `gorm:"column:status"`
		CommandTrackData string `gorm:"column:command_track_data;type:longtext"`
	}{
		EventType:        eventType,
		OperationTime:    time.Now(),
		User:             user,
		ElementId:        elementId,
		Status:           status,
		CommandTrackData: commandTrackData,
	}
	// GORM will infer table name from the embedded TableName method or use
	// the struct's default; we override with a raw table hint via Session.
	if err := r.db.Table("event_log").Create(&row).Error; err != nil {
		return 0, err
	}
	return row.Id, nil
}

// DB returns the underlying *gorm.DB (used by service for Redis dispatch prep).
func (r *Repository) DB() *gorm.DB {
	return r.db
}

// ---------------------------------------------------------------------------
// Batch Backup/Restore
// ---------------------------------------------------------------------------

// FindBackupRestoreTaskById loads a single task by ID.
func (r *Repository) FindBackupRestoreTaskById(id int) (*BackupOrRestoreTask, error) {
	var task BackupOrRestoreTask
	if err := r.db.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateBackupRestoreTask saves changes to a task.
func (r *Repository) UpdateBackupRestoreTask(t *BackupOrRestoreTask) error {
	return r.db.Save(t).Error
}

// CheckDuplicateBackupRestoreTaskName checks if a task name already exists for the tenancy.
func (r *Repository) CheckDuplicateBackupRestoreTaskName(name string, tenancyId int) bool {
	var count int64
	r.db.Model(&BackupOrRestoreTask{}).Where("name = ? AND tenancy_id = ?", name, tenancyId).Count(&count)
	return count > 0
}

// CreateDeviceLog inserts a restore_and_back_up_device_log record.
func (r *Repository) CreateDeviceLog(log *RestoreAndBackUpDeviceLog) error {
	return r.db.Create(log).Error
}

// BatchBackupRestoreProgress returns (total_devices, success_count) for a task.
func (r *Repository) BatchBackupRestoreProgress(taskId int) (total int64, success int64, err error) {
	type row struct {
		Total   int64
		Success int64
	}
	var res row
	err = r.db.Raw(`
		SELECT COUNT(*) AS total,
		       SUM(CASE WHEN results = 1 THEN 1 ELSE 0 END) AS success
		FROM restore_and_back_up_device_log
		WHERE task_id = ?
	`, taskId).Scan(&res).Error
	return res.Total, res.Success, err
}

// BatchBackupRestoreResult returns the overall result for a task.
// nil = pending, 1 = all success, 2 = has failure.
func (r *Repository) BatchBackupRestoreResult(taskId int) (*int, error) {
	type row struct {
		Total    int64
		Failed   int64
		Pending  int64
	}
	var res row
	err := r.db.Raw(`
		SELECT COUNT(*) AS total,
		       SUM(CASE WHEN results = 2 THEN 1 ELSE 0 END) AS failed,
		       SUM(CASE WHEN results IS NULL THEN 1 ELSE 0 END) AS pending
		FROM restore_and_back_up_device_log
		WHERE task_id = ?
	`, taskId).Scan(&res).Error
	if err != nil {
		return nil, err
	}
	if res.Total == 0 {
		return nil, nil
	}
	if res.Pending > 0 {
		return nil, nil // still pending
	}
	if res.Failed > 0 {
		v := 2
		return &v, nil
	}
	v := 1
	return &v, nil
}

// BatchBackupRestoreDetail returns per-device results for a task.
func (r *Repository) BatchBackupRestoreDetail(taskId int) ([]BackupRestoreTaskDetailVo, error) {
	var list []BackupRestoreTaskDetailVo
	err := r.db.Raw(`
		SELECT ce.device_name          AS device_name,
		       ce.serial_number        AS serial_number,
		       ce.ne_neid              AS element_id,
		       dl.results              AS result,
		       IFNULL(dl.failure_reason, '') AS failure_reason,
		       dl.start_time           AS start_time,
		       dl.end_time             AS end_time,
		       IFNULL(dl.configuration_file, '') AS configuration_file
		FROM restore_and_back_up_device_log dl
		JOIN cpe_element ce ON ce.ne_neid = dl.element_id
		WHERE dl.task_id = ?
		ORDER BY dl.id
	`, taskId).Scan(&list).Error
	if err != nil {
		logger.Errorf("BatchBackupRestoreDetail error: %v", err)
		return nil, err
	}
	return list, nil
}

// ---------------------------------------------------------------------------
// ZTP
// ---------------------------------------------------------------------------

// FindZTPResults returns paginated ZTP results with device info.
func (r *Repository) FindZTPResults(req *ListZTPResultsRequest) ([]ZTPResultVo, int64, error) {
	var total int64

	where := "1=1"
	var args []interface{}

	if req.SearchText != "" {
		where += " AND (ce.device_name LIKE ? OR ce.serial_number LIKE ?)"
		args = append(args, "%"+req.SearchText+"%", "%"+req.SearchText+"%")
	}
	if req.Progress != nil {
		where += " AND ztp.progress = ?"
		args = append(args, *req.Progress)
	}
	if req.Succeed != nil {
		if *req.Succeed {
			where += " AND ztp.progress = 6"
		} else {
			where += " AND ztp.progress != 6"
		}
	}
	if req.DeviceGroupId != "" {
		where += " AND ce.device_group_id = ?"
		args = append(args, req.DeviceGroupId)
	}
	if req.SerialNumbers != "" {
		where += " AND ce.serial_number IN (?)"
		args = append(args, req.SerialNumbers)
	}

	countSQL := `SELECT COUNT(*) FROM ztp_log ztp JOIN cpe_element ce ON ce.ne_neid = ztp.element_id WHERE ` + where
	if err := r.db.Raw(countSQL, args...).Count(&total).Error; err != nil {
		logger.Errorf("FindZTPResults count error: %v", err)
		return nil, 0, err
	}

	page, pageSize := req.Page, req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	querySQL := `SELECT ce.ne_neid AS element_id, ce.device_name AS device_name,
		ce.serial_number AS serial_number, ztp.progress AS progress,
		IFNULL(ztp.info, '') AS info,
		CASE WHEN ztp.progress = 6 THEN 'Success' WHEN ztp.has_fault = 1 THEN 'Fault' ELSE 'In Progress' END AS result,
		IFNULL(ce.aos_file_name, '') AS ztp_file_name,
		IFNULL(ztp.start_time, '') AS start_time,
		IFNULL(ztp.end_time, '') AS end_time,
		CASE WHEN ztp.done = 1 THEN 'Done' ELSE 'Running' END AS status,
		IFNULL(ztp.progress, 0) AS current_progress,
		IFNULL(ce.wifi_or_gps_info, '') AS mode,
		IFNULL(ce.mac, '') AS mac
		FROM ztp_log ztp JOIN cpe_element ce ON ce.ne_neid = ztp.element_id
		WHERE ` + where + ` ORDER BY ztp.id DESC LIMIT ? OFFSET ?`

	queryArgs := append(args, pageSize, offset)
	var list []ZTPResultVo
	if err := r.db.Raw(querySQL, queryArgs...).Scan(&list).Error; err != nil {
		logger.Errorf("FindZTPResults query error: %v", err)
		return nil, 0, err
	}
	return list, total, nil
}

// FindZTPRetryLogs returns retry logs for a device.
func (r *Repository) FindZTPRetryLogs(elementId int64) ([]ZTPRetryLogVo, error) {
	var list []ZTPRetryLogVo
	err := r.db.Raw(`
		SELECT IFNULL(retry_time, '') AS operation_date, IFNULL(info, '') AS message
		FROM ztp_retry_log WHERE element_id = ? ORDER BY id DESC
	`, elementId).Scan(&list).Error
	if err != nil {
		logger.Errorf("FindZTPRetryLogs error: %v", err)
		return nil, err
	}
	return list, nil
}

// FindHistoryZTPFiles returns paginated ZTP file history for a device.
func (r *Repository) FindHistoryZTPFiles(elementId int64, page, pageSize int) ([]HistoryZTPFileVo, int64, error) {
	var total int64
	query := r.db.Model(&ZTPFileSendLog{}).Where("element_id = ?", elementId)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var list []HistoryZTPFileVo
	err := r.db.Raw(`
		SELECT f.id, f.element_id, IFNULL(ce.device_name, '') AS ne_name,
			IFNULL(f.file_name, '') AS file_name,
			IFNULL(f.generate_time, '') AS generate_time
		FROM ztp_file_send_log f
		LEFT JOIN cpe_element ce ON ce.ne_neid = f.element_id
		WHERE f.element_id = ? ORDER BY f.id DESC LIMIT ? OFFSET ?
	`, elementId, pageSize, offset).Scan(&list).Error
	if err != nil {
		logger.Errorf("FindHistoryZTPFiles error: %v", err)
		return nil, 0, err
	}
	return list, total, nil
}

// GetSystemConfigValue returns the config value for a given key.
func (r *Repository) GetSystemConfigValue(key string) (string, error) {
	var cfg SystemConfig
	err := r.db.Where("config_key = ?", key).First(&cfg).Error
	if err != nil {
		return "", err
	}
	if cfg.Value == nil {
		return "", nil
	}
	return *cfg.Value, nil
}

// SaveSystemConfigValue inserts or updates a system config entry.
func (r *Repository) SaveSystemConfigValue(key, value string) error {
	var cfg SystemConfig
	err := r.db.Where("config_key = ?", key).First(&cfg).Error
	if err == gorm.ErrRecordNotFound {
		cfg = SystemConfig{Key: &key, Value: &value}
		return r.db.Create(&cfg).Error
	}
	if err != nil {
		return err
	}
	cfg.Value = &value
	return r.db.Save(&cfg).Error
}

// DeleteZTPLogsByElementIds removes ZTP logs for the given devices.
func (r *Repository) DeleteZTPLogsByElementIds(elementIds []int64) error {
	return r.db.Where("element_id IN (?)", elementIds).Delete(&ZTPLog{}).Error
}

// DeleteZTPFileSendLogsByElementIds removes file send logs for the given devices.
func (r *Repository) DeleteZTPFileSendLogsByElementIds(elementIds []int64) error {
	return r.db.Where("element_id IN (?)", elementIds).Delete(&ZTPFileSendLog{}).Error
}

// ClearDeviceAOSFile resets the aos_file_name and read_to_ztp flag on devices.
func (r *Repository) ClearDeviceAOSFile(elementIds []int64) error {
	return r.db.Table("cpe_element").
		Where("ne_neid IN (?)", elementIds).
		Updates(map[string]interface{}{"aos_file_name": nil, "read_to_ztp": 0}).Error
}

// DeleteGnbIdUsedByElementId removes gnbId allocation for a device.
func (r *Repository) DeleteGnbIdUsedByElementId(elementId int64) error {
	return r.db.Where("element_id = ?", elementId).Delete(&ZTPGnbIdUsed{}).Error
}
