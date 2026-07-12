package topology

import (
	"nmsappsrv/internal/device"
	"nmsappsrv/internal/upgrade"

	"gorm.io/gorm"
)

// Repository provides data-access methods for the topology module.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// FindElementById fetches a CpeElement by its primary key (ne_neid).
func (r *Repository) FindElementById(elementId int64) (*device.CpeElement, error) {
	var elem device.CpeElement
	err := r.db.Where("ne_neid = ? AND deleted = ?", elementId, false).First(&elem).Error
	if err != nil {
		return nil, err
	}
	return &elem, nil
}

// FindParamsByElementIdAndNameLike fetches element_basic_info_parameter rows
// where param_name matches the given LIKE pattern.
// Java: elementHasParamService.getParamsByElementIdAndParamNameLike
func (r *Repository) FindParamsByElementIdAndNameLike(elementId int64, nameLike string) ([]device.ElementBasicInfoParameter, error) {
	var params []device.ElementBasicInfoParameter
	err := r.db.Where("element_id = ? AND param_name LIKE ?", elementId, nameLike).Find(&params).Error
	return params, err
}

// FindLeafParamsByElementIdAndNameLike fetches element_basic_info_parameter rows
// where param_name matches the given LIKE pattern AND param_value is not empty (leaf node).
// Java: elementHasParamService.getByElementIdAndParamNameLikeAndLeaf(... true)
func (r *Repository) FindLeafParamsByElementIdAndNameLike(elementId int64, nameLike string) ([]device.ElementBasicInfoParameter, error) {
	var params []device.ElementBasicInfoParameter
	err := r.db.Where("element_id = ? AND param_name LIKE ? AND param_value IS NOT NULL AND param_value != ''", elementId, nameLike).Find(&params).Error
	return params, err
}

// FindParamsByElementIdAndNameIn fetches element_basic_info_parameter rows
// where param_name is in the given list.
// Java: elementHasParamService.getByElementIdAndParamNameIn
func (r *Repository) FindParamsByElementIdAndNameIn(elementId int64, names []string) ([]device.ElementBasicInfoParameter, error) {
	if len(names) == 0 {
		return nil, nil
	}
	var params []device.ElementBasicInfoParameter
	err := r.db.Where("element_id = ? AND param_name IN ?", elementId, names).Find(&params).Error
	return params, err
}

// FindLeafParamByElementIdAndNameLikeAndValue fetches a single param row
// where param_name matches LIKE pattern and param_value equals the given value.
// Java: elementHasParamService.getByElementIdAndParamNameLikeAndParamValue
func (r *Repository) FindLeafParamByElementIdAndNameLikeAndValue(elementId int64, nameLike, value string) (*device.ElementBasicInfoParameter, error) {
	var param device.ElementBasicInfoParameter
	err := r.db.Where("element_id = ? AND param_name LIKE ? AND param_value = ?", elementId, nameLike, value).First(&param).Error
	if err != nil {
		return nil, err
	}
	return &param, nil
}

// DeleteParamsByIds deletes element_basic_info_parameter rows by param_id list for a given element.
// Java: elementHasParamService.deleteByElementIdAndIdIn
func (r *Repository) DeleteParamsByIds(elementId int64, paramIds []int64) error {
	if len(paramIds) == 0 {
		return nil
	}
	return r.db.Where("element_id = ? AND param_id IN ?", elementId, paramIds).
		Delete(&device.ElementBasicInfoParameter{}).Error
}

// FindUpgradeFileById fetches an UpgradeFile by its primary key.
func (r *Repository) FindUpgradeFileById(fileId int) (*upgrade.UpgradeFile, error) {
	var f upgrade.UpgradeFile
	err := r.db.Where("id = ?", fileId).First(&f).Error
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// FindUpgradeFilesByIds fetches UpgradeFile rows by a set of IDs.
func (r *Repository) FindUpgradeFilesByIds(ids []int) ([]upgrade.UpgradeFile, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var files []upgrade.UpgradeFile
	err := r.db.Where("id IN ?", ids).Find(&files).Error
	return files, err
}

// SaveBatchUpgradeLog inserts a new EUAndRUBatchUpgradeLog row.
func (r *Repository) SaveBatchUpgradeLog(log *upgrade.EUAndRUBatchUpgradeLog) error {
	return r.db.Create(log).Error
}

// FindBatchUpgradeLogs queries eu_and_ru_batch_upgrade_log with pagination and filters.
func (r *Repository) FindBatchUpgradeLogs(q *ListBatchUpgradeLogQuery, offset, limit int) ([]upgrade.EUAndRUBatchUpgradeLog, int64, error) {
	tx := r.db.Model(&upgrade.EUAndRUBatchUpgradeLog{}).Where("element_id = ?", q.ElementId)

	if q.OperationUser != "" {
		tx = tx.Where("user LIKE ?", "%"+q.OperationUser+"%")
	}
	if q.StartTime != nil {
		tx = tx.Where("operation_time >= ?", *q.StartTime)
	}
	if q.EndTime != nil {
		tx = tx.Where("operation_time <= ?", *q.EndTime)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []upgrade.EUAndRUBatchUpgradeLog
	err := tx.Order("operation_time DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}
