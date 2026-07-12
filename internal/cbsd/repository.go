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

	query := r.db.Model(&CbsdInfo{}).Where("license_id = ?", licenseId)

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
	if err := r.db.Where("cbsd_serial_number = ? AND license_id = ?", sn, licenseId).First(&info).Error; err != nil {
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

	query := r.db.Model(&CBSDCertFileSendTask{}).Where("tenancy_id = ?", tenancyId)

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
