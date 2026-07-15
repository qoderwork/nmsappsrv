package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"nmsappsrv/pkg/logger"
)

type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	DB            DatabaseConfig      `mapstructure:"database"`
	Redis         RedisConfig         `mapstructure:"redis"`
	JWT           JWTConfig           `mapstructure:"jwt"`
	Logger        LoggerConfig        `mapstructure:"logger"`
	TR069         TR069Config         `mapstructure:"tr069"`
	SNMP          SNMPConfig          `mapstructure:"snmp"`
	MQ            MQConfig            `mapstructure:"mq"`
	Mail          MailConfig          `mapstructure:"mail"`
	STUN          STUNConfig          `mapstructure:"stun"`
	HA            HAConfig            `mapstructure:"ha"`
	PlatformFiles PlatformFilesConfig `mapstructure:"platform_files"`
	Heartbeat     HeartbeatConfig     `mapstructure:"heartbeat"`
	Upgrade       UpgradeConfig       `mapstructure:"upgrade"`
	License       LicenseConfig       `mapstructure:"license"`
	Captcha       CaptchaConfig       `mapstructure:"captcha"`
	FileServer    FileServerConfig    `mapstructure:"file_server"`
	ZTP           ZTPConfig           `mapstructure:"ztp"`
}

// LicenseConfig controls L-2 license enforcement (go-infra/licensing).
//
//	required                   gate all authenticated endpoints unless disabled
//	                          (public build or runtime override). Default true.
//	public_key_path           override the embedded default public key (PKIX PEM).
//	                          Empty => embedded default is used.
//	install_dir               where the active, verified license envelope is
//	                          persisted on disk (survives restarts).
//	max_clock_file            persisted "max time seen" (unix seconds) for the
//	                          anti-rollback check. Empty => <install_dir>/.maxclock.
//	machine_fingerprint_override  force the verifying fingerprint (dev/test only).
//	                          Leave empty in production so the real host
//	                          system-uuid (dmidecode -s system-uuid) is used.
type LicenseConfig struct {
	Required                   bool   `mapstructure:"required"`
	PublicKeyPath              string `mapstructure:"public_key_path"`
	InstallDir                 string `mapstructure:"install_dir"`
	MaxClockFile               string `mapstructure:"max_clock_file"`
	MachineFingerprintOverride string `mapstructure:"machine_fingerprint_override"`
}

// CaptchaConfig controls the login captcha (adaptive risk-control).
//
//	length  number of digits shown in the captcha image (Java\'s kaptchaLength).
//	        Defaults to 4 when unset.
type CaptchaConfig struct {
	Length int `mapstructure:"length"`
}

// FileServerConfig holds the on-disk layout for the device-facing
// /acs-file-server/** endpoints (Java's @Value paths: filePath/configFilePath/
// batchProcessFilePath/logFilePath/mrFilePath/captureFilePath/mmlExecuteResultFilePath/
// pmFilePath/nrmFilePath/deviceMNormalFilePath/cmFilePath/piecemealFileTempPath/
// MRCCSExportFile). nmsappsrv self-serves these files (Option 1 in the ABSENT
// backfill plan), so the same roots live here instead of Spring properties.
type FileServerConfig struct {
	Root             string `mapstructure:"root"`               // base dir for all acs-file-server files
	UpgradeDir       string `mapstructure:"upgrade_dir"`        // filePath (upgrade packages)
	ConfigDir        string `mapstructure:"config_dir"`         // configFilePath (per tenancyId/elementId)
	BatchProcessDir  string `mapstructure:"batch_process_dir"`  // batchProcessFilePath (bpf + ztp templates)
	LogDir           string `mapstructure:"log_dir"`            // logFilePath (device logs)
	MrDir            string `mapstructure:"mr_dir"`             // mrFilePath
	CaptureDir       string `mapstructure:"capture_dir"`        // captureFilePath
	MmlResultDir     string `mapstructure:"mml_result_dir"`     // mmlExecuteResultFilePath
	PmDir            string `mapstructure:"pm_dir"`             // pmFilePath
	NrmDir           string `mapstructure:"nrm_dir"`            // nrmFilePath
	DeviceMNormalDir string `mapstructure:"device_mnormal_dir"` // deviceMNormalFilePath (by DB fileId)
	CmFilePath       string `mapstructure:"cm_file_path"`       // cmFilePath (north-file / ztp)
	PiecemealTempDir string `mapstructure:"piecemeal_temp_dir"` // piecemealFileTempPath
	MrCCSExportDir   string `mapstructure:"mr_ccs_export_dir"`  // MRCCSExportFile (MR CSV reports)
	// Download roots for the device-facing /acs-file-server/** download
	// providers wired in the FileDownload backfill phase.
	CaDir      string `mapstructure:"ca_dir"`      // caFilePath (Java ca file root)
	LicenseDir string `mapstructure:"license_dir"` // licenseFilePath (base_station_license root)
	ZtpDir     string `mapstructure:"ztp_dir"`     // aos/ztp file root (AOSManagementServiceImpl.path)
}

// HAConfig holds High Availability configuration for VIP monitoring.
type HAConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	VIPMonitorInterval int    `mapstructure:"vip_monitor_interval"` // seconds
	CurrentVIP         string `mapstructure:"current_vip"`
}

type ServerConfig struct {
	Name               string   `mapstructure:"name"`
	Port               int      `mapstructure:"port"`
	Mode               string   `mapstructure:"mode"`
	CORSAllowedOrigins []string `mapstructure:"cors_allowed_origins"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	Charset      string `mapstructure:"charset"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	LogLevel     string `mapstructure:"log_level"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// JWTConfig holds the JWT signing key. The secret must be >=32 bytes.
// In production, inject via environment variable NMS_JWT_SECRET.
type JWTConfig struct {
	Secret string `mapstructure:"secret"`
}

type LoggerConfig struct {
	Filename      string `mapstructure:"filename"`
	Level         string `mapstructure:"level"`
	MaxSizeMB     int    `mapstructure:"max_size_mb"`
	MaxBackups    int    `mapstructure:"max_backups"`
	RetentionDays int    `mapstructure:"retention_days"`
	Compress      bool   `mapstructure:"compress"`
	Stdout        bool   `mapstructure:"stdout"`
}

type TR069Config struct {
	ACSUrl                   string `mapstructure:"acs_url"`
	InformInterval           int    `mapstructure:"inform_interval"`
	ConnectionTimeout        int    `mapstructure:"connection_timeout"`
	UDPConnectionRequestPort int    `mapstructure:"udp_connection_request_port"`
	FileServerIp             string `mapstructure:"file_server_ip"`
	EnableAskReboot          bool   `mapstructure:"enable_ask_reboot" yaml:"enable_ask_reboot"`
	EnableXMLSignature       bool   `mapstructure:"enable_xml_signature" yaml:"enable_xml_signature"`
	PrivateKeyPath           string `mapstructure:"private_key_path" yaml:"private_key_path"`
	CertificatePath          string `mapstructure:"certificate_path" yaml:"certificate_path"`
}

type SNMPConfig struct {
	TrapListenPort   int    `mapstructure:"trap_listen_port"`
	EnterpriseOID    string `mapstructure:"enterprise_oid"`
	DefaultVersion   string `mapstructure:"default_version"`
	DefaultCommunity string `mapstructure:"default_community"`
}

type MQConfig struct {
	OperationQueue   string `mapstructure:"operation_queue"`
	InformQueue      string `mapstructure:"inform_queue"`
	EventResultQueue string `mapstructure:"event_result_queue"`
	AlarmQueue       string `mapstructure:"alarm_queue"`
	SNMPQueue        string `mapstructure:"snmp_queue"`
	WebCallbackQueue string `mapstructure:"web_callback_queue"`
	PMQueue          string `mapstructure:"pm_queue"`
}

type MailConfig struct {
	AESKey string `mapstructure:"aes_key"` // hex-encoded 32-byte AES-256 key
}

type STUNConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Port    int  `mapstructure:"port"`
}

// HeartbeatConfig holds SAS/CBSD heartbeat protocol settings.
type HeartbeatConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	IntervalSeconds int    `mapstructure:"interval_seconds"`
	SASEndpoint     string `mapstructure:"sas_endpoint"`
}

// UpgradeConfig holds firmware-upgrade file storage settings.
type UpgradeConfig struct {
	// UploadDir is the local directory where uploaded upgrade files are persisted.
	// The absolute path is stored in the upgrade_file.file_path column and served
	// to devices via the file-server endpoint.
	UploadDir string `mapstructure:"upload_dir" yaml:"upload_dir"`
}

// PlatformFilesConfig holds configurable paths for platform file downloads
type PlatformFilesConfig struct {
	RSAPublicKeyPath string `mapstructure:"rsa_public_key_path" yaml:"rsa_public_key_path"`
	NMSManualDocPath string `mapstructure:"nms_manual_doc_path" yaml:"nms_manual_doc_path"`
	PlatformLogDir   string `mapstructure:"platform_log_dir" yaml:"platform_log_dir"`
}

// ZTPConfig controls the ZTP provisioning subsystem, including the embedded
// SFTP server (Java ZTPSftpServer on port 10022).
//
//	SFTPEnabled    start the embedded SFTP server that serves the AOS XML
//	               root (cfg.FileServer.ZtpDir) to ZTP-capable devices.
//	               Default false (opt-in; the HTTP /acs-file-server/ztpFile
//	               provider in internal/filebase remains the default path).
//	SFTPHost       listen address. Default ":10022" (matches Java).
//	SFTPHostKey    path to the persistent SSH host key (PEM). If empty, an
//	               Ed25519 key is auto-generated and persisted here on first
//	               start, so devices see a stable host key across restarts.
type ZTPConfig struct {
	SFTPEnabled bool   `mapstructure:"sftp_enabled" yaml:"sftp_enabled"`
	SFTPHost    string `mapstructure:"sftp_host"     yaml:"sftp_host"`
	SFTPHostKey string `mapstructure:"sftp_host_key" yaml:"sftp_host_key"`
}

var Cfg *Config

func Load(configPath ...string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	if len(configPath) > 0 {
		v.AddConfigPath(configPath[0])
	}
	v.AddConfigPath("./configs")
	v.AddConfigPath(".")

	// 环境变量覆盖
	v.SetEnvPrefix("NMS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Defaults (overridable via config file or NMS_* env vars).
	v.SetDefault("license.required", true)
	v.SetDefault("captcha.length", 4)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	Cfg = &Config{}
	if err := v.Unmarshal(Cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Apply defaults for platform file paths if not set
	if Cfg.PlatformFiles.RSAPublicKeyPath == "" {
		Cfg.PlatformFiles.RSAPublicKeyPath = "./cert/password/publicKey.pem"
	}
	if Cfg.PlatformFiles.NMSManualDocPath == "" {
		Cfg.PlatformFiles.NMSManualDocPath = "./docs/nms_manual.pdf"
	}
	if Cfg.PlatformFiles.PlatformLogDir == "" {
		Cfg.PlatformFiles.PlatformLogDir = "./logs/platform"
	}

	// Defaults for license enforcement storage.
	if Cfg.License.InstallDir == "" {
		Cfg.License.InstallDir = "./data/license"
	}
	if Cfg.License.MaxClockFile == "" {
		Cfg.License.MaxClockFile = filepath.Join(Cfg.License.InstallDir, ".maxclock")
	}

	// Validate required fields
	if err := Cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// 初始化日志
	logger.Init(logger.Config{
		Filename:      Cfg.Logger.Filename,
		Level:         Cfg.Logger.Level,
		MaxSizeMB:     Cfg.Logger.MaxSizeMB,
		MaxBackups:    Cfg.Logger.MaxBackups,
		RetentionDays: Cfg.Logger.RetentionDays,
		Compress:      Cfg.Logger.Compress,
		Stdout:        Cfg.Logger.Stdout,
	})

	logger.Infof("config loaded: server=%s, port=%d", Cfg.Server.Name, Cfg.Server.Port)
	return Cfg, nil
}

// GetSNMPEnterpriseOID returns the configured SNMP enterprise OID string.
// Returns empty string if config is not loaded or the field is not set.
func GetSNMPEnterpriseOID() string {
	if Cfg == nil {
		return ""
	}
	return Cfg.SNMP.EnterpriseOID
}

// Validate checks that all required configuration fields are set.
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port)
	}
	if c.DB.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if c.DB.Port <= 0 {
		return fmt.Errorf("database.port is required")
	}
	if c.DB.User == "" {
		return fmt.Errorf("database.user is required")
	}
	if c.DB.DBName == "" {
		return fmt.Errorf("database.dbname is required")
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret is required (set via config or NMS_JWT_SECRET env var)")
	}
	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("jwt.secret must be at least 32 bytes, got %d", len(c.JWT.Secret))
	}
	if c.Mail.AESKey != "" && len(c.Mail.AESKey) != 64 {
		return fmt.Errorf("mail.aes_key must be 64 hex characters (32 bytes), got %d", len(c.Mail.AESKey))
	}
	return nil
}
