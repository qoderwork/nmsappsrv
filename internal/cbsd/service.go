package cbsd

import (
	"gorm.io/gorm"
)

// Service contains the business logic for CBSD management.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ListCbsdInfos returns a paginated list of CBSD info records.
func (s *Service) ListCbsdInfos(licenseId int, page, pageSize int) ([]CbsdInfo, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindCbsdInfos(licenseId, offset, pageSize)
}

// GetCbsdInfo returns a single CBSD info by serial number.
func (s *Service) GetCbsdInfo(sn string, licenseId int) (*CbsdInfo, error) {
	return s.repo.FindCbsdInfoBySN(sn, licenseId)
}

// RegisterCbsd persists a new CBSD registration.
func (s *Service) RegisterCbsd(info *CbsdInfo) error {
	return s.repo.CreateCbsdInfo(info)
}

// UpdateCbsdInfo persists changes to an existing CBSD info.
func (s *Service) UpdateCbsdInfo(info *CbsdInfo) error {
	return s.repo.UpdateCbsdInfo(info)
}

// DeregisterCbsd removes a CBSD info by ID.
func (s *Service) DeregisterCbsd(id string) error {
	return s.repo.DeleteCbsdInfo(id)
}

// ListCbrsLogs returns a paginated list of CBRS logs.
func (s *Service) ListCbrsLogs(cbsdId string, logType string, page, pageSize int) ([]CbrsLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindCbrsLogs(cbsdId, logType, offset, pageSize)
}

// CreateCertFileSendTask persists a new cert file send task.
func (s *Service) CreateCertFileSendTask(t *CBSDCertFileSendTask) error {
	return s.repo.CreateCertFileSendTask(t)
}

// ListCertFileSendTasks returns a paginated list of cert file send tasks.
func (s *Service) ListCertFileSendTasks(tenancyId int, page, pageSize int) ([]CBSDCertFileSendTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindCertFileSendTasks(tenancyId, offset, pageSize)
}
