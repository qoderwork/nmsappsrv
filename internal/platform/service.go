package platform

import (
	"archive/zip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nmsappsrv/internal/config"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
)

// Service defines the business-logic contract for platform operations.
type Service interface {
	GetTime() int64
	GetSupportedZone() []string
	GetLogo(licenseLogo string) string
	GetLogConfig() (*LogConfig, error)
	UpdateLogConfig(cfg *LogConfig) error
	GetFTPTransferLogConfig() (*FTPTransferLogConfig, error)
	UpdateFTPTransferLogConfig(cfg *FTPTransferLogConfig) error
	GetHECConfig() (*HECConfig, error)
	UpdateHECConfig(cfg *HECConfig) error
	ListNMSSecret() (*NMSSecret, error)
	UpdateNMSSecret(secret *NMSSecret) error
	StartLogCollection() (string, error)
	GetLogCollectionStatus(taskId string) (status, filePath string, err error)
	DownloadCollectedLogs(taskId string) (string, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo   Repository
	aesKey []byte // 32-byte AES-256 key
}

// NewService creates a new Service
func NewService(repo Repository, aesKeyHex string) Service {
	key := make([]byte, 32)
	if aesKeyHex != "" {
		decoded, err := hex.DecodeString(aesKeyHex)
		if err == nil && len(decoded) >= 16 {
			copy(key, decoded)
		}
	}
	return &service{repo: repo, aesKey: key}
}

// GetTime returns current server time in milliseconds
func (s *service) GetTime() int64 {
	return time.Now().UnixMilli()
}

// GetSupportedZone returns all available timezone IDs
func (s *service) GetSupportedZone() []string {
	// Go doesn't have a direct equivalent of ZoneId.getAvailableZoneIds()
	// Return a comprehensive list of common timezones
	return commonTimezones
}

// GetLogo returns the logo base64 string from license/tenancy
func (s *service) GetLogo(licenseLogo string) string {
	if licenseLogo != "" {
		return licenseLogo
	}
	// Return empty string if no logo configured (frontend should use default)
	return ""
}

// ---------- Log Config ----------

// GetLogConfig returns the current log level configuration
func (s *service) GetLogConfig() (*LogConfig, error) {
	value, err := s.repo.GetSystemConfig("platform_log_config")
	if err != nil {
		return nil, err
	}

	cfg := &LogConfig{Level: "info"} // default
	if value != "" {
		if err := json.Unmarshal([]byte(value), cfg); err != nil {
			return cfg, nil
		}
	}
	if cfg.Level == "" {
		cfg.Level = "info"
	}
	return cfg, nil
}

// UpdateLogConfig updates the log level configuration
func (s *service) UpdateLogConfig(cfg *LogConfig) error {
	if !isValidLogLevel(cfg.Level) {
		return apperror.ErrInvalidInput.WithMessage("invalid log level: " + cfg.Level)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.repo.SaveSystemConfig("platform_log_config", string(data))
}

// ---------- FTP Transfer Log Config ----------

// GetFTPTransferLogConfig returns the FTP transfer log config with decrypted password
func (s *service) GetFTPTransferLogConfig() (*FTPTransferLogConfig, error) {
	value, err := s.repo.GetSystemConfig("log_ftp_transfer_config")
	if err != nil {
		return nil, err
	}

	cfg := &FTPTransferLogConfig{PasswordChange: boolPtr(false)}
	if value == "" {
		return cfg, nil
	}

	if err := json.Unmarshal([]byte(value), cfg); err != nil {
		return &FTPTransferLogConfig{PasswordChange: boolPtr(false)}, nil
	}

	// Decrypt password
	if cfg.Password != "" {
		decrypted := s.decrypt(cfg.Password)
		cfg.Password = maskPassword(decrypted)
	}
	cfg.PasswordChange = boolPtr(false)
	return cfg, nil
}

// UpdateFTPTransferLogConfig updates the FTP transfer log config with encrypted password
func (s *service) UpdateFTPTransferLogConfig(cfg *FTPTransferLogConfig) error {
	if cfg.Host == "" || cfg.Username == "" || cfg.Password == "" || cfg.UploadPath == "" || cfg.Port == nil {
		return apperror.ErrInvalidInput.WithMessage("host, username, password, uploadPath and port are required")
	}

	// Check if existing config exists
	existing, _ := s.repo.GetSystemConfig("log_ftp_transfer_config")
	if existing != "" {
		var existingCfg FTPTransferLogConfig
		if err := json.Unmarshal([]byte(existing), &existingCfg); err == nil {
			existingCfg.Host = cfg.Host
			existingCfg.Port = cfg.Port
			existingCfg.Username = cfg.Username
			existingCfg.UploadPath = cfg.UploadPath
			// Only update password if passwordChange is true
			if cfg.PasswordChange != nil && *cfg.PasswordChange {
				existingCfg.Password = s.encrypt(cfg.Password)
			}
			data, err := json.Marshal(existingCfg)
			if err != nil {
				return err
			}
			return s.repo.SaveSystemConfig("log_ftp_transfer_config", string(data))
		}
	}

	// New config — encrypt password
	cfg.Password = s.encrypt(cfg.Password)
	cfg.PasswordChange = nil
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.repo.SaveSystemConfig("log_ftp_transfer_config", string(data))
}

// ---------- HEC Config ----------

// GetHECConfig returns the HEC configuration
func (s *service) GetHECConfig() (*HECConfig, error) {
	value, err := s.repo.GetSystemConfig("hec_config")
	if err != nil {
		return nil, err
	}

	cfg := &HECConfig{}
	if value == "" {
		return cfg, nil
	}

	if err := json.Unmarshal([]byte(value), cfg); err != nil {
		return &HECConfig{}, nil
	}
	return cfg, nil
}

// UpdateHECConfig updates the HEC configuration
func (s *service) UpdateHECConfig(cfg *HECConfig) error {
	if cfg.URL == "" || cfg.Token == "" {
		return apperror.ErrInvalidInput.WithMessage("url and token are required")
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.repo.SaveSystemConfig("hec_config", string(data))
}

// ---------- NMS Secret ----------

// ListNMSSecret returns NMS secret settings
func (s *service) ListNMSSecret() (*NMSSecret, error) {
	value, err := s.repo.GetSystemConfig("nms_secret")
	if err != nil {
		return nil, err
	}

	secret := &NMSSecret{
		EmailSecret:    defaultEmailSecret,
		AddressSecret:  defaultAddressSecret,
		PasswordSecret: defaultPasswordSecret,
	}

	if value == "" {
		return secret, nil
	}

	if err := json.Unmarshal([]byte(value), secret); err != nil {
		return secret, nil
	}

	if secret.EmailSecret == "" {
		secret.EmailSecret = defaultEmailSecret
	}
	if secret.AddressSecret == "" {
		secret.AddressSecret = defaultAddressSecret
	}
	if secret.PasswordSecret == "" {
		secret.PasswordSecret = defaultPasswordSecret
	}
	return secret, nil
}

// UpdateNMSSecret updates NMS secret settings
func (s *service) UpdateNMSSecret(secret *NMSSecret) error {
	if secret.EmailSecret == "" {
		return apperror.ErrInvalidInput.WithMessage("emailSecret is required")
	}
	if !isValidSecret(secret.EmailSecret) {
		return apperror.ErrInvalidInput.WithMessage("emailSecret can only contain numbers and letters and must be 32 characters long")
	}
	data, err := json.Marshal(secret)
	if err != nil {
		return err
	}
	return s.repo.SaveSystemConfig("nms_secret", string(data))
}

// ---------- AES-GCM encryption ----------

func (s *service) encrypt(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	block, err := aes.NewCipher(s.aesKey)
	if err != nil {
		return plaintext
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return plaintext
	}
	nonce := make([]byte, aead.NonceSize())
	io.ReadFull(rand.Reader, nonce)
	ciphertext := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext)
}

func (s *service) decrypt(ciphertext string) string {
	if ciphertext == "" {
		return ""
	}
	data, err := hex.DecodeString(ciphertext)
	if err != nil {
		return ciphertext
	}
	block, err := aes.NewCipher(s.aesKey)
	if err != nil {
		return ciphertext
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return ciphertext
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return ciphertext
	}
	nonce, ct := data[:nonceSize], data[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return ciphertext
	}
	return string(plaintext)
}

// ---------- helpers ----------

func isValidLogLevel(level string) bool {
	valid := []string{"all", "trace", "debug", "info", "warn", "error", "fatal", "off"}
	for _, v := range valid {
		if v == level {
			return true
		}
	}
	return false
}

func isValidSecret(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

func maskPassword(password string) string {
	if len(password) <= 4 {
		return "****"
	}
	return password[:2] + strings.Repeat("*", len(password)-4) + password[len(password)-2:]
}

func boolPtr(b bool) *bool {
	return &b
}

// ---------- Platform Log Collection (async) ----------

// logDownloadStatus is stored in Redis as JSON for each collection task.
type logDownloadStatus struct {
	Status   string `json:"status"`   // "collecting" | "ready" | "failed"
	FilePath string `json:"filePath"` // populated when status == "ready"
	Error    string `json:"error"`    // populated when status == "failed"
}

const logDownloadKeyPrefix = "platform:log_download:"
const logDownloadTTL = 30 * time.Minute

// StartLogCollection generates a task ID, stores "collecting" status in Redis,
// and launches a background goroutine that zips all .log files from the platform
// log directory. Returns the task ID immediately.
func (s *service) StartLogCollection() (string, error) {
	taskId := fmt.Sprintf("%d", time.Now().UnixNano())
	key := logDownloadKeyPrefix + taskId

	initial := logDownloadStatus{Status: "collecting"}
	data, _ := json.Marshal(initial)
	if err := redis.Set(context.Background(), key, string(data), logDownloadTTL); err != nil {
		return "", apperror.Wrap(err, "INTERNAL", 500, "failed to store log collection status")
	}

	utils.SafeGo("platform-log-collection", func() {
		s.collectLogs(taskId)
	})

	return taskId, nil
}

// collectLogs walks the platform log directory, creates a ZIP of all .log files,
// and updates Redis status accordingly.
func (s *service) collectLogs(taskId string) {
	ctx := context.Background()
	key := logDownloadKeyPrefix + taskId

	// Determine log directory from config or use default
	logDir := "./logs/platform"
	if config.Cfg != nil && config.Cfg.PlatformFiles.PlatformLogDir != "" {
		logDir = config.Cfg.PlatformFiles.PlatformLogDir
	}

	// Create temp directory for the ZIP file
	tmpDir, err := os.MkdirTemp("", "platform-logs-")
	if err != nil {
		s.updateLogStatus(ctx, key, "failed", "", fmt.Sprintf("failed to create temp dir: %v", err))
		return
	}

	zipPath := filepath.Join(tmpDir, "platform-logs.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		s.updateLogStatus(ctx, key, "failed", "", fmt.Sprintf("failed to create zip file: %v", err))
		return
	}

	zw := zip.NewWriter(zipFile)
	fileCount := 0

	err = filepath.Walk(logDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".log" {
			return nil
		}

		// Compute relative path for the archive entry
		relPath, err := filepath.Rel(logDir, path)
		if err != nil {
			relPath = filepath.Base(path)
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			logger.Errorf("platform log collection: zip header error for %s: %v", path, err)
			return nil // skip this file
		}
		header.Name = relPath
		header.Method = zip.Deflate

		writer, err := zw.CreateHeader(header)
		if err != nil {
			logger.Errorf("platform log collection: zip create error for %s: %v", path, err)
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			logger.Errorf("platform log collection: open error for %s: %v", path, err)
			return nil
		}
		defer f.Close()

		if _, err := io.Copy(writer, f); err != nil {
			logger.Errorf("platform log collection: copy error for %s: %v", path, err)
			return nil
		}

		fileCount++
		return nil
	})

	// Close the zip writer and file
	if closeErr := zw.Close(); closeErr != nil {
		logger.Errorf("platform log collection: zip close error: %v", closeErr)
	}
	if closeErr := zipFile.Close(); closeErr != nil {
		logger.Errorf("platform log collection: file close error: %v", closeErr)
	}

	if err != nil {
		os.Remove(zipPath)
		s.updateLogStatus(ctx, key, "failed", "", fmt.Sprintf("failed to walk log directory: %v", err))
		return
	}

	if fileCount == 0 {
		os.Remove(zipPath)
		s.updateLogStatus(ctx, key, "failed", "", "no .log files found in "+logDir)
		return
	}

	logger.Infof("platform log collection: zipped %d files to %s", fileCount, zipPath)
	s.updateLogStatus(ctx, key, "ready", zipPath, "")
}

func (s *service) updateLogStatus(ctx context.Context, key, status, filePath, errMsg string) {
	st := logDownloadStatus{Status: status, FilePath: filePath, Error: errMsg}
	data, _ := json.Marshal(st)
	if err := redis.Set(ctx, key, string(data), logDownloadTTL); err != nil {
		logger.Errorf("platform log collection: failed to update status for key %s: %v", key, err)
	}
}

// GetLogCollectionStatus reads the current status of a log collection task from Redis.
func (s *service) GetLogCollectionStatus(taskId string) (status, filePath string, err error) {
	key := logDownloadKeyPrefix + taskId
	val, err := redis.Get(context.Background(), key)
	if err != nil {
		return "", "", apperror.Wrap(err, "NOT_FOUND", 404, "task not found or expired")
	}

	var st logDownloadStatus
	if err := json.Unmarshal([]byte(val), &st); err != nil {
		return "", "", apperror.Wrap(err, "INTERNAL", 500, "invalid status data")
	}
	return st.Status, st.FilePath, nil
}

// DownloadCollectedLogs checks that the task is "ready" and returns the file path.
func (s *service) DownloadCollectedLogs(taskId string) (string, error) {
	status, filePath, err := s.GetLogCollectionStatus(taskId)
	if err != nil {
		return "", err
	}
	if status != "ready" {
		return "", apperror.New("LOGS_NOT_READY", 409, "logs not ready, current status: "+status)
	}
	if filePath == "" {
		return "", apperror.ErrInternal.WithMessage("file path is empty for task " + taskId)
	}
	return filePath, nil
}

// commonTimezones is a representative subset of IANA timezone names
var commonTimezones = []string{
	"Africa/Abidjan", "Africa/Cairo", "Africa/Johannesburg", "Africa/Lagos", "Africa/Nairobi",
	"America/Anchorage", "America/Argentina/Buenos_Aires", "America/Bogota", "America/Chicago",
	"America/Denver", "America/Halifax", "America/Lima", "America/Los_Angeles", "America/Mexico_City",
	"America/New_York", "America/Phoenix", "America/Santiago", "America/Sao_Paulo", "America/Toronto",
	"America/Vancouver", "America/Winnipeg",
	"Asia/Almaty", "Asia/Baghdad", "Asia/Bangkok", "Asia/Colombo", "Asia/Dhaka", "Asia/Dubai",
	"Asia/Ho_Chi_Minh", "Asia/Hong_Kong", "Asia/Jakarta", "Asia/Karachi", "Asia/Kolkata",
	"Asia/Kuala_Lumpur", "Asia/Kuwait", "Asia/Manila", "Asia/Muscat", "Asia/Riyadh",
	"Asia/Seoul", "Asia/Shanghai", "Asia/Singapore", "Asia/Taipei", "Asia/Tehran",
	"Asia/Tokyo", "Asia/Yangon",
	"Atlantic/Azores", "Atlantic/Reykjavik",
	"Australia/Adelaide", "Australia/Brisbane", "Australia/Darwin", "Australia/Hobart",
	"Australia/Melbourne", "Australia/Perth", "Australia/Sydney",
	"Europe/Amsterdam", "Europe/Athens", "Europe/Belgrade", "Europe/Berlin", "Europe/Brussels",
	"Europe/Bucharest", "Europe/Budapest", "Europe/Copenhagen", "Europe/Dublin", "Europe/Helsinki",
	"Europe/Istanbul", "Europe/Kiev", "Europe/Lisbon", "Europe/London", "Europe/Madrid",
	"Europe/Moscow", "Europe/Oslo", "Europe/Paris", "Europe/Prague", "Europe/Rome",
	"Europe/Stockholm", "Europe/Vienna", "Europe/Warsaw", "Europe/Zurich",
	"Pacific/Auckland", "Pacific/Fiji", "Pacific/Guam", "Pacific/Honolulu", "Pacific/Midway",
	"UTC",
}

// newService creates a Service backed by the given Repository (test/mock helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}
