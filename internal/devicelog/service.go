package devicelog

import (
	"context"
	"encoding/json"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Service defines the business-logic contract for device log management.
type Service interface {
	AddLogCollectionTask(c *gin.Context, req *AddLogCollectionRequest) error
	ListLogCollectionResults(c *gin.Context, req *ListLogCollectionResultRequest) ([]LogCollectionResultVo, int64, error)
	DeleteAllLogFile(req *DeleteAllLogFileRequest) error
	DeleteLogFile(req *DeleteLogFileRequest) error
	GetLogFile(logId int64) (string, error)
	ListLogFiles(c *gin.Context, req *ListLogFileRequest) ([]LogFileVo, int64, error)
	EnablePeriodicUpload(c *gin.Context, req *EnablePeriodicUploadRequest) error
	DisablePeriodicUpload(c *gin.Context, req *DisablePeriodicUploadRequest) error
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{
		repo: NewRepository(db),
	}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) AddLogCollectionTask(c *gin.Context, req *AddLogCollectionRequest) error {
	licenseId := middleware.GetLicenseId(c)
	now := time.Now()
	ctx := context.Background()

	for _, elementId := range req.ElementIds {
		// Create NeLog record with status=1 (pending)
		status := 1
		log := NeLog{
			ElementId:      &elementId,
			Status:         &status,
			CollectionTime: &now,
			LicenseId:      &licenseId,
		}

		if err := s.repo.Create(&log); err != nil {
			logger.Errorf("Create NeLog error: %v", err)
			continue
		}

		// Dispatch TR-069 log collection command
		cmd := map[string]interface{}{
			"eventType": "collectLog",
			"elementId": elementId,
			"logType":   req.LogType,
			"logId":     log.Id,
		}
		cmdBytes, _ := json.Marshal(cmd)

		if err := redis.LPush(ctx, "operation_queue", string(cmdBytes)); err != nil {
			logger.Errorf("Push operation_queue error: %v", err)
		}
	}

	return nil
}

func (s *service) ListLogCollectionResults(c *gin.Context, req *ListLogCollectionResultRequest) ([]LogCollectionResultVo, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	// Tenant isolation: every caller is scoped to its own license; filter the
	// joined ne_log rows by license_id so one tenant cannot read another's logs.
	licenseId := middleware.GetLicenseId(c)
	offset := (req.Page - 1) * req.PageSize
	results, total, err := s.repo.FindByFilter(&licenseId, req.ElementId, req.DeviceType, req.Status, offset, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

func (s *service) DeleteAllLogFile(req *DeleteAllLogFileRequest) error {
	// Fetch all log records first to get file paths for physical file deletion
	logs, err := s.repo.FindAllByElementId(req.ElementId)
	if err != nil {
		logger.Errorf("FindAllByElementId error: %v", err)
		return err
	}

	// Delete physical log files from disk
	for _, log := range logs {
		if log.FilePath != nil && *log.FilePath != "" {
			if err := os.Remove(*log.FilePath); err != nil {
				logger.Warnf("Failed to delete log file %s: %v", *log.FilePath, err)
			}
		}
	}

	// Delete all ne_log records for this elementId
	if err := s.repo.DeleteByElementId(req.ElementId); err != nil {
		logger.Errorf("DeleteByElementId error: %v", err)
		return err
	}

	return nil
}

func (s *service) DeleteLogFile(req *DeleteLogFileRequest) error {
	// Get log record first
	log, err := s.repo.FindByID(req.LogId)
	if err != nil {
		return err
	}

	// Delete record
	if err := s.repo.DeleteByID(req.LogId); err != nil {
		logger.Errorf("Delete NeLog error: %v", err)
		return err
	}

	// Delete actual file from disk if log.FilePath is set
	if log.FilePath != nil && *log.FilePath != "" {
		if err := os.Remove(*log.FilePath); err != nil {
			logger.Warnf("Failed to delete log file %s: %v", *log.FilePath, err)
		}
	}

	return nil
}

func (s *service) GetLogFile(logId int64) (string, error) {
	log, err := s.repo.FindByID(logId)
	if err != nil {
		return "", err
	}

	if log.FilePath == nil {
		return "", apperror.ErrNotFound.WithMessage("log file path is empty")
	}

	return *log.FilePath, nil
}

func (s *service) ListLogFiles(c *gin.Context, req *ListLogFileRequest) ([]LogFileVo, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	// Tenant isolation: scope the element's log files to the caller's license so
	// a user cannot read another tenant's files by passing a foreign elementId.
	licenseId := middleware.GetLicenseId(c)
	offset := (req.Page - 1) * req.PageSize
	logs, total, err := s.repo.FindByElementId(req.ElementId, &licenseId, offset, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]LogFileVo, 0, len(logs))
	for _, log := range logs {
		vo := LogFileVo{
			Id:       log.Id,
			FileName: *log.FileName,
			FileSize: *log.FileSize,
		}
		if log.CollectionTime != nil {
			vo.CollectionTime = log.CollectionTime.Format("2006-01-02 15:04:05")
		}
		result = append(result, vo)
	}

	return result, total, nil
}

func (s *service) EnablePeriodicUpload(c *gin.Context, req *EnablePeriodicUploadRequest) error {
	// Get device info to determine TR-069 path
	rootNode, err := s.repo.FindDeviceRootNode(req.ElementId)
	if err != nil {
		return apperror.ErrDeviceNotFound
	}

	// Determine TR-069 path based on rootNode
	paramPath := "Device.LogPeriodicUpload"
	if rootNode != nil && *rootNode == "InternetGatewayDevice" {
		paramPath = "InternetGatewayDevice.LogPeriodicUpload"
	}

	// Dispatch TR-069 SetParameterValues command
	ctx := context.Background()
	cmd := map[string]interface{}{
		"eventType": "setParameterValues",
		"elementId": req.ElementId,
		"parameters": []map[string]interface{}{
			{
				"name":  paramPath,
				"value": req.Interval,
			},
		},
	}
	cmdBytes, _ := json.Marshal(cmd)

	if err := redis.LPush(ctx, "operation_queue", string(cmdBytes)); err != nil {
		logger.Errorf("Push operation_queue error: %v", err)
		return err
	}

	return nil
}

func (s *service) DisablePeriodicUpload(c *gin.Context, req *DisablePeriodicUploadRequest) error {
	// Get device info to determine TR-069 path
	rootNode, err := s.repo.FindDeviceRootNode(req.ElementId)
	if err != nil {
		return apperror.ErrDeviceNotFound
	}

	// Determine TR-069 path based on rootNode
	paramPath := "Device.LogPeriodicUpload"
	if rootNode != nil && *rootNode == "InternetGatewayDevice" {
		paramPath = "InternetGatewayDevice.LogPeriodicUpload"
	}

	// Dispatch TR-069 SetParameterValues command to disable (set to 0)
	ctx := context.Background()
	cmd := map[string]interface{}{
		"eventType": "setParameterValues",
		"elementId": req.ElementId,
		"parameters": []map[string]interface{}{
			{
				"name":  paramPath,
				"value": 0,
			},
		},
	}
	cmdBytes, _ := json.Marshal(cmd)

	if err := redis.LPush(ctx, "operation_queue", string(cmdBytes)); err != nil {
		logger.Errorf("Push operation_queue error: %v", err)
		return err
	}

	return nil
}
