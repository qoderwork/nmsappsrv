package devicelog

import (
	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository provides data access for device log entities.
// It embeds BaseRepository[NeLog, int64] for standard CRUD on NeLog,
// and retains module-specific methods for custom queries.
type Repository struct {
	*baserepo.BaseRepository[NeLog, int64]
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		BaseRepository: baserepo.New[NeLog, int64](db, "id"),
		db:             db,
	}
}

func (r *Repository) DeleteByElementId(elementId int64) error {
	return r.db.Where("element_id = ?", elementId).Delete(&NeLog{}).Error
}

func (r *Repository) FindByElementId(elementId int64, offset, limit int) ([]NeLog, int64, error) {
	var logs []NeLog
	var total int64

	query := r.db.Where("element_id = ?", elementId)
	query.Model(&NeLog{}).Count(&total)

	err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&logs).Error
	if err != nil {
		logger.Errorf("FindByElementId error: %v", err)
		return nil, 0, err
	}

	return logs, total, nil
}

func (r *Repository) FindAllByElementId(elementId int64) ([]NeLog, error) {
	var logs []NeLog
	err := r.db.Where("element_id = ?", elementId).Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

func (r *Repository) FindByFilter(elementId *int64, deviceType *string, status *int, offset, limit int) ([]LogCollectionResultVo, int64, error) {
	var results []LogCollectionResultVo
	var total int64

	query := r.db.Table("ne_log").
		Select("ne_log.*, cpe_element.device_name, cpe_element.serial_number").
		Joins("LEFT JOIN cpe_element ON ne_log.element_id = cpe_element.ne_neid").
		Where("cpe_element.deleted = ?", false)

	if elementId != nil {
		query = query.Where("ne_log.element_id = ?", *elementId)
	}
	if deviceType != nil && *deviceType != "" {
		query = query.Where("cpe_element.device_type = ?", *deviceType)
	}
	if status != nil {
		query = query.Where("ne_log.status = ?", *status)
	}

	// Count total
	countQuery := query
	countQuery.Count(&total)

	// Fetch results
	err := query.Offset(offset).Limit(limit).Order("ne_log.id DESC").Find(&results).Error
	if err != nil {
		logger.Errorf("FindByFilter error: %v", err)
		return nil, 0, err
	}

	return results, total, nil
}
