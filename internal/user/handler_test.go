package user

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/redis"

	goredis "github.com/go-redis/redis/v8"
)

// ---------------------------------------------------------------------------
// mockService -- implements Service for unit testing the handler layer.
// ---------------------------------------------------------------------------

type mockService struct {
	loginFn               func(string, string) (*SysUser, error)
	logoutFn              func(string, string) error
	recordLoginFn         func(string, string, int, int) error
	recordLogoutFn        func(string, string, int) error
	listUsersFn           func(int, int, bool, string) ([]SysUser, int64, error)
	createUserFn          func(*SysUser) (string, error)
	updateUserFn          func(*SysUser) error
	deleteUserFn          func(int) error
	kickOutUserFn         func(int) error
	unlockUserFn          func(int) error
	modifyPasswordFn      func(string, string, string) error
	enableUserFn          func(int) error
	disableUserFn         func(int) error
	resetPasswordFn       func(int, int) (string, error)
	resetPasswordByLinkFn func(string, string, string) error
	setTenancyForUserFn   func(int, int) error
	getLoginFailedTimesFn func(int) (*LoginFailedTimesResponse, error)
	needChangePasswordFn  func(int) (*NeedChangePasswordResponse, error)
	listRolesFn           func(int) ([]Role, error)
	createRoleFn          func(*Role) error
	updateRoleFn          func(*Role) error
	deleteRoleFn          func(string) error
	getRolePermissionsFn  func(string) ([]RoleHasPermission, error)
	updateRolePermissionsFn func(string, []string) error
	getRoleNamesForUserFn func(int, int) ([]string, error)
}

func (m *mockService) Login(username, password string) (*SysUser, error) {
	if m.loginFn != nil {
		return m.loginFn(username, password)
	}
	return nil, nil
}

func (m *mockService) Logout(username, jwtToken string) error {
	if m.logoutFn != nil {
		return m.logoutFn(username, jwtToken)
	}
	return nil
}

func (m *mockService) RecordLogin(username, ip string, tenantId int, result int) error {
	if m.recordLoginFn != nil {
		return m.recordLoginFn(username, ip, tenantId, result)
	}
	return nil
}

func (m *mockService) RecordLogout(username, ip string, tenantId int) error {
	if m.recordLogoutFn != nil {
		return m.recordLogoutFn(username, ip, tenantId)
	}
	return nil
}

func (m *mockService) ListUsers(page, pageSize int, excludeAdmin bool, creatorName string) ([]SysUser, int64, error) {
	if m.listUsersFn != nil {
		return m.listUsersFn(page, pageSize, excludeAdmin, creatorName)
	}
	return nil, 0, nil
}

func (m *mockService) CreateUser(u *SysUser) (string, error) {
	if m.createUserFn != nil {
		return m.createUserFn(u)
	}
	return "", nil
}

func (m *mockService) UpdateUser(u *SysUser) error {
	if m.updateUserFn != nil {
		return m.updateUserFn(u)
	}
	return nil
}

func (m *mockService) DeleteUser(id int) error {
	if m.deleteUserFn != nil {
		return m.deleteUserFn(id)
	}
	return nil
}

func (m *mockService) KickOutUser(userId int) error {
	if m.kickOutUserFn != nil {
		return m.kickOutUserFn(userId)
	}
	return nil
}

func (m *mockService) UnlockUser(userId int) error {
	if m.unlockUserFn != nil {
		return m.unlockUserFn(userId)
	}
	return nil
}

func (m *mockService) ModifyPassword(username, oldPassword, newPassword string) error {
	if m.modifyPasswordFn != nil {
		return m.modifyPasswordFn(username, oldPassword, newPassword)
	}
	return nil
}

func (m *mockService) EnableUser(userId int) error {
	if m.enableUserFn != nil {
		return m.enableUserFn(userId)
	}
	return nil
}

func (m *mockService) DisableUser(userId int) error {
	if m.disableUserFn != nil {
		return m.disableUserFn(userId)
	}
	return nil
}

func (m *mockService) ResetPassword(adminId, userId int) (string, error) {
	if m.resetPasswordFn != nil {
		return m.resetPasswordFn(adminId, userId)
	}
	return "", nil
}

func (m *mockService) ResetPasswordByLink(username, key, newPassword string) error {
	if m.resetPasswordByLinkFn != nil {
		return m.resetPasswordByLinkFn(username, key, newPassword)
	}
	return nil
}

func (m *mockService) SetTenancyForUser(userId, tenantId int) error {
	if m.setTenancyForUserFn != nil {
		return m.setTenancyForUserFn(userId, tenantId)
	}
	return nil
}

func (m *mockService) GetLoginFailedTimes(userId int) (*LoginFailedTimesResponse, error) {
	if m.getLoginFailedTimesFn != nil {
		return m.getLoginFailedTimesFn(userId)
	}
	return nil, nil
}

func (m *mockService) NeedChangePassword(userId int) (*NeedChangePasswordResponse, error) {
	if m.needChangePasswordFn != nil {
		return m.needChangePasswordFn(userId)
	}
	return nil, nil
}

func (m *mockService) ListRoles(tenantId int) ([]Role, error) {
	if m.listRolesFn != nil {
		return m.listRolesFn(tenantId)
	}
	return nil, nil
}

func (m *mockService) CreateRole(r *Role) error {
	if m.createRoleFn != nil {
		return m.createRoleFn(r)
	}
	return nil
}

func (m *mockService) UpdateRole(r *Role) error {
	if m.updateRoleFn != nil {
		return m.updateRoleFn(r)
	}
	return nil
}

func (m *mockService) DeleteRole(id string) error {
	if m.deleteRoleFn != nil {
		return m.deleteRoleFn(id)
	}
	return nil
}

func (m *mockService) GetRolePermissions(roleId string) ([]RoleHasPermission, error) {
	if m.getRolePermissionsFn != nil {
		return m.getRolePermissionsFn(roleId)
	}
	return nil, nil
}

func (m *mockService) UpdateRolePermissions(roleId string, permissionIds []string) error {
	if m.updateRolePermissionsFn != nil {
		return m.updateRolePermissionsFn(roleId, permissionIds)
	}
	return nil
}

func (m *mockService) GetRoleNamesForUser(userId int, tenantId int) ([]string, error) {
	if m.getRoleNamesForUserFn != nil {
		return m.getRoleNamesForUserFn(userId, tenantId)
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// setupHandlerEnv prepares gin test mode, JWT secret, and a dummy Redis
// client so that the handler's full code paths (including JWT generation)
// can execute without external dependencies.
func setupHandlerEnv(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// JWT secret must be >= 32 bytes.
	middleware.JWTSecret = []byte("test-secret-at-least-32-bytes-long!!")

	// Dummy Redis client: connection will fail, but GenerateToken only
	// logs a warning on Redis errors and still returns the token.
	redis.RDB = goredis.NewClient(&goredis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 10 * time.Millisecond,
	})
	t.Cleanup(func() { redis.RDB.Close() })
}

// newTestHandler creates a Handler wired to the given mock service.
func newTestHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// ---------------------------------------------------------------------------
// Tests: Login handler
// ---------------------------------------------------------------------------

func TestHandler_Login(t *testing.T) {
	setupHandlerEnv(t)

	t.Run("valid credentials returns 200 with token", func(t *testing.T) {
		tenantId := 1
		svc := &mockService{
			loginFn: func(username, password string) (*SysUser, error) {
				return &SysUser{
					Id:        10,
					Username:  strPtr("alice"),
					TenantId: &tenantId,
				}, nil
			},
			getRoleNamesForUserFn: func(userId, tenantId int) ([]string, error) {
				return []string{"admin"}, nil
			},
			recordLoginFn: func(username, ip string, licId int, result int) error {
				assert.Equal(t, "alice", username)
				assert.Equal(t, 1, result) // success
				return nil
			},
		}
		h := newTestHandler(svc)

		body, _ := json.Marshal(loginRequest{Username: "alice", Password: "secret"})
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router := gin.New()
		router.POST("/login", h.Login)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, float64(200), resp["code"])

		data, ok := resp["data"].(map[string]interface{})
		assert.True(t, ok)
		assert.NotEmpty(t, data["token"])
	})

	t.Run("invalid credentials returns 401", func(t *testing.T) {
		var recordedResult int
		svc := &mockService{
			loginFn: func(username, password string) (*SysUser, error) {
				return nil, apperror.ErrInvalidCredentials
			},
			recordLoginFn: func(username, ip string, licId int, result int) error {
				recordedResult = result
				return nil
			},
		}
		h := newTestHandler(svc)

		body, _ := json.Marshal(loginRequest{Username: "bad", Password: "bad"})
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router := gin.New()
		router.POST("/login", h.Login)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, 0, recordedResult) // 0 = failure

		var resp map[string]interface{}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Contains(t, resp["message"], "invalid username or password")
	})

	t.Run("missing body returns 400", func(t *testing.T) {
		svc := &mockService{}
		h := newTestHandler(svc)

		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router := gin.New()
		router.POST("/login", h.Login)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// ---------------------------------------------------------------------------
// Tests: ListUsers handler
// ---------------------------------------------------------------------------

func TestHandler_ListUsers(t *testing.T) {
	setupHandlerEnv(t)

	t.Run("returns paginated user list", func(t *testing.T) {
		var capturedPage, capturedPageSize int
		var capturedExcludeAdmin bool
		var capturedCreatorName string
		svc := &mockService{
			listUsersFn: func(page, pageSize int, excludeAdmin bool, creatorName string) ([]SysUser, int64, error) {
				capturedPage = page
				capturedPageSize = pageSize
				capturedExcludeAdmin = excludeAdmin
				capturedCreatorName = creatorName
				return []SysUser{
					{Id: 1, Username: strPtr("alice")},
					{Id: 2, Username: strPtr("bob")},
				}, int64(50), nil
			},
		}
		h := newTestHandler(svc)

		req := httptest.NewRequest(http.MethodGet, "/users?page=2&pageSize=10", nil)
		w := httptest.NewRecorder()

		router := gin.New()
		// Inject auth context (simulates auth middleware): admin sees all users.
		router.Use(func(c *gin.Context) {
			c.Set("username", "root")
			c.Set("role_names", []string{"admin"})
			c.Next()
		})
		router.GET("/users", h.ListUsers)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 2, capturedPage)
		assert.Equal(t, 10, capturedPageSize)
		assert.True(t, capturedExcludeAdmin)
		assert.Equal(t, "", capturedCreatorName) // admin -> no creator filter

		var resp map[string]interface{}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, float64(200), resp["code"])

		data, ok := resp["data"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, float64(50), data["total"])
		assert.Equal(t, float64(2), data["page"])
		assert.Equal(t, float64(10), data["page_size"])

		list, ok := data["list"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, list, 2)

		// DTO should not contain password or salt fields.
		first := list[0].(map[string]interface{})
		assert.Nil(t, first["password"])
		assert.Nil(t, first["salt"])
	})

	t.Run("non-admin caller is scoped to own created users", func(t *testing.T) {
		var capturedCreatorName string
		var capturedExcludeAdmin bool
		svc := &mockService{
			listUsersFn: func(page, pageSize int, excludeAdmin bool, creatorName string) ([]SysUser, int64, error) {
				capturedExcludeAdmin = excludeAdmin
				capturedCreatorName = creatorName
				return []SysUser{}, int64(0), nil
			},
		}
		h := newTestHandler(svc)

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		w := httptest.NewRecorder()

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("username", "bob")
			c.Set("role_names", []string{"Monitoring"})
			c.Next()
		})
		router.GET("/users", h.ListUsers)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, capturedExcludeAdmin)
		assert.Equal(t, "bob", capturedCreatorName) // non-admin -> filtered by creator
	})

	t.Run("defaults to page 1 and pageSize 20", func(t *testing.T) {
		var capturedPage, capturedPageSize int
		svc := &mockService{
			listUsersFn: func(page, pageSize int, excludeAdmin bool, creatorName string) ([]SysUser, int64, error) {
				capturedPage = page
				capturedPageSize = pageSize
				return []SysUser{}, int64(0), nil
			},
		}
		h := newTestHandler(svc)

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		w := httptest.NewRecorder()

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("username", "root")
			c.Set("role_names", []string{"admin"})
			c.Next()
		})
		router.GET("/users", h.ListUsers)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 1, capturedPage)
		assert.Equal(t, 20, capturedPageSize)
	})

	t.Run("service error returns 500", func(t *testing.T) {
		svc := &mockService{
			listUsersFn: func(page, pageSize int, excludeAdmin bool, creatorName string) ([]SysUser, int64, error) {
				return nil, 0, assert.AnError
			},
		}
		h := newTestHandler(svc)

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		w := httptest.NewRecorder()

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("username", "root")
			c.Set("role_names", []string{"admin"})
			c.Next()
		})
		router.GET("/users", h.ListUsers)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
