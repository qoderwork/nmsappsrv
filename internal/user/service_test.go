package user

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

// ---------------------------------------------------------------------------
// mockRepository -- implements Repository for unit testing the service layer.
// Only the methods needed by the tests are wired up via function fields;
// all others panic so accidental calls surface immediately.
// ---------------------------------------------------------------------------

type mockRepository struct {
	findUserByUsernameFn func(string) (*SysUser, error)
	findUserByIDFn      func(int) (*SysUser, error)
	findUsersFn         func(int, int, bool, string) ([]SysUser, int64, error)
	createUserFn        func(*SysUser) error
	updateUserFieldsFn  func(int, map[string]interface{}) error
	findUserRolesFn     func(int) ([]UserHasRole, error)
}

func (m *mockRepository) FindUserByUsername(username string) (*SysUser, error) {
	if m.findUserByUsernameFn != nil {
		return m.findUserByUsernameFn(username)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRepository) FindUserByID(id int) (*SysUser, error) {
	if m.findUserByIDFn != nil {
		return m.findUserByIDFn(id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRepository) FindUsers(offset, limit int, excludeAdmin bool, creatorName string) ([]SysUser, int64, error) {
	if m.findUsersFn != nil {
		return m.findUsersFn(offset, limit, excludeAdmin, creatorName)
	}
	return nil, 0, errors.New("not implemented")
}

func (m *mockRepository) CreateUser(u *SysUser) error {
	if m.createUserFn != nil {
		return m.createUserFn(u)
	}
	return errors.New("not implemented")
}

func (m *mockRepository) UpdateUser(u *SysUser) error { panic("not implemented") }
func (m *mockRepository) DeleteUser(id int) error      { panic("not implemented") }

func (m *mockRepository) UpdateUserFields(id int, fields map[string]interface{}) error {
	if m.updateUserFieldsFn != nil {
		return m.updateUserFieldsFn(id, fields)
	}
	panic("not implemented")
}

func (m *mockRepository) FindRoles(tenantId int) ([]Role, error) {
	panic("not implemented")
}
func (m *mockRepository) FindRolesByIds(roleIds []string) ([]Role, error) {
	panic("not implemented")
}
func (m *mockRepository) CreateRole(role *Role) error { panic("not implemented") }
func (m *mockRepository) UpdateRole(role *Role) error { panic("not implemented") }
func (m *mockRepository) DeleteRole(id string) error  { panic("not implemented") }
func (m *mockRepository) FindPermissionsByRoleId(roleId string) ([]RoleHasPermission, error) {
	panic("not implemented")
}
func (m *mockRepository) SavePermissions(roleId string, permissionIds []string) error {
	panic("not implemented")
}
func (m *mockRepository) FindUserRoles(userId int) ([]UserHasRole, error) {
	if m.findUserRolesFn != nil {
		return m.findUserRolesFn(userId)
	}
	// Default: user has a role, so the login no-role gate passes.
	return []UserHasRole{{UserId: userId, RoleId: "admin_id"}}, nil
}
func (m *mockRepository) SaveUserRoles(userId int, roleIds []string) error {
	panic("not implemented")
}
func (m *mockRepository) CreateLoginLog(log *LoginLog) error { panic("not implemented") }
func (m *mockRepository) CreatePasswordHistory(h *PasswordHistory) error {
	panic("not implemented")
}
func (m *mockRepository) FindRecentPasswords(username string, limit int) ([]PasswordHistory, error) {
	panic("not implemented")
}
func (m *mockRepository) CountUsersByTenantId(tenantId int) (int64, error) {
	panic("not implemented")
}
func (m *mockRepository) TenantExists(id int) bool {
	return true
}
func (m *mockRepository) FindUsersByCreatorId(creatorId int) ([]SysUser, error) {
	panic("not implemented")
}
func (m *mockRepository) UpdateLastLoginTime(username string, t time.Time) error {
	panic("not implemented")
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestService creates a Service backed by the given mock Repository.
// Needed because the concrete service type is unexported.
func newTestService(repo Repository) Service {
	return &service{repo: repo}
}

// strPtr returns a pointer to the given string (test convenience).
func strPtr(s string) *string { return &s }

// boolPtr returns a pointer to the given bool (test convenience).
func boolPtr(b bool) *bool { return &b }

// ---------------------------------------------------------------------------
// Tests: Login
// ---------------------------------------------------------------------------

func TestService_Login(t *testing.T) {
	// Pre-compute a bcrypt hash for the correct password.
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	assert.NoError(t, err)
	hashedPwd := string(hash)

	testUser := &SysUser{
		Id:       1,
		Username: strPtr("testuser"),
		Password: &hashedPwd,
	}

	t.Run("correct password returns user", func(t *testing.T) {
		repo := &mockRepository{
			findUserByUsernameFn: func(username string) (*SysUser, error) {
				assert.Equal(t, "testuser", username)
				return testUser, nil
			},
			updateUserFieldsFn: func(int, map[string]interface{}) error { return nil },
		}
		svc := newTestService(repo)

		u, err := svc.Login("testuser", "correct-password")
		assert.NoError(t, err)
		assert.NotNil(t, u)
		assert.Equal(t, 1, u.Id)
		assert.Equal(t, "testuser", *u.Username)
	})

	t.Run("wrong password returns error", func(t *testing.T) {
		repo := &mockRepository{
			findUserByUsernameFn: func(username string) (*SysUser, error) {
				return testUser, nil
			},
			updateUserFieldsFn: func(int, map[string]interface{}) error { return nil },
		}
		svc := newTestService(repo)

		u, err := svc.Login("testuser", "wrong-password")
		assert.Error(t, err)
		assert.Nil(t, u)
		assert.Contains(t, err.Error(), "invalid username or password")
	})

	t.Run("user not found returns error", func(t *testing.T) {
		repo := &mockRepository{
			findUserByUsernameFn: func(username string) (*SysUser, error) {
				return nil, errors.New("record not found")
			},
		}
		svc := newTestService(repo)

		u, err := svc.Login("nonexistent", "any-password")
		assert.Error(t, err)
		assert.Nil(t, u)
		assert.Contains(t, err.Error(), "invalid username or password")
	})

	t.Run("nil password field returns error", func(t *testing.T) {
		userNoPwd := &SysUser{
			Id:       2,
			Username: strPtr("nopwd"),
			Password: nil,
		}
		repo := &mockRepository{
			findUserByUsernameFn: func(username string) (*SysUser, error) {
				return userNoPwd, nil
			},
		}
		svc := newTestService(repo)

		u, err := svc.Login("nopwd", "any-password")
		assert.Error(t, err)
		assert.Nil(t, u)
		assert.Contains(t, err.Error(), "invalid username or password")
	})

	t.Run("disabled account returns ErrUserDisabled", func(t *testing.T) {
		disabledUser := &SysUser{
			Id:       3,
			Username: strPtr("disabled"),
			Password: &hashedPwd,
			Enable:   boolPtr(false),
		}
		repo := &mockRepository{
			findUserByUsernameFn: func(username string) (*SysUser, error) {
				return disabledUser, nil
			},
		}
		svc := newTestService(repo)

		u, err := svc.Login("disabled", "correct-password")
		assert.Error(t, err)
		assert.Nil(t, u)
		assert.Contains(t, err.Error(), "account is disabled")
	})

	t.Run("user with no roles returns ErrUserNoRole", func(t *testing.T) {
		userNoRole := &SysUser{
			Id:       8,
			Username: strPtr("norole"),
			Password: &hashedPwd,
		}
		repo := &mockRepository{
			findUserByUsernameFn: func(username string) (*SysUser, error) {
				return userNoRole, nil
			},
			findUserRolesFn: func(userId int) ([]UserHasRole, error) {
				return []UserHasRole{}, nil // no roles assigned
			},
		}
		svc := newTestService(repo)

		u, err := svc.Login("norole", "correct-password")
		assert.Error(t, err)
		assert.Nil(t, u)
		assert.Contains(t, err.Error(), "no role assigned")
	})

	t.Run("inactive user (90+ days) returns ErrUserInactive", func(t *testing.T) {
		old := time.Now().Add(-(UserLockedDays + 1) * 24 * time.Hour)
		inactiveUser := &SysUser{
			Id:            9,
			Username:      strPtr("inactive"),
			Password:      &hashedPwd,
			LastLoginTime: &old,
		}
		repo := &mockRepository{
			findUserByUsernameFn: func(username string) (*SysUser, error) {
				return inactiveUser, nil
			},
		}
		svc := newTestService(repo)

		u, err := svc.Login("inactive", "correct-password")
		assert.Error(t, err)
		assert.Nil(t, u)
		assert.Contains(t, err.Error(), "inactive")
	})

	t.Run("locked account within window returns ErrUserLocked", func(t *testing.T) {
		recent := time.Now().Add(-time.Minute) // 1 min ago, inside the 30-min window
		lockedUser := &SysUser{
			Id:              4,
			Username:        strPtr("locked"),
			Password:        &hashedPwd,
			LoginErrorTimes: intPtr(DefaultMaxLoginFailedTimes),
			LastLockTime:    &recent,
		}
		repo := &mockRepository{
			findUserByUsernameFn: func(username string) (*SysUser, error) {
				return lockedUser, nil
			},
		}
		svc := newTestService(repo)

		u, err := svc.Login("locked", "correct-password")
		assert.Error(t, err)
		assert.Nil(t, u)
		assert.Contains(t, err.Error(), "account is locked")
	})

	t.Run("expired lock auto-unlocks and logs in", func(t *testing.T) {
		expired := time.Now().Add(-(UserLockMinutes + 1) * time.Minute)
		lockedUser := &SysUser{
			Id:              5,
			Username:        strPtr("expiredlock"),
			Password:        &hashedPwd,
			LoginErrorTimes: intPtr(DefaultMaxLoginFailedTimes),
			LastLockTime:    &expired,
		}
		var captured map[string]interface{}
		repo := &mockRepository{
			findUserByUsernameFn: func(username string) (*SysUser, error) {
				return lockedUser, nil
			},
			updateUserFieldsFn: func(id int, fields map[string]interface{}) error {
				captured = fields
				return nil
			},
		}
		svc := newTestService(repo)

		u, err := svc.Login("expiredlock", "correct-password")
		assert.NoError(t, err)
		assert.NotNil(t, u)
		// The final (success) write must have reset the counter.
		assert.Equal(t, 0, captured["login_error_times"])
	})

	t.Run("wrong password increments error counter", func(t *testing.T) {
		user := &SysUser{
			Id:              6,
			Username:        strPtr("incrementme"),
			Password:        &hashedPwd,
			LoginErrorTimes: intPtr(2),
		}
		var captured map[string]interface{}
		repo := &mockRepository{
			findUserByUsernameFn: func(username string) (*SysUser, error) {
				return user, nil
			},
			updateUserFieldsFn: func(id int, fields map[string]interface{}) error {
				captured = fields
				return nil
			},
		}
		svc := newTestService(repo)

		u, err := svc.Login("incrementme", "wrong-password")
		assert.Error(t, err)
		assert.Nil(t, u)
		assert.Equal(t, 3, captured["login_error_times"])
		// Threshold not reached yet -> account must NOT be locked.
		_, hasLock := captured["last_lock_time"]
		assert.False(t, hasLock)
	})

	t.Run("wrong password at threshold locks account", func(t *testing.T) {
		user := &SysUser{
			Id:              7,
			Username:        strPtr("lockme"),
			Password:        &hashedPwd,
			LoginErrorTimes: intPtr(DefaultMaxLoginFailedTimes - 1),
		}
		var captured map[string]interface{}
		repo := &mockRepository{
			findUserByUsernameFn: func(username string) (*SysUser, error) {
				return user, nil
			},
			updateUserFieldsFn: func(id int, fields map[string]interface{}) error {
				captured = fields
				return nil
			},
		}
		svc := newTestService(repo)

		_, err := svc.Login("lockme", "wrong-password")
		assert.Error(t, err)
		assert.Equal(t, DefaultMaxLoginFailedTimes, captured["login_error_times"])
		// Reaching the threshold must engage the lock.
		_, hasLock := captured["last_lock_time"]
		assert.True(t, hasLock)
	})
}

// ---------------------------------------------------------------------------
// Tests: CreateUser
// ---------------------------------------------------------------------------

func TestService_CreateUser(t *testing.T) {
	t.Run("hashes password and sets defaults", func(t *testing.T) {
		var captured *SysUser
		repo := &mockRepository{
			createUserFn: func(u *SysUser) error {
				captured = u
				return nil
			},
		}
		svc := newTestService(repo)

		plainPwd := "plain-password"
		u := &SysUser{
			Username: strPtr("newuser"),
			Password: &plainPwd,
		}

		generated, err := svc.CreateUser(u)
		assert.NoError(t, err)
		assert.Equal(t, "", generated) // caller supplied a password -> nothing generated
		assert.NotNil(t, captured)

		// Password should have been hashed, not the original plaintext.
		assert.NotEqual(t, "plain-password", *captured.Password)
		// The stored hash should validate against the original password.
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(*captured.Password), []byte("plain-password")))

		// Enable should default to true.
		assert.NotNil(t, captured.Enable)
		assert.True(t, *captured.Enable)

		// LoginErrorTimes should default to 0.
		assert.NotNil(t, captured.LoginErrorTimes)
		assert.Equal(t, 0, *captured.LoginErrorTimes)
	})

	t.Run("preserves existing Enable value", func(t *testing.T) {
		var captured *SysUser
		repo := &mockRepository{
			createUserFn: func(u *SysUser) error {
				captured = u
				return nil
			},
		}
		svc := newTestService(repo)

		u := &SysUser{
			Username: strPtr("newuser2"),
			Password: strPtr("pass"),
			Enable:   boolPtr(false),
		}

		_, err := svc.CreateUser(u)
		assert.NoError(t, err)
		assert.NotNil(t, captured.Enable)
		assert.False(t, *captured.Enable)
	})

	t.Run("empty password is auto-generated and hashed", func(t *testing.T) {
		var captured *SysUser
		repo := &mockRepository{
			createUserFn: func(u *SysUser) error {
				captured = u
				return nil
			},
		}
		svc := newTestService(repo)

		emptyPwd := ""
		u := &SysUser{
			Username: strPtr("emptyuser"),
			Password: &emptyPwd,
		}

		generated, err := svc.CreateUser(u)
		assert.NoError(t, err)
		assert.NotEmpty(t, generated) // a password was auto-generated
		// The stored password must be the bcrypt hash of the generated one.
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(*captured.Password), []byte(generated)))
	})

	t.Run("repo error is propagated", func(t *testing.T) {
		repo := &mockRepository{
			createUserFn: func(u *SysUser) error {
				return errors.New("db connection lost")
			},
		}
		svc := newTestService(repo)

		u := &SysUser{Username: strPtr("failuser"), Password: strPtr("pass")}
		_, err := svc.CreateUser(u)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "db connection lost")
	})
}

// ---------------------------------------------------------------------------
// Tests: ListUsers
// ---------------------------------------------------------------------------

func TestService_ListUsers(t *testing.T) {
	var capturedOffset, capturedLimit int
	var capturedExcludeAdmin bool
	var capturedCreatorName string

	repo := &mockRepository{
		findUsersFn: func(offset, limit int, excludeAdmin bool, creatorName string) ([]SysUser, int64, error) {
			capturedOffset = offset
			capturedLimit = limit
			capturedExcludeAdmin = excludeAdmin
			capturedCreatorName = creatorName
			return []SysUser{{Id: 1}, {Id: 2}}, int64(100), nil
		},
	}
	svc := newTestService(repo)

	t.Run("page 1 with pageSize 10", func(t *testing.T) {
		users, total, err := svc.ListUsers(1, 10, true, "")
		assert.NoError(t, err)
		assert.Equal(t, 0, capturedOffset) // (1-1)*10
		assert.Equal(t, 10, capturedLimit)
		assert.True(t, capturedExcludeAdmin)
		assert.Equal(t, "", capturedCreatorName)
		assert.Len(t, users, 2)
		assert.Equal(t, int64(100), total)
	})

	t.Run("page 3 with pageSize 20 passes creator filter", func(t *testing.T) {
		_, _, err := svc.ListUsers(3, 20, true, "bob")
		assert.NoError(t, err)
		assert.Equal(t, 40, capturedOffset) // (3-1)*20
		assert.Equal(t, 20, capturedLimit)
		assert.Equal(t, "bob", capturedCreatorName)
	})

	t.Run("page less than 1 defaults to 1", func(t *testing.T) {
		_, _, err := svc.ListUsers(0, 10, true, "")
		assert.NoError(t, err)
		assert.Equal(t, 0, capturedOffset)
	})

	t.Run("pageSize less than 1 defaults to 20", func(t *testing.T) {
		_, _, err := svc.ListUsers(1, 0, true, "")
		assert.NoError(t, err)
		assert.Equal(t, 20, capturedLimit)
	})

	t.Run("negative page and pageSize use defaults", func(t *testing.T) {
		_, _, err := svc.ListUsers(-5, -1, true, "")
		assert.NoError(t, err)
		assert.Equal(t, 0, capturedOffset) // page->1, (1-1)*20
		assert.Equal(t, 20, capturedLimit) // pageSize->20
	})
}
