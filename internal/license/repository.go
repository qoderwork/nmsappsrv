package license

import (
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository handles database operations for license entities.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ---------------------------------------------------------------------------
// License CRUD
// ---------------------------------------------------------------------------

// FindLicenseByID returns a license by its primary key.
func (r *Repository) FindLicenseByID(id int) (*License, error) {
	var l License
	if err := r.db.Where("id = ?", id).First(&l).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

// FindAllLicenses returns all licenses.
func (r *Repository) FindAllLicenses() ([]License, error) {
	var licenses []License
	if err := r.db.Find(&licenses).Error; err != nil {
		logger.Errorf("FindAllLicenses error: %v", err)
		return nil, err
	}
	return licenses, nil
}

// UpdateLicense saves changes to an existing license.
func (r *Repository) UpdateLicense(l *License) error {
	return r.db.Save(l).Error
}

// ---------------------------------------------------------------------------
// SASConfig
// ---------------------------------------------------------------------------

// FindSASConfig returns the SAS configuration for the given license.
func (r *Repository) FindSASConfig(licenseId int) (*SASConfig, error) {
	var cfg SASConfig
	if err := r.db.Where("license_id = ?", licenseId).First(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveSASConfig creates or updates a SAS configuration row (upsert).
func (r *Repository) SaveSASConfig(cfg *SASConfig) error {
	var existing SASConfig
	err := r.db.Where("license_id = ?", cfg.LicenseId).First(&existing).Error
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
func (r *Repository) FindEntraEndpoints() ([]EntraEndpoint, error) {
	var endpoints []EntraEndpoint
	if err := r.db.Find(&endpoints).Error; err != nil {
		return nil, err
	}
	return endpoints, nil
}

// CreateEntraEndpoint inserts a new Entra endpoint.
func (r *Repository) CreateEntraEndpoint(e *EntraEndpoint) error {
	return r.db.Create(e).Error
}

// UpdateEntraEndpoint saves changes to an existing Entra endpoint.
func (r *Repository) UpdateEntraEndpoint(e *EntraEndpoint) error {
	return r.db.Save(e).Error
}

// DeleteEntraEndpoint removes an Entra endpoint by ID.
func (r *Repository) DeleteEntraEndpoint(id string) error {
	return r.db.Where("id = ?", id).Delete(&EntraEndpoint{}).Error
}
