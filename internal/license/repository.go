package license

import (
	"errors"

	"nmsappsrv/pkg/baserepo"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for license entities.
type Repository interface {
	// Generic CRUD (from BaseRepository[License, int]).
	Create(entity *License) error
	Save(entity *License) error
	FindByID(id int) (*License, error)
	DeleteByID(id int) error
	DeleteByIDs(ids []int) error
	SoftDelete(id int) error
	UpdateFields(id int, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]License, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[License], error)

	// Custom queries.
	FindAllLicenses() ([]License, error)
	FindSASConfig(tenantId int) (*SASConfig, error)
	SaveSASConfig(cfg *SASConfig) error
	FindEntraEndpoints() ([]EntraEndpoint, error)
	CreateEntraEndpoint(e *EntraEndpoint) error
	UpdateEntraEndpoint(e *EntraEndpoint) error
	DeleteEntraEndpoint(id string) error

	// Base station license.
	UpsertBaseStationLicense(e *BaseStationLicense) error
}

func ptrStr(s string) *string { return &s }

// repository is the concrete GORM-backed implementation of Repository.
type repository struct {
	*baserepo.BaseRepository[License, int] // embed generic CRUD
	db *gorm.DB                             // keep for custom queries
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[License, int](db, "id"),
		db:             db,
	}
}

// ---------------------------------------------------------------------------
// License
// ---------------------------------------------------------------------------

// FindAllLicenses returns all licenses.
func (r *repository) FindAllLicenses() ([]License, error) {
	return r.BaseRepository.FindAll(r.DB)
}

// ---------------------------------------------------------------------------
// SASConfig
// ---------------------------------------------------------------------------

// FindSASConfig returns the SAS configuration for the given license.
func (r *repository) FindSASConfig(tenantId int) (*SASConfig, error) {
	var cfg SASConfig
	if err := r.db.Where("tenant_id = ?", tenantId).First(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveSASConfig creates or updates a SAS configuration row (upsert).
func (r *repository) SaveSASConfig(cfg *SASConfig) error {
	var existing SASConfig
	err := r.db.Where("tenant_id = ?", cfg.TenantId).First(&existing).Error
	if err != nil {
		// No existing row — insert.
		return r.db.Create(cfg).Error
	}
	// Existing row — update.
	existing.AutoRegister = cfg.AutoRegister
	return r.db.Save(&existing).Error
}

// ---------------------------------------------------------------------------
// EntraEndpoint
// ---------------------------------------------------------------------------

// FindEntraEndpoints returns all Entra endpoints.
func (r *repository) FindEntraEndpoints() ([]EntraEndpoint, error) {
	var endpoints []EntraEndpoint
	if err := r.db.Find(&endpoints).Error; err != nil {
		return nil, err
	}
	return endpoints, nil
}

// CreateEntraEndpoint inserts a new Entra endpoint.
func (r *repository) CreateEntraEndpoint(e *EntraEndpoint) error {
	return r.db.Create(e).Error
}

// UpdateEntraEndpoint saves changes to an existing Entra endpoint.
func (r *repository) UpdateEntraEndpoint(e *EntraEndpoint) error {
	return r.db.Save(e).Error
}

// DeleteEntraEndpoint removes an Entra endpoint by ID.
func (r *repository) DeleteEntraEndpoint(id string) error {
	return r.db.Where("id = ?", id).Delete(&EntraEndpoint{}).Error
}

// UpsertBaseStationLicense creates or updates a base station license row,
// keyed by element_id.
func (r *repository) UpsertBaseStationLicense(e *BaseStationLicense) error {
	var existing BaseStationLicense
	err := r.db.Where("element_id = ?", e.ElementId).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.db.Create(e).Error
		}
		return err
	}
	e.Id = existing.Id
	return r.db.Save(e).Error
}
