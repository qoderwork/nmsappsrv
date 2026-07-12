package site

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"
)

// Service defines the business-logic contract for site operations.
type Service interface {
	ListAreas(tenancyId int) ([]SysArea, error)
	GetArea(id int) (*SysArea, error)
	CreateArea(a *SysArea) error
	UpdateArea(a *SysArea) error
	DeleteArea(id int) error
	ListSites(licenseId int, search string, page, pageSize int) ([]SiteInfoVo, int64, error)
	ListSiteBasicInfo(licenseId int) ([]SiteBasicInfo, error)
	CreateSite(site *SiteInfo, licenseId int) error
	UpdateSite(id string, site *SiteInfo, licenseId int) error
	DeleteSite(id string) error
	GetSystemConfig() (*SystemConfig, error)
	UpdateSystemConfig(configJSON string) error
	ListSystemParameters() ([]SystemParameter, error)
	UpdateSystemParameter(p *SystemParameter) error
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a new site service.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// ---------- SysArea ----------

func (s *service) ListAreas(tenancyId int) ([]SysArea, error) {
	return s.repo.FindAreas(tenancyId)
}

func (s *service) GetArea(id int) (*SysArea, error) {
	return s.repo.FindAreaByID(id)
}

func (s *service) CreateArea(a *SysArea) error {
	return s.repo.CreateArea(a)
}

func (s *service) UpdateArea(a *SysArea) error {
	return s.repo.UpdateArea(a)
}

func (s *service) DeleteArea(id int) error {
	// Check for child areas before deleting
	children, err := s.repo.FindChildAreas(id)
	if err != nil {
		return err
	}
	if len(children) > 0 {
		return apperror.ErrConflict.WithMessage("this area contains subareas and cannot be deleted")
	}
	return s.repo.DeleteArea(id)
}

// ---------- SiteInfo ----------

// ListSites returns a paginated list of sites with area path resolved.
func (s *service) ListSites(licenseId int, search string, page, pageSize int) ([]SiteInfoVo, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	sites, total, err := s.repo.FindSites(licenseId, search, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	// Load all areas for area path resolution
	allAreas, err := s.repo.FindAreas(licenseId)
	if err != nil {
		logger.Warnf("ListSites: failed to load areas: %v", err)
	}

	// Build area lookup map
	areaMap := make(map[int]*SysArea)
	for i := range allAreas {
		areaMap[allAreas[i].Id] = &allAreas[i]
	}

	// Build response with area path
	result := make([]SiteInfoVo, len(sites))
	for i, site := range sites {
		result[i] = SiteInfoVo{SiteInfo: site}
		if site.AreaId != nil {
			result[i].AreaPath = s.resolveAreaPath(areaMap, *site.AreaId)
		}
	}

	return result, total, nil
}

// ListSiteBasicInfo returns lightweight site info for dropdowns.
func (s *service) ListSiteBasicInfo(licenseId int) ([]SiteBasicInfo, error) {
	return s.repo.FindAllSites(licenseId)
}

// CreateSite creates a new site with UUID and duplicate name check.
func (s *service) CreateSite(site *SiteInfo, licenseId int) error {
	if site.SiteName == nil || *site.SiteName == "" {
		return apperror.ErrInvalidInput.WithMessage("site name is required")
	}

	// Check for duplicate name within the same license
	existing, _ := s.repo.FindSiteByNameAndLicense(*site.SiteName, licenseId)
	if existing != nil {
		return apperror.ErrConflict.WithMessage("site name already exists")
	}

	// Generate UUID
	site.Id = uuid.New().String()
	site.LicenseId = &licenseId
	now := time.Now()
	site.CreationTime = &now

	return s.repo.Create(site)
}

// UpdateSite updates an existing site with duplicate name check.
func (s *service) UpdateSite(id string, site *SiteInfo, licenseId int) error {
	if site.SiteName == nil || *site.SiteName == "" {
		return apperror.ErrInvalidInput.WithMessage("site name is required")
	}

	// Load existing
	existing, err := s.repo.FindByID(id)
	if err != nil {
		return apperror.ErrNotFound.WithMessage("site not found")
	}

	// Check for duplicate name (exclude current site)
	if *site.SiteName != *existing.SiteName {
		dup, _ := s.repo.FindSiteByNameAndLicense(*site.SiteName, licenseId)
		if dup != nil {
			return apperror.ErrConflict.WithMessage("site name already exists")
		}
	}

	// Update fields
	existing.SiteName = site.SiteName
	existing.Description = site.Description
	existing.AreaId = site.AreaId
	existing.Latitude = site.Latitude
	existing.Longitude = site.Longitude

	return s.repo.Save(existing)
}

// DeleteSite deletes a site and nullifies device references.
func (s *service) DeleteSite(id string) error {
	// First, detach all devices from this site
	if err := s.repo.NullifyDeviceSiteId(id); err != nil {
		logger.Warnf("DeleteSite: failed to nullify device site_id: %v", err)
	}

	// Then delete the site
	return s.repo.DeleteByID(id)
}

// resolveAreaPath walks up the area parent chain to build a display path like "Region/District/Zone".
func (s *service) resolveAreaPath(areaMap map[int]*SysArea, areaId int) string {
	area, ok := areaMap[areaId]
	if !ok {
		return ""
	}

	path := ""
	if area.AreaName != nil {
		path = *area.AreaName
	}

	// Walk up the parent chain
	current := area
	for current.PId != nil {
		parent, ok := areaMap[*current.PId]
		if !ok {
			break
		}
		if parent.AreaName != nil {
			path = *parent.AreaName + "/" + path
		}
		current = parent
	}

	return path
}

// ---------- SystemConfig ----------

func (s *service) GetSystemConfig() (*SystemConfig, error) {
	return s.repo.FindSystemConfig()
}

func (s *service) UpdateSystemConfig(configJSON string) error {
	// Validate that the incoming string is valid JSON.
	var js json.RawMessage
	if err := json.Unmarshal([]byte(configJSON), &js); err != nil {
		return apperror.ErrInvalidInput.WithMessage("invalid JSON configuration")
	}
	cfg := &SystemConfig{Config: &configJSON}
	return s.repo.UpdateSystemConfig(cfg)
}

// ---------- SystemParameter ----------

func (s *service) ListSystemParameters() ([]SystemParameter, error) {
	return s.repo.FindSystemParameters()
}

func (s *service) UpdateSystemParameter(p *SystemParameter) error {
	return s.repo.UpdateSystemParameter(p)
}

// Ensure strconv is used (for potential future use)
var _ = strconv.Itoa

// newService creates a Service backed by the given Repository (test/mock helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}
