package deviceauth

import (
	"nmsappsrv/internal/misc"

	"gorm.io/gorm"
)

// Repository handles database operations for device auth
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetConfig loads system_config by id
func (r *Repository) GetConfig(id string) (*misc.SystemConfig, error) {
	var sc misc.SystemConfig
	if err := r.db.Where("id = ?", id).First(&sc).Error; err != nil {
		return nil, err
	}
	return &sc, nil
}

// SaveConfig saves system_config (create or update)
func (r *Repository) SaveConfig(sc *misc.SystemConfig) error {
	return r.db.Save(sc).Error
}
