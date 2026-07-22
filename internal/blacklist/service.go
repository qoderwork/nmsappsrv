package blacklist

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Service defines the business-logic contract for blacklist management.
type Service interface {
	AddDeviceToBlackList(req *AddDeviceToBlackListRequest, tenantId int, username string) (int, error)
	DeleteDeviceFromBlackList(id int, username string) error
	BatchDeleteDeviceFromBlackList(ids []int, username string) error
	ListBlackList(tenantId int, query ListBlackListQuery) ([]ListDeviceBlackListVO, int64, error)
	ListOperationLogs(tenantId int, query ListBlackListOperationLogQuery) ([]ListBlackListOperationLogVO, int64, error)
	LoadAllToRedis() error
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

// AddDeviceToBlackList adds a device to the blacklist.
func (s *service) AddDeviceToBlackList(req *AddDeviceToBlackListRequest, tenantId int, username string) (int, error) {
	// Duplicate check
	existing, _ := s.repo.FindBySNAndDeviceType(tenantId, req.SN, req.DeviceType)
	if existing != nil {
		return 0, fmt.Errorf("device already in blacklist")
	}

	entry := &ElementBlackList{
		SN:         req.SN,
		Username:   username,
		AddTime:    time.Now(),
		TenantId:  tenantId,
		DeviceType: req.DeviceType,
		Reason:     req.Reason,
	}
	if err := s.repo.Create(entry); err != nil {
		return 0, err
	}

	// Insert operation log
	s.repo.InsertOperationLog(&BlackListOperationLog{
		DeviceSN:         req.SN,
		DeviceType:       req.DeviceType,
		OperationType:    "ADD",
		OperatorUsername: username,
		OperationTime:    time.Now(),
		OperationReason:  req.Reason,
		TenantId:        tenantId,
	})

	// Set Redis key
	SetRedisBlackListKey(req.DeviceType, req.SN)

	return entry.Id, nil
}

// DeleteDeviceFromBlackList removes a single device from the blacklist.
func (s *service) DeleteDeviceFromBlackList(id int, username string) error {
	entry, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("blacklist entry not found")
	}

	if err := s.repo.DeleteByID(id); err != nil {
		return err
	}

	// Insert operation log
	s.repo.InsertOperationLog(&BlackListOperationLog{
		DeviceSN:         entry.SN,
		DeviceType:       entry.DeviceType,
		OperationType:    "REMOVE",
		OperatorUsername: username,
		OperationTime:    time.Now(),
		OperationReason:  entry.Reason,
		TenantId:        entry.TenantId,
	})

	// Delete Redis key
	DeleteRedisBlackListKey(entry.DeviceType, entry.SN)

	return nil
}

// BatchDeleteDeviceFromBlackList removes multiple devices from the blacklist.
func (s *service) BatchDeleteDeviceFromBlackList(ids []int, username string) error {
	// Load entries first for logging and Redis cleanup
	entries := make([]*ElementBlackList, 0, len(ids))
	for _, id := range ids {
		entry, err := s.repo.FindByID(id)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	if err := s.repo.DeleteByIDs(ids); err != nil {
		return err
	}

	// Insert operation logs and clean Redis keys
	for _, entry := range entries {
		s.repo.InsertOperationLog(&BlackListOperationLog{
			DeviceSN:         entry.SN,
			DeviceType:       entry.DeviceType,
			OperationType:    "REMOVE",
			OperatorUsername: username,
			OperationTime:    time.Now(),
			OperationReason:  entry.Reason,
			TenantId:        entry.TenantId,
		})
		DeleteRedisBlackListKey(entry.DeviceType, entry.SN)
	}

	return nil
}

// ListBlackList returns paginated blacklist entries.
func (s *service) ListBlackList(tenantId int, query ListBlackListQuery) ([]ListDeviceBlackListVO, int64, error) {
	return s.repo.List(tenantId, query)
}

// ListOperationLogs returns paginated operation logs.
func (s *service) ListOperationLogs(tenantId int, query ListBlackListOperationLogQuery) ([]ListBlackListOperationLogVO, int64, error) {
	return s.repo.ListOperationLogs(tenantId, query)
}

// LoadAllToRedis loads all blacklist entries into Redis (call at startup).
func (s *service) LoadAllToRedis() error {
	return s.repo.LoadAllToRedis()
}
