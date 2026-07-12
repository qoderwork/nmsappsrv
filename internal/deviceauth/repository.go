package deviceauth

import (
	"nmsappsrv/internal/misc"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for device auth configuration.
type Repository interface {
	GetConfig(id string) (*misc.SystemConfig, error)
	SaveConfig(sc *misc.SystemConfig) error
}

// repository is the concrete GORM-backed implementation of Repository.
type repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// GetConfig loads system_config by id
func (r *repository) GetConfig(id string) (*misc.SystemConfig, error) {
	var sc misc.SystemConfig
	if err := r.db.Where("id = ?", id).First(&sc).Error; err != nil {
		return nil, err
	}
	return &sc, nil
}

// SaveConfig saves system_config (create or update)
func (r *repository) SaveConfig(sc *misc.SystemConfig) error {
	return r.db.Save(sc).Error
}
