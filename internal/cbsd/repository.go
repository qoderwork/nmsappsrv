package cbsd

import (
	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for CBSD entities.
type Repository interface {
	Create(entity *CbsdInfo) error
	Save(entity *CbsdInfo) error
	FindByID(id string) (*CbsdInfo, error)
	DeleteByID(id string) error
	DeleteByIDs(ids []string) error
	SoftDelete(id string) error
	UpdateFields(id string, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]CbsdInfo, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[CbsdInfo], error)
	FindCbsdInfos(licenseId int, offset, limit int) ([]CbsdInfo, int64, error)
	FindCbsdInfoBySN(sn string, licenseId int) (*CbsdInfo, error)
	FindCbrsLogs(cbsdId string, logType string, offset, limit int) ([]CbrsLog, int64, error)
	CreateCbrsLog(log *CbrsLog) error
	CreateCertFileSendTask(t *CBSDCertFileSendTask) error
	FindCertFileSendTasks(tenancyId int, offset, limit int) ([]CBSDCertFileSendTask, int64, error)

	// CBSD lifecycle
	FindCbsdInfoByID(id string) (*CbsdInfo, error)
	UpdateCbsdEnable(id string, enable bool) error
	CountCbsdByStatus(licenseId int) ([]CbsdStatusCountItem, error)
	BulkCreateCbsdInfos(infos []CbsdInfo) error

	// SAS config
	FindSasConfigs(licenseId int) ([]SasConfig, error)
	FindSasConfigByID(id int64) (*SasConfig, error)
	UpdateSasConfig(cfg *SasConfig) error

	// SAS operation-state maintenance
	FindCbsdInfosByStates(states []string) ([]CbsdInfo, error)
}

// repository is the concrete GORM-backed implementation of Repository.
// It embeds BaseRepository[CbsdInfo, string] for standard CRUD on CbsdInfo,
// and retains module-specific methods for custom queries and other entity types.
type repository struct {
	*baserepo.BaseRepository[CbsdInfo, string]
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[CbsdInfo, string](db, "id"),
		db:             db,
	}
}

// ---------------------------------------------------------------------------
// CbsdInfo – module-specific queries
// ---------------------------------------------------------------------------

// FindCbsdInfos returns a paginated list of CBSD info records for the given license.
func (r *repository) FindCbsdInfos(licenseId int, offset, limit int) ([]CbsdInfo, int64, error) {
	var infos []CbsdInfo
	var total int64

	query := r.db.Model(&CbsdInfo{})
	if licenseId > 0 {
		query = query.Where("license_id = ?", licenseId)
	}

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindCbsdInfos count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&infos).Error; err != nil {
		logger.Errorf("FindCbsdInfos query error: %v", err)
		return nil, 0, err
	}
	return infos, total, nil
}

// FindCbsdInfoBySN looks up a CBSD info by serial number and license.
func (r *repository) FindCbsdInfoBySN(sn string, licenseId int) (*CbsdInfo, error) {
	var info CbsdInfo
	query := r.db.Where("cbsd_serial_number = ?", sn)
	if licenseId > 0 {
		query = query.Where("license_id = ?", licenseId)
	}
	if err := query.First(&info).Error; err != nil {
		return nil, err
	}
	return &info, nil
}

// ---------------------------------------------------------------------------
// CbrsLog – different entity type, kept as module-specific methods
// ---------------------------------------------------------------------------

// FindCbrsLogs returns a paginated list of CBRS logs filtered by cbsd_id and log_type.
func (r *repository) FindCbrsLogs(cbsdId string, logType string, offset, limit int) ([]CbrsLog, int64, error) {
	var logs []CbrsLog
	var total int64

	query := r.db.Model(&CbrsLog{})
	if cbsdId != "" {
		query = query.Where("cbsd_id = ?", cbsdId)
	}
	if logType != "" {
		query = query.Where("log_type = ?", logType)
	}

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindCbrsLogs count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		logger.Errorf("FindCbrsLogs query error: %v", err)
		return nil, 0, err
	}
	return logs, total, nil
}

// CreateCbrsLog inserts a new CBRS log record.
func (r *repository) CreateCbrsLog(log *CbrsLog) error {
	return r.db.Create(log).Error
}

// ---------------------------------------------------------------------------
// CBSDCertFileSendTask – different entity type, kept as module-specific methods
// ---------------------------------------------------------------------------

// CreateCertFileSendTask inserts a new cert file send task.
func (r *repository) CreateCertFileSendTask(t *CBSDCertFileSendTask) error {
	return r.db.Create(t).Error
}

// FindCertFileSendTasks returns a paginated list of cert file send tasks for the given tenancy.
func (r *repository) FindCertFileSendTasks(tenancyId int, offset, limit int) ([]CBSDCertFileSendTask, int64, error) {
	var tasks []CBSDCertFileSendTask
	var total int64

	query := r.db.Model(&CBSDCertFileSendTask{})
	if tenancyId > 0 {
		query = query.Where("tenancy_id = ?", tenancyId)
	}

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindCertFileSendTasks count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		logger.Errorf("FindCertFileSendTasks query error: %v", err)
		return nil, 0, err
	}
	return tasks, total, nil
}

// ---------------------------------------------------------------------------
// CBSD lifecycle
// ---------------------------------------------------------------------------

// FindCbsdInfoByID returns a single CBSD info by its primary key.
func (r *repository) FindCbsdInfoByID(id string) (*CbsdInfo, error) {
	var info CbsdInfo
	if err := r.db.Where("id = ?", id).First(&info).Error; err != nil {
		return nil, err
	}
	return &info, nil
}

// UpdateCbsdEnable sets the enable flag for a CBSD record.
func (r *repository) UpdateCbsdEnable(id string, enable bool) error {
	return r.db.Model(&CbsdInfo{}).Where("id = ?", id).Update("enable", enable).Error
}

// CountCbsdByStatus returns per-operation_state counts for a given license.
func (r *repository) CountCbsdByStatus(licenseId int) ([]CbsdStatusCountItem, error) {
	var results []CbsdStatusCountItem
	type row struct {
		OperationState string `gorm:"column:operation_state"`
		Cnt            int64  `gorm:"column:cnt"`
	}
	var rows []row
	query := r.db.Model(&CbsdInfo{}).
		Select("operation_state, COUNT(*) as cnt")
	if licenseId > 0 {
		query = query.Where("license_id = ?", licenseId)
	}
	err := query.Group("operation_state").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		results = append(results, CbsdStatusCountItem{Status: row.OperationState, Count: row.Cnt})
	}
	return results, nil
}

// BulkCreateCbsdInfos inserts multiple CBSD info records in a batch.
func (r *repository) BulkCreateCbsdInfos(infos []CbsdInfo) error {
	return r.db.CreateInBatches(infos, 100).Error
}

// ---------------------------------------------------------------------------
// SAS config
// ---------------------------------------------------------------------------

// FindSasConfigs returns all SAS configs for the given license.
func (r *repository) FindSasConfigs(licenseId int) ([]SasConfig, error) {
	var configs []SasConfig
	query := r.db.Model(&SasConfig{})
	if licenseId > 0 {
		query = query.Where("license_id = ?", licenseId)
	}
	if err := query.Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// FindSasConfigByID returns a single SAS config by ID.
func (r *repository) FindSasConfigByID(id int64) (*SasConfig, error) {
	var cfg SasConfig
	if err := r.db.Where("id = ?", id).First(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdateSasConfig saves changes to an existing SAS config.
func (r *repository) UpdateSasConfig(cfg *SasConfig) error {
	return r.db.Save(cfg).Error
}

// FindCbsdInfosByStates returns all CBSD info rows whose operation_state is in
// the given set. Used by the operation-state maintainer to find devices with
// active grants that may need a timeout-driven transition.
func (r *repository) FindCbsdInfosByStates(states []string) ([]CbsdInfo, error) {
	var infos []CbsdInfo
	if err := r.db.Where("operation_state IN ?", states).Find(&infos).Error; err != nil {
		return nil, err
	}
	return infos, nil
}
