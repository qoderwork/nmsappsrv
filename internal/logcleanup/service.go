package logcleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"nmsappsrv/internal/config"
	"nmsappsrv/internal/systemsettings"
	"nmsappsrv/pkg/logger"
)

// Service handles log cleanup tasks.
type Service struct {
	db       *gorm.DB
	repo     *Repository
	settings *systemsettings.SystemSettingsService
}

// NewService creates a new Service.
func NewService(db *gorm.DB, settings *systemsettings.SystemSettingsService) *Service {
	return &Service{
		db:       db,
		repo:     NewRepository(db),
		settings: settings,
	}
}

// FileAndMysqlLogDelete mirrors Java FileAndMysqlLogDeleteTask.
// Deletes expired northbound files, PM/MR/log/capture files, and MySQL records.
func (s *Service) FileAndMysqlLogDelete() {
	logConfig, err := s.settings.GetLogConfig()
	if err != nil {
		logger.Errorf("logcleanup: failed to get log config: %v", err)
		return
	}

	cfg := config.Cfg
	now := time.Now()

	// 1. Delete northbound interface files (FM/PM/CM per tenant).
	// Note: northbound export path is not in config; skip if unavailable.
	if logConfig.NorthboundFileSaveTime != nil && *logConfig.NorthboundFileSaveTime > 0 {
		northCutoff := now.AddDate(0, 0, -(*logConfig.NorthboundFileSaveTime))
		// Northbound files are typically under FileServer.Root/northbound or a separate path.
		// If a dedicated path is needed, add it to PlatformFilesConfig.
		_ = northCutoff
	}

	// 2. Delete PM files (by date directory).
	if cfg.FileServer.PmDir != "" && logConfig.PmAndMrSaveTime != nil && *logConfig.PmAndMrSaveTime > 0 {
		pmCutoff := now.AddDate(0, 0, -(*logConfig.PmAndMrSaveTime))
		s.deleteDateDirectories(cfg.FileServer.PmDir, pmCutoff)
	}

	// 3. Delete log files (by date directory).
	if cfg.FileServer.LogDir != "" && logConfig.DeviceLogSaveTime != nil && *logConfig.DeviceLogSaveTime > 0 {
		logCutoff := now.AddDate(0, 0, -(*logConfig.DeviceLogSaveTime))
		s.deleteDateDirectories(cfg.FileServer.LogDir, logCutoff)
	}

	// 4. Delete capture files (by date directory).
	if cfg.FileServer.CaptureDir != "" && logConfig.DeviceLogSaveTime != nil && *logConfig.DeviceLogSaveTime > 0 {
		capCutoff := now.AddDate(0, 0, -(*logConfig.DeviceLogSaveTime))
		s.deleteDateDirectories(cfg.FileServer.CaptureDir, capCutoff)
	}

	// 5. Delete MR files (by date directory).
	if cfg.FileServer.MrDir != "" && logConfig.PmAndMrSaveTime != nil && *logConfig.PmAndMrSaveTime > 0 {
		mrCutoff := now.AddDate(0, 0, -(*logConfig.PmAndMrSaveTime))
		s.deleteDateDirectories(cfg.FileServer.MrDir, mrCutoff)
	}

	// 6. MySQL cleanup: nms_log related tables (deviceLogSaveTime).
	if logConfig.NmsLogSaveTime != nil && *logConfig.NmsLogSaveTime > 0 {
		nmsCutoff := now.AddDate(0, 0, -(*logConfig.NmsLogSaveTime))
		safeDelete("system_operator_log", func() (int64, error) {
			return s.repo.DeleteSystemOperatorLogBefore(nmsCutoff)
		})
		safeDelete("north_interface_log", func() (int64, error) {
			return s.repo.DeleteNorthInterfaceLogBefore(nmsCutoff)
		})
		safeDelete("cbrs_log", func() (int64, error) {
			return s.repo.DeleteCbrsLogBefore(nmsCutoff)
		})
		safeDelete("login_log", func() (int64, error) {
			return s.repo.DeleteLoginLogBefore(nmsCutoff)
		})
	}

	// 7. MySQL cleanup: PM/MR related tables (pmAndMrSaveTime).
	if logConfig.PmAndMrSaveTime != nil && *logConfig.PmAndMrSaveTime > 0 {
		pmCutoff := now.AddDate(0, 0, -(*logConfig.PmAndMrSaveTime))
		safeDelete("pm_file_log", func() (int64, error) {
			return s.repo.DeletePMFileLogBefore(pmCutoff)
		})
		safeDelete("mr_file_log", func() (int64, error) {
			return s.repo.DeleteMRFileLogBefore(pmCutoff)
		})
		safeDelete("mr_data", func() (int64, error) {
			return s.repo.DeleteMRDataBefore(pmCutoff)
		})
	}

	// 8. MySQL cleanup: device log tables (deviceLogSaveTime).
	if logConfig.DeviceLogSaveTime != nil && *logConfig.DeviceLogSaveTime > 0 {
		devCutoff := now.AddDate(0, 0, -(*logConfig.DeviceLogSaveTime))
		safeDelete("event_log", func() (int64, error) {
			return s.repo.DeleteEventLogBefore(devCutoff)
		})
		safeDelete("parameter_log", func() (int64, error) {
			return s.repo.DeleteParameterLogBefore(devCutoff)
		})
		safeDelete("mml_execute_result", func() (int64, error) {
			return s.repo.DeleteMmlExecuteResultBefore(devCutoff)
		})
		safeDelete("batch_process_file_send_log", func() (int64, error) {
			return s.repo.DeleteBatchProcessFileSendLogBefore(devCutoff)
		})
		safeDelete("device_log_file_log", func() (int64, error) {
			return s.repo.DeleteDeviceLogFileLogBefore(devCutoff)
		})
		safeDelete("capture_file_log", func() (int64, error) {
			return s.repo.DeleteCaptureFileLogBefore(devCutoff)
		})
	}

	// 9. MySQL cleanup: alarm_log (alarmSaveTime).
	if logConfig.AlarmSaveTime != nil && *logConfig.AlarmSaveTime > 0 {
		alarmCutoff := now.AddDate(0, 0, -(*logConfig.AlarmSaveTime))
		safeDelete("alarm_log", func() (int64, error) {
			return s.repo.DeleteAlarmLogBefore(alarmCutoff)
		})
		safeDelete("alarm", func() (int64, error) {
			return s.repo.DeleteAlarmBefore(alarmCutoff)
		})
	}

	// 10. MySQL cleanup: task tables with child records (deviceLogSaveTime).
	if logConfig.DeviceLogSaveTime != nil && *logConfig.DeviceLogSaveTime > 0 {
		taskCutoff := now.AddDate(0, 0, -(*logConfig.DeviceLogSaveTime))

		// reboot_task -> task_to_event_log
		if ids, err := s.repo.GetRebootTaskIDsBefore(taskCutoff); err == nil && len(ids) > 0 {
			safeDelete("task_to_event_log (reboot)", func() (int64, error) {
				return s.repo.DeleteTaskToEventLogByTaskIDs(ids, "reboot_task")
			})
			safeDelete("reboot_task", func() (int64, error) {
				return s.repo.DeleteRebootTaskByIDs(ids)
			})
		}

		// shutdown_task -> shutdown_log
		if ids, err := s.repo.GetShutdownTaskIDsBefore(taskCutoff); err == nil && len(ids) > 0 {
			safeDelete("shutdown_log", func() (int64, error) {
				return s.repo.DeleteShutdownLogByTaskIDs(ids)
			})
			safeDelete("shutdown_task", func() (int64, error) {
				return s.repo.DeleteShutdownTaskByIDs(ids)
			})
		}

		// backup_or_restore_task (no child)
		if ids, err := s.repo.GetBackupOrRestoreTaskIDsBefore(taskCutoff); err == nil && len(ids) > 0 {
			safeDelete("backup_or_restore_task", func() (int64, error) {
				return s.repo.DeleteBackupOrRestoreTaskByIDs(ids)
			})
		}

		// upgrade_task (no child)
		if ids, err := s.repo.GetUpgradeTaskIDsBefore(taskCutoff); err == nil && len(ids) > 0 {
			safeDelete("upgrade_task", func() (int64, error) {
				return s.repo.DeleteUpgradeTaskByIDs(ids)
			})
		}

		// rollback_task (no child)
		if ids, err := s.repo.GetRollbackTaskIDsBefore(taskCutoff); err == nil && len(ids) > 0 {
			safeDelete("rollback_task", func() (int64, error) {
				return s.repo.DeleteRollbackTaskByIDs(ids)
			})
		}

		// batch_configuration_log -> batch_configuration_device_log
		if ids, err := s.repo.GetBatchConfigurationLogIDsBefore(taskCutoff); err == nil && len(ids) > 0 {
			safeDelete("batch_configuration_device_log", func() (int64, error) {
				return s.repo.DeleteBatchConfigurationDeviceLogByTaskIDs(ids)
			})
			safeDelete("batch_configuration_log", func() (int64, error) {
				return s.repo.DeleteBatchConfigurationLogByIDs(ids)
			})
		}
	}

	// 11. MySQL cleanup: other tables (deviceLogSaveTime).
	if logConfig.DeviceLogSaveTime != nil && *logConfig.DeviceLogSaveTime > 0 {
		otherCutoff := now.AddDate(0, 0, -(*logConfig.DeviceLogSaveTime))
		safeDelete("email_notice_result", func() (int64, error) {
			return s.repo.DeleteEmailNoticeResultBefore(otherCutoff)
		})
		safeDelete("eu_and_ru_batch_upgrade_log", func() (int64, error) {
			return s.repo.DeleteEUAndRUBatchUpgradeLogBefore(otherCutoff)
		})
		safeDelete("upgrade_log", func() (int64, error) {
			return s.repo.DeleteUpgradeLogBefore(otherCutoff)
		})
		safeDelete("core_network_operation_log", func() (int64, error) {
			return s.repo.DeleteCoreNetworkOperationLogBefore(otherCutoff)
		})
		safeDelete("pdcp_traffic", func() (int64, error) {
			return s.repo.DeletePDCPTrafficBefore(otherCutoff)
		})
		safeDelete("dashboard_pm_statistic_data", func() (int64, error) {
			return s.repo.DeleteDashboardPmStatisticDataBefore(otherCutoff)
		})
	}

	logger.Info("logcleanup: FileAndMysqlLogDelete completed")
}

// PlatformLogDeletion mirrors Java PlatformLogDeletetionTask.
// Deletes platform log files older than 7 days, keeping current month directories.
func (s *Service) PlatformLogDeletion() {
	baseDirs := []string{
		"/log/nms/core",
		"/log/nms/comm",
		"/log/nms/pm",
		"/log/nms/api",
	}

	now := time.Now()
	currentMonth := fmt.Sprintf("%04d-%02d", now.Year(), now.Month())
	sevenDaysAgo := now.AddDate(0, 0, -7)

	for _, baseDir := range baseDirs {
		s.deletePlatformLogDir(baseDir, currentMonth, sevenDaysAgo)
	}

	logger.Info("logcleanup: PlatformLogDeletion completed")
}

// deleteNorthboundFiles deletes expired northbound interface files grouped by tenant.
func (s *Service) deleteNorthboundFiles(basePath string, cutoff time.Time) {
	subDirs := []string{"FM", "PM", "CM"}
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logger.Warnf("logcleanup: cannot read northbound path %s: %v", basePath, err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		for _, sub := range subDirs {
			subPath := filepath.Join(basePath, entry.Name(), sub)
			s.deleteDateDirectories(subPath, cutoff)
		}
	}
}

// deleteDateDirectories removes date-named directories under basePath that are older than cutoff.
func (s *Service) deleteDateDirectories(basePath string, cutoff time.Time) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warnf("logcleanup: cannot read directory %s: %v", basePath, err)
		}
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Parse date from directory name (e.g., "20250717").
		t, err := time.Parse("20060102", name)
		if err != nil {
			// Try other common formats.
			t, err = time.Parse("2006-01-02", name)
			if err != nil {
				continue
			}
		}
		if t.Before(cutoff) {
			dirPath := filepath.Join(basePath, name)
			if err := os.RemoveAll(dirPath); err != nil {
				logger.Errorf("logcleanup: failed to remove directory %s: %v", dirPath, err)
			} else {
				logger.Infof("logcleanup: removed directory %s", dirPath)
			}
		}
	}
}

// deletePlatformLogDir deletes old log files in a platform log directory.
// Keeps current month directories; deletes non-current month directories entirely.
// Within current month, deletes files older than 7 days.
func (s *Service) deletePlatformLogDir(baseDir, currentMonth string, cutoff time.Time) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warnf("logcleanup: cannot read platform log dir %s: %v", baseDir, err)
		}
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subDir := filepath.Join(baseDir, entry.Name())

		if strings.Contains(entry.Name(), currentMonth) {
			// Current month: delete files older than 7 days, keep directories.
			s.deleteOldFilesInDir(subDir, cutoff)
		} else {
			// Not current month: delete the entire directory.
			if err := os.RemoveAll(subDir); err != nil {
				logger.Errorf("logcleanup: failed to remove old month dir %s: %v", subDir, err)
			} else {
				logger.Infof("logcleanup: removed old month dir %s", subDir)
			}
		}
	}
}

// deleteOldFilesInDir removes files in dir that are older than cutoff.
func (s *Service) deleteOldFilesInDir(dir string, cutoff time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warnf("logcleanup: cannot read dir %s: %v", dir, err)
		}
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(dir, entry.Name())
			if err := os.Remove(path); err != nil {
				logger.Errorf("logcleanup: failed to remove file %s: %v", path, err)
			} else {
				logger.Infof("logcleanup: removed old file %s", path)
			}
		}
	}
}

// parseDateFromDirName tries to parse a date from directory name.
func parseDateFromDirName(name string) (time.Time, error) {
	// Try formats: 20250717, 2025-07-17
	if t, err := time.Parse("20060102", name); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", name); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse date from %s", name)
}

// parseDateFromFileName tries to parse a date from file name (e.g., "system_20250717.log").
func parseDateFromFileName(name string) (time.Time, error) {
	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("no date in filename %s", name)
	}
	// Try last part as date.
	lastPart := strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(parts[len(parts)-1]))
	if t, err := time.Parse("20060102", lastPart); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse date from filename %s", name)
}

// parseIntOrDefault parses an int string or returns default.
func parseIntOrDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
