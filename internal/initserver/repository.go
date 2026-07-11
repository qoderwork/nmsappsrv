package initserver

import (
	"gorm.io/gorm"

	"nmsappsrv/pkg/baserepo"
)

// Repository handles database operations for initserver
type Repository struct {
	*baserepo.BaseRepository[SystemConfig, string] // embedded generic CRUD for SystemConfig
	db *gorm.DB
}

// NewRepository creates a new Repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		BaseRepository: baserepo.New[SystemConfig, string](db, "id"),
		db:             db,
	}
}
