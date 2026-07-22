package devicelog

import (
	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for device log entities.
// It embeds BaseRepository[NeLog, int64] for standard CRUD on NeLog,
// and retains module-specific methods for custom queries.
type Repository interface {
	// Generic CRUD delegated to BaseRepository[NeLog, int64].
	Create(entity *NeLog) error
	Save(entity *NeLog) error
	FindByID(id int64) (*NeLog, error)
	DeleteByID(id int64) error
	DeleteByIDs(ids []int64) error
	SoftDelete(id int64) error
	UpdateFields(id int64, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]NeLog, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[NeLog], error)

	// Module-specific methods.
	DeleteByElementId(elementId int64) error
	FindByElementId(elementId int64, tenantId *int, offset, limit int) ([]NeLog, int64, error)
	FindAllByElementId(elementId int64) ([]NeLog, error)
	FindByFilter(tenantId *int, elementId *int64, deviceType *string, status *int, offset, limit int) ([]LogCollectionResultVo, int64, error)
	FindDeviceRootNode(elementId int64) (*string, error)
}

// repository is the concrete GORM-backed implementation of Repository.
type repository struct {
	*baserepo.BaseRepository[NeLog, int64]
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[NeLog, int64](db, "id"),
		db:             db,
	}
}

// FindDeviceRootNode returns the TR-069 root_node of a device (or nil if not found).
func (r *repository) FindDeviceRootNode(elementId int64) (*string, error) {
	var device struct {
		RootNode *string `gorm:"column:root_node"`
	}
	if err := r.db.Table("cpe_element").
		Where("ne_neid = ? AND deleted = ?", elementId, false).
		First(&device).Error; err != nil {
		return nil, err
	}
	return device.RootNode, nil
}

func (r *repository) DeleteByElementId(elementId int64) error {
	return r.db.Where("ne_id = ?", elementId).Delete(&NeLog{}).Error
}

func (r *repository) FindByElementId(elementId int64, tenantId *int, offset, limit int) ([]NeLog, int64, error) {
	var logs []NeLog
	var total int64

	query := r.db.Where("ne_id = ?", elementId)
	if tenantId != nil {
		query = query.Where("tenant_id = ?", *tenantId)
	}
	query.Model(&NeLog{}).Count(&total)

	err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&logs).Error
	if err != nil {
		logger.Errorf("FindByElementId error: %v", err)
		return nil, 0, err
	}

	return logs, total, nil
}

func (r *repository) FindAllByElementId(elementId int64) ([]NeLog, error) {
	var logs []NeLog
	err := r.db.Where("ne_id = ?", elementId).Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

func (r *repository) FindByFilter(tenantId *int, elementId *int64, deviceType *string, status *int, offset, limit int) ([]LogCollectionResultVo, int64, error) {
	var results []LogCollectionResultVo
	var total int64

	query := r.db.Table("ne_log").
		Select("ne_log.*, cpe_element.device_name, cpe_element.serial_number").
		Joins("LEFT JOIN cpe_element ON ne_log.ne_id = cpe_element.ne_neid").
		Where("cpe_element.deleted = ?", false)

	if tenantId != nil {
		query = query.Where("ne_log.tenant_id = ?", *tenantId)
	}
	if elementId != nil {
		query = query.Where("ne_log.ne_id = ?", *elementId)
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
