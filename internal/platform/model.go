package platform

// systemConfigModel maps to the system_config table (shared table, local definition)
type systemConfigModel struct {
	Id     string  `gorm:"primaryKey;column:id;type:varchar(255)"`
	Config *string `gorm:"column:config;type:longtext"`
}

func (systemConfigModel) TableName() string { return "system_config" }

// LogConfig represents the platform log configuration (SystemConfig key: platform_log_config)
type LogConfig struct {
	Level string `json:"level"`
}

// FTPTransferLogConfig represents the SFTP log transfer configuration (SystemConfig key: log_ftp_transfer_config)
type FTPTransferLogConfig struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	Host           string `json:"host"`
	Port           *int   `json:"port"`
	UploadPath     string `json:"uploadPath"`
	PasswordChange *bool  `json:"passwordChange"`
}

// HECConfig represents the HEC (HTTP Event Collector) configuration (SystemConfig key: hec_config)
type HECConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
	Mode  string `json:"mode"`
}

// NMSSecret represents NMS secret settings (SystemConfig key: nms_secret)
type NMSSecret struct {
	EmailSecret    string `json:"emailSecret"`
	AddressSecret  string `json:"addressSecret"`
	PasswordSecret string `json:"passwordSecret"`
}

const (
	defaultEmailSecret    = "waveoss1waveoss1waveoss1waveamil"
	defaultAddressSecret  = "waveoss1waveoss1waveoss1waddress"
	defaultPasswordSecret = "waveoss1waveoss1waveoss1pwdsec"
)
