package corenet

import (
	"time"

	"gorm.io/gorm"
)

// Service defines the business-logic contract for core network management.
type Service interface {
	ListCoreNetworks(tenancyId int) ([]CoreNetwork, error)
	GetCoreNetwork(id int) (*CoreNetwork, error)
	CreateCoreNetwork(cn *CoreNetwork) error
	UpdateCoreNetwork(cn *CoreNetwork) error
	DeleteCoreNetwork(id int) error
	GetCoreNetworkData(coreNetworkId int) (*CoreNetworkData, error)
	SaveCoreNetworkData(data *CoreNetworkData) error
	GetCoreNetworkKpis(coreNetworkId int, startTime, endTime string) ([]CoreNetworkKpi, error)
	GetStatisticData(coreNetworkId int, startTime, endTime string) ([]CoreNetworkStatisticData, error)
	ListOperationLogs(coreNetworkId int, page, pageSize int) ([]CoreNetworkOperationLog, int64, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// ListCoreNetworks returns all core networks for the given tenancy.
func (s *service) ListCoreNetworks(tenancyId int) ([]CoreNetwork, error) {
	return s.repo.FindCoreNetworks(tenancyId)
}

// GetCoreNetwork returns a single core network by ID.
func (s *service) GetCoreNetwork(id int) (*CoreNetwork, error) {
	return s.repo.FindByID(id)
}

// CreateCoreNetwork persists a new core network.
func (s *service) CreateCoreNetwork(cn *CoreNetwork) error {
	return s.repo.Create(cn)
}

// UpdateCoreNetwork persists changes to an existing core network.
func (s *service) UpdateCoreNetwork(cn *CoreNetwork) error {
	return s.repo.Save(cn)
}

// DeleteCoreNetwork removes a core network by ID.
func (s *service) DeleteCoreNetwork(id int) error {
	return s.repo.DeleteByID(id)
}

// GetCoreNetworkData returns the data record for a core network.
func (s *service) GetCoreNetworkData(coreNetworkId int) (*CoreNetworkData, error) {
	return s.repo.FindCoreNetworkData(coreNetworkId)
}

// SaveCoreNetworkData upserts a core network data record.
func (s *service) SaveCoreNetworkData(data *CoreNetworkData) error {
	return s.repo.SaveCoreNetworkData(data)
}

// GetCoreNetworkKpis returns KPI records within the given time range.
func (s *service) GetCoreNetworkKpis(coreNetworkId int, startTime, endTime string) ([]CoreNetworkKpi, error) {
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
func (s *service) GetStatisticData(coreNetworkId int, startTime, endTime string) ([]CoreNetworkStatisticData, error) {
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
func (s *service) ListOperationLogs(coreNetworkId int, page, pageSize int) ([]CoreNetworkOperationLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindOperationLogs(coreNetworkId, offset, pageSize)
}
