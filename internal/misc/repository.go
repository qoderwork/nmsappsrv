package misc

import (
	"time"

	"nmsappsrv/internal/device"
	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for miscellaneous entities.
// The embedded BaseRepository provides generic CRUD for NorthReport.
type Repository interface {
	// Generic CRUD (BaseRepository[NorthReport, int]).
	Create(entity *NorthReport) error
	Save(entity *NorthReport) error
	FindByID(id int) (*NorthReport, error)
	DeleteByID(id int) error
	DeleteByIDs(ids []int) error
	SoftDelete(id int) error
	UpdateFields(id int, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]NorthReport, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[NorthReport], error)

	// Custom methods.
	FindBackupOrRestoreTasks(tenancyId int, offset, limit int) ([]BackupOrRestoreTask, int64, error)
	CreateBackupOrRestoreTask(t *BackupOrRestoreTask) error
	FindBatchConfigLogs(tenancyId int, offset, limit int) ([]BatchConfigurationLog, int64, error)
	FindMRData(elementId int64, offset, limit int) ([]MRData, int64, error)
	FindZTPLogs(elementId int64) ([]ZTPLog, error)
	CreateZTPLog(log *ZTPLog) error
	FindNorthReports(licenseId int) ([]NorthReport, error)
	CreateNorthReport(report *NorthReport) error
	UpdateNorthReport(report *NorthReport) error
	DeleteNorthReport(id int) error
	FindRadius(tenancyId int) ([]Radius, error)
	SaveRadius(rad *Radius) error
	DeleteRadius(id int) error
	CreateOperatorLog(log *SystemOperatorLog) error
	FindOperatorLogs(tenancyId int, offset, limit int) ([]SystemOperatorLog, int64, error)
	FindUploadFiles(offset, limit int) ([]UploadFile, int64, error)
	CreateUploadFile(f *UploadFile) error
	DeleteUploadFile(id string) error
	FindErrorInfos(tenancyId int) ([]ErrorInfo, error)
	CreateErrorInfo(e *ErrorInfo) error
	CreateBatchAddObjectTask(t *BatchAddObjectTask) error
	FindBatchAddObjectTasks(tenancyId int, offset, limit int) ([]BatchAddObjectTask, int64, error)
	CreateBatchAddObjectTaskLog(log *BatchAddObjectTaskLog) error
	BatchAddObjectTaskProgress(taskId int) (total int64, success int64, err error)
	BatchAddObjectTaskDetail(taskId int) ([]BatchAddObjectTaskDetailVo, error)
	InsertEventLog(eventType string, elementId int64, user string, status int, commandTrackData string) (int64, error)
	DB() *gorm.DB
	FindBackupRestoreTaskById(id int) (*BackupOrRestoreTask, error)
	UpdateBackupRestoreTask(t *BackupOrRestoreTask) error
	CheckDuplicateBackupRestoreTaskName(name string, tenancyId int) bool
	CreateDeviceLog(log *RestoreAndBackUpDeviceLog) error
	BatchBackupRestoreProgress(taskId int) (total int64, success int64, err error)
	BatchBackupRestoreResult(taskId int) (*int, error)
	BatchBackupRestoreDetail(taskId int) ([]BackupRestoreTaskDetailVo, error)
	FindZTPResults(req *ListZTPResultsRequest) ([]ZTPResultVo, int64, error)
	FindZTPRetryLogs(elementId int64) ([]ZTPRetryLogVo, error)
	FindHistoryZTPFiles(elementId int64, page, pageSize int) ([]HistoryZTPFileVo, int64, error)
	GetSystemConfigValue(key string) (string, error)
	SaveSystemConfigValue(key, value string) error
	DeleteZTPLogsByElementIds(elementIds []int64) error
	DeleteZTPFileSendLogsByElementIds(elementIds []int64) error
	ClearDeviceAOSFile(elementIds []int64) error
	DeleteGnbIdUsedByElementId(elementId int64) error
	GetDeviceSerialNumber(elementId int64) (string, error)
	FindReadyForZTPAOS() ([]int64, error)

	// AOS Management — TBG
	FindTBGs(licenseId int, name string, offset, limit int) ([]TBG, int64, error)
	CreateTBG(tbg *TBG) error
	UpdateTBG(tbg *TBG) error
	DeleteTBGs(ids []int64) error
	CreateTBGsFromRows(tbgs []TBG) error

	// AOS Management — PSAPID
	FindPSAPIDs(licenseId int, psapId string, offset, limit int) ([]PSAPID, int64, error)
	SyncPSAPIDs(licenseId int, records []PSAPID) (int, error)
	CreatePSAPIDSyncLog(log *PSAPIDSyncLog) error
	FindPSAPIDSyncLogs(offset, limit int) ([]PSAPIDSyncLog, int64, error)

	// AOS Management — SpatialFile
	FindSpatialFileMarkets(licenseId int) ([]SpatialFileMarket, error)
	FindMarketCoordinates(marketId int) ([]PSAPID, error)
}

// repository is the concrete GORM-backed implementation of Repository.
type repository struct {
	*baserepo.BaseRepository[NorthReport, int]
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[NorthReport, int](db, "id"),
		db:             db,
	}
}

// FindBackupOrRestoreTasks returns a paginated list of backup/restore tasks.
func (r *repository) FindBackupOrRestoreTasks(tenancyId int, offset, limit int) ([]BackupOrRestoreTask, int64, error) {
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
func (r *repository) CreateBackupOrRestoreTask(t *BackupOrRestoreTask) error {
	return r.db.Create(t).Error
}

// FindBatchConfigLogs returns a paginated list of batch configuration logs.
func (r *repository) FindBatchConfigLogs(tenancyId int, offset, limit int) ([]BatchConfigurationLog, int64, error) {
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
func (r *repository) FindMRData(elementId int64, offset, limit int) ([]MRData, int64, error) {
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
func (r *repository) FindZTPLogs(elementId int64) ([]ZTPLog, error) {
	var logs []ZTPLog
	if err := r.db.Where("element_id = ?", elementId).Order("id DESC").Find(&logs).Error; err != nil {
		logger.Errorf("FindZTPLogs error: %v", err)
		return nil, err
	}
	return logs, nil
}

// CreateZTPLog inserts a new ZTP log record.
func (r *repository) CreateZTPLog(log *ZTPLog) error {
	return r.db.Create(log).Error
}

// FindNorthReports returns all north reports for the given license.
func (r *repository) FindNorthReports(licenseId int) ([]NorthReport, error) {
	var reports []NorthReport
	if err := r.db.Where("license_id = ? AND (deleted = ? OR deleted IS NULL)", licenseId, 0).Find(&reports).Error; err != nil {
		logger.Errorf("FindNorthReports error: %v", err)
		return nil, err
	}
	return reports, nil
}

// CreateNorthReport inserts a new north report.
// Delegates to BaseRepository.Create.
func (r *repository) CreateNorthReport(report *NorthReport) error {
	return r.BaseRepository.Create(report)
}

// UpdateNorthReport saves changes to an existing north report.
// Delegates to BaseRepository.Save.
func (r *repository) UpdateNorthReport(report *NorthReport) error {
	return r.BaseRepository.Save(report)
}

// DeleteNorthReport removes a north report by ID.
// Delegates to BaseRepository.DeleteByID.
func (r *repository) DeleteNorthReport(id int) error {
	return r.BaseRepository.DeleteByID(id)
}

// FindRadius returns all RADIUS configurations for the given tenancy.
func (r *repository) FindRadius(tenancyId int) ([]Radius, error) {
	var list []Radius
	if err := r.db.Where("tenancy_id = ?", tenancyId).Find(&list).Error; err != nil {
		logger.Errorf("FindRadius error: %v", err)
		return nil, err
	}
	return list, nil
}

// SaveRadius inserts or updates a RADIUS configuration.
func (r *repository) SaveRadius(rad *Radius) error {
	if rad.Id == 0 {
		return r.db.Create(rad).Error
	}
	return r.db.Save(rad).Error
}

// DeleteRadius removes a RADIUS configuration by ID.
func (r *repository) DeleteRadius(id int) error {
	return r.db.Where("id = ?", id).Delete(&Radius{}).Error
}

// CreateOperatorLog inserts a new operator log record.
func (r *repository) CreateOperatorLog(log *SystemOperatorLog) error {
	return r.db.Create(log).Error
}

// FindOperatorLogs returns a paginated list of operator logs.
func (r *repository) FindOperatorLogs(tenancyId int, offset, limit int) ([]SystemOperatorLog, int64, error) {
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
func (r *repository) FindUploadFiles(offset, limit int) ([]UploadFile, int64, error) {
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
func (r *repository) CreateUploadFile(f *UploadFile) error {
	return r.db.Create(f).Error
}

// DeleteUploadFile removes an upload file by ID.
func (r *repository) DeleteUploadFile(id string) error {
	return r.db.Where("file_id = ?", id).Delete(&UploadFile{}).Error
}

// FindErrorInfos returns all error info records for the given tenancy.
func (r *repository) FindErrorInfos(tenancyId int) ([]ErrorInfo, error) {
	var infos []ErrorInfo
	if err := r.db.Where("tenancy_id = ?", tenancyId).Find(&infos).Error; err != nil {
		logger.Errorf("FindErrorInfos error: %v", err)
		return nil, err
	}
	return infos, nil
}

// CreateErrorInfo inserts a new error info record.
func (r *repository) CreateErrorInfo(e *ErrorInfo) error {
	return r.db.Create(e).Error
}

// ---------------------------------------------------------------------------
// BatchAddObjectTask
// ---------------------------------------------------------------------------

// CreateBatchAddObjectTask inserts a new batch-add-object task.
func (r *repository) CreateBatchAddObjectTask(t *BatchAddObjectTask) error {
	return r.db.Create(t).Error
}

// FindBatchAddObjectTasks returns a paginated list of tasks for the given tenancy.
func (r *repository) FindBatchAddObjectTasks(tenancyId int, offset, limit int) ([]BatchAddObjectTask, int64, error) {
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
func (r *repository) CreateBatchAddObjectTaskLog(log *BatchAddObjectTaskLog) error {
	return r.db.Create(log).Error
}

// BatchAddObjectTaskProgress returns (total, success_count) for a given task.
func (r *repository) BatchAddObjectTaskProgress(taskId int) (total int64, success int64, err error) {
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
func (r *repository) BatchAddObjectTaskDetail(taskId int) ([]BatchAddObjectTaskDetailVo, error) {
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
func (r *repository) InsertEventLog(eventType string, elementId int64, user string, status int, commandTrackData string) (int64, error) {
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
func (r *repository) DB() *gorm.DB {
	return r.db
}

// ---------------------------------------------------------------------------
// Batch Backup/Restore
// ---------------------------------------------------------------------------

// FindBackupRestoreTaskById loads a single task by ID.
func (r *repository) FindBackupRestoreTaskById(id int) (*BackupOrRestoreTask, error) {
	var task BackupOrRestoreTask
	if err := r.db.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateBackupRestoreTask saves changes to a task.
func (r *repository) UpdateBackupRestoreTask(t *BackupOrRestoreTask) error {
	return r.db.Save(t).Error
}

// CheckDuplicateBackupRestoreTaskName checks if a task name already exists for the tenancy.
func (r *repository) CheckDuplicateBackupRestoreTaskName(name string, tenancyId int) bool {
	var count int64
	r.db.Model(&BackupOrRestoreTask{}).Where("name = ? AND tenancy_id = ?", name, tenancyId).Count(&count)
	return count > 0
}

// CreateDeviceLog inserts a restore_and_back_up_device_log record.
func (r *repository) CreateDeviceLog(log *RestoreAndBackUpDeviceLog) error {
	return r.db.Create(log).Error
}

// BatchBackupRestoreProgress returns (total_devices, success_count) for a task.
func (r *repository) BatchBackupRestoreProgress(taskId int) (total int64, success int64, err error) {
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
func (r *repository) BatchBackupRestoreResult(taskId int) (*int, error) {
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
func (r *repository) BatchBackupRestoreDetail(taskId int) ([]BackupRestoreTaskDetailVo, error) {
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
func (r *repository) FindZTPResults(req *ListZTPResultsRequest) ([]ZTPResultVo, int64, error) {
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
func (r *repository) FindZTPRetryLogs(elementId int64) ([]ZTPRetryLogVo, error) {
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
func (r *repository) FindHistoryZTPFiles(elementId int64, page, pageSize int) ([]HistoryZTPFileVo, int64, error) {
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
func (r *repository) GetSystemConfigValue(key string) (string, error) {
	var cfg SystemConfig
	err := r.db.Where("id = ?", key).First(&cfg).Error
	if err != nil {
		return "", err
	}
	if cfg.Config == nil {
		return "", nil
	}
	return *cfg.Config, nil
}

// SaveSystemConfigValue inserts or updates a system config entry.
func (r *repository) SaveSystemConfigValue(key, value string) error {
	var cfg SystemConfig
	err := r.db.Where("id = ?", key).First(&cfg).Error
	if err == gorm.ErrRecordNotFound {
		cfg = SystemConfig{Id: key, Config: &value}
		return r.db.Create(&cfg).Error
	}
	if err != nil {
		return err
	}
	cfg.Config = &value
	return r.db.Save(&cfg).Error
}

// DeleteZTPLogsByElementIds removes ZTP logs for the given devices.
func (r *repository) DeleteZTPLogsByElementIds(elementIds []int64) error {
	return r.db.Where("element_id IN (?)", elementIds).Delete(&ZTPLog{}).Error
}

// DeleteZTPFileSendLogsByElementIds removes file send logs for the given devices.
func (r *repository) DeleteZTPFileSendLogsByElementIds(elementIds []int64) error {
	return r.db.Where("element_id IN (?)", elementIds).Delete(&ZTPFileSendLog{}).Error
}

// ClearDeviceAOSFile resets the aos_file_name and read_to_ztp flag on devices.
func (r *repository) ClearDeviceAOSFile(elementIds []int64) error {
	return r.db.Table("cpe_element").
		Where("ne_neid IN (?)", elementIds).
		Updates(map[string]interface{}{"aos_file_name": nil, "read_to_ztp": 0}).Error
}

// DeleteGnbIdUsedByElementId removes gnbId allocation for a device.
func (r *repository) DeleteGnbIdUsedByElementId(elementId int64) error {
	return r.db.Where("element_id = ?", elementId).Delete(&ZTPGnbIdUsed{}).Error
}

// GetDeviceSerialNumber returns the serial number for a non-deleted device.
// Used by the HTTP handler's ZTP provisioning flow so the handler no longer
// needs a direct *gorm.DB handle.
func (r *repository) GetDeviceSerialNumber(elementId int64) (string, error) {
	var dev device.CpeElement
	if err := r.db.Select("ne_neid, serial_number").Where("ne_neid = ? AND deleted = ?", elementId, false).First(&dev).Error; err != nil {
		return "", err
	}
	if dev.SerialNumber == nil {
		return "", nil
	}
	return *dev.SerialNumber, nil
}

// FindReadyForZTPAOS returns the IDs of devices that are ready for ZTP
// (read_to_ztp = true) but have no AOS file generated yet
// (aos_file_name IS NULL). This is the ZTPTask selection criterion.
func (r *repository) FindReadyForZTPAOS() ([]int64, error) {
	var ids []int64
	err := r.db.Model(&device.CpeElement{}).
		Where("aos_file_name IS NULL AND read_to_ztp = ? AND deleted = ?", true, false).
		Pluck("ne_neid", &ids).Error
	return ids, err
}

// ---------------------------------------------------------------------------
// AOS Management — TBG
// ---------------------------------------------------------------------------

// FindTBGs returns a paginated list of TBG records filtered by license and name.
func (r *repository) FindTBGs(licenseId int, name string, offset, limit int) ([]TBG, int64, error) {
	var list []TBG
	var total int64
	query := r.db.Model(&TBG{}).Where("license_id = ?", licenseId)
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindTBGs count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		logger.Errorf("FindTBGs query error: %v", err)
		return nil, 0, err
	}
	return list, total, nil
}

// CreateTBG inserts a new TBG record.
func (r *repository) CreateTBG(tbg *TBG) error {
	return r.db.Create(tbg).Error
}

// UpdateTBG saves changes to an existing TBG record.
func (r *repository) UpdateTBG(tbg *TBG) error {
	return r.db.Save(tbg).Error
}

// DeleteTBGs removes TBG records by IDs.
func (r *repository) DeleteTBGs(ids []int64) error {
	return r.db.Where("id IN (?)", ids).Delete(&TBG{}).Error
}

// CreateTBGsFromRows batch inserts TBG records (used by import).
func (r *repository) CreateTBGsFromRows(tbgs []TBG) error {
	return r.db.Create(&tbgs).Error
}

// ---------------------------------------------------------------------------
// AOS Management — PSAPID
// ---------------------------------------------------------------------------

// FindPSAPIDs returns a paginated list of PSAP ID records.
func (r *repository) FindPSAPIDs(licenseId int, psapId string, offset, limit int) ([]PSAPID, int64, error) {
	var list []PSAPID
	var total int64
	query := r.db.Model(&PSAPID{}).Where("license_id = ?", licenseId)
	if psapId != "" {
		query = query.Where("psap_id LIKE ?", "%"+psapId+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindPSAPIDs count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		logger.Errorf("FindPSAPIDs query error: %v", err)
		return nil, 0, err
	}
	return list, total, nil
}

// CreatePSAPIDSyncLog inserts a new PSAP ID sync log record.
func (r *repository) CreatePSAPIDSyncLog(log *PSAPIDSyncLog) error {
	return r.db.Create(log).Error
}

// SyncPSAPIDs replaces PSAP ID records for a given license and returns the count.
func (r *repository) SyncPSAPIDs(licenseId int, records []PSAPID) (int, error) {
	tx := r.db.Begin()
	if err := tx.Where("license_id = ?", licenseId).Delete(&PSAPID{}).Error; err != nil {
		tx.Rollback()
		return 0, err
	}
	for i := range records {
		records[i].LicenseId = &licenseId
		if err := tx.Create(&records[i]).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return len(records), nil
}

// FindPSAPIDSyncLogs returns a paginated list of sync logs.
func (r *repository) FindPSAPIDSyncLogs(offset, limit int) ([]PSAPIDSyncLog, int64, error) {
	var list []PSAPIDSyncLog
	var total int64
	query := r.db.Model(&PSAPIDSyncLog{})
	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindPSAPIDSyncLogs count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		logger.Errorf("FindPSAPIDSyncLogs query error: %v", err)
		return nil, 0, err
	}
	return list, total, nil
}

// ---------------------------------------------------------------------------
// AOS Management — SpatialFile
// ---------------------------------------------------------------------------

// FindSpatialFileMarkets returns all spatial file markets for a license.
func (r *repository) FindSpatialFileMarkets(licenseId int) ([]SpatialFileMarket, error) {
	var list []SpatialFileMarket
	if err := r.db.Where("license_id = ?", licenseId).Find(&list).Error; err != nil {
		logger.Errorf("FindSpatialFileMarkets error: %v", err)
		return nil, err
	}
	return list, nil
}

// FindMarketCoordinates returns PSAP IDs for a given market ID.
// Maps market to license_id since spatial_file_market has license_id.
func (r *repository) FindMarketCoordinates(marketId int) ([]PSAPID, error) {
	var market SpatialFileMarket
	if err := r.db.First(&market, marketId).Error; err != nil {
		return nil, err
	}
	licenseId := 0
	if market.LicenseId != nil {
		licenseId = *market.LicenseId
	}
	var list []PSAPID
	if err := r.db.Where("license_id = ?", licenseId).Find(&list).Error; err != nil {
		logger.Errorf("FindMarketCoordinates error: %v", err)
		return nil, err
	}
	return list, nil
}
