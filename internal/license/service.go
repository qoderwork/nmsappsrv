package license

import (
	"gorm.io/gorm"
)

// Service contains the business logic for license management.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ---------------------------------------------------------------------------
// License
// ---------------------------------------------------------------------------

// GetLicense returns a single license by ID.
func (s *Service) GetLicense(id int) (*License, error) {
	return s.repo.FindByID(id)
}

// ListLicenses returns all licenses.
func (s *Service) ListLicenses() ([]License, error) {
	return s.repo.FindAll(s.repo.DB)
}

// UpdateLicense persists changes to an existing license.
func (s *Service) UpdateLicense(l *License) error {
	return s.repo.Save(l)
}

// ---------------------------------------------------------------------------
// SASConfig
// ---------------------------------------------------------------------------

// GetSASConfig returns the SAS configuration for the given license.
func (s *Service) GetSASConfig(licenseId int) (*SASConfig, error) {
	return s.repo.FindSASConfig(licenseId)
}

// SaveSASConfig creates or updates a SAS configuration.
func (s *Service) SaveSASConfig(cfg *SASConfig) error {
	return s.repo.SaveSASConfig(cfg)
}

// ---------------------------------------------------------------------------
// EntraEndpoint
// ---------------------------------------------------------------------------

// ListEntraEndpoints returns all Entra endpoints.
func (s *Service) ListEntraEndpoints() ([]EntraEndpoint, error) {
	return s.repo.FindEntraEndpoints()
}

// CreateEntraEndpoint persists a new Entra endpoint.
func (s *Service) CreateEntraEndpoint(e *EntraEndpoint) error {
	return s.repo.CreateEntraEndpoint(e)
}

// UpdateEntraEndpoint persists changes to an existing Entra endpoint.
func (s *Service) UpdateEntraEndpoint(e *EntraEndpoint) error {
	return s.repo.UpdateEntraEndpoint(e)
}

// DeleteEntraEndpoint removes an Entra endpoint by ID.
func (s *Service) DeleteEntraEndpoint(id string) error {
	return s.repo.DeleteEntraEndpoint(id)
}
