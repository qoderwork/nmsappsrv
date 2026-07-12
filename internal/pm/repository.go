package pm

import (
	"time"

	"nmsappsrv/pkg/baserepo"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for PM-related models.
type Repository interface {
	Create(entity *PerformanceKpi) error
	Save(entity *PerformanceKpi) error
	FindByID(id string) (*PerformanceKpi, error)
	DeleteByID(id string) error
	DeleteByIDs(ids []string) error
	SoftDelete(id string) error
	UpdateFields(id string, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]PerformanceKpi, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[PerformanceKpi], error)

	FindKPIs(tenancyId int) ([]PerformanceKpi, error)
	FindKPISets(tenancyId int) ([]PerformanceKpiSet, error)
	CreateKPISet(s *PerformanceKpiSet) error
	FindKPITemplates(tenancyId int) ([]PerformanceKpiTemplate, error)
	CreateKPITemplate(t *PerformanceKpiTemplate) error
	UpdateKPITemplate(t *PerformanceKpiTemplate) error
	DeleteKPITemplate(id int) error
	FindPMFileLogs(tenancyId int, offset, limit int) ([]PMFileLog, int64, error)
	FindKPIAlarmTemplates(tenancyId int) ([]KpiAlarmTemplate, error)
	CreateKPIAlarmTemplate(t *KpiAlarmTemplate) error
	UpdateKPIAlarmTemplate(t *KpiAlarmTemplate) error
	DeleteKPIAlarmTemplate(id int) error
	FindDashboardData(tenancyId int, startTime, endTime time.Time) ([]DashboardPmStatisticData, error)
	FindPDCPTraffic(tenancyId int, startTime, endTime time.Time) ([]PDCPTraffic, error)
	FindAllActiveElements(licenseId int) ([]elementRow, error)
	FindAllActiveElementsAllTenants() ([]elementRow, error)
}

// repository is the concrete GORM-backed implementation of Repository.
// It embeds BaseRepository[PerformanceKpi, string] for standard CRUD on PerformanceKpi,
// and retains module-specific methods for other entity types and custom queries.
type repository struct {
	*baserepo.BaseRepository[PerformanceKpi, string]
	db *gorm.DB
}

// NewRepository creates a new PM repository.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[PerformanceKpi, string](db, "id"),
		db:             db,
	}
}

// ---------------------------------------------------------------------------
// PerformanceKpi – module-specific queries (base provides Create/Save/FindByID/DeleteByID)
// ---------------------------------------------------------------------------

// FindKPIs returns all KPIs for the given tenancy.
func (r *repository) FindKPIs(tenancyId int) ([]PerformanceKpi, error) {
	var items []PerformanceKpi
	err := r.db.Where("tenancy_id = ?", tenancyId).Find(&items).Error
	return items, err
}

// ---------------------------------------------------------------------------
// PerformanceKpiSet (different entity type)
// ---------------------------------------------------------------------------

func (r *repository) FindKPISets(tenancyId int) ([]PerformanceKpiSet, error) {
	var items []PerformanceKpiSet
	err := r.db.Where("tenancy_id = ?", tenancyId).Find(&items).Error
	return items, err
}

func (r *repository) CreateKPISet(s *PerformanceKpiSet) error {
	return r.db.Create(s).Error
}

// ---------------------------------------------------------------------------
// PerformanceKpiTemplate (different entity type)
// ---------------------------------------------------------------------------

func (r *repository) FindKPITemplates(tenancyId int) ([]PerformanceKpiTemplate, error) {
	var items []PerformanceKpiTemplate
	err := r.db.Where("tenancy_id = ?", tenancyId).Find(&items).Error
	return items, err
}

func (r *repository) CreateKPITemplate(t *PerformanceKpiTemplate) error {
	return r.db.Create(t).Error
}

func (r *repository) UpdateKPITemplate(t *PerformanceKpiTemplate) error {
	return r.db.Save(t).Error
}

func (r *repository) DeleteKPITemplate(id int) error {
	return r.db.Where("id = ?", id).Delete(&PerformanceKpiTemplate{}).Error
}

// ---------------------------------------------------------------------------
// PMFileLog
// ---------------------------------------------------------------------------

func (r *repository) FindPMFileLogs(tenancyId int, offset, limit int) ([]PMFileLog, int64, error) {
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

func (r *repository) FindKPIAlarmTemplates(tenancyId int) ([]KpiAlarmTemplate, error) {
	var items []KpiAlarmTemplate
	err := r.db.Where("tenancy_id = ?", tenancyId).Find(&items).Error
	return items, err
}

func (r *repository) CreateKPIAlarmTemplate(t *KpiAlarmTemplate) error {
	return r.db.Create(t).Error
}

func (r *repository) UpdateKPIAlarmTemplate(t *KpiAlarmTemplate) error {
	return r.db.Save(t).Error
}

func (r *repository) DeleteKPIAlarmTemplate(id int) error {
	return r.db.Where("id = ?", id).Delete(&KpiAlarmTemplate{}).Error
}

// ---------------------------------------------------------------------------
// DashboardPmStatisticData
// ---------------------------------------------------------------------------

func (r *repository) FindDashboardData(tenancyId int, startTime, endTime time.Time) ([]DashboardPmStatisticData, error) {
	var items []DashboardPmStatisticData
	err := r.db.Where("tenancy_id = ? AND time BETWEEN ? AND ?", tenancyId, startTime, endTime).Find(&items).Error
	return items, err
}

// ---------------------------------------------------------------------------
// PDCPTraffic
// ---------------------------------------------------------------------------

func (r *repository) FindPDCPTraffic(tenancyId int, startTime, endTime time.Time) ([]PDCPTraffic, error) {
	var items []PDCPTraffic
	err := r.db.Where("tenancy_id = ? AND statistic_time BETWEEN ? AND ?", tenancyId, startTime, endTime).Find(&items).Error
	return items, err
}

// ---------------------------------------------------------------------------
// Dashboard: cpe_element queries
// ---------------------------------------------------------------------------

// FindAllActiveElements queries all non-deleted devices for the given license.
func (r *repository) FindAllActiveElements(licenseId int) ([]elementRow, error) {
	var rows []elementRow
	err := r.db.Table("cpe_element").
		Select("ne_neid, device_type, generation, model_name").
		Where("deleted = 0 AND license_id = ?", licenseId).
		Find(&rows).Error
	return rows, err
}

// FindAllActiveElementsAllTenants queries all non-deleted devices (no tenancy filter).
func (r *repository) FindAllActiveElementsAllTenants() ([]elementRow, error) {
	var rows []elementRow
	err := r.db.Table("cpe_element").
		Select("ne_neid, device_type, generation, model_name").
		Where("deleted = 0").
		Find(&rows).Error
	return rows, err
}
