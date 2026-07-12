package initserver

import (
	"gorm.io/gorm"

	"nmsappsrv/pkg/baserepo"
)

// Repository defines the data-access contract for initserver.
// It embeds BaseRepository[SystemConfig, string] for standard CRUD on SystemConfig.
type Repository interface {
	// Generic CRUD delegated to BaseRepository[SystemConfig, string].
	Create(entity *SystemConfig) error
	Save(entity *SystemConfig) error
	FindByID(id string) (*SystemConfig, error)
	DeleteByID(id string) error
	DeleteByIDs(ids []string) error
	SoftDelete(id string) error
	UpdateFields(id string, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]SystemConfig, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[SystemConfig], error)
}

// repository is the concrete GORM-backed implementation of Repository.
type repository struct {
	*baserepo.BaseRepository[SystemConfig, string] // embedded generic CRUD for SystemConfig
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[SystemConfig, string](db, "id"),
		db:             db,
	}
}
