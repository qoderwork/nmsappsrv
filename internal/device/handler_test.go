package device

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"nmsappsrv/pkg/apperror"
)

// ---------------------------------------------------------------------------
// mockService -- implements Service for unit testing the handler layer.
// ---------------------------------------------------------------------------

type mockService struct {
	getDeviceFn     func(id int64) (*CpeElement, error)
	listDevicesFn   func(tenantId int, keyword string, page, pageSize int) ([]CpeElement, int64, error)
	createDeviceFn  func(elem *CpeElement, tenantId int) error
	updateDeviceFn  func(elem *CpeElement) error
	deleteDeviceFn  func(id int64) error
	listGroupsFn    func(tenantId int) ([]DeviceGroup, error)
	createGroupFn   func(g *DeviceGroup, tenantId int) error
	updateGroupFn   func(g *DeviceGroup) error
	deleteGroupFn   func(id string) error
	importDevicesFn func(rows []ImportDeviceRow, deviceType string, deviceGroupId string, tenantId int) (*ImportDeviceResult, error)
}

func (m *mockService) GetDevice(id int64) (*CpeElement, error) {
	if m.getDeviceFn != nil {
		return m.getDeviceFn(id)
	}
	return nil, nil
}

func (m *mockService) ListDevices(tenantId int, keyword string, page, pageSize int) ([]CpeElement, int64, error) {
	if m.listDevicesFn != nil {
		return m.listDevicesFn(tenantId, keyword, page, pageSize)
	}
	return nil, 0, nil
}

func (m *mockService) CreateDevice(elem *CpeElement, tenantId int) error {
	if m.createDeviceFn != nil {
		return m.createDeviceFn(elem, tenantId)
	}
	return nil
}

func (m *mockService) UpdateDevice(elem *CpeElement) error {
	if m.updateDeviceFn != nil {
		return m.updateDeviceFn(elem)
	}
	return nil
}

func (m *mockService) DeleteDevice(id int64) error {
	if m.deleteDeviceFn != nil {
		return m.deleteDeviceFn(id)
	}
	return nil
}

func (m *mockService) ListGroups(tenantId int) ([]DeviceGroup, error) {
	if m.listGroupsFn != nil {
		return m.listGroupsFn(tenantId)
	}
	return nil, nil
}

func (m *mockService) CreateGroup(g *DeviceGroup, tenantId int) error {
	if m.createGroupFn != nil {
		return m.createGroupFn(g, tenantId)
	}
	return nil
}

func (m *mockService) UpdateGroup(g *DeviceGroup) error {
	if m.updateGroupFn != nil {
		return m.updateGroupFn(g)
	}
	return nil
}

func (m *mockService) DeleteGroup(id string) error {
	if m.deleteGroupFn != nil {
		return m.deleteGroupFn(id)
	}
	return nil
}

func (m *mockService) ImportDevices(rows []ImportDeviceRow, deviceType string, deviceGroupId string, tenantId int) (*ImportDeviceResult, error) {
	if m.importDevicesFn != nil {
		return m.importDevicesFn(rows, deviceType, deviceGroupId, tenantId)
	}
	return nil, nil
}

func (m *mockService) SetTenantIdNullByTenantId(tenantId int) error {
	return nil
}

func (m *mockService) DeleteDefaultGroupsByTenantId(tenantId int) error {
	return nil
}

func (m *mockService) CreateDefaultGroup(tenantId int) error {
	return nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// setupRouter creates a gin test router with the handler registered and
// a middleware that injects tenant_id into the context (mocking the JWT
// auth middleware).
func setupRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Mock middleware: inject tenant_id so GetTenantId returns 1.
	r.Use(func(c *gin.Context) {
		c.Set("tenant_id", 1)
		c.Next()
	})

	r.GET("/devices", h.ListDevices)
	r.GET("/devices/:id", h.GetDevice)

	return r
}

// parseBody decodes the JSON response body into a generic map.
func parseBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v\nbody: %s", err, w.Body.String())
	}
	return body
}

// ---------------------------------------------------------------------------
// ListDevices handler
// ---------------------------------------------------------------------------

func TestHandler_ListDevices(t *testing.T) {
	sn := "SN001"
	name := "Device1"
	mockSvc := &mockService{
		listDevicesFn: func(tenantId int, keyword string, page, pageSize int) ([]CpeElement, int64, error) {
			assert.Equal(t, 1, tenantId)
			assert.Equal(t, "", keyword)
			assert.Equal(t, 1, page)
			assert.Equal(t, 20, pageSize)
			return []CpeElement{
				{NeNeid: 1, SerialNumber: &sn, DeviceName: &name},
			}, 1, nil
		},
	}
	h := &Handler{svc: mockSvc}
	router := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/devices?page=1&pageSize=20", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	body := parseBody(t, w)
	assert.Equal(t, float64(200), body["code"])
	assert.Equal(t, "Success", body["message"])
	assert.Equal(t, float64(1), body["total"])
	assert.Equal(t, float64(1), body["page"])
	assert.Equal(t, float64(20), body["page_size"])

	data, ok := body["data"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, data, 1)
}

func TestHandler_ListDevices_WithKeyword(t *testing.T) {
	mockSvc := &mockService{
		listDevicesFn: func(tenantId int, keyword string, page, pageSize int) ([]CpeElement, int64, error) {
			assert.Equal(t, "test", keyword)
			return []CpeElement{}, 0, nil
		},
	}
	h := &Handler{svc: mockSvc}
	router := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/devices?keyword=test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, float64(0), body["total"])
}

func TestHandler_ListDevices_ServiceError(t *testing.T) {
	mockSvc := &mockService{
		listDevicesFn: func(tenantId int, keyword string, page, pageSize int) ([]CpeElement, int64, error) {
			return nil, 0, apperror.Wrap(errors.New("db error"), "LIST_DEVICES_FAILED", 500, "failed to list devices")
		},
	}
	h := &Handler{svc: mockSvc}
	router := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/devices", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "failed to list devices", body["message"])
}

// ---------------------------------------------------------------------------
// GetDevice handler
// ---------------------------------------------------------------------------

func TestHandler_GetDevice_ValidID(t *testing.T) {
	sn := "SN001"
	mockSvc := &mockService{
		getDeviceFn: func(id int64) (*CpeElement, error) {
			assert.Equal(t, int64(42), id)
			return &CpeElement{NeNeid: 42, SerialNumber: &sn}, nil
		},
	}
	h := &Handler{svc: mockSvc}
	router := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/devices/42", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, float64(200), body["code"])

	data, ok := body["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "SN001", data["serial_number"])
}

func TestHandler_GetDevice_InvalidID(t *testing.T) {
	mockSvc := &mockService{}
	h := &Handler{svc: mockSvc}
	router := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/devices/abc", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "invalid device id", body["message"])
}

func TestHandler_GetDevice_NotFound(t *testing.T) {
	mockSvc := &mockService{
		getDeviceFn: func(id int64) (*CpeElement, error) {
			return nil, apperror.ErrDeviceNotFound
		},
	}
	h := &Handler{svc: mockSvc}
	router := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/devices/999", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "device not found", body["message"])
}
