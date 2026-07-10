package security

import (
	"gorm.io/gorm"
)

// Repository handles database operations for security
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// FindConfigByKey loads system_config by id
func (r *Repository) FindConfigByKey(key string) (*SystemConfig, error) {
	var sc SystemConfig
	if err := r.db.Where("id = ?", key).First(&sc).Error; err != nil {
		return nil, err
	}
	return &sc, nil
}

// CreateConfig creates a new system_config record
func (r *Repository) CreateConfig(sc *SystemConfig) error {
	return r.db.Create(sc).Error
}

// SaveConfig updates an existing system_config record
func (r *Repository) SaveConfig(sc *SystemConfig) error {
	return r.db.Save(sc).Error
}
