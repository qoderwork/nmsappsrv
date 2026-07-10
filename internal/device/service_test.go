package device

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// mockRepository -- implements Repository for unit testing the service layer.
// Function fields allow individual tests to override behaviour; methods
// without a corresponding function return zero values.
// ---------------------------------------------------------------------------

type mockRepository struct {
	findByIDFn                    func(id int64) (*CpeElement, error)
	findPageFn                    func(licenseId int, keyword string, offset, limit int) ([]CpeElement, int64, error)
	findBySerialNumberFn          func(sn string) (*CpeElement, error)
	createFn                      func(elem *CpeElement) error
	updateFn                      func(elem *CpeElement) error
	softDeleteFn                  func(id int64) error
	findGroupsFn                  func(licenseId int) ([]DeviceGroup, error)
	createGroupFn                 func(g *DeviceGroup) error
	updateGroupFn                 func(g *DeviceGroup) error
	deleteGroupFn                 func(id string) error
	addElementToGroupFn           func(groupId string, elementId int64) error
	removeElementFromGroupFn      func(groupId string, elementId int64) error
	findBySerialNumbersFn         func(serials []string) map[string]*CpeElement
	countAllNonDeletedFn          func() int64
	countNonDeletedByDeviceTypeFn func(deviceType string, licenseId int, generation string) int64
	findDefaultGroupsFn           func(licenseId int) ([]DeviceGroup, error)
	addElementsToGroupFn          func(groupId string, elementIds []int64) error
}

func (m *mockRepository) FindByID(id int64) (*CpeElement, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(id)
	}
	return nil, nil
}

func (m *mockRepository) FindPage(licenseId int, keyword string, offset, limit int) ([]CpeElement, int64, error) {
	if m.findPageFn != nil {
		return m.findPageFn(licenseId, keyword, offset, limit)
	}
	return nil, 0, nil
}

func (m *mockRepository) FindBySerialNumber(sn string) (*CpeElement, error) {
	if m.findBySerialNumberFn != nil {
		return m.findBySerialNumberFn(sn)
	}
	return nil, nil
}

func (m *mockRepository) Create(elem *CpeElement) error {
	if m.createFn != nil {
		return m.createFn(elem)
	}
	return nil
}

func (m *mockRepository) Update(elem *CpeElement) error {
	if m.updateFn != nil {
		return m.updateFn(elem)
	}
	return nil
}

func (m *mockRepository) SoftDelete(id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(id)
	}
	return nil
}

func (m *mockRepository) FindGroups(licenseId int) ([]DeviceGroup, error) {
	if m.findGroupsFn != nil {
		return m.findGroupsFn(licenseId)
	}
	return nil, nil
}

func (m *mockRepository) CreateGroup(g *DeviceGroup) error {
	if m.createGroupFn != nil {
		return m.createGroupFn(g)
	}
	return nil
}

func (m *mockRepository) UpdateGroup(g *DeviceGroup) error {
	if m.updateGroupFn != nil {
		return m.updateGroupFn(g)
	}
	return nil
}

func (m *mockRepository) DeleteGroup(id string) error {
	if m.deleteGroupFn != nil {
		return m.deleteGroupFn(id)
	}
	return nil
}

func (m *mockRepository) AddElementToGroup(groupId string, elementId int64) error {
	if m.addElementToGroupFn != nil {
		return m.addElementToGroupFn(groupId, elementId)
	}
	return nil
}

func (m *mockRepository) RemoveElementFromGroup(groupId string, elementId int64) error {
	if m.removeElementFromGroupFn != nil {
		return m.removeElementFromGroupFn(groupId, elementId)
	}
	return nil
}

func (m *mockRepository) FindBySerialNumbers(serials []string) map[string]*CpeElement {
	if m.findBySerialNumbersFn != nil {
		return m.findBySerialNumbersFn(serials)
	}
	return nil
}

func (m *mockRepository) CountAllNonDeleted() int64 {
	if m.countAllNonDeletedFn != nil {
		return m.countAllNonDeletedFn()
	}
	return 0
}

func (m *mockRepository) CountNonDeletedByDeviceType(deviceType string, licenseId int, generation string) int64 {
	if m.countNonDeletedByDeviceTypeFn != nil {
		return m.countNonDeletedByDeviceTypeFn(deviceType, licenseId, generation)
	}
	return 0
}

func (m *mockRepository) FindDefaultGroups(licenseId int) ([]DeviceGroup, error) {
	if m.findDefaultGroupsFn != nil {
		return m.findDefaultGroupsFn(licenseId)
	}
	return nil, nil
}

func (m *mockRepository) AddElementsToGroup(groupId string, elementIds []int64) error {
	if m.addElementsToGroupFn != nil {
		return m.addElementsToGroupFn(groupId, elementIds)
	}
	return nil
}

// newTestService creates a service with the given mock repository.
// The db field is nil because the tested methods only use repo.
func newTestService(repo Repository) Service {
	return &service{repo: repo}
}

// ---------------------------------------------------------------------------
// GetDevice
// ---------------------------------------------------------------------------

func TestService_GetDevice(t *testing.T) {
	sn := "SN001"
	expected := &CpeElement{NeNeid: 1, SerialNumber: &sn}

	repo := &mockRepository{
		findByIDFn: func(id int64) (*CpeElement, error) {
			assert.Equal(t, int64(1), id)
			return expected, nil
		},
	}
	svc := newTestService(repo)

	result, err := svc.GetDevice(1)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestService_GetDevice_NotFound(t *testing.T) {
	repo := &mockRepository{
		findByIDFn: func(id int64) (*CpeElement, error) {
			return nil, errors.New("record not found")
		},
	}
	svc := newTestService(repo)

	result, err := svc.GetDevice(999)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// ---------------------------------------------------------------------------
// ListDevices
// ---------------------------------------------------------------------------

func TestService_ListDevices(t *testing.T) {
	sn := "SN001"
	elems := []CpeElement{{NeNeid: 1, SerialNumber: &sn}}

	repo := &mockRepository{
		findPageFn: func(licenseId int, keyword string, offset, limit int) ([]CpeElement, int64, error) {
			// page 1, pageSize 20 -> offset 0, limit 20
			assert.Equal(t, 1, licenseId)
			assert.Equal(t, "", keyword)
			assert.Equal(t, 0, offset)
			assert.Equal(t, 20, limit)
			return elems, 1, nil
		},
	}
	svc := newTestService(repo)

	data, total, err := svc.ListDevices(1, "", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, data, 1)
}

func TestService_ListDevices_Page2(t *testing.T) {
	repo := &mockRepository{
		findPageFn: func(licenseId int, keyword string, offset, limit int) ([]CpeElement, int64, error) {
			// page 2, pageSize 10 -> offset 10, limit 10
			assert.Equal(t, 10, offset)
			assert.Equal(t, 10, limit)
			return nil, 0, nil
		},
	}
	svc := newTestService(repo)

	_, _, err := svc.ListDevices(1, "", 2, 10)
	assert.NoError(t, err)
}

func TestService_ListDevices_DefaultsInvalidPage(t *testing.T) {
	repo := &mockRepository{
		findPageFn: func(licenseId int, keyword string, offset, limit int) ([]CpeElement, int64, error) {
			// page < 1 defaults to 1 -> offset 0
			// pageSize < 1 defaults to 20
			assert.Equal(t, 0, offset)
			assert.Equal(t, 20, limit)
			return nil, 0, nil
		},
	}
	svc := newTestService(repo)

	_, _, err := svc.ListDevices(1, "", 0, 0)
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// CreateDevice
// ---------------------------------------------------------------------------

func TestService_CreateDevice(t *testing.T) {
	var capturedElem *CpeElement

	repo := &mockRepository{
		createFn: func(elem *CpeElement) error {
			capturedElem = elem
			return nil
		},
	}
	svc := newTestService(repo)

	sn := "SN-NEW"
	elem := &CpeElement{
		SerialNumber:    &sn,
		LoadedBasicInfo: true, // should be overridden to false
		IsInitialized:   true, // should be overridden to false
		Deleted:         true, // should be overridden to false
	}

	err := svc.CreateDevice(elem)
	assert.NoError(t, err)

	// Verify the service applied default values.
	assert.NotNil(t, capturedElem)
	assert.False(t, capturedElem.LoadedBasicInfo)
	assert.False(t, capturedElem.IsInitialized)
	assert.False(t, capturedElem.Deleted)
}

func TestService_CreateDevice_RepoError(t *testing.T) {
	repo := &mockRepository{
		createFn: func(elem *CpeElement) error {
			return errors.New("db connection lost")
		},
	}
	svc := newTestService(repo)

	err := svc.CreateDevice(&CpeElement{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db connection lost")
}

// ---------------------------------------------------------------------------
// DeleteDevice
// ---------------------------------------------------------------------------

func TestService_DeleteDevice(t *testing.T) {
	var capturedID int64

	repo := &mockRepository{
		softDeleteFn: func(id int64) error {
			capturedID = id
			return nil
		},
	}
	svc := newTestService(repo)

	err := svc.DeleteDevice(42)
	assert.NoError(t, err)
	assert.Equal(t, int64(42), capturedID)
}

func TestService_DeleteDevice_Error(t *testing.T) {
	repo := &mockRepository{
		softDeleteFn: func(id int64) error {
			return errors.New("update failed")
		},
	}
	svc := newTestService(repo)

	err := svc.DeleteDevice(1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update failed")
}
