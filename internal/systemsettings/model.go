package systemsettings

// DeviceConfig represents the device configuration stored per-tenancy in system_config
// (key = "device_config_<tenancyId>"). Field set matches Java UpdateDeviceSettingsDTO
// (systemsettings P0 S-2).
type DeviceConfig struct {
	DeviceInformPeriod       *int     `json:"deviceInformPeriod"`
	CpeSignalWeakThreshold   *float64 `json:"cpeSignalWeakThreshold"`
	CpeSignalStrongThreshold *float64 `json:"cpeSignalStrongThreshold"`
	AutoRegistrationEnable   *bool    `json:"autoRegistrationEnable"`
	PmFileSaveTime           *int     `json:"pmFileSaveTime"`
	DeviceLogFileSaveTime    *int     `json:"deviceLogFileSaveTime"`
	AlarmSaveTime            *int     `json:"alarmSaveTime"`
	DeviceAuthentication     *bool    `json:"deviceAuthentication"`
	AuthenticationAlgorithm  *string  `json:"authenticationAlgorithm"`
	AcsUsername              *string  `json:"acsUsername"`
	AcsPassword              *string  `json:"acsPassword"`
	// MaxDeviceCount is a Go-only extension (Java has no such field) used by
	// tr069 checkDeviceLimit to cap device creation. Kept for that feature.
	MaxDeviceCount *int `json:"maxDeviceCount"`
}

// ACSConfig represents the NMS ACS configuration stored in system_config as key "nms_config".
// Field set matches Java UpdateACSSettingsDTO (systemsettings P0 S-3).
type ACSConfig struct {
	FileServer                             *string `json:"fileServer"`
	NmsIP                                  *string `json:"nmsIP"`
	StunServer                             *string `json:"stunServer"`
	StunPort                               *int    `json:"stunPort"`
	StunUsername                           *string `json:"stunUsername"`
	StunPassword                           *string `json:"stunPassword"`
	StunMaximumKeepAlivePeriod             *int    `json:"stunMaximumKeepAlivePeriod"`
	StunMinimumKeepAlivePeriod             *int    `json:"stunMinimumKeepAlivePeriod"`
	UdpConnectionRequestAddressNotificationLimit *int `json:"udpConnectionRequestAddressNotificationLimit"`
	ConnectionRequestUsername              *string `json:"connectionRequestUsername"`
	ConnectionRequestPassword              *string `json:"connectionRequestPassword"`
	FileServerUsername                     *string `json:"fileServerUsername"`
	FileServerPassword                     *string `json:"fileServerPassword"`
	ConnectionRequestPasswordChange        *bool   `json:"connectionRequestPasswordChange"`
	FileServerPasswordChange               *bool   `json:"fileServerPasswordChange"`
	StunPasswordChange                     *bool   `json:"stunPasswordChange"`
	HaAlarmProxyIP                         *string `json:"haAlarmProxyIP"`
	ParameterSyncPeriod                    *int    `json:"parameterSyncPeriod"`
	PasswordEncryption                     *bool   `json:"passwordEncryption"`
	LogUploadPeriod                        *int    `json:"logUploadPeriod"`
	Vip                                    *string `json:"vip"`
}

// LogConfig represents the NMS log configuration stored in system_config as key "nms_log_config".
// Field set matches Java UpdateLogSettingsDTO (systemsettings P0 S-4).
type LogConfig struct {
	PmAndMrSaveTime        *int `json:"pmAndMrSaveTime"`
	DeviceLogSaveTime      *int `json:"deviceLogSaveTime"`
	NmsLogSaveTime         *int `json:"nmsLogSaveTime"`
	AlarmSaveTime          *int `json:"alarmSaveTime"`
	NorthboundFileSaveTime *int `json:"northboundFileSaveTime"`
}

// SysParameter represents a row from sys_parameter table (key-value system params).
type SysParameter struct {
	Id    int     `gorm:"primaryKey;autoIncrement" json:"id"`
	Key   *string `gorm:"column:config_key;type:varchar(255);uniqueIndex" json:"key"`
	Value *string `gorm:"column:config_value;type:longtext" json:"value"`
}

func (SysParameter) TableName() string { return "sys_parameter" }

// UpdateDeviceSettingsRequest represents the request to update device settings.
type UpdateDeviceSettingsRequest struct {
	DeviceInformPeriod       *int     `json:"deviceInformPeriod"`
	CpeSignalWeakThreshold   *float64 `json:"cpeSignalWeakThreshold"`
	CpeSignalStrongThreshold *float64 `json:"cpeSignalStrongThreshold"`
	AutoRegistrationEnable   *bool    `json:"autoRegistrationEnable"`
	PmFileSaveTime           *int     `json:"pmFileSaveTime"`
	DeviceLogFileSaveTime    *int     `json:"deviceLogFileSaveTime"`
	AlarmSaveTime            *int     `json:"alarmSaveTime"`
	DeviceAuthentication     *bool    `json:"deviceAuthentication"`
	AuthenticationAlgorithm  *string  `json:"authenticationAlgorithm"`
	AcsUsername              *string  `json:"acsUsername"`
	AcsPassword              *string  `json:"acsPassword"`
	MaxDeviceCount           *int     `json:"maxDeviceCount"`
}

// UpdateACSConfigRequest represents the request to update ACS configuration.
type UpdateACSConfigRequest struct {
	FileServer                             *string `json:"fileServer"`
	NmsIP                                  *string `json:"nmsIP"`
	StunServer                             *string `json:"stunServer"`
	StunPort                               *int    `json:"stunPort"`
	StunUsername                           *string `json:"stunUsername"`
	StunPassword                           *string `json:"stunPassword"`
	StunMaximumKeepAlivePeriod             *int    `json:"stunMaximumKeepAlivePeriod"`
	StunMinimumKeepAlivePeriod             *int    `json:"stunMinimumKeepAlivePeriod"`
	UdpConnectionRequestAddressNotificationLimit *int `json:"udpConnectionRequestAddressNotificationLimit"`
	ConnectionRequestUsername              *string `json:"connectionRequestUsername"`
	ConnectionRequestPassword              *string `json:"connectionRequestPassword"`
	FileServerUsername                     *string `json:"fileServerUsername"`
	FileServerPassword                     *string `json:"fileServerPassword"`
	ConnectionRequestPasswordChange        *bool   `json:"connectionRequestPasswordChange"`
	FileServerPasswordChange               *bool   `json:"fileServerPasswordChange"`
	StunPasswordChange                     *bool   `json:"stunPasswordChange"`
	HaAlarmProxyIP                         *string `json:"haAlarmProxyIP"`
	ParameterSyncPeriod                    *int    `json:"parameterSyncPeriod"`
	PasswordEncryption                     *bool   `json:"passwordEncryption"`
	LogUploadPeriod                        *int    `json:"logUploadPeriod"`
	Vip                                    *string `json:"vip"`
}

// UpdateLogConfigRequest represents the request to update log configuration.
type UpdateLogConfigRequest struct {
	PmAndMrSaveTime        *int `json:"pmAndMrSaveTime"`
	DeviceLogSaveTime      *int `json:"deviceLogSaveTime"`
	NmsLogSaveTime         *int `json:"nmsLogSaveTime"`
	AlarmSaveTime          *int `json:"alarmSaveTime"`
	NorthboundFileSaveTime *int `json:"northboundFileSaveTime"`
}
