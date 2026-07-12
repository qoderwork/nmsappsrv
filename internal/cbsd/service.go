package cbsd

import (
	"gorm.io/gorm"
)

// Service defines the business-logic contract for CBSD management.
type Service interface {
	ListCbsdInfos(licenseId int, page, pageSize int) ([]CbsdInfo, int64, error)
	GetCbsdInfo(sn string, licenseId int) (*CbsdInfo, error)
	RegisterCbsd(info *CbsdInfo) error
	UpdateCbsdInfo(info *CbsdInfo) error
	DeregisterCbsd(id string) error
	ListCbrsLogs(cbsdId string, logType string, page, pageSize int) ([]CbrsLog, int64, error)
	CreateCertFileSendTask(t *CBSDCertFileSendTask) error
	ListCertFileSendTasks(tenancyId int, page, pageSize int) ([]CBSDCertFileSendTask, int64, error)
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

// ListCbsdInfos returns a paginated list of CBSD info records.
func (s *service) ListCbsdInfos(licenseId int, page, pageSize int) ([]CbsdInfo, int64, error) {
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
func (s *service) GetCbsdInfo(sn string, licenseId int) (*CbsdInfo, error) {
	return s.repo.FindCbsdInfoBySN(sn, licenseId)
}

// RegisterCbsd persists a new CBSD registration.
func (s *service) RegisterCbsd(info *CbsdInfo) error {
	return s.repo.Create(info)
}

// UpdateCbsdInfo persists changes to an existing CBSD info.
func (s *service) UpdateCbsdInfo(info *CbsdInfo) error {
	return s.repo.Save(info)
}

// DeregisterCbsd removes a CBSD info by ID.
func (s *service) DeregisterCbsd(id string) error {
	return s.repo.DeleteByID(id)
}

// ListCbrsLogs returns a paginated list of CBRS logs.
func (s *service) ListCbrsLogs(cbsdId string, logType string, page, pageSize int) ([]CbrsLog, int64, error) {
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
func (s *service) CreateCertFileSendTask(t *CBSDCertFileSendTask) error {
	return s.repo.CreateCertFileSendTask(t)
}

// ListCertFileSendTasks returns a paginated list of cert file send tasks.
func (s *service) ListCertFileSendTasks(tenancyId int, page, pageSize int) ([]CBSDCertFileSendTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindCertFileSendTasks(tenancyId, offset, pageSize)
}
