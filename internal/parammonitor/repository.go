package parammonitor

import (
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateConfig(config *ParameterMonitorConfig) error {
	return r.db.Create(config).Error
}

func (r *Repository) UpdateConfig(config *ParameterMonitorConfig) error {
	return r.db.Save(config).Error
}

func (r *Repository) DeleteConfig(id int) error {
	return r.db.Delete(&ParameterMonitorConfig{}, id).Error
}

func (r *Repository) GetConfig(id int) (*ParameterMonitorConfig, error) {
	var config ParameterMonitorConfig
	err := r.db.First(&config, id).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *Repository) ListConfigs(licenseId int, page, pageSize int) ([]ParameterMonitorConfig, int64, error) {
	var configs []ParameterMonitorConfig
	var total int64

	query := r.db.Where("license_id = ?", licenseId)
	query.Model(&ParameterMonitorConfig{}).Count(&total)

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&configs).Error
	if err != nil {
		logger.Errorf("ListConfigs error: %v", err)
		return nil, 0, err
	}

	return configs, total, nil
}

func (r *Repository) SetConfigParameters(configId int, parameterIds []string) error {
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

func (r *Repository) GetConfigParameters(configId int) ([]string, error) {
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

func (r *Repository) GetParameterByIds(ids []string) (map[string]string, error) {
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
// Threshold Rule CRUD
// ---------------------------------------------------------------------------

// CreateThresholdRule inserts a new threshold rule.
func (r *Repository) CreateThresholdRule(rule *ThresholdRule) error {
	return r.db.Create(rule).Error
}

// UpdateThresholdRule saves changes to an existing threshold rule.
func (r *Repository) UpdateThresholdRule(rule *ThresholdRule) error {
	return r.db.Save(rule).Error
}

// DeleteThresholdRule removes a threshold rule by ID.
func (r *Repository) DeleteThresholdRule(id uint) error {
	return r.db.Where("id = ?", id).Delete(&ThresholdRule{}).Error
}

// GetThresholdRule returns a single threshold rule by ID.
func (r *Repository) GetThresholdRule(id uint) (*ThresholdRule, error) {
	var rule ThresholdRule
	if err := r.db.First(&rule, id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

// ListThresholdRules returns a paginated list of threshold rules.
// If enabled is non-nil, only rules matching that flag are returned.
func (r *Repository) ListThresholdRules(enabled *bool, page, pageSize int) ([]ThresholdRule, int64, error) {
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
func (r *Repository) GetLatestParameterValue(deviceSN, parameterName string) (*ParameterRecord, error) {
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
