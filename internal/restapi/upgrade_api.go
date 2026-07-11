package restapi

import (
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"

	"github.com/gin-gonic/gin"
)

// ============================
// Upgrade file operations
// ============================

func (s *Service) UploadUpgradeFile(c *gin.Context, fileName string, filePath string, fileSize int64) (*RestUpgradeFileVo, error) {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)
	now := time.Now()

	record := map[string]interface{}{
		"file_name":   fileName,
		"file_path":   filePath,
		"file_size":   fileSize,
		"license_id":  licenseId,
		"user":        username,
		"upload_time": now,
		"create_time": now,
	}

	id, err := s.repo.CreateUpgradeFile(record)
	if err != nil {
		logger.Errorf("Failed to create upgrade file record: %v", err)
		return nil, apperror.Wrap(err, "UPLOAD_UPGRADE_FILE_FAILED", 500, "failed to upload upgrade file")
	}

	logger.Infof("Upgrade file uploaded: %s by user %s", fileName, username)

	return &RestUpgradeFileVo{
		Id:         int(id),
		FileName:   fileName,
		FileSize:   fileSize,
		UploadTime: now.Format("2006-01-02 15:04:05"),
	}, nil
}

func (s *Service) ListUpgradeFiles(c *gin.Context, offset, limit int) ([]RestUpgradeFileVo, int64, error) {
	licenseId := middleware.GetLicenseId(c)

	files, total, err := s.repo.ListUpgradeFiles(licenseId, offset, limit)
	if err != nil {
		return nil, 0, apperror.Wrap(err, "LIST_UPGRADE_FILES_FAILED", 500, "failed to list upgrade files")
	}

	var result []RestUpgradeFileVo
	for _, f := range files {
		vo := RestUpgradeFileVo{}
		if v, ok := f["id"].(int64); ok {
			vo.Id = int(v)
		}
		if v, ok := f["file_name"].(string); ok {
			vo.FileName = v
		}
		if v, ok := f["version"].(string); ok {
			vo.Version = v
		}
		if v, ok := f["device_type"].(string); ok {
			vo.DeviceType = v
		}
		if v, ok := f["file_size"].(int64); ok {
			vo.FileSize = v
		}
		if v, ok := f["upload_time"].(time.Time); ok {
			vo.UploadTime = v.Format("2006-01-02 15:04:05")
		}
		result = append(result, vo)
	}

	return result, total, nil
}

func (s *Service) DeleteUpgradeFile(c *gin.Context, id int) error {
	licenseId := middleware.GetLicenseId(c)

	if err := s.repo.DeleteUpgradeFile(id, licenseId); err != nil {
		logger.Errorf("Failed to delete upgrade file %d: %v", id, err)
		return apperror.Wrap(err, "DELETE_UPGRADE_FILE_FAILED", 500, "failed to delete upgrade file")
	}

	return nil
}

// ============================
// Upgrade task operations
// ============================

func (s *Service) CreateUpgradeTask(c *gin.Context, req *RestUpgradeTaskRequest) (*RestUpgradeTaskVo, error) {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)
	now := time.Now()

	elementIdJSON, _ := json.Marshal(req.ElementIds)

	record := map[string]interface{}{
		"name":            req.Name,
		"upgrade_file_id": req.UpgradeFileId,
		"element_ids":     string(elementIdJSON),
		"status":          1, // pending
		"license_id":      licenseId,
		"user":            username,
		"create_time":     now,
		"update_time":     now,
	}

	id, err := s.repo.CreateUpgradeTask(record)
	if err != nil {
		logger.Errorf("Failed to create upgrade task: %v", err)
		return nil, apperror.Wrap(err, "CREATE_UPGRADE_TASK_FAILED", 500, "failed to create upgrade task")
	}

	logger.Infof("Upgrade task %d created by user %s for %d devices", id, username, len(req.ElementIds))

	return &RestUpgradeTaskVo{
		Id:       int(id),
		Name:     req.Name,
		Status:   1,
		Progress: "0/" + fmt.Sprintf("%d", len(req.ElementIds)),
	}, nil
}

func (s *Service) GetUpgradeTask(c *gin.Context, id int) (*RestUpgradeTaskVo, error) {
	task, err := s.repo.GetUpgradeTask(id)
	if err != nil {
		return nil, apperror.ErrNotFound.WithMessage("upgrade task not found")
	}

	vo := &RestUpgradeTaskVo{}
	if v, ok := task["id"].(int64); ok {
		vo.Id = int(v)
	}
	if v, ok := task["name"].(string); ok {
		vo.Name = v
	}
	if v, ok := task["status"].(int64); ok {
		vo.Status = int(v)
	}

	return vo, nil
}

func (s *Service) ListUpgradeTasks(c *gin.Context, offset, limit int) ([]RestUpgradeTaskVo, int64, error) {
	licenseId := middleware.GetLicenseId(c)

	tasks, total, err := s.repo.ListUpgradeTasks(licenseId, offset, limit)
	if err != nil {
		return nil, 0, apperror.Wrap(err, "LIST_UPGRADE_TASKS_FAILED", 500, "failed to list upgrade tasks")
	}

	var result []RestUpgradeTaskVo
	for _, t := range tasks {
		vo := RestUpgradeTaskVo{}
		if v, ok := t["id"].(int64); ok {
			vo.Id = int(v)
		}
		if v, ok := t["name"].(string); ok {
			vo.Name = v
		}
		if v, ok := t["status"].(int64); ok {
			vo.Status = int(v)
		}
		result = append(result, vo)
	}

	return result, total, nil
}
