package corenet

import (
	"time"

	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository handles database operations for core network entities.
// It embeds BaseRepository[CoreNetwork, int] for standard CRUD on CoreNetwork,
// and retains module-specific methods for custom queries and other entity types.
type Repository struct {
	*baserepo.BaseRepository[CoreNetwork, int]
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		BaseRepository: baserepo.New[CoreNetwork, int](db, "id"),
		db:             db,
	}
}

// FindCoreNetworks returns all core networks for the given tenancy.
func (r *Repository) FindCoreNetworks(tenancyId int) ([]CoreNetwork, error) {
	var networks []CoreNetwork
	if err := r.db.Where("tenancy_id = ? AND (deleted = ? OR deleted IS NULL)", tenancyId, false).Find(&networks).Error; err != nil {
		logger.Errorf("FindCoreNetworks error: %v", err)
		return nil, err
	}
	return networks, nil
}

// FindCoreNetworkData returns the data record for the given core network.
func (r *Repository) FindCoreNetworkData(coreNetworkId int) (*CoreNetworkData, error) {
	var data CoreNetworkData
	if err := r.db.Where("core_network_id = ?", coreNetworkId).First(&data).Error; err != nil {
		return nil, err
	}
	return &data, nil
}

// SaveCoreNetworkData upserts a core network data record by core_network_id.
func (r *Repository) SaveCoreNetworkData(data *CoreNetworkData) error {
	var existing CoreNetworkData
	err := r.db.Where("core_network_id = ?", data.CoreNetworkId).First(&existing).Error
	if err == nil {
		data.Id = existing.Id
		return r.db.Save(data).Error
	}
	return r.db.Create(data).Error
}

// FindCoreNetworkKpis returns KPI records within the given time range.
func (r *Repository) FindCoreNetworkKpis(coreNetworkId int, startTime, endTime time.Time) ([]CoreNetworkKpi, error) {
	var kpis []CoreNetworkKpi
	if err := r.db.Where("rm_uid = ? AND start_time >= ? AND start_time <= ?", coreNetworkId, startTime, endTime).Find(&kpis).Error; err != nil {
		logger.Errorf("FindCoreNetworkKpis error: %v", err)
		return nil, err
	}
	return kpis, nil
}

// FindCoreNetworkStatisticData returns statistic data within the given time range.
func (r *Repository) FindCoreNetworkStatisticData(coreNetworkId int, startTime, endTime time.Time) ([]CoreNetworkStatisticData, error) {
	var stats []CoreNetworkStatisticData
	if err := r.db.Where("core_network_id = ? AND statistic_time >= ? AND statistic_time <= ?", coreNetworkId, startTime, endTime).Find(&stats).Error; err != nil {
		logger.Errorf("FindCoreNetworkStatisticData error: %v", err)
		return nil, err
	}
	return stats, nil
}

// CreateOperationLog inserts a new operation log record.
func (r *Repository) CreateOperationLog(log *CoreNetworkOperationLog) error {
	return r.db.Create(log).Error
}

// FindOperationLogs returns a paginated list of operation logs for a core network.
func (r *Repository) FindOperationLogs(coreNetworkId int, offset, limit int) ([]CoreNetworkOperationLog, int64, error) {
	var logs []CoreNetworkOperationLog
	var total int64

	query := r.db.Model(&CoreNetworkOperationLog{}).Where("core_network_id = ?", coreNetworkId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindOperationLogs count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		logger.Errorf("FindOperationLogs query error: %v", err)
		return nil, 0, err
	}
	return logs, total, nil
}
