package user

import "time"

// SysUser 对应 sys_user 表
type SysUser struct {
	Id                   int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Username             *string    `gorm:"column:username;type:varchar(255)" json:"username"`
	Password             *string    `gorm:"column:password;type:varchar(255)" json:"password"`
	Email                *string    `gorm:"column:email;type:varchar(255)" json:"email"`
	PhoneNumber          *string    `gorm:"column:phone_number;type:varchar(255)" json:"phone_number"`
	RealName             *string    `gorm:"column:real_name;type:varchar(255)" json:"real_name"`
	Status               *int       `gorm:"column:status" json:"status"`
	LicenseId            *int       `gorm:"column:license_id" json:"license_id"`
	CreateTime           *time.Time `gorm:"column:create_time" json:"create_time"`
	LastLoginTime        *time.Time `gorm:"column:last_login_time" json:"last_login_time"`
	LoginErrorTimes      *int       `gorm:"column:login_error_times" json:"login_error_times"`
	LastLockTime         *time.Time `gorm:"column:last_lock_time" json:"last_lock_time"`
	Enable               *bool      `gorm:"column:enable" json:"enable"`
	PasswordModifyTime   *time.Time `gorm:"column:password_modify_time" json:"password_modify_time"`
	Salt                 *string    `gorm:"column:salt;type:varchar(255)" json:"salt"`
	CreateUserId         *int       `gorm:"column:create_user_id" json:"create_user_id"`
}

func (SysUser) TableName() string { return "sys_user" }

// Role 对应 role 表
type Role struct {
	Id          int     `gorm:"primaryKey;autoIncrement" json:"id"`
	RoleName    *string `gorm:"column:role_name;type:varchar(255)" json:"role_name"`
	Description *string `gorm:"column:description;type:varchar(255)" json:"description"`
	LicenseId   *int    `gorm:"column:license_id" json:"license_id"`
}

func (Role) TableName() string { return "role" }

// RoleHasPermission 对应 role_has_permission 表
type RoleHasPermission struct {
	Id           int     `gorm:"primaryKey;autoIncrement" json:"id"`
	RoleId       *int    `gorm:"column:role_id" json:"role_id"`
	PermissionId *string `gorm:"column:permission_id;type:varchar(255)" json:"permission_id"`
}

func (RoleHasPermission) TableName() string { return "role_has_permission" }

// UserHasRole 对应 user_has_role 表
type UserHasRole struct {
	Id     int `gorm:"primaryKey;autoIncrement" json:"id"`
	UserId int `gorm:"column:user_id" json:"user_id"`
	RoleId int `gorm:"column:role_id" json:"role_id"`
}

func (UserHasRole) TableName() string { return "user_has_role" }

// LoginLog 对应 login_log 表
type LoginLog struct {
	Id         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Username   *string    `gorm:"column:username;type:varchar(255)" json:"username"`
	IpAddress  *string    `gorm:"column:ip_address;type:varchar(255)" json:"ip_address"`
	LoginTime  *time.Time `gorm:"column:login_time" json:"login_time"`
	Result     *int       `gorm:"column:result" json:"result"`
	LicenseId  *int       `gorm:"column:license_id" json:"license_id"`
	Type       *int       `gorm:"column:type" json:"type"`
	Info       *string    `gorm:"column:info;type:varchar(500)" json:"info"`
}

func (LoginLog) TableName() string { return "login_log" }

// PasswordHistory 对应 password_history 表
type PasswordHistory struct {
	Id         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Username   *string    `gorm:"column:username;type:varchar(255)" json:"username"`
	Password   *string    `gorm:"column:password;type:varchar(255)" json:"password"`
	CreateTime *time.Time `gorm:"column:create_time" json:"create_time"`
}

func (PasswordHistory) TableName() string { return "password_history" }

// LoginFailedTimesResponse 登录失败次数查询响应
type LoginFailedTimesResponse struct {
	MaxFailedTime int `json:"maxFailedTime"`
	FailedTime    int `json:"failedTime"`
}

// NeedChangePasswordResponse 密码是否需要修改响应
type NeedChangePasswordResponse struct {
	NeedChange bool `json:"needChange"`
	Reason     int  `json:"reason"`
}

// ModifyPasswordRequest 修改密码请求
type ModifyPasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

// ResetPasswordRequest 重置密码请求（管理员）
type ResetPasswordRequest struct {
	UserId int `json:"userId" binding:"required"`
}

// ResetPasswordByLinkRequest 通过链接重置密码请求
type ResetPasswordByLinkRequest struct {
	Username    string `json:"username" binding:"required"`
	Key         string `json:"key" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

// SetTenancyRequest 设置用户租户请求
type SetTenancyRequest struct {
	UserId    int `json:"userId" binding:"required"`
	LicenseId int `json:"licenseId" binding:"required"`
}

// Redis key 常量
const (
	RedisKeyJWTLogin = "security:jwt:login:"
	RedisKeyJWTBlack = "security:jwt:black:"
	RedisKeyIPFailed = "ip_failed_login_times_"
	RedisKeyIPLock   = "ip_lock_login_"
	RedisKeyPwdReset = "change_password_key_"
)

// 登录类型常量
const (
	LoginTypeLogin  = 1
	LoginTypeLogout = 2
)

// 密码过期天数
const PasswordExpiredDays = 180

// 用户不活跃锁定天数
const UserLockedDays = 90

// 用户锁定时长（分钟）
const UserLockMinutes = 30

// 最大登录失败次数（默认）
const DefaultMaxLoginFailedTimes = 5

// IP锁定阈值
const IPLockThreshold = 20

// IP锁定时长（分钟）
const IPLockMinutes = 30

// JWT TTL（分钟）
const JTTTLMintues = 60
