package database

import (
	"fmt"
	"time"

	"nmsappsrv/internal/license"
	"nmsappsrv/internal/user"
	"nmsappsrv/pkg/logger"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	DefaultAdminUsername = "admin"
	DefaultAdminPassword = "Admin@123"
	DefaultLicenseName   = "Default"
	DefaultLicenseId     = "1"
	DefaultAdminRoleId   = "admin"
)

// SeedInitialData creates default tenant, admin user, and admin role if they don't exist.
// This is idempotent — safe to call on every startup.
func SeedInitialData(db *gorm.DB) error {
	// 1. Ensure default license/tenant
	lic, err := ensureDefaultLicense(db)
	if err != nil {
		return fmt.Errorf("ensure default license: %w", err)
	}

	// 2. Ensure admin role
	role, err := ensureAdminRole(db, lic.Id)
	if err != nil {
		return fmt.Errorf("ensure admin role: %w", err)
	}

	// 3. Ensure admin user
	err = ensureAdminUser(db, lic.Id, role.Id)
	if err != nil {
		return fmt.Errorf("ensure admin user: %w", err)
	}

	logger.Info("initial data seeded (or already exists)")
	return nil
}

func ensureDefaultLicense(db *gorm.DB) (*license.License, error) {
	var count int64
	db.Model(&license.License{}).Count(&count)
	if count > 0 {
		var lic license.License
		if err := db.First(&lic).Error; err != nil {
			return nil, err
		}
		return &lic, nil
	}

	name := DefaultLicenseName
	lid := DefaultLicenseId
	licType := "full"
	expiry := time.Now().Add(10 * 365 * 24 * time.Hour) // 10 years
	lic := license.License{
		LicenseName: &name,
		LicenseId:   &lid,
		LicenseType: &licType,
		ExpiryDate:  &expiry,
		EnbQuantity: 9999,
		GnbQuantity: intPtr(9999),
		CpeQuantity: intPtr(9999),
		UserQuantity: 9999,
	}
	if err := db.Create(&lic).Error; err != nil {
		return nil, err
	}
	logger.Info("default license created")
	return &lic, nil
}

func ensureAdminRole(db *gorm.DB, licenseId int) (*user.Role, error) {
	roleName := "Admin"
	var role user.Role
	err := db.Where("role_name = ? AND license_id = ?", roleName, licenseId).First(&role).Error
	if err == nil {
		return &role, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	desc := "System Administrator"
	role = user.Role{
		Id:          DefaultAdminRoleId,
		RoleName:    &roleName,
		Description: &desc,
		LicenseId:   &licenseId,
	}
	if err := db.Create(&role).Error; err != nil {
		return nil, err
	}
	logger.Info("admin role created")
	return &role, nil
}

func ensureAdminUser(db *gorm.DB, licenseId int, roleId string) error {
	var count int64
	db.Model(&user.SysUser{}).Where("username = ?", DefaultAdminUsername).Count(&count)
	if count > 0 {
		return nil
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(DefaultAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	hashedStr := string(hashed)
	now := time.Now()
	enable := true
	status := 1

	usr := user.SysUser{
		Username:           strPtr(DefaultAdminUsername),
		Password:           &hashedStr,
		RealName:           strPtr("Administrator"),
		Status:             &status,
		LicenseId:          &licenseId,
		CreateTime:         &now,
		Enable:             &enable,
		LoginErrorTimes:    intPtr(0),
		PasswordModifyTime: &now,
	}
	if err := db.Create(&usr).Error; err != nil {
		return err
	}

	// Link user ↔ role
	link := user.UserHasRole{
		UserId: usr.Id,
		RoleId: roleId,
	}
	if err := db.Create(&link).Error; err != nil {
		return err
	}

	logger.Infof("admin user created (username=%s, password=%s) — please change password on first login",
		DefaultAdminUsername, DefaultAdminPassword)
	return nil
}

func strPtr(s string) *string    { return &s }
func intPtr(i int) *int          { return &i }
