package pm

import (
	"strconv"
	"strings"
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
	FindAllKPIs() ([]PerformanceKpi, error)
	FindKPISets(tenancyId int) ([]PerformanceKpiSet, error)
	CreateKPISet(s *PerformanceKpiSet) error
	DeleteKPISet(id int) error
	FindKPITemplates(tenancyId int) ([]PerformanceKpiTemplate, error)
	FindKPITemplate(id int) (*PerformanceKpiTemplate, error)
	CreateKPITemplate(t *PerformanceKpiTemplate) error
	UpdateKPITemplate(t *PerformanceKpiTemplate) error
	DeleteKPITemplate(id int) error
	FindPMFileLogs(tenancyId int, offset, limit int) ([]PMFileLog, int64, error)
	FindKPIAlarmTemplates(tenancyId int) ([]KpiAlarmTemplate, error)
	FindKPIAlarmTemplate(id int) (*KpiAlarmTemplate, error)
	CreateKPIAlarmTemplate(t *KpiAlarmTemplate) error
	UpdateKPIAlarmTemplate(t *KpiAlarmTemplate) error
	DeleteKPIAlarmTemplate(id int) error
	UpdateKPIAlarmTemplateStatus(id int, enable bool) error
	FindDashboardData(tenancyId int, startTime, endTime time.Time) ([]DashboardPmStatisticData, error)
	FindPDCPTraffic(tenancyId int, startTime, endTime time.Time) ([]PDCPTraffic, error)
	FindAllActiveElements(licenseId int) ([]elementRow, error)
	FindAllActiveElementsAllTenants() ([]elementRow, error)
	BulkCreateKPIs(items []PerformanceKpi) error
	FindPMFileLogsInRange(elementId int64, startTime, endTime time.Time) ([]PMFileLog, error)
	FindENBDevicesForMeas(licenseId int, searchText string, offset, limit int) ([]MeasDeviceVo, int64, error)
	CreateReplenishTask(t *PMReplenishTask) error
	FindReplenishTasks(tenancyId int, name string, offset, limit int) ([]PMReplenishTask, int64, error)
	FindReplenishTask(id int) (*PMReplenishTask, error)
	FindReplenishTaskDevices(taskId int) ([]ReplenishDeviceVo, error)
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

func (r *repository) FindAllKPIs() ([]PerformanceKpi, error) {
	var items []PerformanceKpi
	err := r.db.Find(&items).Error
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

func (r *repository) DeleteKPISet(id int) error {
	return r.db.Where("id = ?", id).Delete(&PerformanceKpiSet{}).Error
}

// ---------------------------------------------------------------------------
// PerformanceKpiTemplate (different entity type)
// ---------------------------------------------------------------------------

func (r *repository) FindKPITemplates(tenancyId int) ([]PerformanceKpiTemplate, error) {
	var items []PerformanceKpiTemplate
	err := r.db.Where("tenancy_id = ?", tenancyId).Find(&items).Error
	return items, err
}

func (r *repository) FindKPITemplate(id int) (*PerformanceKpiTemplate, error) {
	var item PerformanceKpiTemplate
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
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

func (r *repository) FindKPIAlarmTemplate(id int) (*KpiAlarmTemplate, error) {
	var item KpiAlarmTemplate
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *repository) CreateKPIAlarmTemplate(t *KpiAlarmTemplate) error {
	return r.db.Create(t).Error
}

func (r *repository) UpdateKPIAlarmTemplate(t *KpiAlarmTemplate) error {
	return r.db.Save(t).Error
}

func (r *repository) UpdateKPIAlarmTemplateStatus(id int, enable bool) error {
	return r.db.Model(&KpiAlarmTemplate{}).
		Where("id = ?", id).
		Update("enable", enable).Error
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

// BulkCreateKPIs inserts a batch of KPI rows in one statement. Mirrors the
// bulk path of Java importKPI (Go uses JSON; Java uses xlsx).
func (r *repository) BulkCreateKPIs(items []PerformanceKpi) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.Create(&items).Error
}

// FindPMFileLogsInRange returns the PMFileLog rows for a device inside
// [startTime, endTime], newest first. Used by DownloadPMFile to pick the
// file to serve. Mirrors Java pmFileLogService.getByElementIdAndStartTimeBetween.
func (r *repository) FindPMFileLogsInRange(elementId int64, startTime, endTime time.Time) ([]PMFileLog, error) {
	var items []PMFileLog
	err := r.db.Where("ne_id = ? AND start_time BETWEEN ? AND ?", elementId, startTime, endTime).
		Order("start_time DESC").
		Find(&items).Error
	return items, err
}

// FindENBDevicesForMeas returns paginated eNB devices (device_type='enb')
// for the license, optionally filtered by a name/serial search text.
// Mirrors Java listKPIMeas (which queries NeElement where device_type='enb').
func (r *repository) FindENBDevicesForMeas(licenseId int, searchText string, offset, limit int) ([]MeasDeviceVo, int64, error) {
	var items []MeasDeviceVo
	var total int64

	q := r.db.Table("cpe_element").
		Select("ne_neid, device_name, serial_number, device_type, root_node").
		Where("deleted = 0 AND device_type = 'enb' AND license_id = ?", licenseId)
	if searchText != "" {
		like := "%" + searchText + "%"
		q = q.Where("device_name LIKE ? OR serial_number LIKE ?", like, like)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Offset(offset).Limit(limit).Order("ne_neid ASC").Scan(&items).Error
	return items, total, err
}

// CreateReplenishTask inserts a new pm_replenish_task row. Mirrors Java
// addReplenishTask.
func (r *repository) CreateReplenishTask(t *PMReplenishTask) error {
	return r.db.Create(t).Error
}

// FindReplenishTasks returns paginated replenish tasks for the license,
// optionally filtered by name. Mirrors Java listReplenishTask.
func (r *repository) FindReplenishTasks(tenancyId int, name string, offset, limit int) ([]PMReplenishTask, int64, error) {
	var items []PMReplenishTask
	var total int64
	q := r.db.Model(&PMReplenishTask{}).Where("tenancy_id = ?", tenancyId)
	if name != "" {
		like := "%" + name + "%"
		q = q.Where("name LIKE ?", like)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}

// FindReplenishTask returns a single replenish task by id. Mirrors Java
// viewReplenishTask.
func (r *repository) FindReplenishTask(id int) (*PMReplenishTask, error) {
	var t PMReplenishTask
	if err := r.db.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// FindReplenishTaskDevices returns the cpe_element rows listed in the
// task's comma-separated element_ids column. The Go side does not have
// a separate pm_replenish_task_device join table; the Java entity
// stores element ids as a @Lob String. Mirrors Java listDeviceReplenish.
func (r *repository) FindReplenishTaskDevices(taskId int) ([]ReplenishDeviceVo, error) {
	t, err := r.FindReplenishTask(taskId)
	if err != nil {
		return nil, err
	}
	if t.ElementIds == nil || *t.ElementIds == "" {
		return []ReplenishDeviceVo{}, nil
	}
	parts := strings.Split(*t.ElementIds, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return []ReplenishDeviceVo{}, nil
	}
	type row struct {
		NeNeid       int64   `gorm:"column:ne_neid"`
		DeviceName   *string `gorm:"column:device_name"`
		SerialNumber *string `gorm:"column:serial_number"`
	}
	var rows []row
	if err := r.db.Table("cpe_element").
		Select("ne_neid, device_name, serial_number").
		Where("ne_neid IN ? AND deleted = 0", ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ReplenishDeviceVo, 0, len(rows))
	for _, r := range rows {
		out = append(out, ReplenishDeviceVo{
			NeNeid:       r.NeNeid,
			DeviceName:   r.DeviceName,
			SerialNumber: r.SerialNumber,
			Done:         false, // Go side: replenish worker not ported yet.
		})
	}
	return out, nil
}
