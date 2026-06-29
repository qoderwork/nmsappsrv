package corenet

import (
	"time"

	"gorm.io/gorm"
)

// Service contains the business logic for core network management.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ListCoreNetworks returns all core networks for the given tenancy.
func (s *Service) ListCoreNetworks(tenancyId int) ([]CoreNetwork, error) {
	return s.repo.FindCoreNetworks(tenancyId)
}

// GetCoreNetwork returns a single core network by ID.
func (s *Service) GetCoreNetwork(id int) (*CoreNetwork, error) {
	return s.repo.FindCoreNetworkByID(id)
}

// CreateCoreNetwork persists a new core network.
func (s *Service) CreateCoreNetwork(cn *CoreNetwork) error {
	return s.repo.CreateCoreNetwork(cn)
}

// UpdateCoreNetwork persists changes to an existing core network.
func (s *Service) UpdateCoreNetwork(cn *CoreNetwork) error {
	return s.repo.UpdateCoreNetwork(cn)
}

// DeleteCoreNetwork removes a core network by ID.
func (s *Service) DeleteCoreNetwork(id int) error {
	return s.repo.DeleteCoreNetwork(id)
}

// GetCoreNetworkData returns the data record for a core network.
func (s *Service) GetCoreNetworkData(coreNetworkId int) (*CoreNetworkData, error) {
	return s.repo.FindCoreNetworkData(coreNetworkId)
}

// SaveCoreNetworkData upserts a core network data record.
func (s *Service) SaveCoreNetworkData(data *CoreNetworkData) error {
	return s.repo.SaveCoreNetworkData(data)
}

// GetCoreNetworkKpis returns KPI records within the given time range.
func (s *Service) GetCoreNetworkKpis(coreNetworkId int, startTime, endTime string) ([]CoreNetworkKpi, error) {
	st, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		st, err = time.Parse("2006-01-02 15:04:05", startTime)
		if err != nil {
			return nil, err
		}
	}
	et, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		et, err = time.Parse("2006-01-02 15:04:05", endTime)
		if err != nil {
			return nil, err
		}
	}
	return s.repo.FindCoreNetworkKpis(coreNetworkId, st, et)
}

// GetStatisticData returns statistic data within the given time range.
func (s *Service) GetStatisticData(coreNetworkId int, startTime, endTime string) ([]CoreNetworkStatisticData, error) {
	st, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		st, err = time.Parse("2006-01-02 15:04:05", startTime)
		if err != nil {
			return nil, err
		}
	}
	et, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		et, err = time.Parse("2006-01-02 15:04:05", endTime)
		if err != nil {
			return nil, err
		}
	}
	return s.repo.FindCoreNetworkStatisticData(coreNetworkId, st, et)
}

// ListOperationLogs returns a paginated list of operation logs.
func (s *Service) ListOperationLogs(coreNetworkId int, page, pageSize int) ([]CoreNetworkOperationLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindOperationLogs(coreNetworkId, offset, pageSize)
}
