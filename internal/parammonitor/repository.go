package parammonitor

import (
	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for parameter monitoring entities.
type Repository interface {
	// Generic BaseRepository[ParameterMonitorConfig, int] methods.
	Create(entity *ParameterMonitorConfig) error
	Save(entity *ParameterMonitorConfig) error
	FindByID(id int) (*ParameterMonitorConfig, error)
	DeleteByID(id int) error
	DeleteByIDs(ids []int) error
	SoftDelete(id int) error
	UpdateFields(id int, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]ParameterMonitorConfig, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[ParameterMonitorConfig], error)

	// Module-specific methods.
	ListConfigs(tenantId int, page, pageSize int) ([]ParameterMonitorConfig, int64, error)
	SetConfigParameters(configId int, parameterIds []string) error
	GetConfigParameters(configId int) ([]string, error)
	GetParameterByIds(ids []string) (map[string]string, error)
	CreateThresholdRule(rule *ThresholdRule) error
	UpdateThresholdRule(rule *ThresholdRule) error
	DeleteThresholdRule(id uint) error
	GetThresholdRule(id uint) (*ThresholdRule, error)
	ListThresholdRules(enabled *bool, page, pageSize int) ([]ThresholdRule, int64, error)
	GetLatestParameterValue(deviceSN, parameterName string) (*ParameterRecord, error)

	// DB returns the underlying *gorm.DB.
	DB() *gorm.DB
}

// repository is the concrete GORM-backed implementation of Repository.
// It embeds BaseRepository[ParameterMonitorConfig, int] for standard CRUD.
type repository struct {
	*baserepo.BaseRepository[ParameterMonitorConfig, int]
	db *gorm.DB
}

// DB returns the underlying *gorm.DB.
func (r *repository) DB() *gorm.DB {
	return r.db
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[ParameterMonitorConfig, int](db, "id"),
		db:             db,
	}
}

// ---------------------------------------------------------------------------
// ParameterMonitorConfig – module-specific queries (base provides Create/Save/FindByID/DeleteByID)
// ---------------------------------------------------------------------------

// ListConfigs returns paginated monitor configs for the given license.
func (r *repository) ListConfigs(tenantId int, page, pageSize int) ([]ParameterMonitorConfig, int64, error) {
	var configs []ParameterMonitorConfig
	var total int64

	query := r.db.Where("tenant_id = ?", tenantId)
	query.Model(&ParameterMonitorConfig{}).Count(&total)

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&configs).Error
	if err != nil {
		logger.Errorf("ListConfigs error: %v", err)
		return nil, 0, err
	}

	return configs, total, nil
}

// ---------------------------------------------------------------------------
// MonitorConfigHasParameter – association table operations
// ---------------------------------------------------------------------------

func (r *repository) SetConfigParameters(configId int, parameterIds []string) error {
	// Delete old associations
	err := r.db.Where("config_id = ?", configId).Delete(&MonitorConfigHasParameter{}).Error
	if err != nil {
		return err
	}

	// Insert new associations
	for _, paramId := range parameterIds {
		assoc := MonitorConfigHasParameter{
			ConfigId:    &configId,
			ParameterId: &paramId,
		}
		if err := r.db.Create(&assoc).Error; err != nil {
			return err
		}
	}

	return nil
}

func (r *repository) GetConfigParameters(configId int) ([]string, error) {
	var assocs []MonitorConfigHasParameter
	err := r.db.Where("config_id = ?", configId).Find(&assocs).Error
	if err != nil {
		return nil, err
	}

	paramIds := make([]string, 0, len(assocs))
	for _, assoc := range assocs {
		if assoc.ParameterId != nil {
			paramIds = append(paramIds, *assoc.ParameterId)
		}
	}

	return paramIds, nil
}

func (r *repository) GetParameterByIds(ids []string) (map[string]string, error) {
	if len(ids) == 0 {
		return make(map[string]string), nil
	}

	var params []struct {
		Id   string
		Path *string
	}
	err := r.db.Table("parameter").Select("id, path").Where("id IN ?", ids).Find(&params).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, p := range params {
		if p.Path != nil {
			result[p.Id] = *p.Path
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Threshold Rule CRUD (different entity type)
// ---------------------------------------------------------------------------

// CreateThresholdRule inserts a new threshold rule.
func (r *repository) CreateThresholdRule(rule *ThresholdRule) error {
	return r.db.Create(rule).Error
}

// UpdateThresholdRule saves changes to an existing threshold rule.
func (r *repository) UpdateThresholdRule(rule *ThresholdRule) error {
	return r.db.Save(rule).Error
}

// DeleteThresholdRule removes a threshold rule by ID.
func (r *repository) DeleteThresholdRule(id uint) error {
	return r.db.Where("id = ?", id).Delete(&ThresholdRule{}).Error
}

// GetThresholdRule returns a single threshold rule by ID.
func (r *repository) GetThresholdRule(id uint) (*ThresholdRule, error) {
	var rule ThresholdRule
	if err := r.db.First(&rule, id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

// ListThresholdRules returns a paginated list of threshold rules.
// If enabled is non-nil, only rules matching that flag are returned.
func (r *repository) ListThresholdRules(enabled *bool, page, pageSize int) ([]ThresholdRule, int64, error) {
	var rules []ThresholdRule
	var total int64

	query := r.db.Model(&ThresholdRule{})
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&rules).Error; err != nil {
		return nil, 0, err
	}
	return rules, total, nil
}

// GetLatestParameterValue returns the most recent parameter reading for a device.
func (r *repository) GetLatestParameterValue(deviceSN, parameterName string) (*ParameterRecord, error) {
	var rec ParameterRecord
	err := r.db.Table("element_basic_info_parameter").
		Select("element_basic_info_parameter.element_id, cpe_element.serial_number, element_basic_info_parameter.param_name, element_basic_info_parameter.param_value").
		Joins("JOIN cpe_element ON cpe_element.ne_neid = element_basic_info_parameter.element_id AND cpe_element.deleted = 0").
		Where("cpe_element.serial_number = ? AND element_basic_info_parameter.param_name = ?", deviceSN, parameterName).
		First(&rec).Error
	if err != nil {
		return nil, err
	}
	return &rec, nil
}
