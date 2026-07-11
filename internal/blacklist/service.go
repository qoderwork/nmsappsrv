package blacklist

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Service contains blacklist business logic.
type Service struct {
	repo *Repository
}

// NewService creates a new blacklist service.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// AddDeviceToBlackList adds a device to the blacklist.
func (s *Service) AddDeviceToBlackList(req *AddDeviceToBlackListRequest, licenseId int, username string) (int, error) {
	// Duplicate check
	existing, _ := s.repo.FindBySNAndDeviceType(licenseId, req.SN, req.DeviceType)
	if existing != nil {
		return 0, fmt.Errorf("device already in blacklist")
	}

	entry := &ElementBlackList{
		SN:         req.SN,
		Username:   username,
		AddTime:    time.Now(),
		LicenseId:  licenseId,
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
		LicenseId:        licenseId,
	})

	// Set Redis key
	SetRedisBlackListKey(req.DeviceType, req.SN)

	return entry.Id, nil
}

// DeleteDeviceFromBlackList removes a single device from the blacklist.
func (s *Service) DeleteDeviceFromBlackList(id int, username string) error {
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
		LicenseId:        entry.LicenseId,
	})

	// Delete Redis key
	DeleteRedisBlackListKey(entry.DeviceType, entry.SN)

	return nil
}

// BatchDeleteDeviceFromBlackList removes multiple devices from the blacklist.
func (s *Service) BatchDeleteDeviceFromBlackList(ids []int, username string) error {
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
			LicenseId:        entry.LicenseId,
		})
		DeleteRedisBlackListKey(entry.DeviceType, entry.SN)
	}

	return nil
}

// ListBlackList returns paginated blacklist entries.
func (s *Service) ListBlackList(licenseId int, query ListBlackListQuery) ([]ListDeviceBlackListVO, int64, error) {
	return s.repo.List(licenseId, query)
}

// ListOperationLogs returns paginated operation logs.
func (s *Service) ListOperationLogs(licenseId int, query ListBlackListOperationLogQuery) ([]ListBlackListOperationLogVO, int64, error) {
	return s.repo.ListOperationLogs(licenseId, query)
}

// LoadAllToRedis loads all blacklist entries into Redis (call at startup).
func (s *Service) LoadAllToRedis() error {
	return s.repo.LoadAllToRedis()
}
