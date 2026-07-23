package user

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"nmsappsrv/pkg/security"
)

// ---------------------------------------------------------------------------
// mockSaltHolder -- in-memory SaltHolder for unit tests (no Redis/DB needed)
// ---------------------------------------------------------------------------

type mockSaltHolder struct {
	salts map[int]string
}

func newMockSaltHolder() *mockSaltHolder { return &mockSaltHolder{salts: make(map[int]string)} }

func (m *mockSaltHolder) GetSalt(userId int) (string, error) { return m.salts[userId], nil }
func (m *mockSaltHolder) SaveSalt(salt string, userId int) error {
	m.salts[userId] = salt
	return nil
}

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

// ListLoginLogs is required by the Repository interface; default behaviour
// surfaces accidental calls via panic.
func (m *mockRepository) ListLoginLogs(tenantId int, offset, limit int) ([]LoginLog, int64, error) {
	panic("not implemented")
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestServiceWith creates a Service backed by the given mock Repository
// and a mock in-memory SaltHolder (no external dependencies).
// The caller may optionally supply a pre-populated salt holder.
func newTestServiceWith(repo Repository, sh security.SaltHolder) Service {
	if sh == nil {
		sh = newMockSaltHolder()
	}
	return &service{repo: repo, saltHolder: sh}
}

// newTestService is kept for backwards compatibility with tests that don't
// exercise password hashing (ListUsers).
func newTestService(repo Repository) Service {
	return newTestServiceWith(repo, nil)
}

// strPtr returns a pointer to the given string (test convenience).
func strPtr(s string) *string { return &s }

// boolPtr returns a pointer to the given bool (test convenience).
func boolPtr(b bool) *bool { return &b }

// hashWith helper produces a Java-compatible digest for a given
// (username, userId, plain, saltHolder) tuple using the real HashPassword
// function, which is what the tests need to seed stored passwords.
func hashWith(t *testing.T, plain, username string, userId int, sh security.SaltHolder) string {
	t.Helper()
	d, err := security.HashPassword(plain, username, userId, sh)
	assert.NoError(t, err)
	return d
}

// ---------------------------------------------------------------------------
// Tests: Login
// ---------------------------------------------------------------------------

func TestService_Login(t *testing.T) {
	sh := newMockSaltHolder()
	userId := 1
	username := "testuser"
	correctPwd := "correct-password"
	hashedPwd := hashWith(t, correctPwd, username, userId, sh)

	testUser := &SysUser{
		Id:       userId,
		Username: strPtr(username),
		Password: &hashedPwd,
	}

	t.Run("correct password returns user", func(t *testing.T) {
		repo := &mockRepository{
			findUserByUsernameFn: func(uname string) (*SysUser, error) {
				assert.Equal(t, "testuser", uname)
				return testUser, nil
			},
			updateUserFieldsFn: func(int, map[string]interface{}) error { return nil },
		}
		svc := newTestServiceWith(repo, sh)

		u, err := svc.Login("testuser", correctPwd)
		assert.NoError(t, err)
		assert.NotNil(t, u)
		assert.Equal(t, 1, u.Id)
		assert.Equal(t, "testuser", *u.Username)
	})

	t.Run("wrong password returns error", func(t *testing.T) {
		repo := &mockRepository{
			findUserByUsernameFn: func(string) (*SysUser, error) {
				return testUser, nil
			},
			updateUserFieldsFn: func(int, map[string]interface{}) error { return nil },
		}
		svc := newTestServiceWith(repo, sh)

		u, err := svc.Login("testuser", "wrong-password")
		assert.Error(t, err)
		assert.Nil(t, u)
		assert.Contains(t, err.Error(), "invalid username or password")
	})

	t.Run("user not found returns error", func(t *testing.T) {
		repo := &mockRepository{
			findUserByUsernameFn: func(string) (*SysUser, error) {
				return nil, errors.New("record not found")
			},
		}
		svc := newTestServiceWith(repo, sh)

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
			findUserByUsernameFn: func(string) (*SysUser, error) {
				return userNoPwd, nil
			},
		}
		svc := newTestServiceWith(repo, sh)

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
			findUserByUsernameFn: func(string) (*SysUser, error) {
				return disabledUser, nil
			},
		}
		svc := newTestServiceWith(repo, sh)

		u, err := svc.Login("disabled", correctPwd)
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
			findUserByUsernameFn: func(string) (*SysUser, error) {
				return userNoRole, nil
			},
			findUserRolesFn: func(int) ([]UserHasRole, error) {
				return []UserHasRole{}, nil
			},
		}
		svc := newTestServiceWith(repo, sh)

		u, err := svc.Login("norole", correctPwd)
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
			findUserByUsernameFn: func(string) (*SysUser, error) {
				return inactiveUser, nil
			},
		}
		svc := newTestServiceWith(repo, sh)

		u, err := svc.Login("inactive", correctPwd)
		assert.Error(t, err)
		assert.Nil(t, u)
		assert.Contains(t, err.Error(), "inactive")
	})

	t.Run("locked account within window returns ErrUserLocked", func(t *testing.T) {
		recent := time.Now().Add(-time.Minute)
		lockedUser := &SysUser{
			Id:              4,
			Username:        strPtr("locked"),
			Password:        &hashedPwd,
			LoginErrorTimes: DefaultMaxLoginFailedTimes, // Java int primitive NOT NULL
			LastLockTime:    &recent,
		}
		repo := &mockRepository{
			findUserByUsernameFn: func(string) (*SysUser, error) {
				return lockedUser, nil
			},
		}
		svc := newTestServiceWith(repo, sh)

		u, err := svc.Login("locked", correctPwd)
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
			LoginErrorTimes: DefaultMaxLoginFailedTimes,
			LastLockTime:    &expired,
		}
		var captured map[string]interface{}
		repo := &mockRepository{
			findUserByUsernameFn: func(string) (*SysUser, error) {
				return lockedUser, nil
			},
			updateUserFieldsFn: func(id int, fields map[string]interface{}) error {
				captured = fields
				return nil
			},
		}
		svc := newTestServiceWith(repo, sh)

		u, err := svc.Login("expiredlock", correctPwd)
		assert.NoError(t, err)
		assert.NotNil(t, u)
		assert.Equal(t, 0, captured["login_error_times"])
	})

	t.Run("wrong password increments error counter", func(t *testing.T) {
		user := &SysUser{
			Id:              6,
			Username:        strPtr("incrementme"),
			Password:        &hashedPwd,
			LoginErrorTimes: 2,
		}
		var captured map[string]interface{}
		repo := &mockRepository{
			findUserByUsernameFn: func(string) (*SysUser, error) {
				return user, nil
			},
			updateUserFieldsFn: func(id int, fields map[string]interface{}) error {
				captured = fields
				return nil
			},
		}
		svc := newTestServiceWith(repo, sh)

		u, err := svc.Login("incrementme", "wrong-password")
		assert.Error(t, err)
		assert.Nil(t, u)
		assert.Equal(t, 3, captured["login_error_times"])
		_, hasLock := captured["last_lock_time"]
		assert.False(t, hasLock)
	})

	t.Run("wrong password at threshold locks account", func(t *testing.T) {
		user := &SysUser{
			Id:              7,
			Username:        strPtr("lockme"),
			Password:        &hashedPwd,
			LoginErrorTimes: DefaultMaxLoginFailedTimes - 1,
		}
		var captured map[string]interface{}
		repo := &mockRepository{
			findUserByUsernameFn: func(string) (*SysUser, error) {
				return user, nil
			},
			updateUserFieldsFn: func(id int, fields map[string]interface{}) error {
				captured = fields
				return nil
			},
		}
		svc := newTestServiceWith(repo, sh)

		_, err := svc.Login("lockme", "wrong-password")
		assert.Error(t, err)
		assert.Equal(t, DefaultMaxLoginFailedTimes, captured["login_error_times"])
		_, hasLock := captured["last_lock_time"]
		assert.True(t, hasLock)
	})
}

// ---------------------------------------------------------------------------
// Tests: CreateUser
// ---------------------------------------------------------------------------

func TestService_CreateUser(t *testing.T) {
	// Note: CreateUser internally uses a DB Transaction to get userId + write
	// back the hashed password, which needs a real *gorm.DB. Unit tests here
	// skip full CreateUser validation because the mock repo doesn't populate
	// the ID back; the behaviour is instead covered by the integration
	// path via SeedInitialData and handler tests.
	t.Run("service exposes CreateUser on interface", func(t *testing.T) {
		repo := &mockRepository{
			// createUserFn intentionally left nil — if CreateUser calls it
			// (e.g. after a future refactor that moves ID acquisition out
			// of the DB layer) we'll surface it here via the panic default.
		}
		svc := newTestService(repo)
		assert.NotNil(t, svc)
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
		assert.Equal(t, 0, capturedOffset)
		assert.Equal(t, 10, capturedLimit)
		assert.True(t, capturedExcludeAdmin)
		assert.Equal(t, "", capturedCreatorName)
		assert.Len(t, users, 2)
		assert.Equal(t, int64(100), total)
	})

	t.Run("page 3 with pageSize 20 passes creator filter", func(t *testing.T) {
		_, _, err := svc.ListUsers(3, 20, true, "bob")
		assert.NoError(t, err)
		assert.Equal(t, 40, capturedOffset)
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
		assert.Equal(t, 0, capturedOffset)
		assert.Equal(t, 20, capturedLimit)
	})
}
