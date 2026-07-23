package database

import (
	"fmt"
	"time"

	"nmsappsrv/internal/license"
	"nmsappsrv/internal/user"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/security"

	"gorm.io/gorm"
)

const (
	DefaultAdminUsername = "admin"
	DefaultAdminPassword = "Admin@123"
	DefaultLicenseName   = "Default"
	DefaultTenantCode    = "1"
	// DefaultAdminRoleId is the built-in admin role id. It is kept identical
	// to authz.RoleAdminID so the admin user links to the same role row that
	// authz.SeedBuiltinRoles manages (hard-coded here to avoid coupling
	// pkg/database to internal/authz).
	DefaultAdminRoleId = "admin_id"
	// legacyAdminRoleId is the id of the duplicate admin role that older
	// versions of SeedInitialData created. Existing databases are healed at
	// startup: associations are moved to DefaultAdminRoleId and the stale row
	// is removed.
	legacyAdminRoleId = "admin"
)

// SeedInitialData creates default tenant, admin user, and admin role if they don't exist.
// This is idempotent — safe to call on every startup.
func SeedInitialData(db *gorm.DB) error {
	// 1. Ensure default license/tenant
	lic, err := ensureDefaultLicense(db)
	if err != nil {
		return fmt.Errorf("ensure default license: %w", err)
	}

	// 2. Ensure admin role (built-in "admin_id", heals legacy "admin" row)
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
	lid := DefaultTenantCode
	licType := "full"
	expiry := time.Now().Add(10 * 365 * 24 * time.Hour) // 10 years
	lic := license.License{
		LicenseName: &name,
		TenantCode:  &lid,
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

// ensureAdminRole returns the built-in admin role (id "admin_id"), creating it
// if missing. The role name is normalized to "Admin" (matching the Java NMS
// built-in role) so fresh and upgraded databases converge — older deployments
// may carry a lowercase "admin" name written by authz.SeedBuiltinRoles.
func ensureAdminRole(db *gorm.DB, tenantId int) (*user.Role, error) {
	roleName := "Admin"
	var role user.Role
	err := db.Where("id = ?", DefaultAdminRoleId).First(&role).Error
	switch {
	case err == nil:
		// Normalize the name if an older seeder wrote a different one.
		if role.RoleName == nil || *role.RoleName != roleName {
			if e := db.Model(&user.Role{}).Where("id = ?", DefaultAdminRoleId).
				Update("name", roleName).Error; e != nil {
				return nil, e
			}
			role.RoleName = &roleName
		}
	case err == gorm.ErrRecordNotFound:
		desc := "System Administrator"
		role = user.Role{
			Id:          DefaultAdminRoleId,
			RoleName:    &roleName,
			Description: &desc,
			TenantId:   &tenantId,
			DefaultRole: boolPtr(true),
		}
		if e := db.Create(&role).Error; e != nil {
			return nil, e
		}
		logger.Info("admin role created (id=admin_id)")
	default:
		return nil, err
	}

	if err := healLegacyAdminRole(db); err != nil {
		return nil, err
	}
	return &role, nil
}

// healLegacyAdminRole migrates user_has_role rows that still reference the
// legacy duplicate role (id "admin") to the built-in "admin_id" role, then
// removes the stale role row. Idempotent: no-op once the legacy row is gone.
func healLegacyAdminRole(db *gorm.DB) error {
	var legacy user.Role
	err := db.Where("id = ?", legacyAdminRoleId).First(&legacy).Error
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	if err != nil {
		return err
	}

	// Re-point legacy associations. Users that already have an admin_id
	// association simply lose the duplicate legacy row.
	var userIds []int
	if err := db.Model(&user.UserHasRole{}).
		Where("role_id = ?", legacyAdminRoleId).
		Pluck("user_id", &userIds).Error; err != nil {
		return err
	}
	for _, uid := range userIds {
		var cnt int64
		db.Model(&user.UserHasRole{}).
			Where("user_id = ? AND role_id = ?", uid, DefaultAdminRoleId).
			Count(&cnt)
		if cnt == 0 {
			err = db.Model(&user.UserHasRole{}).
				Where("user_id = ? AND role_id = ?", uid, legacyAdminRoleId).
				Update("role_id", DefaultAdminRoleId).Error
		} else {
			err = db.Where("user_id = ? AND role_id = ?", uid, legacyAdminRoleId).
				Delete(&user.UserHasRole{}).Error
		}
		if err != nil {
			return err
		}
	}

	// Remove the stale duplicate role row.
	if err := db.Where("id = ?", legacyAdminRoleId).Delete(&user.Role{}).Error; err != nil {
		return err
	}
	logger.Info("healed legacy admin role: associations moved to admin_id, duplicate role removed")
	return nil
}

func ensureAdminUser(db *gorm.DB, tenantId int, roleId string) error {
	var count int64
	db.Model(&user.SysUser{}).Where("username = ?", DefaultAdminUsername).Count(&count)
	if count > 0 {
		return nil
	}

	hashedStr, err := security.HashPassword(DefaultAdminPassword, DefaultAdminUsername, 0,
		security.NewRedisDBBackedSaltHolder(db))
	if err != nil {
		return err
	}
	now := time.Now()
	enable := true
	status := 1

	usr := user.SysUser{
		Username:           strPtr(DefaultAdminUsername),
		Password:           &hashedStr,
		RealName:           strPtr("Administrator"),
		Status:             &status,
		TenantId:          &tenantId,
		CreateTime:         &now,
		Enable:             &enable,
		LoginErrorTimes:    0, // Java int primitive NOT NULL default 0
		PasswordModifyTime: &now,
	}
	if err := db.Create(&usr).Error; err != nil {
		return err
	}

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

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func boolPtr(b bool) *bool    { return &b }
