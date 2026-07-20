package user

import (
	"context"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// Service defines the business-logic contract for user management.
type Service interface {
	Login(username, password string) (*SysUser, error)
	Logout(username, jwtToken string) error
	RecordLogin(username, ip string, licenseId int, result int) error
	RecordLogout(username, ip string, licenseId int) error
	ListUsers(page, pageSize int, excludeAdmin bool, creatorName string) ([]SysUser, int64, error)
	CreateUser(u *SysUser) (string, error)
	UpdateUser(u *SysUser) error
	DeleteUser(id int) error
	KickOutUser(userId int) error
	UnlockUser(userId int) error
	ModifyPassword(username, oldPassword, newPassword string) error
	EnableUser(userId int) error
	DisableUser(userId int) error
	ResetPassword(adminId, userId int) (string, error)
	ResetPasswordByLink(username, key, newPassword string) error
	SetTenancyForUser(userId, licenseId int) error
	GetLoginFailedTimes(userId int) (*LoginFailedTimesResponse, error)
	NeedChangePassword(userId int) (*NeedChangePasswordResponse, error)
	ListRoles(licenseId int) ([]Role, error)
	CreateRole(r *Role) error
	UpdateRole(r *Role) error
	DeleteRole(id string) error
	GetRolePermissions(roleId string) ([]RoleHasPermission, error)
	UpdateRolePermissions(roleId string, permissionIds []string) error
	GetRoleNamesForUser(userId int, licenseId int) ([]string, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

// Login validates credentials and enforces the login security gate:
//   - disabled accounts (Enable == false) are rejected outright;
//   - accounts locked by too many recent failures are rejected until the lock
//     window (UserLockMinutes) elapses, after which they auto-unlock;
//   - every failed password attempt increments the per-user error counter and
//     locks the account once it reaches DefaultMaxLoginFailedTimes;
//   - a successful login resets the counter and records the login time.
//
// The gate operates entirely on existing sys_user columns (Enable,
// LoginErrorTimes, LastLockTime, LastLoginTime) via repo.UpdateUserFields, so
// it reuses the same primitives already exposed by UnlockUser / GetLoginFailedTimes.
func (s *service) Login(username, password string) (*SysUser, error) {
	u, err := s.repo.FindUserByUsername(username)
	if err != nil {
		return nil, apperror.ErrInvalidCredentials
	}

	// 1) Account disabled by an administrator.
	if u.Enable != nil && !*u.Enable {
		return nil, apperror.ErrUserDisabled
	}

	// 2) Account locked due to too many failed attempts?
	if u.LoginErrorTimes != nil && *u.LoginErrorTimes >= DefaultMaxLoginFailedTimes {
		locked := u.LastLockTime != nil && time.Since(*u.LastLockTime) < UserLockMinutes*time.Minute
		if locked {
			return nil, apperror.ErrUserLocked
		}
		// Lock window has elapsed (or LastLockTime was never set): auto-unlock
		// so the user can retry, then fall through to the password check.
		if err := s.repo.UpdateUserFields(u.Id, map[string]interface{}{
			"login_error_times": 0,
			"last_lock_time":    nil,
		}); err != nil {
			logger.Warnf("login: failed to auto-unlock user %q: %v", username, err)
		}
		u.LoginErrorTimes = intPtr(0)
		u.LastLockTime = nil
	}

	if u.Password == nil {
		return nil, apperror.ErrInvalidCredentials
	}

	// 3) Verify the password.
	if err := bcrypt.CompareHashAndPassword([]byte(*u.Password), []byte(password)); err != nil {
		// Increment the failure counter and lock the account at the threshold.
		newCount := 1
		if u.LoginErrorTimes != nil {
			newCount = *u.LoginErrorTimes + 1
		}
		fields := map[string]interface{}{
			"login_error_times": newCount,
			"login_error_time":  time.Now(),
		}
		if newCount >= DefaultMaxLoginFailedTimes {
			fields["last_lock_time"] = time.Now()
		}
		if err := s.repo.UpdateUserFields(u.Id, fields); err != nil {
			logger.Warnf("login: failed to record failed attempt for %q: %v", username, err)
		}
		return nil, apperror.ErrInvalidCredentials
	}

	// 4) Success: reset the counter, clear the lock, record the login time.
	if err := s.repo.UpdateUserFields(u.Id, map[string]interface{}{
		"login_error_times": 0,
		"last_lock_time":    nil,
		"last_login_time":   time.Now(),
	}); err != nil {
		logger.Warnf("login: failed to reset login state for %q: %v", username, err)
	}

	return u, nil
}

// Logout invalidates the current JWT token.
// 1. Delete SECURITY_JWT_LOGIN:{username}
// 2. Add JWT to blacklist SECURITY_JWT_BLACK:{jwt} with TTL
func (s *service) Logout(username, jwtToken string) error {
	ctx := context.Background()

	// Delete login key
	loginKey := RedisKeyJWTLogin + username
	if err := redis.Del(ctx, loginKey); err != nil {
		logger.Warnf("logout: failed to delete login key for %s: %v", username, err)
	}

	// Add JWT to blacklist with TTL matching remaining token lifetime
	blackKey := RedisKeyJWTBlack + jwtToken
	if err := redis.Set(ctx, blackKey, time.Now().UnixMilli(), JTTTLMintues*time.Minute); err != nil {
		logger.Warnf("logout: failed to blacklist JWT for %s: %v", username, err)
	}

	return nil
}

// RecordLogin creates a login log entry.
func (s *service) RecordLogin(username, ip string, licenseId int, result int) error {
	now := time.Now()
	log := LoginLog{
		Username:  &username,
		IpAddress: &ip,
		LoginTime: &now,
		Result:    &result,
		LicenseId: &licenseId,
	}
	if err := s.repo.CreateLoginLog(&log); err != nil {
		return apperror.Wrap(err, "RECORD_LOGIN_FAILED", 500, "failed to record login")
	}
	return nil
}

// RecordLogout creates a logout log entry.
func (s *service) RecordLogout(username, ip string, licenseId int) error {
	now := time.Now()
	logType := LoginTypeLogout
	info := "Logout"
	log := LoginLog{
		Username:  &username,
		IpAddress: &ip,
		LoginTime: &now,
		Result:    intPtr(1),
		LicenseId: &licenseId,
		Type:      &logType,
		Info:      &info,
	}
	if err := s.repo.CreateLoginLog(&log); err != nil {
		return apperror.Wrap(err, "RECORD_LOGOUT_FAILED", 500, "failed to record logout")
	}
	return nil
}

// ---------------------------------------------------------------------------
// SysUser CRUD
// ---------------------------------------------------------------------------

// ListUsers returns a paginated user list.
func (s *service) ListUsers(page, pageSize int, excludeAdmin bool, creatorName string) ([]SysUser, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	users, total, err := s.repo.FindUsers(offset, pageSize, excludeAdmin, creatorName)
	if err != nil {
		return nil, 0, apperror.Wrap(err, "LIST_USERS_FAILED", 500, "failed to list users")
	}
	return users, total, nil
}

// CreateUser auto-generates a password if not provided, hashes it, and persists a new user.
// Returns the generated password for display to the admin.
// Mirrors Java SystemUserManagementServiceImpl.addUser.
func (s *service) CreateUser(u *SysUser) (string, error) {
	var generatedPassword string
	var err error

	// Auto-generate password if not provided (mirrors Java behavior)
	if u.Password == nil || *u.Password == "" {
		generatedPassword, err = GeneratePassword(12)
		if err != nil {
			return "", apperror.Wrap(err, "PASSWORD_GENERATE_FAILED", 500, "failed to generate password")
		}
		u.Password = &generatedPassword
	}

	// Hash the password
	hash, err := bcrypt.GenerateFromPassword([]byte(*u.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", apperror.Wrap(err, "PASSWORD_HASH_FAILED", 500, "failed to hash password")
	}
	hashed := string(hash)
	u.Password = &hashed

	// Set defaults
	if u.Enable == nil {
		enabled := true
		u.Enable = &enabled
	}
	if u.LoginErrorTimes == nil {
		zero := 0
		u.LoginErrorTimes = &zero
	}

	// Set timestamps (mirrors Java: sysUser.setCreateTime(new Date()); sysUser.setUpdateTime(new Date());)
	now := time.Now()
	u.CreateTime = &now
	u.UpdateTime = &now

	if err := s.repo.CreateUser(u); err != nil {
		return "", apperror.Wrap(err, "CREATE_USER_FAILED", 500, "failed to create user")
	}

	// Return the plaintext password for display (only for auto-generated)
	return generatedPassword, nil
}

// UpdateUser persists changes to an existing user. If the password field has
// changed it is re-hashed before saving.
func (s *service) UpdateUser(u *SysUser) error {
	if u.Password != nil && *u.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*u.Password), bcrypt.DefaultCost)
		if err != nil {
			return apperror.Wrap(err, "PASSWORD_HASH_FAILED", 500, "failed to hash password")
		}
		hashed := string(hash)
		u.Password = &hashed
	}
	// Update timestamp (mirrors Java behavior on modification)
	now := time.Now()
	u.UpdateTime = &now

	if err := s.repo.UpdateUser(u); err != nil {
		return apperror.Wrap(err, "UPDATE_USER_FAILED", 500, "failed to update user")
	}
	return nil
}

// DeleteUser removes a user by ID and invalidates their session.
func (s *service) DeleteUser(id int) error {
	u, err := s.repo.FindUserByID(id)
	if err != nil {
		return apperror.ErrUserNotFound
	}
	// Invalidate session before deleting
	if u.Username != nil {
		s.invalidateUser(*u.Username)
	}
	if err := s.repo.DeleteUser(id); err != nil {
		return apperror.Wrap(err, "DELETE_USER_FAILED", 500, "failed to delete user")
	}
	return nil
}

// ---------------------------------------------------------------------------
// User Management Features
// ---------------------------------------------------------------------------

// KickOutUser forces a user to logout by removing their JWT login key.
func (s *service) KickOutUser(userId int) error {
	u, err := s.repo.FindUserByID(userId)
	if err != nil {
		return apperror.ErrUserNotFound
	}
	if u.Username == nil {
		return apperror.ErrUserNotFound.WithMessage("user has no username")
	}
	s.invalidateUser(*u.Username)
	return nil
}

// UnlockUser unlocks a locked user by resetting error counters.
func (s *service) UnlockUser(userId int) error {
	now := time.Now()
	if err := s.repo.UpdateUserFields(userId, map[string]interface{}{
		"last_login_time":   now,
		"last_lock_time":    nil,
		"login_error_times": 0,
	}); err != nil {
		return apperror.Wrap(err, "UNLOCK_USER_FAILED", 500, "failed to unlock user")
	}
	return nil
}

// ModifyPassword changes a user's password after validating the old one.
func (s *service) ModifyPassword(username, oldPassword, newPassword string) error {
	u, err := s.repo.FindUserByUsername(username)
	if err != nil {
		return apperror.ErrUserNotFound
	}

	// Validate old password
	if u.Password == nil {
		return apperror.ErrInvalidCredentials.WithMessage("invalid old password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*u.Password), []byte(oldPassword)); err != nil {
		return apperror.ErrInvalidCredentials.WithMessage("invalid old password")
	}

	// Validate new password strength
	if err := validatePasswordStrength(newPassword, username); err != nil {
		return err
	}

	// Check password history (last 24 passwords)
	if err := s.checkPasswordHistory(username, newPassword, 24); err != nil {
		return err
	}

	// Hash and save new password
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return apperror.Wrap(err, "PASSWORD_HASH_FAILED", 500, "failed to hash password")
	}
	hashed := string(hash)

	now := time.Now()
	if err := s.repo.UpdateUserFields(u.Id, map[string]interface{}{
		"password":             hashed,
		"password_modify_time": now,
	}); err != nil {
		return apperror.Wrap(err, "MODIFY_PASSWORD_FAILED", 500, "failed to modify password")
	}

	// Save to password history
	s.savePasswordHistory(username, hashed)

	// Force re-login
	s.invalidateUser(username)

	return nil
}

// EnableUser enables a user account.
func (s *service) EnableUser(userId int) error {
	enable := true
	if err := s.repo.UpdateUserFields(userId, map[string]interface{}{
		"enable": enable,
	}); err != nil {
		return apperror.Wrap(err, "ENABLE_USER_FAILED", 500, "failed to enable user")
	}
	return nil
}

// DisableUser disables a user account and invalidates their session.
func (s *service) DisableUser(userId int) error {
	u, err := s.repo.FindUserByID(userId)
	if err != nil {
		return apperror.ErrUserNotFound
	}

	enable := false
	if err := s.repo.UpdateUserFields(userId, map[string]interface{}{
		"enable": enable,
	}); err != nil {
		return apperror.Wrap(err, "DISABLE_USER_FAILED", 500, "failed to disable user")
	}

	// Force logout
	if u.Username != nil {
		s.invalidateUser(*u.Username)
	}
	return nil
}

// ResetPassword resets a user's password (admin action).
// Generates a new password, saves it to database, and returns the plaintext password.
// Mirrors Java SystemUserManagementServiceImpl.resetPassword (simplified - no email link).
func (s *service) ResetPassword(adminId, userId int) (string, error) {
	u, err := s.repo.FindUserByID(userId)
	if err != nil {
		return "", apperror.ErrUserNotFound
	}
	if u.Username == nil {
		return "", apperror.ErrUserNotFound.WithMessage("user has no username")
	}

	// Generate a new random password
	generatedPassword, err := GeneratePassword(12)
	if err != nil {
		return "", apperror.Wrap(err, "RESET_PASSWORD_FAILED", 500, "failed to generate password")
	}

	// Hash the new password
	hash, err := bcrypt.GenerateFromPassword([]byte(generatedPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", apperror.Wrap(err, "PASSWORD_HASH_FAILED", 500, "failed to hash password")
	}
	hashed := string(hash)

	// Update password and reset login error times
	now := time.Now()
	if err := s.repo.UpdateUserFields(userId, map[string]interface{}{
		"password":             hashed,
		"login_error_times":    0,
		"last_login_time":      now,
		"password_modify_time": nil, // Reset to nil so user must change on first login
	}); err != nil {
		return "", apperror.Wrap(err, "RESET_PASSWORD_FAILED", 500, "failed to update password")
	}

	// Force logout
	s.invalidateUser(*u.Username)

	return generatedPassword, nil
}

// ResetPasswordByLink validates a reset key and sets a new password.
func (s *service) ResetPasswordByLink(username, key, newPassword string) error {
	ctx := context.Background()
	redisKey := RedisKeyPwdReset + key

	// Validate the reset key
	storedUsername, err := redis.Get(ctx, redisKey)
	if err != nil {
		return apperror.ErrInvalidInput.WithMessage("invalid or expired reset link")
	}
	if storedUsername != username {
		return apperror.ErrInvalidInput.WithMessage("reset link does not match user")
	}

	// Delete the key (one-time use)
	_ = redis.Del(ctx, redisKey)

	// Validate password strength
	if err := validatePasswordStrength(newPassword, username); err != nil {
		return err
	}

	// Check password history (last 5 passwords for reset)
	if err := s.checkPasswordHistory(username, newPassword, 5); err != nil {
		return err
	}

	// Hash and save
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return apperror.Wrap(err, "PASSWORD_HASH_FAILED", 500, "failed to hash password")
	}
	hashed := string(hash)

	now := time.Now()
	u, err := s.repo.FindUserByUsername(username)
	if err != nil {
		return apperror.ErrUserNotFound
	}
	if err := s.repo.UpdateUserFields(u.Id, map[string]interface{}{
		"password":             hashed,
		"password_modify_time": now,
		"last_login_time":      now,
	}); err != nil {
		return apperror.Wrap(err, "RESET_PASSWORD_FAILED", 500, "failed to reset password")
	}

	s.savePasswordHistory(username, hashed)

	return nil
}

// SetTenancyForUser updates a user's license/tenancy and forces re-login.
func (s *service) SetTenancyForUser(userId, licenseId int) error {
	u, err := s.repo.FindUserByID(userId)
	if err != nil {
		return apperror.ErrUserNotFound
	}

	if err := s.repo.UpdateUserFields(userId, map[string]interface{}{
		"license_id": licenseId,
	}); err != nil {
		return apperror.Wrap(err, "SET_TENANCY_FAILED", 500, "failed to set tenancy")
	}

	// Force re-login with new tenancy
	if u.Username != nil {
		s.invalidateUser(*u.Username)
	}
	return nil
}

// GetLoginFailedTimes returns the failed login count for a user.
func (s *service) GetLoginFailedTimes(userId int) (*LoginFailedTimesResponse, error) {
	u, err := s.repo.FindUserByID(userId)
	if err != nil {
		return nil, apperror.ErrUserNotFound
	}

	maxFailed := DefaultMaxLoginFailedTimes
	failedTime := 0
	if u.LoginErrorTimes != nil {
		failedTime = *u.LoginErrorTimes
	}

	// If already at max, return 0 (lock has been applied)
	if failedTime >= maxFailed {
		failedTime = 0
	}

	return &LoginFailedTimesResponse{
		MaxFailedTime: maxFailed,
		FailedTime:    failedTime,
	}, nil
}

// NeedChangePassword checks if a user needs to change their password.
func (s *service) NeedChangePassword(userId int) (*NeedChangePasswordResponse, error) {
	u, err := s.repo.FindUserByID(userId)
	if err != nil {
		return nil, apperror.ErrUserNotFound
	}

	// Admin users don't need to change
	if u.Username != nil && *u.Username == "admin" {
		return &NeedChangePasswordResponse{NeedChange: false}, nil
	}

	// Reason 1: passwordModifyTime is null (never changed / initial password)
	if u.PasswordModifyTime == nil {
		return &NeedChangePasswordResponse{NeedChange: true, Reason: 1}, nil
	}

	// Reason 3: password is older than PASSWORD_EXPIRED_DAYS (180 days)
	daysSinceChange := time.Since(*u.PasswordModifyTime).Hours() / 24
	if daysSinceChange > PasswordExpiredDays {
		return &NeedChangePasswordResponse{NeedChange: true, Reason: 3}, nil
	}

	return &NeedChangePasswordResponse{NeedChange: false}, nil
}

// ---------------------------------------------------------------------------
// Role
// ---------------------------------------------------------------------------

// ListRoles returns all roles for the given license.
func (s *service) ListRoles(licenseId int) ([]Role, error) {
	roles, err := s.repo.FindRoles(licenseId)
	if err != nil {
		return nil, apperror.Wrap(err, "LIST_ROLES_FAILED", 500, "failed to list roles")
	}
	return roles, nil
}

// CreateRole persists a new role.
func (s *service) CreateRole(r *Role) error {
	if err := s.repo.CreateRole(r); err != nil {
		return apperror.Wrap(err, "CREATE_ROLE_FAILED", 500, "failed to create role")
	}
	return nil
}

// UpdateRole persists changes to an existing role.
func (s *service) UpdateRole(r *Role) error {
	if err := s.repo.UpdateRole(r); err != nil {
		return apperror.Wrap(err, "UPDATE_ROLE_FAILED", 500, "failed to update role")
	}
	return nil
}

// DeleteRole removes a role by ID (string UUID).
func (s *service) DeleteRole(id string) error {
	if err := s.repo.DeleteRole(id); err != nil {
		return apperror.Wrap(err, "DELETE_ROLE_FAILED", 500, "failed to delete role")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Role Permissions
// ---------------------------------------------------------------------------

// GetRolePermissions returns all permission associations for a role.
func (s *service) GetRolePermissions(roleId string) ([]RoleHasPermission, error) {
	perms, err := s.repo.FindPermissionsByRoleId(roleId)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_ROLE_PERMISSIONS_FAILED", 500, "failed to get role permissions")
	}
	return perms, nil
}

// UpdateRolePermissions replaces the permission set for a role.
func (s *service) UpdateRolePermissions(roleId string, permissionIds []string) error {
	if err := s.repo.SavePermissions(roleId, permissionIds); err != nil {
		return apperror.Wrap(err, "UPDATE_ROLE_PERMISSIONS_FAILED", 500, "failed to update role permissions")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Role Name Resolution (for JWT claims)
// ---------------------------------------------------------------------------

// GetRoleNamesForUser returns the role names for a given user.
// Mirrors Java RoleManagementServiceImpl: query user_has_role by userId,
// then load roles by id IN (roleIds) — NOT by tenancy/license filter.
func (s *service) GetRoleNamesForUser(userId int, licenseId int) ([]string, error) {
	userRoles, err := s.repo.FindUserRoles(userId)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_ROLE_NAMES_FAILED", 500, "failed to get user roles")
	}
	if len(userRoles) == 0 {
		return nil, nil
	}

	roleIds := make([]string, 0, len(userRoles))
	for _, ur := range userRoles {
		roleIds = append(roleIds, ur.RoleId)
	}

	roles, err := s.repo.FindRolesByIds(roleIds)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_ROLE_NAMES_FAILED", 500, "failed to get roles")
	}

	// Build lookup map: role ID → role name
	roleMap := make(map[string]string, len(roles))
	for _, r := range roles {
		if r.RoleName != nil {
			roleMap[r.Id] = *r.RoleName
		}
	}

	var names []string
	for _, roleId := range roleIds {
		if name, ok := roleMap[roleId]; ok {
			names = append(names, name)
		}
	}
	return names, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// invalidateUser removes the user's JWT login key from Redis, effectively
// forcing them to re-authenticate on their next request.
func (s *service) invalidateUser(username string) {
	ctx := context.Background()
	loginKey := RedisKeyJWTLogin + username
	if err := redis.Del(ctx, loginKey); err != nil {
		logger.Warnf("invalidateUser: failed to delete login key for %s: %v", username, err)
	}
}

// checkPasswordHistory checks if the new password matches any of the last N passwords.
func (s *service) checkPasswordHistory(username, newPassword string, limit int) error {
	history, err := s.repo.FindRecentPasswords(username, limit)
	if err != nil {
		logger.Warnf("checkPasswordHistory: %v", err)
		return nil // Don't block on error
	}
	for _, h := range history {
		if h.Password != nil {
			if err := bcrypt.CompareHashAndPassword([]byte(*h.Password), []byte(newPassword)); err == nil {
				return apperror.ErrInvalidInput.WithMessage("new password cannot be the same as a recent password")
			}
		}
	}
	return nil
}

// savePasswordHistory saves a password hash to the history table.
func (s *service) savePasswordHistory(username, hashedPassword string) {
	now := time.Now()
	h := PasswordHistory{
		Username:   &username,
		Password:   &hashedPassword,
		CreateTime: &now,
	}
	if err := s.repo.CreatePasswordHistory(&h); err != nil {
		logger.Warnf("savePasswordHistory: %v", err)
	}
}

// validatePasswordStrength checks password complexity requirements.
func validatePasswordStrength(password, username string) error {
	if len(password) < 8 {
		return apperror.ErrInvalidInput.WithMessage("password must be at least 8 characters")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return apperror.ErrInvalidInput.WithMessage("password must contain uppercase, lowercase, digit, and special character")
	}

	// Password cannot contain username
	if username != "" && len(username) >= 3 {
		lowerPwd := toLower(password)
		lowerUser := toLower(username)
		if contains(lowerPwd, lowerUser) {
			return apperror.ErrInvalidInput.WithMessage("password cannot contain username")
		}
	}

	// No more than 2 consecutive same characters
	for i := 0; i < len(password)-2; i++ {
		if password[i] == password[i+1] && password[i+1] == password[i+2] {
			return apperror.ErrInvalidInput.WithMessage("password cannot have more than 2 consecutive same characters")
		}
	}

	return nil
}

// generateRandomString generates a random hex string of given length.
func generateRandomString(n int) string {
	b := make([]byte, n/2+1)
	// Use crypto/rand for better randomness
	_, _ = randRead(b)
	return encodeHex(b)[:n]
}

// intPtr returns a pointer to an int.
func intPtr(v int) *int {
	return &v
}

// toLower converts ASCII string to lowercase.
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// contains checks if s contains substr (simple implementation).
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// encodeHex encodes bytes to hex string.
func encodeHex(b []byte) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, len(b)*2)
	for i, v := range b {
		result[i*2] = hexChars[v>>4]
		result[i*2+1] = hexChars[v&0x0f]
	}
	return string(result)
}
