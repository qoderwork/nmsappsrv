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
	RoleId               *string    `gorm:"column:role_id;type:varchar(32)" json:"role_id"`
	UserQuantity         int        `gorm:"column:user_quantity" json:"user_quantity"`
	AcsUrl               *string    `gorm:"column:acs_url;type:varchar(255)" json:"acs_url"`
	OmcName              *string    `gorm:"column:omc_name;type:varchar(255)" json:"omc_name"`
	ProvinceAbbreviation *string    `gorm:"column:province_abbreviation;type:varchar(255)" json:"province_abbreviation"`
	VendorCode           *string    `gorm:"column:vendor_code;type:varchar(255)" json:"vendor_code"`
	Timezone             *string    `gorm:"column:timezone;type:varchar(255)" json:"timezone"`
	LogoBase64           *string    `gorm:"column:logo_base64;type:longtext" json:"logo_base64"`
	GnbQuantity          *int       `gorm:"column:gnb_quantity" json:"gnb_quantity"`
	CpeQuantity          *int       `gorm:"column:cpe_quantity" json:"cpe_quantity"`

	// --- L-2 signed-license enforcement fields (go-infra/licensing) ---
	// Slot marks the canonical enforced license row ("active"). Unique so there
	// is exactly one active license. Null for the legacy Java-compatible rows.
	Slot *string `gorm:"column:slot;type:varchar(32);uniqueIndex" json:"-"`
	// Subject/Issuer are copied from the verified signed license for display.
	Subject *string `gorm:"column:subject;type:varchar(255)" json:"subject,omitempty"`
	Issuer  *string `gorm:"column:issuer;type:varchar(255)" json:"issuer,omitempty"`
	// Features/Capacity are JSON-encoded slices/maps from the signed license.
	Features *string `gorm:"column:features;type:longtext" json:"features,omitempty"`
	Capacity *string `gorm:"column:capacity;type:longtext" json:"capacity,omitempty"`
	// NotBefore is the signed license validity start (may be nil).
	NotBefore *time.Time `gorm:"column:not_before" json:"not_before,omitempty"`
	// MachineFingerprint is the host binding the license was issued for.
	MachineFingerprint *string `gorm:"column:machine_fingerprint;type:varchar(255);index" json:"machine_fingerprint,omitempty"`
	// Status is the enforcement state: active | expired | invalid | missing.
	Status *string `gorm:"column:status;type:varchar(32)" json:"status,omitempty"`
	// VerifiedAt is when the active license was last verified & activated.
	VerifiedAt *time.Time `gorm:"column:verified_at" json:"verified_at,omitempty"`
	// LicenseFilePath is the on-disk path of the verified envelope (informational).
	LicenseFilePath *string `gorm:"column:license_file_path;type:varchar(512)" json:"license_file_path,omitempty"`
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

// SASConfig 对应 sas_config 表 (Java: SASConfig entity, JPA default naming).
// Java entity has only id/licenseId/autoRegister — CBSD connection params
// (sasUrl/certPath/etc) live in a separate Go table (cbsd_sas_config).
type SASConfig struct {
	Id           int   `gorm:"primaryKey;autoIncrement" json:"id"`
	LicenseId    *int  `gorm:"column:license_id;uniqueIndex" json:"license_id"`
	AutoRegister *bool `gorm:"column:auto_register" json:"auto_register"`
}

func (SASConfig) TableName() string { return "sas_config" }

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
