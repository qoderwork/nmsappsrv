package mail

// SystemConfig mirrors system_config table for key-value config storage.
type SystemConfig struct {
	Id     string  `gorm:"primaryKey;column:id;type:varchar(255)" json:"id"`
	Config *string `gorm:"column:config;type:longtext" json:"config"`
}

func (SystemConfig) TableName() string { return "system_config" }

// MailConfig is the JSON payload stored in system_config (key="mail").
type MailConfig struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	MailAuthentication bool   `json:"mailAuthentication"`
	SuperUserEmail     string `json:"superUserEmail"`
}

// UpdateMailConfigRequest is the JSON body for POST /updateMailConfig.
type UpdateMailConfigRequest struct {
	Host               string `json:"host" binding:"required"`
	Port               int    `json:"port" binding:"required"`
	Username           string `json:"username" binding:"required"`
	Password           string `json:"password"`
	MailAuthentication bool   `json:"mailAuthentication"`
	SuperUserEmail     string `json:"superUserEmail"`
}

// GetEmailCodeRequest is the JSON body for POST /getEmailCode.
type GetEmailCodeRequest struct {
	Username  string `json:"username" binding:"required"`
	GrantType string `json:"grantType"`
}

// CheckEmailCodeRequest is the JSON body for POST /checkEmailCode.
type CheckEmailCodeRequest struct {
	EmailCode string `json:"emailCode" binding:"required"`
}

// IsEnabledResponse is the response for POST /isEnabledEmailAuthentication.
type IsEnabledResponse struct {
	Enabled bool `json:"enabled"`
}

// EmailCodeResponse is the response for POST /getEmailCode.
type EmailCodeResponse struct {
	Sent bool `json:"sent"`
}
