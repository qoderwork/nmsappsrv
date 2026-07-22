package site

import (
	"gorm.io/gorm"

	"nmsappsrv/pkg/baserepo"
)

// Repository defines the data-access contract for site-related models.
type Repository interface {
	Create(entity *SiteInfo) error
	Save(entity *SiteInfo) error
	FindByID(id string) (*SiteInfo, error)
	DeleteByID(id string) error
	DeleteByIDs(ids []string) error
	SoftDelete(id string) error
	UpdateFields(id string, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]SiteInfo, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[SiteInfo], error)

	FindAreas(tenantId int) ([]SysArea, error)
	FindAreaByID(id int) (*SysArea, error)
	CreateArea(a *SysArea) error
	UpdateArea(a *SysArea) error
	DeleteArea(id int) error
	FindChildAreas(parentId int) ([]SysArea, error)
	FindSites(tenantId int, search string, offset, limit int) ([]SiteInfo, int64, error)
	FindAllSites(tenantId int) ([]SiteBasicInfo, error)
	FindSiteByNameAndLicense(name string, tenantId int) (*SiteInfo, error)
	NullifyDeviceSiteId(siteId string) error
	FindSystemConfig() (*SystemConfig, error)
	UpdateSystemConfig(cfg *SystemConfig) error
	FindSystemParameters() ([]SystemParameter, error)
	UpdateSystemParameter(p *SystemParameter) error
}

// repository is the concrete GORM-backed implementation of Repository.
type repository struct {
	*baserepo.BaseRepository[SiteInfo, string] // embedded generic CRUD for SiteInfo
	db *gorm.DB                                // kept for custom / cross-model queries
}

// NewRepository creates a new site repository.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[SiteInfo, string](db, "id"),
		db:             db,
	}
}

// ---------- SysArea ----------

func (r *repository) FindAreas(tenantId int) ([]SysArea, error) {
	var items []SysArea
	err := r.db.Where("tenant_id = ?", tenantId).Find(&items).Error
	return items, err
}

func (r *repository) FindAreaByID(id int) (*SysArea, error) {
	var item SysArea
	err := r.db.Where("id = ?", id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *repository) CreateArea(a *SysArea) error {
	return r.db.Create(a).Error
}

func (r *repository) UpdateArea(a *SysArea) error {
	return r.db.Save(a).Error
}

func (r *repository) DeleteArea(id int) error {
	return r.db.Where("id = ?", id).Delete(&SysArea{}).Error
}

// FindChildAreas returns areas that have the given parent ID.
func (r *repository) FindChildAreas(parentId int) ([]SysArea, error) {
	var items []SysArea
	err := r.db.Where("p_id = ?", parentId).Find(&items).Error
	return items, err
}

// ---------- SiteInfo ----------

// FindSites returns a paginated list of sites for the given license, optionally filtered by name.
func (r *repository) FindSites(tenantId int, search string, offset, limit int) ([]SiteInfo, int64, error) {
	query := r.DB.Model(&SiteInfo{}).Where("tenant_id = ?", tenantId)
	if search != "" {
		query = query.Where("site_name LIKE ?", "%"+search+"%")
	}

	result, err := r.FindPage(query, "id DESC", offset, limit)
	if err != nil {
		return nil, 0, err
	}
	return result.Items, result.Total, nil
}

// FindAllSites returns all sites for the given license (for dropdown).
func (r *repository) FindAllSites(tenantId int) ([]SiteBasicInfo, error) {
	var items []SiteBasicInfo
	err := r.db.Model(&SiteInfo{}).
		Select("id, site_name").
		Where("tenant_id = ?", tenantId).
		Order("site_name ASC").
		Find(&items).Error
	return items, err
}

// FindSiteByNameAndLicense returns a site with the given name and license (for duplicate check).
func (r *repository) FindSiteByNameAndLicense(name string, tenantId int) (*SiteInfo, error) {
	var site SiteInfo
	err := r.db.Where("site_name = ? AND tenant_id = ?", name, tenantId).First(&site).Error
	if err != nil {
		return nil, err
	}
	return &site, nil
}

// NullifyDeviceSiteId sets site_id = NULL on all devices referencing this site.
func (r *repository) NullifyDeviceSiteId(siteId string) error {
	return r.db.Exec("UPDATE cpe_element SET site_id = NULL WHERE site_id = ?", siteId).Error
}

// ---------- SystemConfig ----------

func (r *repository) FindSystemConfig() (*SystemConfig, error) {
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

func (r *repository) UpdateSystemConfig(cfg *SystemConfig) error {
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

func (r *repository) FindSystemParameters() ([]SystemParameter, error) {
	var items []SystemParameter
	err := r.db.Find(&items).Error
	return items, err
}

func (r *repository) UpdateSystemParameter(p *SystemParameter) error {
	return r.db.Save(p).Error
}
