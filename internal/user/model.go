package user

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"gorm.io/gorm"
)

// SysUser 对应 sys_user 表（Java com.waveoss.core.dao.entity.SysUser 1:1 字段对齐）
//
// 类型对齐规则（重要！否则 DB 迁移和 JDBC 映射不一致）：
//   - Java int 原语 → Go int, NOT NULL 默认 0（如 loginErrorTimes, loginErrorTimes primitive）
//   - java.lang.Integer → Go *int（可空）
//   - java.lang.String  firstLogin → Go *string（Java 存 "Y"/"N" 语义，非 TINYINT boolean）
//   - java.util.Date → Go *time.Time
type SysUser struct {
	Id                 int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Username           *string    `gorm:"column:username;type:varchar(255)" json:"username"`
	Password           *string    `gorm:"column:password;type:varchar(255)" json:"password"`
	Email              *string    `gorm:"column:email;type:varchar(255)" json:"email"`
	PhoneNumber        *string    `gorm:"column:phone_number;type:varchar(255)" json:"phone_number"`
	RealName           *string    `gorm:"column:real_name;type:varchar(255)" json:"real_name"`
	Status             *int       `gorm:"column:status" json:"status"`
	TenantId          *int       `gorm:"column:license_id" json:"license_id"`
	CreateTime         *time.Time `gorm:"column:create_time" json:"create_time"`
	LastLoginTime      *time.Time `gorm:"column:last_login_time" json:"last_login_time"`
	LoginErrorTimes    int        `gorm:"column:login_error_times;not null;default:0" json:"login_error_times"` // Java int 原语 NOT NULL
	LastLockTime       *time.Time `gorm:"column:last_lock_time" json:"last_lock_time"`
	Enable             *bool      `gorm:"column:enable" json:"enable"`
	PasswordModifyTime *time.Time `gorm:"column:password_modify_time" json:"password_modify_time"`
	Salt               *string    `gorm:"column:salt;type:varchar(255)" json:"salt"`
	CreateUserId       *int       `gorm:"column:create_user_id" json:"create_user_id"`
	Avatar             *string    `gorm:"column:avatar;type:varchar(255)" json:"avatar"`
	LoginErrorTime     *time.Time `gorm:"column:login_error_time" json:"login_error_time"`
	CreateUserName     *string    `gorm:"column:create_user_name;type:varchar(255)" json:"create_user_name"`
	UpdateTime         *time.Time `gorm:"column:update_time" json:"update_time"`
	FirstLogin         *string    `gorm:"column:first_login;type:varchar(8)" json:"first_login"` // Java String ("Y"/"N")，不是布尔
	Deleted            *bool      `gorm:"column:deleted" json:"deleted"`
	HistoryPasswords   *string    `gorm:"column:history_passwords;type:longtext" json:"-"`
}

func (SysUser) TableName() string { return "sys_user" }

// UserDTO is the API response DTO for user data, excluding sensitive fields (password, salt).
// Aligned with Java ListUserVO:
//   id, username, createUsername, enable, updateTime, createTime, tenancy, email, loginState
type UserDTO struct {
	Id             int        `json:"id"`
	Username       *string    `json:"username"`
	CreateUsername *string    `json:"createUsername"`
	Enable         *bool      `json:"enable"`
	UpdateTime     *time.Time `json:"updateTime"`
	CreateTime     *time.Time `json:"createTime"`
	Tenancy        string     `json:"tenancy"`
	Email          *string    `json:"email"`
	// LoginState indicates whether the user is currently online, determined by
	// the WebSocket heartbeat. Mirrors Java's ListUserVO.loginState derived from
	// ResultWebSocket.lastHeartbeatTime.
	LoginState bool `json:"loginState"`
}

// AddUserVO is the response for addUser, containing the generated password.
// Mirrors Java AddUserVo.
type AddUserVO struct {
	UserId   int    `json:"userId"`
	Password string `json:"password"`
}

// GeneratePassword generates a random password with the specified length.
// Includes lowercase, uppercase, digits, and special characters.
// Mirrors Java PasswordGenerator.generatePassword.
func GeneratePassword(length int) (string, error) {
	const (
		lowercase = "abcdefghijklmnopqrstuvwxyz"
		uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		digits    = "0123456789"
		special   = "!@#$%^&*"
		all       = lowercase + uppercase + digits + special
	)

	// Ensure at least 4 characters to include each category
	if length < 4 {
		length = 4
	}

	password := make([]byte, length)

	// Guarantee at least one character from each category
	categories := []string{lowercase, uppercase, digits, special}
	for i := 0; i < 4; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(categories[i]))))
		if err != nil {
			return "", err
		}
		password[i] = categories[i][idx.Int64()]
	}

	// Fill remaining positions with random characters
	for i := 4; i < length; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(all))))
		if err != nil {
			return "", err
		}
		password[i] = all[idx.Int64()]
	}

	// Shuffle the password
	for i := len(password) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		password[i], password[j.Int64()] = password[j.Int64()], password[i]
	}

	return string(password), nil
}

// ToUserDTO converts a SysUser to a UserDTO (strips password/salt).
// Aligned with Java ListUserVO fields.
func ToUserDTO(u *SysUser) UserDTO {
	return UserDTO{
		Id:             u.Id,
		Username:       u.Username,
		CreateUsername: u.CreateUserName,
		Enable:         u.Enable,
		UpdateTime:     u.UpdateTime,
		CreateTime:     u.CreateTime,
		Email:          u.Email,
	}
}

// ToUserDTOs converts a slice of SysUser to a slice of UserDTO.
func ToUserDTOs(users []SysUser) []UserDTO {
	dtos := make([]UserDTO, len(users))
	for i, u := range users {
		dtos[i] = ToUserDTO(&u)
	}
	return dtos
}

// Role 对应 role 表 (Java: String id, not auto-increment)
// Column names name/tenant_id align with Java Role entity (JPA default naming
// from name/tenantId fields). Go field names retained for code stability.
type Role struct {
	Id          string     `gorm:"primaryKey;type:varchar(32)" json:"id"`
	RoleName    *string    `gorm:"column:name;type:varchar(255)" json:"role_name"`
	Description *string    `gorm:"column:description;type:varchar(255)" json:"description"`
	TenantId   *int       `gorm:"column:license_id" json:"license_id"`
	UseToSSO    *bool      `gorm:"column:use_to_sso" json:"use_to_sso"`
	DefaultRole *bool      `gorm:"column:default_role" json:"default_role"`
	User        *string    `gorm:"column:user;type:varchar(255)" json:"user"`
	UpdateTime  *time.Time `gorm:"column:update_time" json:"update_time"`
}

func (Role) TableName() string { return "role" }

// BeforeCreate auto-generates a UUID v4 primary key if not already set.
// Signature must be exactly BeforeCreate(*gorm.DB) error to satisfy GORM's
// BeforeCreateInterface; otherwise the hook is never invoked.
func (r *Role) BeforeCreate(tx *gorm.DB) error {
	if r.Id == "" {
		r.Id = generateUUID()
	}
	return nil
}

// RoleHasPermission 对应 role_has_permission 表
type RoleHasPermission struct {
	Id           int     `gorm:"primaryKey;autoIncrement" json:"id"`
	RoleId       *string `gorm:"column:role_id;type:varchar(32)" json:"role_id"`
	PermissionId *string `gorm:"column:permission_id;type:varchar(255)" json:"permission_id"`
}

func (RoleHasPermission) TableName() string { return "role_has_permission" }

// UserHasRole 对应 user_has_role 表 (Java id 是 Long / BigInt，这里用 int64 对齐)
type UserHasRole struct {
	Id     int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	UserId int    `gorm:"column:user_id" json:"user_id"`
	RoleId string `gorm:"column:role_id;type:varchar(32)" json:"role_id"`
}

func (UserHasRole) TableName() string { return "user_has_role" }

// LoginLog 对应 login_log 表
// Column names ip/operation_time align with Java LoginLog entity (JPA default
// naming from ip/operationTime fields). Go field names retained for code stability.
type LoginLog struct {
	Id         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Username   *string    `gorm:"column:username;type:varchar(255)" json:"username"`
	IpAddress  *string    `gorm:"column:ip;type:varchar(255)" json:"ip_address"`
	LoginTime  *time.Time `gorm:"column:operation_time" json:"login_time"`
	Result     *int       `gorm:"column:result" json:"result"`
	TenantId  *int       `gorm:"column:license_id" json:"license_id"`
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
	TenantId int `json:"tenantId" binding:"required"`
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

// 密码过期天的
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

// generateUUID returns a random UUID v4 string (32 hex chars with dashes).
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
