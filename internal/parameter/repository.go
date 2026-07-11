package parameter

import (
	"time"

	"nmsappsrv/internal/misc"
	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository handles database operations for parameter entities.
// It embeds BaseRepository[ParameterSet, string] for standard CRUD on ParameterSet,
// and retains module-specific methods for other entity types and custom queries.
type Repository struct {
	*baserepo.BaseRepository[ParameterSet, string]
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		BaseRepository: baserepo.New[ParameterSet, string](db, "id"),
		db:             db,
	}
}

// ---------------------------------------------------------------------------
// ParameterAttributes
// ---------------------------------------------------------------------------

// FindParametersByElementId returns all parameter attributes for the given
// element, joined with the parameter table to ensure only valid parameters
// are returned.
func (r *Repository) FindParametersByElementId(elementId int64) ([]ParameterAttributes, error) {
	var pas []ParameterAttributes
	if err := r.db.Joins("JOIN parameter ON parameter.id = parameter_attributes.id").
		Where("parameter_attributes.element_id = ?", elementId).
		Find(&pas).Error; err != nil {
		logger.Errorf("FindParametersByElementId error: %v", err)
		return nil, err
	}
	return pas, nil
}

// FindParameterVosByElementId returns enriched ParameterVo for a device by joining
// parameter_attributes + parameter definition + element_basic_info_parameter (current values).
// This matches Java's getListParameterForDeviceVO behavior.
func (r *Repository) FindParameterVosByElementId(elementId int64) ([]ParameterVo, error) {
	var vos []ParameterVo
	err := r.db.Raw(`
		SELECT
			p.name AS param_name,
			p.name AS custom_name,
			COALESCE(p.path, '') AS tr069_name,
			COALESCE(ebp.param_value, '') AS value,
			COALESCE(p.param_type, '') AS type,
			COALESCE(p.regular_expression, '') AS regular_expression,
			p.length,
			COALESCE(p.unit, '') AS unit,
			DATE_FORMAT(p.update_time, '%Y-%m-%d %H:%i:%s') AS update_time,
			p.id AS parameter_id,
			COALESCE(p.param_range, '') AS mapping_value,
			IFNULL(p.is_writable, false) AS writable,
			COALESCE(p.remark, '') AS remark,
			IFNULL(p.need_reboot, false) AS need_reboot,
			COALESCE(p.hint, '') AS hint,
			IFNULL(p.multiple_check, false) AS multiple_check,
			COALESCE(p.separator, '') AS separator
		FROM parameter_attributes pa
		JOIN parameter p ON p.id = pa.id
		LEFT JOIN element_basic_info_parameter ebp
			ON ebp.element_id = pa.element_id AND ebp.param_name = COALESCE(p.path, p.name)
		WHERE pa.element_id = ?
		ORDER BY p.sort, p.name
	`, elementId).Scan(&vos).Error
	if err != nil {
		logger.Errorf("FindParameterVosByElementId error: %v", err)
		return nil, err
	}
	return vos, nil
}

// FindParameterAttributes returns a single parameter attribute row for the
// given element and parameter name.
func (r *Repository) FindParameterAttributes(elementId int64, paramName string) (*ParameterAttributes, error) {
	var pa ParameterAttributes
	if err := r.db.Where("element_id = ? AND parameter_name = ?", elementId, paramName).First(&pa).Error; err != nil {
		return nil, err
	}
	return &pa, nil
}

// CreateParameterAttributes inserts a new parameter attribute row.
func (r *Repository) CreateParameterAttributes(pa *ParameterAttributes) error {
	return r.db.Create(pa).Error
}

// UpdateParameterAttributes saves changes to an existing parameter attribute row.
func (r *Repository) UpdateParameterAttributes(pa *ParameterAttributes) error {
	return r.db.Save(pa).Error
}

// ---------------------------------------------------------------------------
// ParameterLog
// ---------------------------------------------------------------------------

// CreateParameterLog inserts a new parameter change log entry.
func (r *Repository) CreateParameterLog(log *ParameterLog) error {
	return r.db.Create(log).Error
}

// FindParameterLogs returns a paginated list of parameter change logs for the
// given element together with the total count.
func (r *Repository) FindParameterLogs(elementId int64, offset, limit int) ([]ParameterLog, int64, error) {
	var logs []ParameterLog
	var total int64

	query := r.db.Model(&ParameterLog{}).Where("element_id = ?", elementId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindParameterLogs count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("change_time DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		logger.Errorf("FindParameterLogs query error: %v", err)
		return nil, 0, err
	}
	return logs, total, nil
}

// ---------------------------------------------------------------------------
// ParameterSet – module-specific queries (base provides Create/Save/FindByID/DeleteByID)
// ---------------------------------------------------------------------------

// FindParameterSets returns all parameter sets for the given license.
func (r *Repository) FindParameterSets(licenseId int) ([]ParameterSet, error) {
	var sets []ParameterSet
	if err := r.db.Where("license_id = ?", licenseId).Find(&sets).Error; err != nil {
		return nil, err
	}
	return sets, nil
}

// ---------------------------------------------------------------------------
// ParameterTemplate (different entity type)
// ---------------------------------------------------------------------------

// FindParameterTemplates returns all parameter templates for the given tenancy.
func (r *Repository) FindParameterTemplates(tenancyId int) ([]ParameterTemplate, error) {
	var templates []ParameterTemplate
	if err := r.db.Where("tenancy_id = ?", tenancyId).Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

// CreateParameterTemplate inserts a new parameter template.
func (r *Repository) CreateParameterTemplate(t *ParameterTemplate) error {
	return r.db.Create(t).Error
}

// UpdateParameterTemplate saves changes to an existing parameter template.
func (r *Repository) UpdateParameterTemplate(t *ParameterTemplate) error {
	return r.db.Save(t).Error
}

// ---------------------------------------------------------------------------
// ParameterBackupLog
// ---------------------------------------------------------------------------

// CreateParameterBackupLog inserts a new backup log entry.
func (r *Repository) CreateParameterBackupLog(log *ParameterBackupLog) error {
	return r.db.Create(log).Error
}

// FindParameterBackupLogs returns all backup logs for the given element.
func (r *Repository) FindParameterBackupLogs(elementId int64) ([]ParameterBackupLog, error) {
	var logs []ParameterBackupLog
	if err := r.db.Where("element_id = ?", elementId).Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// ---------------------------------------------------------------------------
// Batch Configuration
// ---------------------------------------------------------------------------

// CreateBatchConfigLog inserts a new batch_configuration_log record.
func (r *Repository) CreateBatchConfigLog(log *misc.BatchConfigurationLog) error {
	return r.db.Create(log).Error
}

// CreateBatchConfigDeviceLog inserts a new batch_configuration_device_log record.
func (r *Repository) CreateBatchConfigDeviceLog(log *misc.BatchConfigurationDeviceLog) error {
	return r.db.Create(log).Error
}

// FindBatchConfigLogs returns a paginated list of batch configuration logs.
func (r *Repository) FindBatchConfigLogs(tenancyId int, offset, limit int) ([]misc.BatchConfigurationLog, int64, error) {
	var logs []misc.BatchConfigurationLog
	var total int64

	query := r.db.Model(&misc.BatchConfigurationLog{}).Where("tenancy_id = ?", tenancyId)

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

// BatchConfigProgress returns (total_devices, success_count) for a batch task.
func (r *Repository) BatchConfigProgress(taskId int64) (total int64, success int64, err error) {
	type row struct {
		Total   int64
		Success int64
	}
	var res row
	err = r.db.Raw(`
		SELECT COUNT(*) AS total,
		       SUM(CASE WHEN el.status = 3 THEN 1 ELSE 0 END) AS success
		FROM batch_configuration_device_log dl
		JOIN event_log el ON el.id = dl.event_log_id
		WHERE dl.task_id = ?
	`, taskId).Scan(&res).Error
	return res.Total, res.Success, err
}

// BatchConfigDetail returns per-device results for a batch configuration task.
func (r *Repository) BatchConfigDetail(taskId int64) ([]BatchConfigTaskDetailVo, error) {
	var list []BatchConfigTaskDetailVo
	err := r.db.Raw(`
		SELECT ce.device_name   AS device_name,
		       ce.serial_number AS serial_number,
		       ce.ne_neid       AS element_id,
		       el.status        AS result,
		       IFNULL(el.fault_info, '') AS fault_info,
		       IFNULL(dl.data, '')       AS data
		FROM batch_configuration_device_log dl
		JOIN cpe_element ce ON ce.ne_neid = dl.element_id
		LEFT JOIN event_log el ON el.id = dl.event_log_id
		WHERE dl.task_id = ?
		ORDER BY dl.id
	`, taskId).Scan(&list).Error
	if err != nil {
		logger.Errorf("BatchConfigDetail error: %v", err)
		return nil, err
	}
	return list, nil
}

// InsertEventLog creates an event_log row for SetParameterValues and returns
// its auto-generated ID. Uses a local struct to avoid importing the eventlog package.
func (r *Repository) InsertEventLog(eventType string, elementId int64, user string, status int, commandTrackData string) (int64, error) {
	row := struct {
		Id               int64     `gorm:"primaryKey;autoIncrement"`
		EventType        string    `gorm:"column:event_type;type:varchar(255)"`
		OperationTime    time.Time `gorm:"column:operation_time"`
		User             string    `gorm:"column:user;type:varchar(255)"`
		ElementId        int64     `gorm:"column:element_id"`
		Status           int       `gorm:"column:status"`
		CommandTrackData string    `gorm:"column:command_track_data;type:longtext"`
	}{
		EventType:        eventType,
		OperationTime:    time.Now(),
		User:             user,
		ElementId:        elementId,
		Status:           status,
		CommandTrackData: commandTrackData,
	}
	if err := r.db.Table("event_log").Create(&row).Error; err != nil {
		return 0, err
	}
	return row.Id, nil
}

// CreateParameterLogWithID inserts a parameter_log with a UUID primary key.
func (r *Repository) CreateParameterLogWithID(log *ParameterLog) error {
	if log.Id == "" {
		log.Id = uuid.NewString()
	}
	return r.db.Create(log).Error
}

// DB returns the underlying *gorm.DB.
func (r *Repository) DB() *gorm.DB {
	return r.db
}
