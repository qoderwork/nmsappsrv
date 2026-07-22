package user

import (
	"time"

	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for user-related entities.
type Repository interface {
	FindUserByUsername(username string) (*SysUser, error)
	FindUserByID(id int) (*SysUser, error)
	FindUsers(offset, limit int, excludeAdmin bool, creatorName string) ([]SysUser, int64, error)
	CreateUser(u *SysUser) error
	UpdateUser(u *SysUser) error
	DeleteUser(id int) error
	UpdateUserFields(id int, fields map[string]interface{}) error
	FindRoles(tenantId int) ([]Role, error)
	FindRolesByIds(roleIds []string) ([]Role, error)
	CreateRole(role *Role) error
	UpdateRole(role *Role) error
	DeleteRole(id string) error
	FindPermissionsByRoleId(roleId string) ([]RoleHasPermission, error)
	SavePermissions(roleId string, permissionIds []string) error
	FindUserRoles(userId int) ([]UserHasRole, error)
	SaveUserRoles(userId int, roleIds []string) error
	CreateLoginLog(log *LoginLog) error
	CreatePasswordHistory(h *PasswordHistory) error
	FindRecentPasswords(username string, limit int) ([]PasswordHistory, error)
	CountUsersByTenantId(tenantId int) (int64, error)
	FindUsersByCreatorId(creatorId int) ([]SysUser, error)
	UpdateLastLoginTime(username string, t time.Time) error
}

// repository is the concrete GORM-backed implementation of Repository.
// The embedded BaseRepository provides generic CRUD for SysUser that is
// reused through delegation methods below.
type repository struct {
	*baserepo.BaseRepository[SysUser, int]
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[SysUser, int](db, "id"),
		db:             db,
	}
}

// ---------------------------------------------------------------------------
// SysUser CRUD
// ---------------------------------------------------------------------------

// FindUserByUsername returns a user by username.
func (r *repository) FindUserByUsername(username string) (*SysUser, error) {
	var u SysUser
	if err := r.db.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// FindUserByID returns a user by primary key.
// Delegates to BaseRepository.FindByID.
func (r *repository) FindUserByID(id int) (*SysUser, error) {
	return r.BaseRepository.FindByID(id)
}

// FindUsers returns a paginated list of users.
// Mirrors Java SystemUserManagementServiceImpl.listUser:
//   - excludes admin user (admin should not appear in user list)
//   - non-admin users can only see users they created (via createUserName filter)
//   - excludes deleted users
// Note: Java does NOT filter by tenant_id in listUser
func (r *repository) FindUsers(offset, limit int, excludeAdmin bool, creatorName string) ([]SysUser, int64, error) {
	var users []SysUser
	var total int64

	query := r.db.Model(&SysUser{})

	// Exclude admin user (mirrors Java: predicates.add(criteriaBuilder.notEqual(root.get("username"), "admin")))
	if excludeAdmin {
		query = query.Where("username != ?", "admin")
	}

	// Non-admin users can only see users they created
	// (mirrors Java: predicates.add(criteriaBuilder.equal(root.get("createUserName"), SecurityUtil.getCurrentUsername())))
	if creatorName != "" {
		query = query.Where("create_user_name = ?", creatorName)
	}

	// Exclude deleted users (mirrors Java: deleted = false or null)
	query = query.Where("deleted IS NULL OR deleted = ?", false)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindUsers count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("update_time DESC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		logger.Errorf("FindUsers query error: %v", err)
		return nil, 0, err
	}
	return users, total, nil
}

// CreateUser inserts a new user row.
// Delegates to BaseRepository.Create.
func (r *repository) CreateUser(u *SysUser) error {
	return r.BaseRepository.Create(u)
}

// UpdateUser saves all fields of an existing user.
// Delegates to BaseRepository.Save.
func (r *repository) UpdateUser(u *SysUser) error {
	return r.BaseRepository.Save(u)
}

// DeleteUser removes a user by ID.
// Delegates to BaseRepository.DeleteByID.
func (r *repository) DeleteUser(id int) error {
	return r.BaseRepository.DeleteByID(id)
}

// UpdateUserFields updates specific fields of a user.
// Delegates to BaseRepository.UpdateFields.
func (r *repository) UpdateUserFields(id int, fields map[string]interface{}) error {
	return r.BaseRepository.UpdateFields(id, fields)
}

// ---------------------------------------------------------------------------
// Role CRUD
// ---------------------------------------------------------------------------

// FindRoles returns all roles for the given license.
func (r *repository) FindRoles(tenantId int) ([]Role, error) {
	var roles []Role
	if err := r.db.Where("tenant_id = ?", tenantId).Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// FindRolesByIds returns roles matching the given role IDs.
// Mirrors Java roleService.getByIdIn(roleIds) — used to resolve role names
// for a user via the user_has_role association table.
func (r *repository) FindRolesByIds(roleIds []string) ([]Role, error) {
	if len(roleIds) == 0 {
		return nil, nil
	}
	var roles []Role
	if err := r.db.Where("id IN ?", roleIds).Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// CreateRole inserts a new role.
func (r *repository) CreateRole(role *Role) error {
	return r.db.Create(role).Error
}

// UpdateRole saves changes to an existing role.
func (r *repository) UpdateRole(role *Role) error {
	return r.db.Save(role).Error
}

// DeleteRole removes a role by ID (string UUID).
func (r *repository) DeleteRole(id string) error {
	return r.db.Where("id = ?", id).Delete(&Role{}).Error
}

// ---------------------------------------------------------------------------
// Role ↔ Permission association
// ---------------------------------------------------------------------------

// FindPermissionsByRoleId returns all permission associations for a role.
func (r *repository) FindPermissionsByRoleId(roleId string) ([]RoleHasPermission, error) {
	var perms []RoleHasPermission
	if err := r.db.Where("role_id = ?", roleId).Find(&perms).Error; err != nil {
		return nil, err
	}
	return perms, nil
}

// SavePermissions replaces all permission associations for a role. It deletes
// existing rows and inserts the new set inside a transaction.
func (r *repository) SavePermissions(roleId string, permissionIds []string) error {
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
func (r *repository) FindUserRoles(userId int) ([]UserHasRole, error) {
	var roles []UserHasRole
	if err := r.db.Where("user_id = ?", userId).Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// SaveUserRoles replaces all role associations for a user.
func (r *repository) SaveUserRoles(userId int, roleIds []string) error {
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
func (r *repository) CreateLoginLog(log *LoginLog) error {
	return r.db.Create(log).Error
}

// ---------------------------------------------------------------------------
// PasswordHistory
// ---------------------------------------------------------------------------

// CreatePasswordHistory inserts a password history record.
func (r *repository) CreatePasswordHistory(h *PasswordHistory) error {
	return r.db.Create(h).Error
}

// FindRecentPasswords returns the most recent N password hashes for a user.
func (r *repository) FindRecentPasswords(username string, limit int) ([]PasswordHistory, error) {
	var history []PasswordHistory
	if err := r.db.Where("username = ?", username).Order("create_time DESC").Limit(limit).Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

// ---------------------------------------------------------------------------
// Query helpers
// ---------------------------------------------------------------------------

// CountUsersByTenantId returns the number of users for a license.
func (r *repository) CountUsersByTenantId(tenantId int) (int64, error) {
	var count int64
	if err := r.db.Model(&SysUser{}).Where("tenant_id = ?", tenantId).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// FindUsersByCreatorId returns users created by a specific user.
func (r *repository) FindUsersByCreatorId(creatorId int) ([]SysUser, error) {
	var users []SysUser
	if err := r.db.Where("create_user_id = ?", creatorId).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateLastLoginTime sets the last_login_time for a user.
func (r *repository) UpdateLastLoginTime(username string, t time.Time) error {
	return r.db.Model(&SysUser{}).Where("username = ?", username).
		Update("last_login_time", t).Error
}
