package parameter

import (
	"time"

	"nmsappsrv/internal/misc"
	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository defines the data-access contract for parameter entities.
type Repository interface {
	// Generic BaseRepository[ParameterSet, string] methods.
	Create(entity *ParameterSet) error
	Save(entity *ParameterSet) error
	FindByID(id string) (*ParameterSet, error)
	DeleteByID(id string) error
	DeleteByIDs(ids []string) error
	SoftDelete(id string) error
	UpdateFields(id string, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]ParameterSet, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[ParameterSet], error)

	// Module-specific methods.
	FindParametersByElementId(elementId int64) ([]ParameterAttributes, error)
	FindParameterVosByElementId(elementId int64) ([]ParameterVo, error)
	FindParameterAttributes(elementId int64, paramName string) (*ParameterAttributes, error)
	CreateParameterAttributes(pa *ParameterAttributes) error
	UpdateParameterAttributes(pa *ParameterAttributes) error
	CreateParameterLog(log *ParameterLog) error
	FindParameterLogs(elementId int64, keyword string, offset, limit int) ([]ParameterLog, int64, error)
	FindParameterSets(licenseId int) ([]ParameterSet, error)
	FindParameterTemplates(tenancyId int) ([]ParameterTemplate, error)
	FindParameterTemplate(id int64) (*ParameterTemplate, []TemplateParameter, error)
	CreateParameterTemplate(t *ParameterTemplate) error
	UpdateParameterTemplate(t *ParameterTemplate) error
	DeleteParameterTemplate(id int64) error
	SaveTemplateParameters(templateId int64, params []TemplateParameter) error
	CreateParameterBackupLog(log *ParameterBackupLog) error
	FindParameterBackupLogs(elementId int64) ([]ParameterBackupLog, error)
	FindParameterBackupLogsWithPage(elementId int64, keyword string, page, pageSize int) ([]ParameterBackupLog, int64, error)
	CreateBatchConfigLog(log *misc.BatchConfigurationLog) error
	CreateBatchConfigDeviceLog(log *misc.BatchConfigurationDeviceLog) error
	FindBatchConfigLogs(tenancyId int, offset, limit int) ([]misc.BatchConfigurationLog, int64, error)
	BatchConfigProgress(taskId int64) (total int64, success int64, err error)
	BatchConfigDetail(taskId int64) ([]BatchConfigTaskDetailVo, error)
	InsertEventLog(eventType string, elementId int64, user string, status int, commandTrackData string) (int64, error)
	CreateParameterLogWithID(log *ParameterLog) error
	CreateDeployTemplateLog(log *ParameterDeploymentLog) error
	FindDeployTemplateLogs(templateId int64, offset, limit int) ([]ParameterDeploymentLog, int64, error)

	// TR-069 Parameter Definition CRUD.
	CreateTR069Parameter(param *TR069Parameter) error
	FindTR069Parameters(offset, limit int) ([]TR069Parameter, int64, error)
	FindTR069ParameterByID(id int64) (*TR069Parameter, error)
	UpdateTR069Parameter(param *TR069Parameter) error
	DeleteTR069Parameter(id int64) error

	DB() *gorm.DB
}

// repository is the concrete GORM-backed implementation of Repository.
// It embeds BaseRepository[ParameterSet, string] for standard CRUD on ParameterSet.
type repository struct {
	*baserepo.BaseRepository[ParameterSet, string]
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
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
func (r *repository) FindParametersByElementId(elementId int64) ([]ParameterAttributes, error) {
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
func (r *repository) FindParameterVosByElementId(elementId int64) ([]ParameterVo, error) {
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
func (r *repository) FindParameterAttributes(elementId int64, paramName string) (*ParameterAttributes, error) {
	var pa ParameterAttributes
	if err := r.db.Where("element_id = ? AND parameter_name = ?", elementId, paramName).First(&pa).Error; err != nil {
		return nil, err
	}
	return &pa, nil
}

// CreateParameterAttributes inserts a new parameter attribute row.
func (r *repository) CreateParameterAttributes(pa *ParameterAttributes) error {
	return r.db.Create(pa).Error
}

// UpdateParameterAttributes saves changes to an existing parameter attribute row.
func (r *repository) UpdateParameterAttributes(pa *ParameterAttributes) error {
	return r.db.Save(pa).Error
}

// ---------------------------------------------------------------------------
// ParameterLog
// ---------------------------------------------------------------------------

// CreateParameterLog inserts a new parameter change log entry.
func (r *repository) CreateParameterLog(log *ParameterLog) error {
	return r.db.Create(log).Error
}

// FindParameterLogs returns a paginated list of parameter change logs.
// When elementId is 0, logs for all devices are returned.
// keyword matches parameter_name (LIKE).
func (r *repository) FindParameterLogs(elementId int64, keyword string, offset, limit int) ([]ParameterLog, int64, error) {
	var logs []ParameterLog
	var total int64

	query := r.db.Model(&ParameterLog{})
	if elementId > 0 {
		query = query.Where("element_id = ?", elementId)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("parameter_name LIKE ?", like)
	}

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
func (r *repository) FindParameterSets(licenseId int) ([]ParameterSet, error) {
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
func (r *repository) FindParameterTemplates(tenancyId int) ([]ParameterTemplate, error) {
	var templates []ParameterTemplate
	if err := r.db.Where("tenancy_id = ?", tenancyId).Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

// CreateParameterTemplate inserts a new parameter template.
func (r *repository) CreateParameterTemplate(t *ParameterTemplate) error {
	return r.db.Create(t).Error
}

// UpdateParameterTemplate saves changes to an existing parameter template.
func (r *repository) UpdateParameterTemplate(t *ParameterTemplate) error {
	return r.db.Save(t).Error
}

// FindParameterTemplate returns a single template plus its parameter
// association rows (parameter_id + value, in association order). The
// `path` is resolved via a separate `parameter` table join inside the SQL
// (the VO layer may look it up separately if needed; for now we return
// ParameterId + Value, matching how the create/update request payloads
// look). Mirrors Java `getParameterDeployTemplateInfo`.
func (r *repository) FindParameterTemplate(id int64) (*ParameterTemplate, []TemplateParameter, error) {
	var tpl ParameterTemplate
	if err := r.db.Where("id = ?", id).First(&tpl).Error; err != nil {
		return nil, nil, err
	}
	var params []TemplateParameter
	err := r.db.Raw(`
		SELECT pth.parameter_id AS parameter_id,
		       COALESCE(pth.parameter_value, '') AS value,
		       COALESCE(p.path, '') AS path
		FROM parameter_template_has_parameter pth
		LEFT JOIN parameter p ON p.id = pth.parameter_id
		WHERE pth.template_id = ?
		ORDER BY pth.id
	`, id).Scan(&params).Error
	if err != nil {
		return nil, nil, err
	}
	return &tpl, params, nil
}

// DeleteParameterTemplate removes a template and its `parameter_template_has_parameter`
// rows in a single transaction. Mirrors Java `deleteParameterDeployTemplate`.
func (r *repository) DeleteParameterTemplate(id int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("template_id = ?", id).
			Delete(&ParameterTemplateHasParameter{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&ParameterTemplate{}).Error
	})
}

// SaveTemplateParameters replaces ALL parameter associations (with their DEFINED
// values) for a template. Templates are edited as a full set, so this deletes
// the existing rows then bulk-inserts the new ones inside a transaction.
// 对齐 Java ParameterDeploymentTemplate: 模板携带的是"定义值"而非设备当前值.
func (r *repository) SaveTemplateParameters(templateId int64, params []TemplateParameter) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("template_id = ?", templateId).
			Delete(&ParameterTemplateHasParameter{}).Error; err != nil {
			return err
		}
		if len(params) == 0 {
			return nil
		}
		return tx.Create(toTemplateParamRows(templateId, params)).Error
	})
}

// toTemplateParamRows maps a template's DEFINED parameter values into the
// association rows stored in parameter_template_has_parameter.
func toTemplateParamRows(templateId int64, params []TemplateParameter) []ParameterTemplateHasParameter {
	rows := make([]ParameterTemplateHasParameter, 0, len(params))
	for _, p := range params {
		pid := p.ParameterId
		val := p.Value
		rows = append(rows, ParameterTemplateHasParameter{
			TemplateId:     &templateId,
			ParameterId:    &pid,
			ParameterValue: &val,
		})
	}
	return rows
}

// ---------------------------------------------------------------------------
// ParameterBackupLog
// ---------------------------------------------------------------------------

// CreateParameterBackupLog inserts a new backup log entry.
func (r *repository) CreateParameterBackupLog(log *ParameterBackupLog) error {
	return r.db.Create(log).Error
}

// FindParameterBackupLogs returns all backup logs for the given element.
func (r *repository) FindParameterBackupLogs(elementId int64) ([]ParameterBackupLog, error) {
	var logs []ParameterBackupLog
	if err := r.db.Where("element_id = ?", elementId).Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// FindParameterBackupLogsWithPage returns paginated backup logs with optional filtering.
func (r *repository) FindParameterBackupLogsWithPage(elementId int64, keyword string, page, pageSize int) ([]ParameterBackupLog, int64, error) {
	var logs []ParameterBackupLog
	var total int64

	query := r.db.Model(&ParameterBackupLog{})
	if elementId > 0 {
		query = query.Where("element_id = ?", elementId)
	}
	if keyword != "" {
		query = query.Where("filename LIKE ? OR task_id LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("generate_time DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// ---------------------------------------------------------------------------
// Batch Configuration
// ---------------------------------------------------------------------------

// CreateBatchConfigLog inserts a new batch_configuration_log record.
func (r *repository) CreateBatchConfigLog(log *misc.BatchConfigurationLog) error {
	return r.db.Create(log).Error
}

// CreateBatchConfigDeviceLog inserts a new batch_configuration_device_log record.
func (r *repository) CreateBatchConfigDeviceLog(log *misc.BatchConfigurationDeviceLog) error {
	return r.db.Create(log).Error
}

// FindBatchConfigLogs returns a paginated list of batch configuration logs.
func (r *repository) FindBatchConfigLogs(tenancyId int, offset, limit int) ([]misc.BatchConfigurationLog, int64, error) {
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
func (r *repository) BatchConfigProgress(taskId int64) (total int64, success int64, err error) {
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
func (r *repository) BatchConfigDetail(taskId int64) ([]BatchConfigTaskDetailVo, error) {
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
func (r *repository) InsertEventLog(eventType string, elementId int64, user string, status int, commandTrackData string) (int64, error) {
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
func (r *repository) CreateParameterLogWithID(log *ParameterLog) error {
	if log.Id == "" {
		log.Id = uuid.NewString()
	}
	return r.db.Create(log).Error
}

// ---------------------------------------------------------------------------
// ParameterDeploymentLog
// ---------------------------------------------------------------------------

// CreateDeployTemplateLog inserts a new parameter_deployment_log entry.
func (r *repository) CreateDeployTemplateLog(log *ParameterDeploymentLog) error {
	return r.db.Create(log).Error
}

// FindDeployTemplateLogs returns a paginated list of deployment logs for the
// given template together with the total count.
func (r *repository) FindDeployTemplateLogs(templateId int64, offset, limit int) ([]ParameterDeploymentLog, int64, error) {
	var logs []ParameterDeploymentLog
	var total int64

	query := r.db.Model(&ParameterDeploymentLog{}).Where("template_id = ?", templateId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindDeployTemplateLogs count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("operation_time DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		logger.Errorf("FindDeployTemplateLogs query error: %v", err)
		return nil, 0, err
	}
	return logs, total, nil
}

// DB returns the underlying *gorm.DB.
func (r *repository) DB() *gorm.DB {
	return r.db
}

// ---------------------------------------------------------------------------
// TR-069 Parameter Definition
// ---------------------------------------------------------------------------

// CreateTR069Parameter inserts a new TR-069 parameter definition.
func (r *repository) CreateTR069Parameter(param *TR069Parameter) error {
	return r.db.Create(param).Error
}

// FindTR069Parameters returns a paginated list of TR-069 parameter definitions.
func (r *repository) FindTR069Parameters(offset, limit int) ([]TR069Parameter, int64, error) {
	var params []TR069Parameter
	var total int64

	query := r.db.Model(&TR069Parameter{})

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindTR069Parameters count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&params).Error; err != nil {
		logger.Errorf("FindTR069Parameters query error: %v", err)
		return nil, 0, err
	}
	return params, total, nil
}

// FindTR069ParameterByID returns a single TR-069 parameter definition by ID.
func (r *repository) FindTR069ParameterByID(id int64) (*TR069Parameter, error) {
	var param TR069Parameter
	if err := r.db.Where("id = ?", id).First(&param).Error; err != nil {
		return nil, err
	}
	return &param, nil
}

// UpdateTR069Parameter saves changes to an existing TR-069 parameter definition.
func (r *repository) UpdateTR069Parameter(param *TR069Parameter) error {
	return r.db.Save(param).Error
}

// DeleteTR069Parameter removes a TR-069 parameter definition by ID.
func (r *repository) DeleteTR069Parameter(id int64) error {
	return r.db.Delete(&TR069Parameter{}, id).Error
}
