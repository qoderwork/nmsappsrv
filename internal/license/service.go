package license

import (
	"gorm.io/gorm"
)

// Service contains the business logic for license management.
type Service interface {
	GetLicense(id int) (*License, error)
	ListLicenses() ([]License, error)
	UpdateLicense(l *License) error
	GetSASConfig(licenseId int) (*SASConfig, error)
	SaveSASConfig(cfg *SASConfig) error
	ListEntraEndpoints() ([]EntraEndpoint, error)
	CreateEntraEndpoint(e *EntraEndpoint) error
	UpdateEntraEndpoint(e *EntraEndpoint) error
	DeleteEntraEndpoint(id string) error
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// ---------------------------------------------------------------------------
// License
// ---------------------------------------------------------------------------

// GetLicense returns a single license by ID.
func (s *service) GetLicense(id int) (*License, error) {
	return s.repo.FindByID(id)
}

// ListLicenses returns all licenses.
func (s *service) ListLicenses() ([]License, error) {
	return s.repo.FindAllLicenses()
}

// UpdateLicense persists changes to an existing license.
func (s *service) UpdateLicense(l *License) error {
	return s.repo.Save(l)
}

// ---------------------------------------------------------------------------
// SASConfig
// ---------------------------------------------------------------------------

// GetSASConfig returns the SAS configuration for the given license.
func (s *service) GetSASConfig(licenseId int) (*SASConfig, error) {
	return s.repo.FindSASConfig(licenseId)
}

// SaveSASConfig creates or updates a SAS configuration.
func (s *service) SaveSASConfig(cfg *SASConfig) error {
	return s.repo.SaveSASConfig(cfg)
}

// ---------------------------------------------------------------------------
// EntraEndpoint
// ---------------------------------------------------------------------------

// ListEntraEndpoints returns all Entra endpoints.
func (s *service) ListEntraEndpoints() ([]EntraEndpoint, error) {
	return s.repo.FindEntraEndpoints()
}

// CreateEntraEndpoint persists a new Entra endpoint.
func (s *service) CreateEntraEndpoint(e *EntraEndpoint) error {
	return s.repo.CreateEntraEndpoint(e)
}

// UpdateEntraEndpoint persists changes to an existing Entra endpoint.
func (s *service) UpdateEntraEndpoint(e *EntraEndpoint) error {
	return s.repo.UpdateEntraEndpoint(e)
}

// DeleteEntraEndpoint removes an Entra endpoint by ID.
func (s *service) DeleteEntraEndpoint(id string) error {
	return s.repo.DeleteEntraEndpoint(id)
}
