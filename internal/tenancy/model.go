package tenancy

import "time"

// tenancyModel maps to the license table (same table as license module)
type tenancyModel struct {
	Id                   int        `gorm:"primaryKey;autoIncrement"`
	LicenseName          *string    `gorm:"column:license_name;type:varchar(255)"`
	TenantCode           *string    `gorm:"column:tenant_code;type:varchar(255)"`
	LicenseType          *string    `gorm:"column:license_type;type:varchar(255)"`
	ExpiryDate           *time.Time `gorm:"column:expiry_date"`
	EnbQuantity          int        `gorm:"column:enb_quantity"`
	RoleId               *string    `gorm:"column:role_id;type:varchar(32)"`
	UserQuantity         int        `gorm:"column:user_quantity"`
	AcsUrl               *string    `gorm:"column:acs_url;type:varchar(255)"`
	OmcName              *string    `gorm:"column:omc_name;type:varchar(255)"`
	ProvinceAbbreviation *string    `gorm:"column:province_abbreviation;type:varchar(255)"`
	VendorCode           *string    `gorm:"column:vendor_code;type:varchar(255)"`
	Timezone             *string    `gorm:"column:timezone;type:varchar(255)"`
	LogoBase64           *string    `gorm:"column:logo_base64;type:longtext"`
	GnbQuantity          *int       `gorm:"column:gnb_quantity"`
	CpeQuantity          *int       `gorm:"column:cpe_quantity"`
}

func (tenancyModel) TableName() string { return "tenant" }

// AddTenancyRequest represents the request body for adding a tenancy
type AddTenancyRequest struct {
	LicenseName          string `json:"licenseName" binding:"required"`
	ExpiryDate           *int64 `json:"expiryDate" binding:"required"`
	EnbQuantity          int    `json:"enbQuantity"`
	UserQuantity         int    `json:"userQuantity"`
	ProvinceAbbreviation string `json:"provinceAbbreviation"`
	VendorCode           string `json:"vendorCode"`
	OmcName              string `json:"omcName"`
	Timezone             string `json:"timezone"`
	LogoBase64           string `json:"logoBase64"`
	GnbQuantity          *int   `json:"gnbQuantity"`
	CpeQuantity          *int   `json:"cpeQuantity"`
}

// UpdateTenancyRequest represents the request body for updating a tenancy
type UpdateTenancyRequest struct {
	Id                   int    `json:"id" binding:"required"`
	LicenseName          string `json:"licenseName" binding:"required"`
	ExpiryDate           *int64 `json:"expiryDate" binding:"required"`
	EnbQuantity          int    `json:"enbQuantity"`
	UserQuantity         int    `json:"userQuantity"`
	ProvinceAbbreviation string `json:"provinceAbbreviation"`
	VendorCode           string `json:"vendorCode"`
	OmcName              string `json:"omcName"`
	Timezone             string `json:"timezone"`
	LogoBase64           string `json:"logoBase64"`
	GnbQuantity          *int   `json:"gnbQuantity"`
	CpeQuantity          *int   `json:"cpeQuantity"`
}

// ViewTenancyResponse represents the response for viewing a tenancy
type ViewTenancyResponse struct {
	Id                   int     `json:"id"`
	LicenseName          string  `json:"licenseName"`
	TenantCode           string  `json:"tenantCode"`
	ExpiryDate           int64   `json:"expiryDate"`
	EnbQuantity          int     `json:"enbQuantity"`
	UserQuantity         int     `json:"userQuantity"`
	ProvinceAbbreviation string  `json:"provinceAbbreviation"`
	VendorCode           string  `json:"vendorCode"`
	OmcName              string  `json:"omcName"`
	Timezone             string  `json:"timezone"`
	LogoBase64           string  `json:"logoBase64"`
	GnbQuantity          *int    `json:"gnbQuantity"`
	CpeQuantity          *int    `json:"cpeQuantity"`
}

// ListTenancyQuery represents the query parameters for listing tenancies
type ListTenancyQuery struct {
	LicenseName string `json:"licenseName"`
	Page        int    `json:"page"`
	PageSize    int    `json:"pageSize"`
}

// TenancyVO represents a tenancy in the list response
type TenancyVO struct {
	Id                   int     `json:"id"`
	LicenseName          string  `json:"licenseName"`
	TenantCode           string  `json:"tenantCode"`
	ExpiryDate           int64   `json:"expiryDate"`
	EnbQuantity          int     `json:"enbQuantity"`
	UserQuantity         int     `json:"userQuantity"`
	ProvinceAbbreviation string  `json:"provinceAbbreviation"`
	VendorCode           string  `json:"vendorCode"`
	OmcName              string  `json:"omcName"`
	Timezone             string  `json:"timezone"`
	LogoBase64           string  `json:"logoBase64"`
	GnbQuantity          *int    `json:"gnbQuantity"`
	CpeQuantity          *int    `json:"cpeQuantity"`
}

// DeleteTenancyRequest represents the request body for deleting a tenancy
type DeleteTenancyRequest struct {
	Id int `json:"id" binding:"required"`
}
