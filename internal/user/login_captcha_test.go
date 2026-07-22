package user

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	goredis "github.com/go-redis/redis/v8"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"nmsappsrv/internal/captcha"
)

// newCaptchaManager spins up an in-process Redis (miniredis) and returns a
// captcha Manager backed by it, plus the miniredis handle for test-only reads.
func newCaptchaManager(t *testing.T) (*captcha.Manager, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return captcha.NewManager(rdb, 4), mr
}

func TestLoginCaptchaGate(t *testing.T) {
	setupHandlerEnv(t)
	mgr, mr := newCaptchaManager(t)
	const ip = "1.2.3.4"

	newSVC := func() *mockService {
		return &mockService{
			loginFn: func(username, password string) (*SysUser, error) {
				lic := 1
				return &SysUser{Id: 10, Username: strPtr("alice"), LicenseId: &lic}, nil
			},
			getRoleNamesForUserFn: func(userId, licenseId int) ([]string, error) {
				return []string{"admin"}, nil
			},
			recordLoginFn: func(username, ip string, licId int, result int) error { return nil },
		}
	}

	doLogin := func(svc *mockService, key, code string) *httptest.ResponseRecorder {
		h := &Handler{svc: svc, captchaMgr: mgr}
		body, _ := json.Marshal(loginRequest{
			Username:        "alice",
			Password:        "secret",
			VerificationKey: key,
			VerificationCode: code,
		})
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = ip + ":9999"
		w := httptest.NewRecorder()
		router := gin.New()
		router.POST("/login", h.Login)
		router.ServeHTTP(w, req)
		return w
	}

	t.Run("no captcha required before failures -> 200", func(t *testing.T) {
		w := doLogin(newSVC(), "", "")
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("after 3 failures -> required, missing captcha -> 400 required:true", func(t *testing.T) {
		mgr.OnFailure("alice", ip)
		mgr.OnFailure("alice", ip)
		mgr.OnFailure("alice", ip)
		assert.True(t, mgr.IsRequired("alice", ip))

		w := doLogin(newSVC(), "", "")
		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]interface{}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		data, ok := resp["data"].(map[string]interface{})
		assert.True(t, ok, "data should be a map")
		assert.Equal(t, true, data["required"])
	})

	t.Run("with valid captcha -> 200 and requirement cleared", func(t *testing.T) {
		key, _, err := mgr.Generate(context.Background())
		assert.NoError(t, err)
		answer, _ := mr.Get("captcha_code_" + key)
		assert.NotEmpty(t, answer)

		w := doLogin(newSVC(), key, answer)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.False(t, mgr.IsRequired("alice", ip), "OnSuccess should clear requirement")
	})

	t.Run("with wrong captcha -> 400 required:true", func(t *testing.T) {
		// re-arm the requirement (cleared by previous successful login)
		mgr.OnFailure("alice", ip)
		mgr.OnFailure("alice", ip)
		mgr.OnFailure("alice", ip)
		assert.True(t, mgr.IsRequired("alice", ip))

		key, _, err := mgr.Generate(context.Background())
		assert.NoError(t, err)
		w := doLogin(newSVC(), key, "0000")
		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]interface{}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		data, ok := resp["data"].(map[string]interface{})
		assert.True(t, ok, "data should be a map")
		assert.Equal(t, true, data["required"])
	})
}
