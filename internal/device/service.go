package device

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// Service contains the business logic for device management.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by a fresh Repository.
// The *gorm.DB is forwarded via dependency injection to avoid a circular
// import with pkg/database (which already imports internal/device for
// model registration in AutoMigrateAll).
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ---------------------------------------------------------------------------
// CpeElement
// ---------------------------------------------------------------------------

// GetDevice returns a single device by ID.
func (s *Service) GetDevice(id int64) (*CpeElement, error) {
	return s.repo.FindByID(id)
}

// ListDevices returns a paginated device list. The page number (1-based) is
// converted to an offset before querying.
func (s *Service) ListDevices(licenseId int, keyword string, page, pageSize int) ([]CpeElement, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindPage(licenseId, keyword, offset, pageSize)
}

// CreateDevice applies sensible defaults and persists a new device.
func (s *Service) CreateDevice(elem *CpeElement) error {
	elem.LoadedBasicInfo = false
	elem.IsInitialized = false
	elem.Deleted = false
	return s.repo.Create(elem)
}

// UpdateDevice persists changes to an existing device.
func (s *Service) UpdateDevice(elem *CpeElement) error {
	return s.repo.Update(elem)
}

// DeleteDevice performs a soft-delete (sets deleted = true).
func (s *Service) DeleteDevice(id int64) error {
	return s.repo.SoftDelete(id)
}

// ---------------------------------------------------------------------------
// DeviceGroup
// ---------------------------------------------------------------------------

// ListGroups returns all groups for the given license.
func (s *Service) ListGroups(licenseId int) ([]DeviceGroup, error) {
	return s.repo.FindGroups(licenseId)
}

// CreateGroup generates a random 32-char hex ID when none is supplied, then
// persists the group.
func (s *Service) CreateGroup(g *DeviceGroup) error {
	if g.Id == "" {
		g.Id = generateHexID()
	}
	return s.repo.CreateGroup(g)
}

// UpdateGroup persists changes to an existing group.
func (s *Service) UpdateGroup(g *DeviceGroup) error {
	return s.repo.UpdateGroup(g)
}

// DeleteGroup removes a group and its element associations.
func (s *Service) DeleteGroup(id string) error {
	return s.repo.DeleteGroup(id)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// generateHexID returns a 32-character random hex string (16 random bytes).
func generateHexID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// ---------------------------------------------------------------------------
// Device Import
// ---------------------------------------------------------------------------

// ParseImportExcel reads an Excel file from the reader and returns parsed rows.
// Expected columns (first row is header):
// Column 0: Serial Number
// Column 1: Device Name
// Column 2: Location
// Column 3: Longitude
// Column 4: Latitude
// Column 5: Operation (Add/Modify/Delete)
func ParseImportExcel(r io.Reader) ([]ImportDeviceRow, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel file: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("no sheets found in excel file")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet %s: %w", sheetName, err)
	}

	var result []ImportDeviceRow
	// Skip header row (index 0), process data rows.
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}

		// Pad row to at least 6 columns.
		for len(row) < 6 {
			row = append(row, "")
		}

		sn := strings.TrimSpace(row[0])
		if sn == "" {
			continue // Skip rows with empty serial number.
		}

		result = append(result, ImportDeviceRow{
			SerialNumber: sn,
			DeviceName:   strings.TrimSpace(row[1]),
			Location:     strings.TrimSpace(row[2]),
			Longitude:    strings.TrimSpace(row[3]),
			Latitude:     strings.TrimSpace(row[4]),
			Operation:    strings.TrimSpace(row[5]),
		})
	}

	return result, nil
}

// ImportDevices processes the import rows with Add/Modify/Delete operations.
// It uses a Redis distributed lock to prevent race conditions.
func (s *Service) ImportDevices(rows []ImportDeviceRow, deviceType string, deviceGroupId string, licenseId int) (*ImportDeviceResult, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("no device data to import")
	}

	// --- Validate: check for duplicate serial numbers within the file ---
	snSet := make(map[string]bool, len(rows))
	for _, row := range rows {
		lower := strings.ToLower(row.SerialNumber)
		if snSet[lower] {
			return nil, fmt.Errorf("duplicate serial number in file: %s", row.SerialNumber)
		}
		snSet[lower] = true
	}

	// --- Validate: check each row has a valid operation ---
	for _, row := range rows {
		op := strings.ToLower(row.Operation)
		if op != "add" && op != "modify" && op != "delete" {
			return nil, fmt.Errorf("invalid operation '%s' for serial number %s (must be Add, Modify, or Delete)", row.Operation, row.SerialNumber)
		}
	}

	// --- Acquire Redis distributed lock ---
	ctx := context.Background()
	lockKey := "nms_add_device_lock"
	if !redis.Lock(ctx, lockKey, 60*time.Second) {
		return nil, fmt.Errorf("another import operation is in progress, please try again later")
	}
	defer redis.Unlock(ctx, lockKey)

	// --- Look up existing devices by serial number ---
	var serials []string
	for _, row := range rows {
		serials = append(serials, row.SerialNumber)
	}
	existingMap := s.repo.FindBySerialNumbers(serials)

	result := &ImportDeviceResult{}
	now := time.Now()

	// Determine device type and generation based on import type.
	var dbDeviceType string
	var generation string
	var newVersion bool
	switch strings.ToLower(deviceType) {
	case "gnb":
		dbDeviceType = "enb" // Java uses "enb" for both GNB and ENB
		generation = "NR"
		newVersion = true
	case "enb":
		dbDeviceType = "enb"
		generation = "LTE"
		newVersion = true
	case "cpe":
		dbDeviceType = "cpe"
		generation = ""
		newVersion = false
	default:
		dbDeviceType = "enb"
		generation = "NR"
		newVersion = true
	}

	// --- Process each row ---
	for _, row := range rows {
		op := strings.ToLower(row.Operation)
		existing, exists := existingMap[row.SerialNumber]

		switch op {
		case "add":
			if exists {
				// Serial number already exists, skip.
				result.Errors = append(result.Errors, fmt.Sprintf("serial number %s already exists, skipping add", row.SerialNumber))
				result.FailedCount++
				continue
			}
			elem := &CpeElement{
				SerialNumber:         strPtr(row.SerialNumber),
				DeviceName:           strPtr(row.DeviceName),
				InstallationLocation: strPtr(row.Location),
				Longitude:            strPtr(row.Longitude),
				Latitude:             strPtr(row.Latitude),
				DeviceType:           strPtr(dbDeviceType),
				Generation:           strPtr(generation),
				IsNewVersion:         newVersion,
				LoadedBasicInfo:      false,
				IsInitialized:        false,
				Deleted:              false,
				CreationTime:         &now,
				LicenseId:            intPtr(licenseId),
			}
			if err := s.repo.Create(elem); err != nil {
				logger.Errorf("import add device %s error: %v", row.SerialNumber, err)
				result.Errors = append(result.Errors, fmt.Sprintf("failed to add %s: %v", row.SerialNumber, err))
				result.FailedCount++
				continue
			}
			result.AddedCount++
			result.AddedIds = append(result.AddedIds, elem.NeNeid)

		case "modify":
			if !exists {
				result.Errors = append(result.Errors, fmt.Sprintf("serial number %s not found, skipping modify", row.SerialNumber))
				result.FailedCount++
				continue
			}
			existing.DeviceName = strPtr(row.DeviceName)
			existing.InstallationLocation = strPtr(row.Location)
			existing.Longitude = strPtr(row.Longitude)
			existing.Latitude = strPtr(row.Latitude)
			if err := s.repo.Update(existing); err != nil {
				logger.Errorf("import modify device %s error: %v", row.SerialNumber, err)
				result.Errors = append(result.Errors, fmt.Sprintf("failed to modify %s: %v", row.SerialNumber, err))
				result.FailedCount++
				continue
			}
			result.ModifiedCount++

		case "delete":
			if !exists {
				result.Errors = append(result.Errors, fmt.Sprintf("serial number %s not found, skipping delete", row.SerialNumber))
				result.FailedCount++
				continue
			}
			if err := s.repo.SoftDelete(existing.NeNeid); err != nil {
				logger.Errorf("import delete device %s error: %v", row.SerialNumber, err)
				result.Errors = append(result.Errors, fmt.Sprintf("failed to delete %s: %v", row.SerialNumber, err))
				result.FailedCount++
				continue
			}
			result.DeletedCount++
		}
	}

	// --- Device group assignment (after lock released in Java, but we do it here) ---
	if deviceGroupId != "" && len(result.AddedIds) > 0 {
		if err := s.repo.AddElementsToGroup(deviceGroupId, result.AddedIds); err != nil {
			logger.Errorf("failed to add devices to group %s: %v", deviceGroupId, err)
		}
	}

	// --- Default group assignment for newly added devices ---
	if len(result.AddedIds) > 0 {
		// Platform-level default groups.
		platformGroups, err := s.repo.FindDefaultGroups(0)
		if err == nil {
			for _, g := range platformGroups {
				_ = s.repo.AddElementsToGroup(g.Id, result.AddedIds)
			}
		}
		// Tenant-level default groups.
		if licenseId > 0 {
			tenantGroups, err := s.repo.FindDefaultGroups(licenseId)
			if err == nil {
				for _, g := range tenantGroups {
					_ = s.repo.AddElementsToGroup(g.Id, result.AddedIds)
				}
			}
		}
	}

	return result, nil
}

// --- pointer helpers local to this file ---

func strPtr(s string) *string  { return &s }
func intPtr(i int) *int        { return &i }
