package filebase

import (
	"os"
	"path/filepath"
)

// Read-only view structs for the tables that back the device-facing
// /acs-file-server/** download endpoints. They intentionally carry only the
// columns needed to resolve the on-disk path, so filebase stays decoupled from
// the owning modules (license / cacert / device / upgrade) and avoids import
// cycles. The GORM column tags + TableName make the queries self-contained.

type bsLicenseView struct {
	FileName         *string `gorm:"column:file_name"`
	OriginalFileName *string `gorm:"column:original_file_name"`
}

func (bsLicenseView) TableName() string { return "base_station_license" }

type caFileView struct {
	FileName *string `gorm:"column:file_name"`
}

func (caFileView) TableName() string { return "ca_file" }

type cpeElementView struct {
	ConfigFile  *string `gorm:"column:config_file"`
	LicenseId   *int    `gorm:"column:license_id"`
	AosFileName *string `gorm:"column:aos_file_name"`
}

func (cpeElementView) TableName() string { return "cpe_element" }

type upgradeFileView struct {
	FilePath         *string `gorm:"column:file_path"`
	FileName         *string `gorm:"column:file_name"`
	OriginalFileName *string `gorm:"column:original_file_name"`
}

func (upgradeFileView) TableName() string { return "upgrade_file" }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// resolveLicense mirrors Java BaseStationLicenseManagementServiceImpl.
// downloadLicense: licenseFilePath + "/" + <file_name>, served as the stored
// original_file_name. Returns ok=false (→ empty 200, like Java) when the
// base_station_license row or its file_name is absent.
func (s *Service) resolveLicense(elementId int64) (absPath, name string, ok bool) {
	var v bsLicenseView
	if err := s.db.Where("element_id = ?", elementId).First(&v).Error; err != nil || v.FileName == nil {
		return "", "", false
	}
	dir := s.cfg.LicenseDir
	if dir == "" {
		dir = filepath.Join(s.cfg.Root, "license")
	}
	return filepath.Join(dir, filepath.Base(deref(v.FileName))), filepath.Base(deref(v.OriginalFileName)), true
}

// resolveCa mirrors Java CaFileServiceImpl.downloadcaFile: caFilePath + "/" +
// <file_name>, served as the base file name.
func (s *Service) resolveCa(fileId int) (absPath, name string, ok bool) {
	var v caFileView
	if err := s.db.Where("id = ?", fileId).First(&v).Error; err != nil || v.FileName == nil {
		return "", "", false
	}
	dir := s.cfg.CaDir
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "ca_files")
	}
	base := filepath.Base(deref(v.FileName))
	return filepath.Join(dir, base), base, true
}

// resolveConfigFile mirrors Java BaseStationBackupAndRestoreManagementServiceImpl.
// downloadConfigFile: configFilePath + "/" + (licenseId|0) + "/" + elementId +
// "/" + <config_file>, served as the config_file base name.
func (s *Service) resolveConfigFile(elementId int64) (absPath, name string, ok bool) {
	var v cpeElementView
	if err := s.db.Where("id = ?", elementId).First(&v).Error; err != nil || v.ConfigFile == nil || *v.ConfigFile == "" {
		return "", "", false
	}
	licenseId := 0
	if v.LicenseId != nil {
		licenseId = *v.LicenseId
	}
	base := filepath.Base(deref(v.ConfigFile))
	p := filepath.Join(s.cfg.ConfigDir, itoa(int64(licenseId)), itoa(elementId), base)
	return p, base, true
}

// resolveZtpFile mirrors Java AOSManagementServiceImpl.downloadZTPFile:
// ztp/aos root + "/" + <aos_file_name>, served as the base name.
func (s *Service) resolveZtpFile(elementId int64) (absPath, name string, ok bool) {
	var v cpeElementView
	if err := s.db.Where("id = ?", elementId).First(&v).Error; err != nil || v.AosFileName == nil || *v.AosFileName == "" {
		return "", "", false
	}
	dir := s.cfg.ZtpDir
	if dir == "" {
		dir = filepath.Join(s.cfg.Root, "ztp")
	}
	base := filepath.Base(deref(v.AosFileName))
	return filepath.Join(dir, base), base, true
}

// resolveUpgradeFile mirrors Java UpgradeManagementServiceImpl.downloadUpgradeFile:
// serves the absolute upgrade_file.file_path, named by original_file_name (or
// file_name when the original is empty).
func (s *Service) resolveUpgradeFile(fileId int) (absPath, name string, ok bool) {
	var v upgradeFileView
	if err := s.db.Where("id = ?", fileId).First(&v).Error; err != nil || v.FilePath == nil || *v.FilePath == "" {
		return "", "", false
	}
	// Java: name = original_file_name, or file_name when the original is empty.
	nm := filepath.Base(deref(v.FileName))
	if v.OriginalFileName != nil && *v.OriginalFileName != "" {
		nm = *v.OriginalFileName
	}
	return *v.FilePath, nm, true
}

// resolveMNormalFile: Java's DeviceMNormalFileManagementServiceImpl.
// downloadMNormalFile reads device_m_normal_file by id. nmsappsrv has no such
// table yet (the M-normal / ztp file domain is a later backfill), so we mirror
// Java's "record absent → empty 200" behaviour. The configured device_mnormal_dir
// is kept so the domain can be wired later without a config change.
func (s *Service) resolveMNormalFile(_ int) (absPath, name string, ok bool) {
	return "", "", false
}
