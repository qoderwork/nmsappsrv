package filebase

import (
	"os"
	"path/filepath"

	"nmsappsrv/internal/config"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// MRIngester is implemented by the mr package. filebase calls it when a device
// uploads an MR file so parsing happens inside the same service (no shared
// volume between the Java api and corefunction modules).
type MRIngester interface {
	IngestMR(filePath string, elementId int64) error
}

// Service owns the on-disk layout for the device-facing /acs-file-server/**
// endpoints and provides safe save helpers. It mirrors Java's FileServiceImpl
// path roots (config.FileServerConfig), partitioned per tenancy/element where
// Java does the same. The *gorm.DB handle lets the FileDownload providers
// resolve on-disk paths from the same records Java reads (base_station_license,
// ca_file, cpe_element, upgrade_file).
type Service struct {
	cfg config.FileServerConfig
	db  *gorm.DB
}

// NewService creates the filebase service and ensures all root directories
// exist (Java's FileServiceImpl.@PostConstruct mkdirs).
func NewService(cfg config.FileServerConfig, db *gorm.DB) *Service {
	s := &Service{cfg: cfg, db: db}
	s.ensureDirs()
	return s
}

func (s *Service) ensureDirs() {
	dirs := []string{
		s.cfg.Root, s.cfg.UpgradeDir, s.cfg.ConfigDir, s.cfg.BatchProcessDir,
		s.cfg.LogDir, s.cfg.MrDir, s.cfg.CaptureDir, s.cfg.MmlResultDir,
		s.cfg.PmDir, s.cfg.NrmDir, s.cfg.DeviceMNormalDir, s.cfg.CmFilePath,
		s.cfg.PiecemealTempDir, s.cfg.MrCCSExportDir,
	}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			logger.Errorf("filebase: failed to create dir %q: %v", d, err)
		}
	}
}

// BpfPath returns the path for a bpf (batch-process) file, honouring the
// optional licenseId sub-directory exactly like Java's FileDownloadController
// (batchProcessFilePath/{licenseId}/{fileName}).
func (s *Service) BpfPath(licenseID string, fileName string) string {
	if licenseID != "" {
		return filepath.Join(s.cfg.BatchProcessDir, licenseID, fileName)
	}
	return filepath.Join(s.cfg.BatchProcessDir, fileName)
}

// ConfigPath returns configFilePath/{tenancyId}/{elementId}/{fileName}.
func (s *Service) ConfigPath(tenancyID, elementID int64, fileName string) string {
	return filepath.Join(s.cfg.ConfigDir, itoa(tenancyID), itoa(elementID), fileName)
}

// MmlResultPath returns mmlExecuteResultFilePath/{elementId}/{fileName}.
func (s *Service) MmlResultPath(elementID int64, fileName string) string {
	return filepath.Join(s.cfg.MmlResultDir, itoa(elementID), fileName)
}
