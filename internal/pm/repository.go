package pm

import (
	"time"

	"nmsappsrv/pkg/baserepo"

	"gorm.io/gorm"
)

// Repository provides data access for PM-related models.
// It embeds BaseRepository[PerformanceKpi, string] for standard CRUD on PerformanceKpi,
// and retains module-specific methods for other entity types and custom queries.
type Repository struct {
	*baserepo.BaseRepository[PerformanceKpi, string]
	db *gorm.DB
}

// NewRepository creates a new PM repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		BaseRepository: baserepo.New[PerformanceKpi, string](db, "id"),
		db:             db,
	}
}

// ---------------------------------------------------------------------------
// PerformanceKpi – module-specific queries (base provides Create/Save/FindByID/DeleteByID)
// ---------------------------------------------------------------------------

// FindKPIs returns all KPIs for the given tenancy.
func (r *Repository) FindKPIs(tenancyId int) ([]PerformanceKpi, error) {
	var items []PerformanceKpi
	err := r.db.Where("tenancy_id = ?", tenancyId).Find(&items).Error
	return items, err
}

// ---------------------------------------------------------------------------
// PerformanceKpiSet (different entity type)
// ---------------------------------------------------------------------------

func (r *Repository) FindKPISets(tenancyId int) ([]PerformanceKpiSet, error) {
	var items []PerformanceKpiSet
	err := r.db.Where("tenancy_id = ?", tenancyId).Find(&items).Error
	return items, err
}

func (r *Repository) CreateKPISet(s *PerformanceKpiSet) error {
	return r.db.Create(s).Error
}

// ---------------------------------------------------------------------------
// PerformanceKpiTemplate (different entity type)
// ---------------------------------------------------------------------------

func (r *Repository) FindKPITemplates(tenancyId int) ([]PerformanceKpiTemplate, error) {
	var items []PerformanceKpiTemplate
	err := r.db.Where("tenancy_id = ?", tenancyId).Find(&items).Error
	return items, err
}

func (r *Repository) CreateKPITemplate(t *PerformanceKpiTemplate) error {
	return r.db.Create(t).Error
}

func (r *Repository) UpdateKPITemplate(t *PerformanceKpiTemplate) error {
	return r.db.Save(t).Error
}

func (r *Repository) DeleteKPITemplate(id int) error {
	return r.db.Where("id = ?", id).Delete(&PerformanceKpiTemplate{}).Error
}

// ---------------------------------------------------------------------------
// PMFileLog
// ---------------------------------------------------------------------------

func (r *Repository) FindPMFileLogs(tenancyId int, offset, limit int) ([]PMFileLog, int64, error) {
	var items []PMFileLog
	var total int64
	q := r.db.Model(&PMFileLog{}).Where("tenancy_id = ?", tenancyId)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Offset(offset).Limit(limit).Order("id DESC").Find(&items).Error
	return items, total, err
}

// ---------------------------------------------------------------------------
// KpiAlarmTemplate (different entity type)
// ---------------------------------------------------------------------------

func (r *Repository) FindKPIAlarmTemplates(tenancyId int) ([]KpiAlarmTemplate, error) {
	var items []KpiAlarmTemplate
	err := r.db.Where("tenancy_id = ?", tenancyId).Find(&items).Error
	return items, err
}

func (r *Repository) CreateKPIAlarmTemplate(t *KpiAlarmTemplate) error {
	return r.db.Create(t).Error
}

func (r *Repository) UpdateKPIAlarmTemplate(t *KpiAlarmTemplate) error {
	return r.db.Save(t).Error
}

func (r *Repository) DeleteKPIAlarmTemplate(id int) error {
	return r.db.Where("id = ?", id).Delete(&KpiAlarmTemplate{}).Error
}

// ---------------------------------------------------------------------------
// DashboardPmStatisticData
// ---------------------------------------------------------------------------

func (r *Repository) FindDashboardData(tenancyId int, startTime, endTime time.Time) ([]DashboardPmStatisticData, error) {
	var items []DashboardPmStatisticData
	err := r.db.Where("tenancy_id = ? AND time BETWEEN ? AND ?", tenancyId, startTime, endTime).Find(&items).Error
	return items, err
}

// ---------------------------------------------------------------------------
// PDCPTraffic
// ---------------------------------------------------------------------------

func (r *Repository) FindPDCPTraffic(tenancyId int, startTime, endTime time.Time) ([]PDCPTraffic, error) {
	var items []PDCPTraffic
	err := r.db.Where("tenancy_id = ? AND statistic_time BETWEEN ? AND ?", tenancyId, startTime, endTime).Find(&items).Error
	return items, err
}

// ---------------------------------------------------------------------------
// Dashboard: cpe_element queries
// ---------------------------------------------------------------------------

// FindAllActiveElements queries all non-deleted devices for the given license.
func (r *Repository) FindAllActiveElements(licenseId int) ([]elementRow, error) {
	var rows []elementRow
	err := r.db.Table("cpe_element").
		Select("ne_neid, device_type, generation, model_name").
		Where("deleted = 0 AND license_id = ?", licenseId).
		Find(&rows).Error
	return rows, err
}

// FindAllActiveElementsAllTenants queries all non-deleted devices (no tenancy filter).
func (r *Repository) FindAllActiveElementsAllTenants() ([]elementRow, error) {
	var rows []elementRow
	err := r.db.Table("cpe_element").
		Select("ne_neid, device_type, generation, model_name").
		Where("deleted = 0").
		Find(&rows).Error
	return rows, err
}
