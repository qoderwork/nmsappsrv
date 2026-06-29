package license

import "time"

// License 对应 license 表 (Java: Tenancy / License)
type License struct {
	Id                   int        `gorm:"primaryKey;autoIncrement" json:"id"`
	LicenseName          *string    `gorm:"column:license_name;type:varchar(255)" json:"license_name"`
	LicenseId            *string    `gorm:"column:license_id;type:varchar(255)" json:"license_id"`
	LicenseType          *string    `gorm:"column:license_type;type:varchar(255)" json:"license_type"`
	ExpiryDate           *time.Time `gorm:"column:expiry_date" json:"expiry_date"`
	EnbQuantity          int        `gorm:"column:enb_quantity" json:"enb_quantity"`
	RoleId               *int       `gorm:"column:role_id" json:"role_id"`
	UserQuantity         int        `gorm:"column:user_quantity" json:"user_quantity"`
	AcsUrl               *string    `gorm:"column:acs_url;type:varchar(255)" json:"acs_url"`
	OmcName              *string    `gorm:"column:omc_name;type:varchar(255)" json:"omc_name"`
	ProvinceAbbreviation *string    `gorm:"column:province_abbreviation;type:varchar(255)" json:"province_abbreviation"`
	VendorCode           *string    `gorm:"column:vendor_code;type:varchar(255)" json:"vendor_code"`
	Timezone             *string    `gorm:"column:timezone;type:varchar(255)" json:"timezone"`
	LogoBase64           *string    `gorm:"column:logo_base64;type:longtext" json:"logo_base64"`
	GnbQuantity          *int       `gorm:"column:gnb_quantity" json:"gnb_quantity"`
	CpeQuantity          *int       `gorm:"column:cpe_quantity" json:"cpe_quantity"`
}

func (License) TableName() string { return "license" }

// BaseStationLicense 对应 base_station_license 表 (UUID主键)
type BaseStationLicense struct {
	Id               string     `gorm:"primaryKey;type:varchar(32)" json:"id"`
	ElementId        *int64     `gorm:"column:element_id;uniqueIndex" json:"element_id"`
	FileName         *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	OriginalFileName *string    `gorm:"column:original_file_name;type:varchar(255)" json:"original_file_name"`
	UploadTime       *time.Time `gorm:"column:upload_time" json:"upload_time"`
}

func (BaseStationLicense) TableName() string { return "base_station_license" }

// SASConfig 对应 sasconfig 表
type SASConfig struct {
	Id           int  `gorm:"primaryKey;autoIncrement" json:"id"`
	LicenseId    *int `gorm:"column:license_id;uniqueIndex" json:"license_id"`
	AutoRegister *bool `gorm:"column:auto_register" json:"auto_register"`
}

func (SASConfig) TableName() string { return "sasconfig" }

// EntraEndpoint 对应 entra_endpoint 表 (UUID主键)
type EntraEndpoint struct {
	Id             string  `gorm:"primaryKey;type:varchar(32)" json:"id"`
	TenancyId      *string `gorm:"column:tenancy_id;type:varchar(255)" json:"tenancy_id"`
	ClientId       *string `gorm:"column:client_id;type:varchar(255)" json:"client_id"`
	SecretKey      *string `gorm:"column:secret_key;type:varchar(255)" json:"secret_key"`
	TenancyIdInNMS *int    `gorm:"column:tenancy_id_in_nms" json:"tenancy_id_in_nms"`
	EndpointName   *string `gorm:"column:endpoint_name;type:varchar(255);uniqueIndex" json:"endpoint_name"`
	NmsFqdn        *string `gorm:"column:nms_fqdn;type:varchar(255)" json:"nms_fqdn"`
}

func (EntraEndpoint) TableName() string { return "entra_endpoint" }
