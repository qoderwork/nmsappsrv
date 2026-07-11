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

type Service struct {
	repo *Repository
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		repo: NewRepository(db),
	}
}

func (s *Service) AddLogCollectionTask(c *gin.Context, req *AddLogCollectionRequest) error {
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

func (s *Service) ListLogCollectionResults(c *gin.Context, req *ListLogCollectionResultRequest) ([]LogCollectionResultVo, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	offset := (req.Page - 1) * req.PageSize
	results, total, err := s.repo.FindByFilter(req.ElementId, req.DeviceType, req.Status, offset, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

func (s *Service) DeleteAllLogFile(req *DeleteAllLogFileRequest) error {
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

func (s *Service) DeleteLogFile(req *DeleteLogFileRequest) error {
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

func (s *Service) GetLogFile(logId int64) (string, error) {
	log, err := s.repo.FindByID(logId)
	if err != nil {
		return "", err
	}

	if log.FilePath == nil {
		return "", apperror.ErrNotFound.WithMessage("log file path is empty")
	}

	return *log.FilePath, nil
}

func (s *Service) ListLogFiles(req *ListLogFileRequest) ([]LogFileVo, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	offset := (req.Page - 1) * req.PageSize
	logs, total, err := s.repo.FindByElementId(req.ElementId, offset, req.PageSize)
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

func (s *Service) EnablePeriodicUpload(c *gin.Context, req *EnablePeriodicUploadRequest) error {
	// Get device info to determine TR-069 path
	var device struct {
		NeNeid   int64   `gorm:"column:ne_neid"`
		RootNode *string `gorm:"column:root_node"`
	}
	err := s.repo.db.Table("cpe_element").
		Where("ne_neid = ? AND deleted = ?", req.ElementId, false).
		First(&device).Error
	if err != nil {
		return apperror.ErrDeviceNotFound
	}

	// Determine TR-069 path based on rootNode
	paramPath := "Device.LogPeriodicUpload"
	if device.RootNode != nil && *device.RootNode == "InternetGatewayDevice" {
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

func (s *Service) DisablePeriodicUpload(c *gin.Context, req *DisablePeriodicUploadRequest) error {
	// Get device info to determine TR-069 path
	var device struct {
		NeNeid   int64   `gorm:"column:ne_neid"`
		RootNode *string `gorm:"column:root_node"`
	}
	err := s.repo.db.Table("cpe_element").
		Where("ne_neid = ? AND deleted = ?", req.ElementId, false).
		First(&device).Error
	if err != nil {
		return apperror.ErrDeviceNotFound
	}

	// Determine TR-069 path based on rootNode
	paramPath := "Device.LogPeriodicUpload"
	if device.RootNode != nil && *device.RootNode == "InternetGatewayDevice" {
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
