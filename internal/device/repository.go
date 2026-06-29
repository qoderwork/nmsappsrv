package device

import (
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository handles database operations for device entities.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
// Dependency injection is used because pkg/database already imports
// internal/device for model registration, so importing it back would
// create a circular dependency.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ---------------------------------------------------------------------------
// CpeElement CRUD
// ---------------------------------------------------------------------------

// FindByID returns a non-deleted CpeElement by its primary key.
func (r *Repository) FindByID(id int64) (*CpeElement, error) {
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
func (r *Repository) FindPage(licenseId int, keyword string, offset, limit int) ([]CpeElement, int64, error) {
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
func (r *Repository) FindBySerialNumber(sn string) (*CpeElement, error) {
	var elem CpeElement
	if err := r.db.Where("serial_number = ? AND deleted = ?", sn, false).First(&elem).Error; err != nil {
		return nil, err
	}
	return &elem, nil
}

// Create inserts a new CpeElement row.
func (r *Repository) Create(elem *CpeElement) error {
	return r.db.Create(elem).Error
}

// Update saves all fields of an existing CpeElement.
func (r *Repository) Update(elem *CpeElement) error {
	return r.db.Save(elem).Error
}

// SoftDelete marks a device as deleted without removing the row.
func (r *Repository) SoftDelete(id int64) error {
	return r.db.Model(&CpeElement{}).Where("ne_neid = ?", id).Update("deleted", true).Error
}

// ---------------------------------------------------------------------------
// DeviceGroup CRUD
// ---------------------------------------------------------------------------

// FindGroups returns all device groups for the given license.
func (r *Repository) FindGroups(licenseId int) ([]DeviceGroup, error) {
	var groups []DeviceGroup
	if err := r.db.Where("license_id = ?", licenseId).Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// CreateGroup inserts a new device group.
func (r *Repository) CreateGroup(g *DeviceGroup) error {
	return r.db.Create(g).Error
}

// UpdateGroup saves changes to an existing device group.
func (r *Repository) UpdateGroup(g *DeviceGroup) error {
	return r.db.Save(g).Error
}

// DeleteGroup removes a device group and its element associations.
func (r *Repository) DeleteGroup(id string) error {
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
func (r *Repository) AddElementToGroup(groupId string, elementId int64) error {
	rel := GroupHasElement{GroupId: groupId, ElementId: elementId}
	return r.db.Create(&rel).Error
}

// RemoveElementFromGroup deletes the many-to-many link.
func (r *Repository) RemoveElementFromGroup(groupId string, elementId int64) error {
	return r.db.Where("group_id = ? AND element_id = ?", groupId, elementId).Delete(&GroupHasElement{}).Error
}

// ---------------------------------------------------------------------------
// Device Import helpers
// ---------------------------------------------------------------------------

// FindBySerialNumbers returns a map of serial_number → CpeElement for all
// non-deleted devices matching the given serial numbers.
func (r *Repository) FindBySerialNumbers(serials []string) map[string]*CpeElement {
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
func (r *Repository) CountAllNonDeleted() int64 {
	var count int64
	r.db.Model(&CpeElement{}).Where("deleted = ?", false).Count(&count)
	return count
}

// CountNonDeletedByDeviceType counts non-deleted devices of a specific type.
// If licenseId > 0, it filters by license_id as well.
// If generation is non-empty, it also filters by generation.
func (r *Repository) CountNonDeletedByDeviceType(deviceType string, licenseId int, generation string) int64 {
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
func (r *Repository) FindDefaultGroups(licenseId int) ([]DeviceGroup, error) {
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
func (r *Repository) AddElementsToGroup(groupId string, elementIds []int64) error {
	if len(elementIds) == 0 {
		return nil
	}
	var rels []GroupHasElement
	for _, eid := range elementIds {
		rels = append(rels, GroupHasElement{GroupId: groupId, ElementId: eid})
	}
	return r.db.Create(&rels).Error
}
