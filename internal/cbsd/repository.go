package cbsd

import (
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository handles database operations for CBSD entities.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// FindCbsdInfos returns a paginated list of CBSD info records for the given license.
func (r *Repository) FindCbsdInfos(licenseId int, offset, limit int) ([]CbsdInfo, int64, error) {
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
func (r *Repository) FindCbsdInfoBySN(sn string, licenseId int) (*CbsdInfo, error) {
	var info CbsdInfo
	if err := r.db.Where("cbsd_serial_number = ? AND license_id = ?", sn, licenseId).First(&info).Error; err != nil {
		return nil, err
	}
	return &info, nil
}

// CreateCbsdInfo inserts a new CBSD info record.
func (r *Repository) CreateCbsdInfo(info *CbsdInfo) error {
	return r.db.Create(info).Error
}

// UpdateCbsdInfo saves changes to an existing CBSD info record.
func (r *Repository) UpdateCbsdInfo(info *CbsdInfo) error {
	return r.db.Save(info).Error
}

// DeleteCbsdInfo removes a CBSD info by its primary key.
func (r *Repository) DeleteCbsdInfo(id string) error {
	return r.db.Where("id = ?", id).Delete(&CbsdInfo{}).Error
}

// FindCbrsLogs returns a paginated list of CBRS logs filtered by cbsd_id and log_type.
func (r *Repository) FindCbrsLogs(cbsdId string, logType string, offset, limit int) ([]CbrsLog, int64, error) {
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
func (r *Repository) CreateCbrsLog(log *CbrsLog) error {
	return r.db.Create(log).Error
}

// CreateCertFileSendTask inserts a new cert file send task.
func (r *Repository) CreateCertFileSendTask(t *CBSDCertFileSendTask) error {
	return r.db.Create(t).Error
}

// FindCertFileSendTasks returns a paginated list of cert file send tasks for the given tenancy.
func (r *Repository) FindCertFileSendTasks(tenancyId int, offset, limit int) ([]CBSDCertFileSendTask, int64, error) {
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
