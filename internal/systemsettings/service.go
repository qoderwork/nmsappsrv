package systemsettings

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"gorm.io/gorm"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// SystemSettingsService provides business logic for system settings.
type SystemSettingsService struct {
	repo   *SystemSettingsRepository
	aesKey []byte
}

// NewSystemSettingsService creates a new SystemSettingsService.
func NewSystemSettingsService(db *gorm.DB, aesKeyHex string) *SystemSettingsService {
	aesKey, _ := hex.DecodeString(aesKeyHex)
	return &SystemSettingsService{
		repo:   NewSystemSettingsRepository(db),
		aesKey: aesKey,
	}
}

// GetDeviceSettings reads device configuration for a specific tenancy.
func (s *SystemSettingsService) GetDeviceSettings(tenancyId int) (*DeviceConfig, error) {
	key := fmt.Sprintf("device_config_%d", tenancyId)
	value, err := s.repo.GetSystemConfig(key)
	if err != nil {
		return nil, err
	}

	if value == "" {
		return &DeviceConfig{}, nil
	}

	var cfg DeviceConfig
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		return nil, apperror.Wrap(err, "UNMARSHAL_DEVICE_CONFIG_FAILED", 500, "failed to unmarshal device config")
	}

	return &cfg, nil
}

// UpdateDeviceSettings updates device configuration for a specific tenancy.
func (s *SystemSettingsService) UpdateDeviceSettings(req *UpdateDeviceSettingsRequest, tenancyId int) error {
	key := fmt.Sprintf("device_config_%d", tenancyId)

	// Get existing config
	existing, err := s.GetDeviceSettings(tenancyId)
	if err != nil {
		return err
	}

	// Merge with request
	if req.AutoRegistration != nil {
		existing.AutoRegistration = req.AutoRegistration
	}
	if req.AutoRegistrationKey != nil {
		existing.AutoRegistrationKey = req.AutoRegistrationKey
	}
	if req.MaxDeviceCount != nil {
		existing.MaxDeviceCount = req.MaxDeviceCount
	}
	if req.DefaultDeviceName != nil {
		existing.DefaultDeviceName = req.DefaultDeviceName
	}

	// Marshal to JSON
	data, err := json.Marshal(existing)
	if err != nil {
		return apperror.Wrap(err, "MARSHAL_DEVICE_CONFIG_FAILED", 500, "failed to marshal device config")
	}

	// Save to DB
	if err := s.repo.SaveSystemConfig(key, string(data)); err != nil {
		return err
	}

	// Save auto-registration key to Redis
	if req.AutoRegistrationKey != nil {
		redisKey := fmt.Sprintf("auto_registration_key:%d", tenancyId)
		if err := redis.Set(context.Background(), redisKey, *req.AutoRegistrationKey, 0); err != nil {
			logger.Errorf("Failed to save auto-registration key to Redis: %v", err)
		}
	}

	return nil
}

// GetACSConfig reads the ACS configuration from system_config and sys_parameter.
func (s *SystemSettingsService) GetACSConfig() (*ACSConfig, error) {
	value, err := s.repo.GetSystemConfig("nms_config")
	if err != nil {
		return nil, err
	}

	var cfg ACSConfig
	if value != "" {
		if err := json.Unmarshal([]byte(value), &cfg); err != nil {
			return nil, apperror.Wrap(err, "UNMARSHAL_ACS_CONFIG_FAILED", 500, "failed to unmarshal ACS config")
		}
	}

	// Read additional parameters from sys_parameter
	udpPortStr, err := s.repo.GetSysParameter("acs_udp_port")
	if err != nil {
		return nil, err
	}
	if udpPortStr != "" {
		if port, err := strconv.Atoi(udpPortStr); err == nil {
			cfg.UDPPort = intPtr(port)
		}
	}

	return &cfg, nil
}

// UpdateACSConfig updates the ACS configuration with password encryption.
func (s *SystemSettingsService) UpdateACSConfig(req *UpdateACSConfigRequest) error {
	// Get existing config
	existing, err := s.GetACSConfig()
	if err != nil {
		return err
	}

	// Merge with request
	if req.AcsUrl != nil {
		existing.AcsUrl = req.AcsUrl
	}
	if req.AcsUsername != nil {
		existing.AcsUsername = req.AcsUsername
	}
	if req.AcsPassword != nil {
		// Encrypt password
		encrypted, err := s.encrypt(*req.AcsPassword)
		if err != nil {
			return apperror.Wrap(err, "ENCRYPT_PASSWORD_FAILED", 500, "failed to encrypt password")
		}
		existing.AcsPassword = &encrypted
	}
	if req.ConnectionTimeout != nil {
		existing.ConnectionTimeout = req.ConnectionTimeout
	}
	if req.InformInterval != nil {
		existing.InformInterval = req.InformInterval
	}
	if req.TR069Enabled != nil {
		existing.TR069Enabled = req.TR069Enabled
	}

	// Marshal to JSON
	data, err := json.Marshal(existing)
	if err != nil {
		return apperror.Wrap(err, "MARSHAL_ACS_CONFIG_FAILED", 500, "failed to marshal ACS config")
	}

	// Save to DB
	if err := s.repo.SaveSystemConfig("nms_config", string(data)); err != nil {
		return err
	}

	// Save to Redis cache
	if err := redis.Set(context.Background(), "nms_config", string(data), 0); err != nil {
		logger.Errorf("Failed to save ACS config to Redis: %v", err)
	}

	// Save UDP port to sys_parameter if provided
	if req.UDPPort != nil {
		if err := s.repo.SaveSysParameter("acs_udp_port", strconv.Itoa(*req.UDPPort)); err != nil {
			logger.Errorf("Failed to save UDP port to sys_parameter: %v", err)
		}
	}

	return nil
}

// GetLogConfig reads the log configuration from system_config.
func (s *SystemSettingsService) GetLogConfig() (*LogConfig, error) {
	value, err := s.repo.GetSystemConfig("nms_log_config")
	if err != nil {
		return nil, err
	}

	if value == "" {
		return &LogConfig{}, nil
	}

	var cfg LogConfig
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		return nil, apperror.Wrap(err, "UNMARSHAL_LOG_CONFIG_FAILED", 500, "failed to unmarshal log config")
	}

	return &cfg, nil
}

// UpdateLogConfig updates the log configuration in system_config.
func (s *SystemSettingsService) UpdateLogConfig(req *UpdateLogConfigRequest) error {
	// Get existing config
	existing, err := s.GetLogConfig()
	if err != nil {
		return err
	}

	// Merge with request
	if req.RetentionDays != nil {
		existing.RetentionDays = req.RetentionDays
	}
	if req.MaxFileSizeMb != nil {
		existing.MaxFileSizeMb = req.MaxFileSizeMb
	}
	if req.AutoCleanup != nil {
		existing.AutoCleanup = req.AutoCleanup
	}

	// Marshal to JSON
	data, err := json.Marshal(existing)
	if err != nil {
		return apperror.Wrap(err, "MARSHAL_LOG_CONFIG_FAILED", 500, "failed to marshal log config")
	}

	// Save to DB
	return s.repo.SaveSystemConfig("nms_log_config", string(data))
}

// encrypt encrypts plaintext using AES-GCM.
func (s *SystemSettingsService) encrypt(plaintext string) (string, error) {
	if len(s.aesKey) == 0 {
		return plaintext, nil // No encryption if key not configured
	}

	block, err := aes.NewCipher(s.aesKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Helper functions

func intPtr(i int) *int {
	return &i
}
