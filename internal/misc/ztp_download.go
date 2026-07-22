package misc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// ---------- downloadZTPTemplate ----------

// DownloadZTPTemplatePath returns the absolute path to the NR AOS xlsm
// template file (ZTP_Template.xlsm), mirroring Java's ClassPathResource
// lookup. The file is expected under BatchProcessDir/ztp-templates/, then
// ZtpDir. It must be provisioned from the Java project (nms-common/nms-serv/
// src/main/resources/) — a copy lives under data/ztp-templates/ for local dev.
func (s *service) DownloadZTPTemplatePath() (string, error) {
	fileName := "ZTP_Template.xlsm"
	dirs := []string{
		filepath.Join(s.cfg.FileServer.BatchProcessDir, "ztp-templates"),
		s.cfg.FileServer.ZtpDir,
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, fileName)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("template %s not found in BatchProcessDir/ztp-templates or ZtpDir; provision it from nms-common/nms-serv/src/main/resources/%s", fileName, fileName)
}

// ---------- downloadAOSFile ----------

// DownloadAOSFilePath resolves the on-disk path (and display filename) for the
// generated AOS file of a device, mirroring Java AOSManagementServiceImpl.
// downloadAOSFile which reads cpe_element.aos_file_name under AOSFilePath.
func (s *service) DownloadAOSFilePath(elementId int64) (string, string, error) {
	var record struct {
		AosFileName *string `gorm:"column:aos_file_name"`
	}
	if err := s.repo.DB().Table("cpe_element").Select("aos_file_name").
		Where("ne_neid = ? AND deleted = ?", elementId, false).
		First(&record).Error; err != nil {
		return "", "", fmt.Errorf("device %d not found", elementId)
	}
	if record.AosFileName == nil || *record.AosFileName == "" {
		return "", "", fmt.Errorf("device %d has no AOS file", elementId)
	}
	fileName := *record.AosFileName
	dir := s.cfg.FileServer.ZtpDir
	if dir == "" {
		dir = filepath.Join(s.cfg.FileServer.Root, "ztp")
	}
	filePath := filepath.Join(dir, fileName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("AOS file %s not found on disk", fileName)
	}
	return filePath, fileName, nil
}

// ---------- downloadHistoryZTPFile ----------

// DownloadHistoryFilePath resolves the on-disk path for a historical ZTP file
// by the ztp_file_send_log primary key, mirroring Java downloadHistoryZTPFile.
func (s *service) DownloadHistoryFilePath(logId int64) (string, string, error) {
	logEntry, err := s.repo.FindZTPFileSendLogByID(logId)
	if err != nil {
		return "", "", fmt.Errorf("history log %d not found", logId)
	}
	if logEntry.FileName == nil || *logEntry.FileName == "" {
		return "", "", fmt.Errorf("history log %d has no file_name", logId)
	}
	fileName := *logEntry.FileName
	dir := s.cfg.FileServer.ZtpDir
	if dir == "" {
		dir = filepath.Join(s.cfg.FileServer.Root, "ztp")
	}
	filePath := filepath.Join(dir, fileName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("history file %s not found on disk", fileName)
	}
	return filePath, fileName, nil
}

// ---------- getGenerateAOSFileTaskProgress ----------

const generateAOSTaskPrefix = "generate_aos_task_prefix_"
const aosTaskTTL = 1 * time.Hour

// GetAOSTaskProgress reads the AOS import progress from Redis, mirroring Java
// getGenerateAOSFileTaskProgress (Redis get, 1h TTL).
func (s *service) GetAOSTaskProgress(taskId string) (*AOSTaskProgressVO, error) {
	ctx := context.Background()
	key := generateAOSTaskPrefix + taskId
	val, err := redis.Get(ctx, key)
	if err != nil {
		// Key not found → not started / expired → return zero-state.
		return &AOSTaskProgressVO{Complete: false, CurrentProgress: 0, TotalProgress: 0}, nil
	}
	var vo AOSTaskProgressVO
	if err := json.Unmarshal([]byte(val), &vo); err != nil {
		return &AOSTaskProgressVO{Complete: false, CurrentProgress: 0, TotalProgress: 0}, nil
	}
	return &vo, nil
}

// flushAOSTaskProgress writes progress snapshot to Redis with 1h TTL.
func (s *service) flushAOSTaskProgress(taskId string, vo *AOSTaskProgressVO) {
	ctx := context.Background()
	b, _ := json.Marshal(vo)
	if err := redis.Set(ctx, generateAOSTaskPrefix+taskId, string(b), aosTaskTTL); err != nil {
		logger.Warnf("flush aos task progress %s: %v", taskId, err)
	}
}

// ---------- updateEnableGeofence ----------

// UpdateEnableGeofence toggles tbg.enable_geofence for a TBG record.
// Java's standalone endpoint is a stub; the real toggle lives in modifyTBG
// (Java writes tbg.enable_geofence). We implement the same semantic here.
func (s *service) UpdateEnableGeofence(tbgId int64, enableGeofence *int) error {
	var exists int64
	if err := s.repo.DB().Table("tbg").Where("id = ?", tbgId).Count(&exists).Error; err != nil || exists == 0 {
		return fmt.Errorf("TBG %d not found", tbgId)
	}
	upd := map[string]interface{}{}
	if enableGeofence != nil {
		upd["enable_geofence"] = *enableGeofence
	}
	if len(upd) == 0 {
		return nil
	}
	if err := s.repo.DB().Table("tbg").Where("id = ?", tbgId).Updates(upd).Error; err != nil {
		return fmt.Errorf("update TBG %d geofence: %w", tbgId, err)
	}
	return nil
}
