package paramcompare

import (
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository handles database reads for the param-compare module.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetDeviceParameters returns all (param_name, param_value) pairs for a device
// from the element_basic_info_parameter table.
func (r *Repository) GetDeviceParameters(deviceID uint) ([]DeviceParam, error) {
	var params []DeviceParam
	if err := r.db.Table("element_basic_info_parameter").
		Select("param_name, param_value").
		Where("element_id = ?", deviceID).
		Find(&params).Error; err != nil {
		logger.Errorf("GetDeviceParameters error: %v", err)
		return nil, err
	}
	return params, nil
}

// GetTemplateParameters returns all (param_path, param_value) pairs for a
// template from the parameter_template_value table.
func (r *Repository) GetTemplateParameters(templateID uint) ([]TemplateParam, error) {
	var params []TemplateParam
	if err := r.db.Table("parameter_template_value").
		Select("param_path, param_value").
		Where("template_id = ?", templateID).
		Find(&params).Error; err != nil {
		logger.Errorf("GetTemplateParameters error: %v", err)
		return nil, err
	}
	return params, nil
}

// GetTemplateName returns the template name for the given ID.
func (r *Repository) GetTemplateName(templateID uint) (string, error) {
	var name string
	if err := r.db.Table("parameter_template").
		Select("name").
		Where("id = ?", templateID).
		Scan(&name).Error; err != nil {
		return "", err
	}
	return name, nil
}

// ListTemplates returns lightweight template info including parameter counts.
func (r *Repository) ListTemplates() ([]TemplateInfo, error) {
	var list []TemplateInfo
	if err := r.db.Raw(`
		SELECT pt.id,
		       IFNULL(pt.name, '')        AS name,
		       IFNULL(pt.description, '') AS description,
		       COUNT(ptv.id)              AS param_count
		FROM parameter_template pt
		LEFT JOIN parameter_template_value ptv ON ptv.template_id = pt.id
		GROUP BY pt.id, pt.name, pt.description
		ORDER BY pt.id
	`).Scan(&list).Error; err != nil {
		logger.Errorf("ListTemplates error: %v", err)
		return nil, err
	}
	return list, nil
}
