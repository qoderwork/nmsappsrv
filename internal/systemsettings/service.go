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

// passwordMask is returned for ACS secret fields on read, matching Java's
// masked (never-plaintext) representation.
const passwordMask = "******"

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
func (s *SystemSettingsService) GetDeviceSettings(tenantId int) (*DeviceConfig, error) {
	key := fmt.Sprintf("device_config_%d", tenantId)
	value, err := s.repo.GetSystemConfig(key)
	if err != nil {
		return nil, err
	}

	if value == "" {
		return defaultDeviceConfig(), nil
	}

	var cfg DeviceConfig
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		return nil, apperror.Wrap(err, "UNMARSHAL_DEVICE_CONFIG_FAILED", 500, "failed to unmarshal device config")
	}

	return &cfg, nil
}

// UpdateDeviceSettings updates device configuration for a specific tenancy.
func (s *SystemSettingsService) UpdateDeviceSettings(req *UpdateDeviceSettingsRequest, tenantId int) error {
	key := fmt.Sprintf("device_config_%d", tenantId)

	existing, err := s.GetDeviceSettings(tenantId)
	if err != nil {
		return err
	}

	// Overlay request fields
	if req.DeviceInformPeriod != nil {
		existing.DeviceInformPeriod = req.DeviceInformPeriod
	}
	if req.CpeSignalWeakThreshold != nil {
		existing.CpeSignalWeakThreshold = req.CpeSignalWeakThreshold
	}
	if req.CpeSignalStrongThreshold != nil {
		existing.CpeSignalStrongThreshold = req.CpeSignalStrongThreshold
	}
	if req.AutoRegistrationEnable != nil {
		existing.AutoRegistrationEnable = req.AutoRegistrationEnable
	}
	if req.PmFileSaveTime != nil {
		existing.PmFileSaveTime = req.PmFileSaveTime
	}
	if req.DeviceLogFileSaveTime != nil {
		existing.DeviceLogFileSaveTime = req.DeviceLogFileSaveTime
	}
	if req.AlarmSaveTime != nil {
		existing.AlarmSaveTime = req.AlarmSaveTime
	}
	if req.DeviceAuthentication != nil {
		existing.DeviceAuthentication = req.DeviceAuthentication
	}
	if req.AuthenticationAlgorithm != nil {
		existing.AuthenticationAlgorithm = req.AuthenticationAlgorithm
	}
	if req.AcsUsername != nil {
		existing.AcsUsername = req.AcsUsername
	}
	if req.AcsPassword != nil && *req.AcsPassword != "" {
		existing.AcsPassword = req.AcsPassword
	}
	if req.MaxDeviceCount != nil {
		existing.MaxDeviceCount = req.MaxDeviceCount
	}

	// Fill defaults for any still-nil field (Java SystemSettingsManagementServiceImpl).
	applyDeviceConfigDefaults(existing)

	data, err := json.Marshal(existing)
	if err != nil {
		return apperror.Wrap(err, "MARSHAL_DEVICE_CONFIG_FAILED", 500, "failed to marshal device config")
	}

	if err := s.repo.SaveSystemConfig(key, string(data)); err != nil {
		return err
	}

	// Sync to Redis — mirrors Java SystemSettingsManagementServiceImpl.updateDeviceSettings:
	// 1) nms_allow_auto_registration_{tenantId} gates auto-registration in the ACS.
	// 2) device_authentication_{tenantId} carries the ACS-level credentials
	//    (acsUsername/acsPassword) that the ACS AuthenticateInterceptor uses to
	//    validate CPE ConnectionRequest Authorization headers.
	ctx := context.Background()
	autoReg := false
	if existing.AutoRegistrationEnable != nil {
		autoReg = *existing.AutoRegistrationEnable
	}
	if err := redis.Set(ctx, fmt.Sprintf("nms_allow_auto_registration_%d", tenantId), strconv.FormatBool(autoReg), 0); err != nil {
		logger.Errorf("Failed to sync nms_allow_auto_registration_%d to Redis: %v", tenantId, err)
	}

	username := ""
	if existing.AcsUsername != nil {
		username = *existing.AcsUsername
	}
	password := ""
	if existing.AcsPassword != nil {
		password = *existing.AcsPassword
	}
	algorithm := "Null"
	if existing.AuthenticationAlgorithm != nil {
		algorithm = *existing.AuthenticationAlgorithm
	}
	enable := false
	if existing.DeviceAuthentication != nil {
		enable = *existing.DeviceAuthentication
	}
	authDTO := map[string]interface{}{
		"enable":    enable,
		"algorithm": algorithm,
		"username":  username,
		"password":  password,
	}
	authJSON, err := json.Marshal(authDTO)
	if err != nil {
		logger.Errorf("Failed to marshal ACS auth config for tenant %d: %v", tenantId, err)
	} else {
		if err := redis.Set(ctx, fmt.Sprintf("device_authentication_%d", tenantId), string(authJSON), 0); err != nil {
			logger.Errorf("Failed to sync device_authentication_%d to Redis: %v", tenantId, err)
		}
	}

	return nil
}

// loadACSConfig reads the raw ACS configuration (secrets unmasked).
func (s *SystemSettingsService) loadACSConfig() (*ACSConfig, error) {
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

	return &cfg, nil
}

// GetACSConfig reads the ACS configuration, masking secret fields on return.
func (s *SystemSettingsService) GetACSConfig() (*ACSConfig, error) {
	cfg, err := s.loadACSConfig()
	if err != nil {
		return nil, err
	}
	maskACSSecrets(cfg)
	return cfg, nil
}

// UpdateACSConfig updates the ACS configuration with password encryption.
func (s *SystemSettingsService) UpdateACSConfig(req *UpdateACSConfigRequest) error {
	existing, err := s.loadACSConfig()
	if err != nil {
		return err
	}

	// Overlay request fields. Secret fields are skipped when empty or masked
	// (i.e. treated as "unchanged"), otherwise encrypted before persisting.
	if req.FileServer != nil {
		existing.FileServer = req.FileServer
	}
	if req.NmsIP != nil {
		existing.NmsIP = req.NmsIP
	}
	if req.StunServer != nil {
		existing.StunServer = req.StunServer
	}
	if req.StunPort != nil {
		existing.StunPort = req.StunPort
	}
	if req.StunUsername != nil {
		existing.StunUsername = req.StunUsername
	}
	if req.StunPassword != nil && *req.StunPassword != "" && *req.StunPassword != passwordMask {
		if enc, e := s.encrypt(*req.StunPassword); e == nil {
			existing.StunPassword = &enc
		}
	}
	if req.StunMaximumKeepAlivePeriod != nil {
		existing.StunMaximumKeepAlivePeriod = req.StunMaximumKeepAlivePeriod
	}
	if req.StunMinimumKeepAlivePeriod != nil {
		existing.StunMinimumKeepAlivePeriod = req.StunMinimumKeepAlivePeriod
	}
	if req.UdpConnectionRequestAddressNotificationLimit != nil {
		existing.UdpConnectionRequestAddressNotificationLimit = req.UdpConnectionRequestAddressNotificationLimit
	}
	if req.ConnectionRequestUsername != nil {
		existing.ConnectionRequestUsername = req.ConnectionRequestUsername
	}
	if req.ConnectionRequestPassword != nil && *req.ConnectionRequestPassword != "" && *req.ConnectionRequestPassword != passwordMask {
		if enc, e := s.encrypt(*req.ConnectionRequestPassword); e == nil {
			existing.ConnectionRequestPassword = &enc
		}
	}
	if req.FileServerUsername != nil {
		existing.FileServerUsername = req.FileServerUsername
	}
	if req.FileServerPassword != nil && *req.FileServerPassword != "" && *req.FileServerPassword != passwordMask {
		if enc, e := s.encrypt(*req.FileServerPassword); e == nil {
			existing.FileServerPassword = &enc
		}
	}
	if req.ConnectionRequestPasswordChange != nil {
		existing.ConnectionRequestPasswordChange = req.ConnectionRequestPasswordChange
	}
	if req.FileServerPasswordChange != nil {
		existing.FileServerPasswordChange = req.FileServerPasswordChange
	}
	if req.StunPasswordChange != nil {
		existing.StunPasswordChange = req.StunPasswordChange
	}
	if req.HaAlarmProxyIP != nil {
		existing.HaAlarmProxyIP = req.HaAlarmProxyIP
	}
	if req.ParameterSyncPeriod != nil {
		existing.ParameterSyncPeriod = req.ParameterSyncPeriod
	}
	if req.PasswordEncryption != nil {
		existing.PasswordEncryption = req.PasswordEncryption
	}
	if req.LogUploadPeriod != nil {
		existing.LogUploadPeriod = req.LogUploadPeriod
	}
	if req.Vip != nil {
		existing.Vip = req.Vip
	}

	data, err := json.Marshal(existing)
	if err != nil {
		return apperror.Wrap(err, "MARSHAL_ACS_CONFIG_FAILED", 500, "failed to marshal ACS config")
	}

	if err := s.repo.SaveSystemConfig("nms_config", string(data)); err != nil {
		return err
	}

	if err := redis.Set(context.Background(), "nms_config", string(data), 0); err != nil {
		logger.Errorf("Failed to save ACS config to Redis: %v", err)
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
		return defaultLogConfig(), nil
	}

	var cfg LogConfig
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		return nil, apperror.Wrap(err, "UNMARSHAL_LOG_CONFIG_FAILED", 500, "failed to unmarshal log config")
	}

	return &cfg, nil
}

// UpdateLogConfig updates the log configuration in system_config.
func (s *SystemSettingsService) UpdateLogConfig(req *UpdateLogConfigRequest) error {
	existing, err := s.GetLogConfig()
	if err != nil {
		return err
	}

	if req.PmAndMrSaveTime != nil {
		existing.PmAndMrSaveTime = req.PmAndMrSaveTime
	}
	if req.DeviceLogSaveTime != nil {
		existing.DeviceLogSaveTime = req.DeviceLogSaveTime
	}
	if req.NmsLogSaveTime != nil {
		existing.NmsLogSaveTime = req.NmsLogSaveTime
	}
	if req.AlarmSaveTime != nil {
		existing.AlarmSaveTime = req.AlarmSaveTime
	}
	if req.NorthboundFileSaveTime != nil {
		existing.NorthboundFileSaveTime = req.NorthboundFileSaveTime
	}

	applyLogConfigDefaults(existing)

	data, err := json.Marshal(existing)
	if err != nil {
		return apperror.Wrap(err, "MARSHAL_LOG_CONFIG_FAILED", 500, "failed to marshal log config")
	}

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

// decrypt reverses encrypt (AES-GCM). Used to recover the plaintext
// FileServer password for device-facing Basic auth on /acs-file-server/**.
func (s *SystemSettingsService) decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if len(s.aesKey) == 0 {
		return ciphertext, nil // No encryption if key not configured
	}

	data, err := hex.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.aesKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(data) < aesGCM.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, sealed := data[:aesGCM.NonceSize()], data[aesGCM.NonceSize():]
	plaintext, err := aesGCM.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// GetFileServerCredentials returns the device-facing Basic-auth username and
// (decrypted) password configured via UpdateACSConfig (FileServerUsername/
// FileServerPassword). Mirrors Java's FileServerUsernameAndPasswordHolder,
// which the device uses when pulling files from /acs-file-server/**.
func (s *SystemSettingsService) GetFileServerCredentials() (username, password string, err error) {
	cfg, err := s.GetACSConfig()
	if err != nil {
		return "", "", err
	}
	if cfg.FileServerUsername != nil {
		username = *cfg.FileServerUsername
	}
	if cfg.FileServerPassword != nil && *cfg.FileServerPassword != "" && *cfg.FileServerPassword != passwordMask {
		password, err = s.decrypt(*cfg.FileServerPassword)
		if err != nil {
			return "", "", apperror.Wrap(err, "DECRYPT_FILESERVER_PASSWORD_FAILED", 500, "failed to decrypt file server password")
		}
	}
	return username, password, nil
}

// GetACSConfigRaw returns the ASCConfig without masking secrets. Intended for
// internal consumers (e.g. device-facing Basic auth) that need the real values.
func (s *SystemSettingsService) GetACSConfigRaw() (*ACSConfig, error) {
	return s.loadACSConfig()
}

// ---------- NorthBoundConfig (ABSENT backfill #3) ----------

// GetNorthBoundConfig reads the northbound integration configuration for a tenancy.
// Mirrors Java NorthBoundManagementServiceImpl.getNorthBoundConfig: missing row
// yields a config with only useDefaultOid=false; the stored password (AES-GCM
// encrypted) is decrypted then masked on read, exactly like Java's
// AESGCMUtil.decrypt + LogUtil.maskName.
func (s *SystemSettingsService) GetNorthBoundConfig(tenantId int) (*NorthBoundConfig, error) {
	value, err := s.repo.GetSystemConfig(NorthBoundConfigKey(tenantId))
	if err != nil {
		return nil, err
	}

	if value == "" {
		return &NorthBoundConfig{UseDefaultOid: boolPtr(false)}, nil
	}

	cfg := &NorthBoundConfig{}
	if err := json.Unmarshal([]byte(value), cfg); err != nil {
		return nil, apperror.Wrap(err, "UNMARSHAL_NORTH_BOUND_CONFIG_FAILED", 500, "failed to unmarshal northbound config")
	}

	if cfg.Password != nil && *cfg.Password != "" {
		if plain, e := s.decrypt(*cfg.Password); e == nil {
			cfg.Password = strPtr(passwordMask)
			_ = plain
		} else {
			cfg.Password = strPtr("Failed to decrypt")
		}
	}

	return cfg, nil
}

// UpdateNorthBoundConfig upserts the northbound integration configuration for a
// tenancy. Mirrors Java NorthBoundManagementServiceImpl.updateNorthBoundConfig:
//   - tenantId==0 (Java's null tenancy) clears the connection fields,
//   - useDefaultOid is required (invalid-argument otherwise),
//   - supplied (non-nil) fields are overlaid (null-safe; Java's quirk of
//     null-overwriting the whole block when type is present is intentionally not
//     reproduced — it is a lossy footgun and the rest of this package uses
//     null-safe overlay),
//   - when type==2 the privateKey/fileName fields are accepted,
//   - the password is only re-encrypted when passwordChange is true,
//   - the cached redis key is invalidated.
func (s *SystemSettingsService) UpdateNorthBoundConfig(req *NorthBoundConfig, tenantId int) error {
	if req == nil || req.UseDefaultOid == nil {
		return apperror.New("INVALID_ARGUMENT", 400, "useDefaultOid is required")
	}

	key := NorthBoundConfigKey(tenantId)

	// Java: when tenantId is null, null out the connection fields.
	if tenantId == 0 {
		req.Ip = nil
		req.Port = nil
		req.Username = nil
		req.Password = nil
		req.PasswordChange = nil
		req.PrivateKey = nil
		req.Type = nil
		req.Enable = nil
		req.FileName = nil
	}

	// Load existing to preserve the encrypted password when not being changed.
	existing := &NorthBoundConfig{}
	if val, err := s.repo.GetSystemConfig(key); err == nil && val != "" {
		_ = json.Unmarshal([]byte(val), existing)
	}

	if req.Type != nil {
		if req.Path != nil {
			existing.Path = req.Path
		}
		if req.Ip != nil {
			existing.Ip = req.Ip
		}
		if req.Port != nil {
			existing.Port = req.Port
		}
		if req.Username != nil {
			existing.Username = req.Username
		}
		existing.Type = req.Type
		if req.Enable != nil {
			existing.Enable = req.Enable
		}
		if *req.Type == 2 {
			if req.PrivateKey != nil {
				existing.PrivateKey = req.PrivateKey
			}
			if req.FileName != nil {
				existing.FileName = req.FileName
			}
		}
		if req.PasswordChange != nil && *req.PasswordChange && req.Password != nil && *req.Password != "" && *req.Password != passwordMask {
			if enc, e := s.encrypt(*req.Password); e == nil {
				existing.Password = &enc
			}
		}
	}

	if req.EnterpriseOid != nil {
		existing.EnterpriseOid = req.EnterpriseOid
	}
	if req.UseDefaultOid != nil {
		existing.UseDefaultOid = req.UseDefaultOid
	}

	data, err := json.Marshal(existing)
	if err != nil {
		return apperror.Wrap(err, "MARSHAL_NORTH_BOUND_CONFIG_FAILED", 500, "failed to marshal northbound config")
	}

	if err := s.repo.SaveSystemConfig(key, string(data)); err != nil {
		return err
	}

	if err := redis.Del(context.Background(), key); err != nil {
		logger.Errorf("Failed to invalidate northbound config cache for %s: %v", key, err)
	}

	return nil
}

// ---------- defaults ----------

func defaultDeviceConfig() *DeviceConfig {
	c := &DeviceConfig{}
	applyDeviceConfigDefaults(c)
	return c
}

func applyDeviceConfigDefaults(c *DeviceConfig) {
	if c.DeviceInformPeriod == nil {
		c.DeviceInformPeriod = intPtr(60)
	}
	if c.CpeSignalWeakThreshold == nil {
		c.CpeSignalWeakThreshold = float64Ptr(-105)
	}
	if c.CpeSignalStrongThreshold == nil {
		c.CpeSignalStrongThreshold = float64Ptr(-85)
	}
	if c.AutoRegistrationEnable == nil {
		c.AutoRegistrationEnable = boolPtr(true)
	}
	if c.PmFileSaveTime == nil {
		c.PmFileSaveTime = intPtr(120)
	}
	if c.DeviceLogFileSaveTime == nil {
		c.DeviceLogFileSaveTime = intPtr(120)
	}
	if c.AlarmSaveTime == nil {
		c.AlarmSaveTime = intPtr(120)
	}
	if c.DeviceAuthentication == nil {
		c.DeviceAuthentication = boolPtr(false)
	}
	if c.AuthenticationAlgorithm == nil {
		c.AuthenticationAlgorithm = strPtr("Null")
	}
}

func defaultLogConfig() *LogConfig {
	c := &LogConfig{}
	applyLogConfigDefaults(c)
	return c
}

func applyLogConfigDefaults(c *LogConfig) {
	if c.PmAndMrSaveTime == nil {
		c.PmAndMrSaveTime = intPtr(120)
	}
	if c.DeviceLogSaveTime == nil {
		c.DeviceLogSaveTime = intPtr(120)
	}
	if c.NmsLogSaveTime == nil {
		c.NmsLogSaveTime = intPtr(120)
	}
	if c.AlarmSaveTime == nil {
		c.AlarmSaveTime = intPtr(120)
	}
	if c.NorthboundFileSaveTime == nil {
		c.NorthboundFileSaveTime = intPtr(120)
	}
}

func maskACSSecrets(c *ACSConfig) {
	c.StunPassword = strPtr(passwordMask)
	c.ConnectionRequestPassword = strPtr(passwordMask)
	c.FileServerPassword = strPtr(passwordMask)
}

// ---------- helpers ----------

func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

func float64Ptr(f float64) *float64 {
	return &f
}

func strPtr(s string) *string {
	return &s
}
