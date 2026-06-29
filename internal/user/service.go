package user

import (
	"context"
	"errors"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// Service contains the business logic for user management.
type Service struct {
	repo *Repository
	db   *gorm.DB
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db), db: db}
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

// Login validates credentials and returns the user on success.
func (s *Service) Login(username, password string) (*SysUser, error) {
	u, err := s.repo.FindUserByUsername(username)
	if err != nil {
		return nil, errors.New("invalid username or password")
	}

	if u.Password == nil {
		return nil, errors.New("invalid username or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*u.Password), []byte(password)); err != nil {
		return nil, errors.New("invalid username or password")
	}

	return u, nil
}

// Logout invalidates the current JWT token.
// 1. Delete SECURITY_JWT_LOGIN:{username}
// 2. Add JWT to blacklist SECURITY_JWT_BLACK:{jwt} with TTL
func (s *Service) Logout(username, jwtToken string) error {
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
func (s *Service) RecordLogin(username, ip string, licenseId int, result int) error {
	now := time.Now()
	log := LoginLog{
		Username:  &username,
		IpAddress: &ip,
		LoginTime: &now,
		Result:    &result,
		LicenseId: &licenseId,
	}
	return s.repo.CreateLoginLog(&log)
}

// RecordLogout creates a logout log entry.
func (s *Service) RecordLogout(username, ip string, licenseId int) error {
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
	return s.repo.CreateLoginLog(&log)
}

// ---------------------------------------------------------------------------
// SysUser CRUD
// ---------------------------------------------------------------------------

// ListUsers returns a paginated user list.
func (s *Service) ListUsers(licenseId int, page, pageSize int) ([]SysUser, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindUsers(licenseId, offset, pageSize)
}

// CreateUser hashes the password and persists a new user.
func (s *Service) CreateUser(u *SysUser) error {
	if u.Password != nil && *u.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*u.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		hashed := string(hash)
		u.Password = &hashed
	}
	// Set defaults
	if u.Enable == nil {
		enabled := true
		u.Enable = &enabled
	}
	if u.LoginErrorTimes == nil {
		zero := 0
		u.LoginErrorTimes = &zero
	}
	return s.repo.CreateUser(u)
}

// UpdateUser persists changes to an existing user. If the password field has
// changed it is re-hashed before saving.
func (s *Service) UpdateUser(u *SysUser) error {
	if u.Password != nil && *u.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*u.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		hashed := string(hash)
		u.Password = &hashed
	}
	return s.repo.UpdateUser(u)
}

// DeleteUser removes a user by ID and invalidates their session.
func (s *Service) DeleteUser(id int) error {
	u, err := s.repo.FindUserByID(id)
	if err != nil {
		return err
	}
	// Invalidate session before deleting
	if u.Username != nil {
		s.invalidateUser(*u.Username)
	}
	return s.repo.DeleteUser(id)
}

// ---------------------------------------------------------------------------
// User Management Features
// ---------------------------------------------------------------------------

// KickOutUser forces a user to logout by removing their JWT login key.
func (s *Service) KickOutUser(userId int) error {
	u, err := s.repo.FindUserByID(userId)
	if err != nil {
		return err
	}
	if u.Username == nil {
		return errors.New("user has no username")
	}
	s.invalidateUser(*u.Username)
	return nil
}

// UnlockUser unlocks a locked user by resetting error counters.
func (s *Service) UnlockUser(userId int) error {
	now := time.Now()
	return s.repo.UpdateUserFields(userId, map[string]interface{}{
		"last_login_time":   now,
		"last_lock_time":    nil,
		"login_error_times": 0,
	})
}

// ModifyPassword changes a user's password after validating the old one.
func (s *Service) ModifyPassword(username, oldPassword, newPassword string) error {
	u, err := s.repo.FindUserByUsername(username)
	if err != nil {
		return errors.New("user not found")
	}

	// Validate old password
	if u.Password == nil {
		return errors.New("invalid old password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*u.Password), []byte(oldPassword)); err != nil {
		return errors.New("invalid old password")
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
		return err
	}
	hashed := string(hash)

	now := time.Now()
	if err := s.repo.UpdateUserFields(u.Id, map[string]interface{}{
		"password":             hashed,
		"password_modify_time": now,
	}); err != nil {
		return err
	}

	// Save to password history
	s.savePasswordHistory(username, hashed)

	// Force re-login
	s.invalidateUser(username)

	return nil
}

// EnableUser enables a user account.
func (s *Service) EnableUser(userId int) error {
	enable := true
	return s.repo.UpdateUserFields(userId, map[string]interface{}{
		"enable": enable,
	})
}

// DisableUser disables a user account and invalidates their session.
func (s *Service) DisableUser(userId int) error {
	u, err := s.repo.FindUserByID(userId)
	if err != nil {
		return err
	}

	enable := false
	if err := s.repo.UpdateUserFields(userId, map[string]interface{}{
		"enable": enable,
	}); err != nil {
		return err
	}

	// Force logout
	if u.Username != nil {
		s.invalidateUser(*u.Username)
	}
	return nil
}

// ResetPassword resets a user's password (admin action).
// Returns a reset key that can be used to construct a reset link.
func (s *Service) ResetPassword(adminId, userId int) (string, error) {
	u, err := s.repo.FindUserByID(userId)
	if err != nil {
		return "", err
	}
	if u.Username == nil {
		return "", errors.New("user has no username")
	}

	// Generate a random reset key
	resetKey := generateRandomString(32)

	// Store in Redis with 5-minute TTL
	ctx := context.Background()
	redisKey := RedisKeyPwdReset + resetKey
	if err := redis.Set(ctx, redisKey, *u.Username, 5*time.Minute); err != nil {
		return "", err
	}

	// Reset login error times and update last login time
	now := time.Now()
	_ = s.repo.UpdateUserFields(userId, map[string]interface{}{
		"login_error_times": 0,
		"last_login_time":   now,
	})

	// Force logout
	s.invalidateUser(*u.Username)

	return resetKey, nil
}

// ResetPasswordByLink validates a reset key and sets a new password.
func (s *Service) ResetPasswordByLink(username, key, newPassword string) error {
	ctx := context.Background()
	redisKey := RedisKeyPwdReset + key

	// Validate the reset key
	storedUsername, err := redis.Get(ctx, redisKey)
	if err != nil {
		return errors.New("invalid or expired reset link")
	}
	if storedUsername != username {
		return errors.New("reset link does not match user")
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
		return err
	}
	hashed := string(hash)

	now := time.Now()
	u, err := s.repo.FindUserByUsername(username)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateUserFields(u.Id, map[string]interface{}{
		"password":             hashed,
		"password_modify_time": now,
		"last_login_time":      now,
	}); err != nil {
		return err
	}

	s.savePasswordHistory(username, hashed)

	return nil
}

// SetTenancyForUser updates a user's license/tenancy and forces re-login.
func (s *Service) SetTenancyForUser(userId, licenseId int) error {
	u, err := s.repo.FindUserByID(userId)
	if err != nil {
		return err
	}

	if err := s.repo.UpdateUserFields(userId, map[string]interface{}{
		"license_id": licenseId,
	}); err != nil {
		return err
	}

	// Force re-login with new tenancy
	if u.Username != nil {
		s.invalidateUser(*u.Username)
	}
	return nil
}

// GetLoginFailedTimes returns the failed login count for a user.
func (s *Service) GetLoginFailedTimes(userId int) (*LoginFailedTimesResponse, error) {
	u, err := s.repo.FindUserByID(userId)
	if err != nil {
		return nil, err
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
func (s *Service) NeedChangePassword(userId int) (*NeedChangePasswordResponse, error) {
	u, err := s.repo.FindUserByID(userId)
	if err != nil {
		return nil, err
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
func (s *Service) ListRoles(licenseId int) ([]Role, error) {
	return s.repo.FindRoles(licenseId)
}

// CreateRole persists a new role.
func (s *Service) CreateRole(r *Role) error {
	return s.repo.CreateRole(r)
}

// UpdateRole persists changes to an existing role.
func (s *Service) UpdateRole(r *Role) error {
	return s.repo.UpdateRole(r)
}

// DeleteRole removes a role by ID.
func (s *Service) DeleteRole(id int) error {
	return s.repo.DeleteRole(id)
}

// ---------------------------------------------------------------------------
// Role Permissions
// ---------------------------------------------------------------------------

// GetRolePermissions returns all permission associations for a role.
func (s *Service) GetRolePermissions(roleId int) ([]RoleHasPermission, error) {
	return s.repo.FindPermissionsByRoleId(roleId)
}

// UpdateRolePermissions replaces the permission set for a role.
func (s *Service) UpdateRolePermissions(roleId int, permissionIds []string) error {
	return s.repo.SavePermissions(roleId, permissionIds)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// invalidateUser removes the user's JWT login key from Redis, effectively
// forcing them to re-authenticate on their next request.
func (s *Service) invalidateUser(username string) {
	ctx := context.Background()
	loginKey := RedisKeyJWTLogin + username
	if err := redis.Del(ctx, loginKey); err != nil {
		logger.Warnf("invalidateUser: failed to delete login key for %s: %v", username, err)
	}
}

// checkPasswordHistory checks if the new password matches any of the last N passwords.
func (s *Service) checkPasswordHistory(username, newPassword string, limit int) error {
	history, err := s.repo.FindRecentPasswords(username, limit)
	if err != nil {
		logger.Warnf("checkPasswordHistory: %v", err)
		return nil // Don't block on error
	}
	for _, h := range history {
		if h.Password != nil {
			if err := bcrypt.CompareHashAndPassword([]byte(*h.Password), []byte(newPassword)); err == nil {
				return errors.New("new password cannot be the same as a recent password")
			}
		}
	}
	return nil
}

// savePasswordHistory saves a password hash to the history table.
func (s *Service) savePasswordHistory(username, hashedPassword string) {
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
		return errors.New("password must be at least 8 characters")
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
		return errors.New("password must contain uppercase, lowercase, digit, and special character")
	}

	// Password cannot contain username
	if username != "" && len(username) >= 3 {
		lowerPwd := toLower(password)
		lowerUser := toLower(username)
		if contains(lowerPwd, lowerUser) {
			return errors.New("password cannot contain username")
		}
	}

	// No more than 2 consecutive same characters
	for i := 0; i < len(password)-2; i++ {
		if password[i] == password[i+1] && password[i+1] == password[i+2] {
			return errors.New("password cannot have more than 2 consecutive same characters")
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
