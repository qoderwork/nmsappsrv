package scheduledtask

import (
	"os"
	"path/filepath"
	"time"

	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// ExpiredBackupCleanupTask 清理过期NMS备份文件
// 镜像 Java ExpiredBackupCleanupTask
// 每小时执行，扫描备份目录下超过保留天数的 .zip 文件并删除，
// 同时清理 nms_backup_and_revert_log 表中超过保留天数的记录
type ExpiredBackupCleanupTask struct {
	db            *gorm.DB
	backupDir     string
	retentionDays int
}

// NewExpiredBackupCleanupTask 创建 ExpiredBackupCleanupTask 实例
func NewExpiredBackupCleanupTask(db *gorm.DB, backupDir string, retentionDays int) *ExpiredBackupCleanupTask {
	if retentionDays <= 0 {
		retentionDays = 7
	}
	return &ExpiredBackupCleanupTask{
		db:            db,
		backupDir:     backupDir,
		retentionDays: retentionDays,
	}
}

// Cleanup 执行过期备份清理
// 1. 扫描 backupDir 下超过 retentionDays 的 .zip 文件并删除
// 2. 清理 nms_backup_and_revert_log 表中 create_time < now - retentionDays 的记录
func (t *ExpiredBackupCleanupTask) Cleanup() {
	cutoff := time.Now().AddDate(0, 0, -t.retentionDays)

	// 1. 扫描并删除过期的 .zip 文件
	t.cleanupExpiredFiles(cutoff)

	// 2. 清理 nms_backup_and_revert_log 表中过期记录
	t.cleanupExpiredLogs(cutoff)
}

// cleanupExpiredFiles 扫描备份目录下超过 cutoff 时间的 .zip 文件并删除
func (t *ExpiredBackupCleanupTask) cleanupExpiredFiles(cutoff time.Time) {
	// 检查备份目录是否存在
	info, err := os.Stat(t.backupDir)
	if err != nil || !info.IsDir() {
		logger.Warnf("ExpiredBackupCleanupTask: backup dir %s does not exist or is not a directory", t.backupDir)
		return
	}

	// 遍历 .zip 文件
	pattern := filepath.Join(t.backupDir, "*.zip")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		logger.Errorf("ExpiredBackupCleanupTask: failed to glob %s: %v", pattern, err)
		return
	}

	deletedCount := 0
	for _, f := range matches {
		fileInfo, err := os.Stat(f)
		if err != nil {
			logger.Warnf("ExpiredBackupCleanupTask: failed to stat %s: %v", f, err)
			continue
		}

		if fileInfo.ModTime().Before(cutoff) {
			if err := os.Remove(f); err != nil {
				logger.Errorf("ExpiredBackupCleanupTask: failed to delete %s: %v", f, err)
				continue
			}
			deletedCount++
			logger.Infof("ExpiredBackupCleanupTask: deleted expired backup file %s (modTime=%s)", f, fileInfo.ModTime().Format("2006-01-02 15:04:05"))
		}
	}

	if deletedCount > 0 {
		logger.Infof("ExpiredBackupCleanupTask: deleted %d expired backup files", deletedCount)
	}
}

// cleanupExpiredLogs 清理 nms_backup_and_revert_log 表中 create_time 超过 cutoff 的记录
func (t *ExpiredBackupCleanupTask) cleanupExpiredLogs(cutoff time.Time) {
	result := t.db.Table("nms_backup_and_revert_log").
		Where("create_time < ?", cutoff).
		Delete(nil)
	if result.Error != nil {
		logger.Errorf("ExpiredBackupCleanupTask: failed to delete expired backup logs: %v", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		logger.Infof("ExpiredBackupCleanupTask: deleted %d expired backup log records", result.RowsAffected)
	}
}
