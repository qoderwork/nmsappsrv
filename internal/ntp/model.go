package ntp

// SystemConfig mirrors system_config table for key-value config storage.
type SystemConfig struct {
	Id     string  `gorm:"primaryKey;column:id;type:varchar(255)" json:"id"`
	Config *string `gorm:"column:config;type:longtext" json:"config"`
}

func (SystemConfig) TableName() string { return "system_config" }

// NTPConfig is the JSON payload stored in system_config (key="ntp").
type NTPConfig struct {
	NTPServer string `json:"ntpServer"`
	Enable    bool   `json:"enable"`
}

// NTPConfigRequest is the JSON body for POST /updateNTPConfig.
type NTPConfigRequest struct {
	NTPServer string `json:"ntpServer"`
	Enable    bool   `json:"enable"`
}

// NTPStatusResponse is the response for POST /getNTPStatus.
type NTPStatusResponse struct {
	Status string `json:"status"`
}
