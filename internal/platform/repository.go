package platform

import (
	"errors"

	"gorm.io/gorm"
)

// Repository provides database operations for platform settings
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetSystemConfig reads a system_config entry by config_key
func (r *Repository) GetSystemConfig(key string) (string, error) {
	var cfg systemConfigModel
	if err := r.db.Where("id = ?", key).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	if cfg.Config == nil {
		return "", nil
	}
	return *cfg.Config, nil
}

// SaveSystemConfig upserts a system_config entry
func (r *Repository) SaveSystemConfig(key, value string) error {
	var cfg systemConfigModel
	err := r.db.Where("id = ?", key).First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cfg = systemConfigModel{
				Id:     key,
				Config: &value,
			}
			return r.db.Create(&cfg).Error
		}
		return err
	}
	cfg.Config = &value
	return r.db.Save(&cfg).Error
}
