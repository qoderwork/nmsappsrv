package user

import (
	"time"

	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository handles database operations for user entities.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ---------------------------------------------------------------------------
// SysUser CRUD
// ---------------------------------------------------------------------------

// FindUserByUsername returns a user by username.
func (r *Repository) FindUserByUsername(username string) (*SysUser, error) {
	var u SysUser
	if err := r.db.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// FindUserByID returns a user by primary key.
func (r *Repository) FindUserByID(id int) (*SysUser, error) {
	var u SysUser
	if err := r.db.Where("id = ?", id).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// FindUsers returns a paginated list of users for the given license.
func (r *Repository) FindUsers(licenseId int, offset, limit int) ([]SysUser, int64, error) {
	var users []SysUser
	var total int64

	query := r.db.Model(&SysUser{}).Where("license_id = ?", licenseId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindUsers count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		logger.Errorf("FindUsers query error: %v", err)
		return nil, 0, err
	}
	return users, total, nil
}

// CreateUser inserts a new user row.
func (r *Repository) CreateUser(u *SysUser) error {
	return r.db.Create(u).Error
}

// UpdateUser saves all fields of an existing user.
func (r *Repository) UpdateUser(u *SysUser) error {
	return r.db.Save(u).Error
}

// DeleteUser removes a user by ID.
func (r *Repository) DeleteUser(id int) error {
	return r.db.Where("id = ?", id).Delete(&SysUser{}).Error
}

// UpdateUserFields updates specific fields of a user.
func (r *Repository) UpdateUserFields(id int, fields map[string]interface{}) error {
	return r.db.Model(&SysUser{}).Where("id = ?", id).Updates(fields).Error
}

// ---------------------------------------------------------------------------
// Role CRUD
// ---------------------------------------------------------------------------

// FindRoles returns all roles for the given license.
func (r *Repository) FindRoles(licenseId int) ([]Role, error) {
	var roles []Role
	if err := r.db.Where("license_id = ?", licenseId).Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// CreateRole inserts a new role.
func (r *Repository) CreateRole(role *Role) error {
	return r.db.Create(role).Error
}

// UpdateRole saves changes to an existing role.
func (r *Repository) UpdateRole(role *Role) error {
	return r.db.Save(role).Error
}

// DeleteRole removes a role by ID (string UUID).
func (r *Repository) DeleteRole(id string) error {
	return r.db.Where("id = ?", id).Delete(&Role{}).Error
}

// ---------------------------------------------------------------------------
// Role ↔ Permission association
// ---------------------------------------------------------------------------

// FindPermissionsByRoleId returns all permission associations for a role.
func (r *Repository) FindPermissionsByRoleId(roleId string) ([]RoleHasPermission, error) {
	var perms []RoleHasPermission
	if err := r.db.Where("role_id = ?", roleId).Find(&perms).Error; err != nil {
		return nil, err
	}
	return perms, nil
}

// SavePermissions replaces all permission associations for a role. It deletes
// existing rows and inserts the new set inside a transaction.
func (r *Repository) SavePermissions(roleId string, permissionIds []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleId).Delete(&RoleHasPermission{}).Error; err != nil {
			return err
		}
		for _, pid := range permissionIds {
			rel := RoleHasPermission{RoleId: &roleId, PermissionId: &pid}
			if err := tx.Create(&rel).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ---------------------------------------------------------------------------
// User ↔ Role association
// ---------------------------------------------------------------------------

// FindUserRoles returns all role associations for a user.
func (r *Repository) FindUserRoles(userId int) ([]UserHasRole, error) {
	var roles []UserHasRole
	if err := r.db.Where("user_id = ?", userId).Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// SaveUserRoles replaces all role associations for a user.
func (r *Repository) SaveUserRoles(userId int, roleIds []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userId).Delete(&UserHasRole{}).Error; err != nil {
			return err
		}
		for _, rid := range roleIds {
			rel := UserHasRole{UserId: userId, RoleId: rid}
			if err := tx.Create(&rel).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ---------------------------------------------------------------------------
// LoginLog
// ---------------------------------------------------------------------------

// CreateLoginLog inserts a new login log entry.
func (r *Repository) CreateLoginLog(log *LoginLog) error {
	return r.db.Create(log).Error
}

// ---------------------------------------------------------------------------
// PasswordHistory
// ---------------------------------------------------------------------------

// CreatePasswordHistory inserts a password history record.
func (r *Repository) CreatePasswordHistory(h *PasswordHistory) error {
	return r.db.Create(h).Error
}

// FindRecentPasswords returns the most recent N password hashes for a user.
func (r *Repository) FindRecentPasswords(username string, limit int) ([]PasswordHistory, error) {
	var history []PasswordHistory
	if err := r.db.Where("username = ?", username).Order("create_time DESC").Limit(limit).Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

// ---------------------------------------------------------------------------
// Query helpers
// ---------------------------------------------------------------------------

// CountUsersByLicenseId returns the number of users for a license.
func (r *Repository) CountUsersByLicenseId(licenseId int) (int64, error) {
	var count int64
	if err := r.db.Model(&SysUser{}).Where("license_id = ?", licenseId).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// FindUsersByCreatorId returns users created by a specific user.
func (r *Repository) FindUsersByCreatorId(creatorId int) ([]SysUser, error) {
	var users []SysUser
	if err := r.db.Where("create_user_id = ?", creatorId).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateLastLoginTime sets the last_login_time for a user.
func (r *Repository) UpdateLastLoginTime(username string, t time.Time) error {
	return r.db.Model(&SysUser{}).Where("username = ?", username).
		Update("last_login_time", t).Error
}
