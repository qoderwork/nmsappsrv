package site

import "time"

// SiteInfo 对应 site_info 表
type SiteInfo struct {
	Id           string     `gorm:"primaryKey;type:varchar(32)" json:"id"`
	SiteName     *string    `gorm:"column:site_name;type:varchar(255);uniqueIndex:idx_license_site" json:"site_name"`
	Description  *string    `gorm:"column:description;type:varchar(255)" json:"description"`
	AreaId       *int       `gorm:"column:area_id" json:"area_id"`
	TenantId    *int       `gorm:"column:tenant_id;uniqueIndex:idx_license_site" json:"tenant_id"`
	Latitude     *string    `gorm:"column:latitude;type:varchar(255)" json:"latitude"`
	Longitude    *string    `gorm:"column:longitude;type:varchar(255)" json:"longitude"`
	CreationTime *time.Time `gorm:"column:creation_time" json:"creation_time"`
}

func (SiteInfo) TableName() string { return "site_info" }

// SiteInfoVo 站点列表响应（包含区域路径）
type SiteInfoVo struct {
	SiteInfo
	AreaPath string `json:"area_path"`
}

// SiteBasicInfo 站点下拉框简要信息
type SiteBasicInfo struct {
	Id       string  `json:"id"`
	SiteName *string `json:"site_name"`
}

// SysArea 对应 sys_area 表
type SysArea struct {
	Id           int     `gorm:"primaryKey;autoIncrement" json:"id"`
	PId          *int    `gorm:"column:p_id" json:"p_id"`
	AreaName     *string `gorm:"column:area_name;type:varchar(255)" json:"area_name"`
	Sort         *string `gorm:"column:sort;type:varchar(255)" json:"sort"`
	Level        *int    `gorm:"column:level" json:"level"`
	Abbreviation *string `gorm:"column:abbreviation;type:varchar(255)" json:"abbreviation"`
	Code         *string `gorm:"column:code;type:varchar(255)" json:"code"`
	TenantId    *int    `gorm:"column:tenant_id" json:"tenant_id"`
}

func (SysArea) TableName() string { return "sys_area" }

// SystemConfig 对应 system_config 表
type SystemConfig struct {
	Id     string  `gorm:"primaryKey;type:varchar(32)" json:"id"`
	Config *string `gorm:"column:config;type:longtext" json:"config"`
}

func (SystemConfig) TableName() string { return "system_config" }

// SystemParameter 对应 sys_parameter 表
type SystemParameter struct {
	Id          int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	ParKeyname  *string `gorm:"column:par_keyname;type:varchar(255)" json:"par_keyname"`
	ParKeyvalue *string `gorm:"column:par_keyvalue;type:varchar(255)" json:"par_keyvalue"`
}

func (SystemParameter) TableName() string { return "sys_parameter" }
