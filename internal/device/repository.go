package device

import (
	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for device entities.
type Repository interface {
	FindByID(id int64) (*CpeElement, error)
	FindPage(licenseId int, keyword string, offset, limit int) ([]CpeElement, int64, error)
	FindBySerialNumber(sn string) (*CpeElement, error)
	Create(elem *CpeElement) error
	Update(elem *CpeElement) error
	SoftDelete(id int64) error
	FindGroups(licenseId int) ([]DeviceGroup, error)
	CreateGroup(g *DeviceGroup) error
	UpdateGroup(g *DeviceGroup) error
	DeleteGroup(id string) error
	AddElementToGroup(groupId string, elementId int64) error
	RemoveElementFromGroup(groupId string, elementId int64) error
	FindBySerialNumbers(serials []string) map[string]*CpeElement
	CountAllNonDeleted() int64
	CountNonDeletedByDeviceType(deviceType string, licenseId int, generation string) int64
	FindDefaultGroups(licenseId int) ([]DeviceGroup, error)
	AddElementsToGroup(groupId string, elementIds []int64) error

	// System/license lookups previously done via a raw *gorm.DB in the service.
	GetLicenseQuota(licenseId int, deviceType string) (int, error)
	CountNonDeleted(licenseId int, deviceType string) (int64, error)
	GetLocationEncryptionKey() (string, error)
}

// repository handles database operations for device entities.
// The embedded BaseRepository provides promoted Create and SoftDelete methods
// that satisfy the corresponding interface methods.
type repository struct {
	*baserepo.BaseRepository[CpeElement, int64]
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
// Dependency injection is used because pkg/database already imports
// internal/device for model registration, so importing it back would
// create a circular dependency.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[CpeElement, int64](db, "ne_neid"),
		db:             db,
	}
}

// ---------------------------------------------------------------------------
// CpeElement CRUD
// ---------------------------------------------------------------------------

// FindByID returns a non-deleted CpeElement by its primary key.
func (r *repository) FindByID(id int64) (*CpeElement, error) {
	var elem CpeElement
	if err := r.db.Where("ne_neid = ? AND deleted = ?", id, false).First(&elem).Error; err != nil {
		return nil, err
	}
	return &elem, nil
}

// FindPage returns a paginated list of non-deleted devices for the given
// license. When keyword is non-empty it filters on device_name and
// serial_number (case-insensitive LIKE). The total count is returned so the
// caller can build pagination metadata.
func (r *repository) FindPage(licenseId int, keyword string, offset, limit int) ([]CpeElement, int64, error) {
	var elems []CpeElement
	var total int64

	query := r.db.Model(&CpeElement{}).Where("license_id = ? AND deleted = ?", licenseId, false)

	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("device_name LIKE ? OR serial_number LIKE ?", like, like)
	}

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindPage count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("ne_neid DESC").Offset(offset).Limit(limit).Find(&elems).Error; err != nil {
		logger.Errorf("FindPage query error: %v", err)
		return nil, 0, err
	}
	return elems, total, nil
}

// FindBySerialNumber looks up a non-deleted device by its serial number.
func (r *repository) FindBySerialNumber(sn string) (*CpeElement, error) {
	var elem CpeElement
	if err := r.db.Where("serial_number = ? AND deleted = ?", sn, false).First(&elem).Error; err != nil {
		return nil, err
	}
	return &elem, nil
}

// Create is provided by the embedded BaseRepository[CpeElement, int64].
// Its promoted method satisfies the Repository.Create interface method.

// Update saves all fields of an existing CpeElement.
// Delegates to BaseRepository.Save because the interface uses the name "Update".
func (r *repository) Update(elem *CpeElement) error {
	return r.BaseRepository.Save(elem)
}

// SoftDelete is provided by the embedded BaseRepository[CpeElement, int64].
// Its promoted method satisfies the Repository.SoftDelete interface method.

// ---------------------------------------------------------------------------
// DeviceGroup CRUD
// ---------------------------------------------------------------------------

// FindGroups returns all device groups for the given license.
func (r *repository) FindGroups(licenseId int) ([]DeviceGroup, error) {
	var groups []DeviceGroup
	if err := r.db.Where("license_id = ?", licenseId).Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// CreateGroup inserts a new device group.
func (r *repository) CreateGroup(g *DeviceGroup) error {
	return r.db.Create(g).Error
}

// UpdateGroup saves changes to an existing device group.
func (r *repository) UpdateGroup(g *DeviceGroup) error {
	return r.db.Save(g).Error
}

// DeleteGroup removes a device group and its element associations.
func (r *repository) DeleteGroup(id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", id).Delete(&GroupHasElement{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&DeviceGroup{}).Error
	})
}

// ---------------------------------------------------------------------------
// Group ↔ Element association
// ---------------------------------------------------------------------------

// AddElementToGroup creates a many-to-many link between a group and a device.
func (r *repository) AddElementToGroup(groupId string, elementId int64) error {
	rel := GroupHasElement{GroupId: groupId, ElementId: elementId}
	return r.db.Create(&rel).Error
}

// RemoveElementFromGroup deletes the many-to-many link.
func (r *repository) RemoveElementFromGroup(groupId string, elementId int64) error {
	return r.db.Where("group_id = ? AND element_id = ?", groupId, elementId).Delete(&GroupHasElement{}).Error
}

// ---------------------------------------------------------------------------
// Device Import helpers
// ---------------------------------------------------------------------------

// FindBySerialNumbers returns a map of serial_number → CpeElement for all
// non-deleted devices matching the given serial numbers.
func (r *repository) FindBySerialNumbers(serials []string) map[string]*CpeElement {
	if len(serials) == 0 {
		return nil
	}
	var elems []CpeElement
	r.db.Where("serial_number IN ? AND deleted = ?", serials, false).Find(&elems)
	m := make(map[string]*CpeElement, len(elems))
	for i := range elems {
		if elems[i].SerialNumber != nil {
			m[*elems[i].SerialNumber] = &elems[i]
		}
	}
	return m
}

// CountAllNonDeleted returns the total number of non-deleted devices across
// the entire platform (used for platform-level license checking).
func (r *repository) CountAllNonDeleted() int64 {
	var count int64
	r.db.Model(&CpeElement{}).Where("deleted = ?", false).Count(&count)
	return count
}

// CountNonDeletedByDeviceType counts non-deleted devices of a specific type.
// If licenseId > 0, it filters by license_id as well.
// If generation is non-empty, it also filters by generation.
func (r *repository) CountNonDeletedByDeviceType(deviceType string, licenseId int, generation string) int64 {
	var count int64
	q := r.db.Model(&CpeElement{}).Where("deleted = ?", false)
	if deviceType != "" {
		q = q.Where("device_type = ?", deviceType)
	}
	if licenseId > 0 {
		q = q.Where("license_id = ?", licenseId)
	}
	if generation != "" {
		q = q.Where("generation = ?", generation)
	}
	q.Count(&count)
	return count
}

// FindDefaultGroups returns device groups marked as default for the given
// license scope. If licenseId is 0, it finds platform-level defaults.
func (r *repository) FindDefaultGroups(licenseId int) ([]DeviceGroup, error) {
	var groups []DeviceGroup
	q := r.db.Where("default_group = ?", true)
	if licenseId > 0 {
		q = q.Where("license_id = ?", licenseId)
	} else {
		q = q.Where("license_id IS NULL")
	}
	if err := q.Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// AddElementsToGroup batch-creates group-element associations.
func (r *repository) AddElementsToGroup(groupId string, elementIds []int64) error {
	if len(elementIds) == 0 {
		return nil
	}
	var rels []GroupHasElement
	for _, eid := range elementIds {
		rels = append(rels, GroupHasElement{GroupId: groupId, ElementId: eid})
	}
	return r.db.Create(&rels).Error
}

// ---------------------------------------------------------------------------
// System / license lookups (route former raw-db access through the repo)
// ---------------------------------------------------------------------------

// GetLicenseQuota reads the device quota for a given device type from the
// license (tenancy) table. The column is selected based on deviceType:
// "enb" -> enb_quantity, "gnb" -> gnb_quantity, default -> cpe_quantity.
func (r *repository) GetLicenseQuota(licenseId int, deviceType string) (int, error) {
	quotaCol := "cpe_quantity"
	switch deviceType {
	case "enb":
		quotaCol = "enb_quantity"
	case "gnb":
		quotaCol = "gnb_quantity"
	}
	var quota int
	if err := r.db.Table("license").Select(quotaCol).Where("id = ?", licenseId).Scan(&quota).Error; err != nil {
		return 0, err
	}
	return quota, nil
}

// CountNonDeleted counts non-deleted devices of a given type for a license.
// An empty deviceType counts CPE devices (device_type = 'cpe' OR NULL),
// matching the original inline query in the service.
func (r *repository) CountNonDeleted(licenseId int, deviceType string) (int64, error) {
	var existing int64
	query := r.db.Model(&CpeElement{}).Where("license_id = ? AND deleted = ?", licenseId, false)
	switch deviceType {
	case "enb":
		query = query.Where("device_type = ?", "enb")
	case "gnb":
		query = query.Where("device_type = ?", "gnb")
	default:
		query = query.Where("device_type = ? OR device_type IS NULL", "cpe")
	}
	if err := query.Count(&existing).Error; err != nil {
		return 0, err
	}
	return existing, nil
}

// GetLocationEncryptionKey reads the AES key string from system_config.
// The live table uses an "id" varchar PK and a "config" longtext column, so the
// key is matched against id and the value is read from config.
func (r *repository) GetLocationEncryptionKey() (string, error) {
	var row struct {
		Config *string `gorm:"column:config"`
	}
	if err := r.db.Table("system_config").
		Where("id = ?", "location_encryption_key").
		Limit(1).
		Scan(&row).Error; err != nil {
		return "", err
	}
	if row.Config == nil {
		return "", nil
	}
	return *row.Config, nil
}
