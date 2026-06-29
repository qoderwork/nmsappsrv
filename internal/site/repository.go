package site

import (
	"gorm.io/gorm"
)

// Repository provides data access for site-related models.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new site repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ---------- SysArea ----------

func (r *Repository) FindAreas(tenancyId int) ([]SysArea, error) {
	var items []SysArea
	err := r.db.Where("tenancy_id = ?", tenancyId).Find(&items).Error
	return items, err
}

func (r *Repository) FindAreaByID(id int) (*SysArea, error) {
	var item SysArea
	err := r.db.Where("id = ?", id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateArea(a *SysArea) error {
	return r.db.Create(a).Error
}

func (r *Repository) UpdateArea(a *SysArea) error {
	return r.db.Save(a).Error
}

func (r *Repository) DeleteArea(id int) error {
	return r.db.Where("id = ?", id).Delete(&SysArea{}).Error
}

// FindChildAreas returns areas that have the given parent ID.
func (r *Repository) FindChildAreas(parentId int) ([]SysArea, error) {
	var items []SysArea
	err := r.db.Where("p_id = ?", parentId).Find(&items).Error
	return items, err
}

// ---------- SiteInfo ----------

// FindSites returns a paginated list of sites for the given license, optionally filtered by name.
func (r *Repository) FindSites(licenseId int, search string, offset, limit int) ([]SiteInfo, int64, error) {
	var sites []SiteInfo
	var total int64

	query := r.db.Model(&SiteInfo{}).Where("license_id = ?", licenseId)
	if search != "" {
		query = query.Where("site_name LIKE ?", "%"+search+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&sites).Error; err != nil {
		return nil, 0, err
	}
	return sites, total, nil
}

// FindAllSites returns all sites for the given license (for dropdown).
func (r *Repository) FindAllSites(licenseId int) ([]SiteBasicInfo, error) {
	var items []SiteBasicInfo
	err := r.db.Model(&SiteInfo{}).
		Select("id, site_name").
		Where("license_id = ?", licenseId).
		Order("site_name ASC").
		Find(&items).Error
	return items, err
}

// FindSiteByID returns a site by its ID.
func (r *Repository) FindSiteByID(id string) (*SiteInfo, error) {
	var site SiteInfo
	err := r.db.Where("id = ?", id).First(&site).Error
	if err != nil {
		return nil, err
	}
	return &site, nil
}

// FindSiteByNameAndLicense returns a site with the given name and license (for duplicate check).
func (r *Repository) FindSiteByNameAndLicense(name string, licenseId int) (*SiteInfo, error) {
	var site SiteInfo
	err := r.db.Where("site_name = ? AND license_id = ?", name, licenseId).First(&site).Error
	if err != nil {
		return nil, err
	}
	return &site, nil
}

// CreateSite inserts a new site.
func (r *Repository) CreateSite(site *SiteInfo) error {
	return r.db.Create(site).Error
}

// UpdateSite saves changes to an existing site.
func (r *Repository) UpdateSite(site *SiteInfo) error {
	return r.db.Save(site).Error
}

// DeleteSite removes a site by ID.
func (r *Repository) DeleteSite(id string) error {
	return r.db.Where("id = ?", id).Delete(&SiteInfo{}).Error
}

// NullifyDeviceSiteId sets site_id = NULL on all devices referencing this site.
func (r *Repository) NullifyDeviceSiteId(siteId string) error {
	return r.db.Exec("UPDATE cpe_element SET site_id = NULL WHERE site_id = ?", siteId).Error
}

// ---------- SystemConfig ----------

func (r *Repository) FindSystemConfig() (*SystemConfig, error) {
	var cfg SystemConfig
	err := r.db.First(&cfg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &SystemConfig{}, nil
		}
		return nil, err
	}
	return &cfg, nil
}

func (r *Repository) UpdateSystemConfig(cfg *SystemConfig) error {
	var existing SystemConfig
	err := r.db.First(&existing).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return r.db.Create(cfg).Error
		}
		return err
	}
	existing.Config = cfg.Config
	return r.db.Save(&existing).Error
}

// ---------- SystemParameter ----------

func (r *Repository) FindSystemParameters() ([]SystemParameter, error) {
	var items []SystemParameter
	err := r.db.Find(&items).Error
	return items, err
}

func (r *Repository) UpdateSystemParameter(p *SystemParameter) error {
	return r.db.Save(p).Error
}
