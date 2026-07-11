package tenancy

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"nmsappsrv/pkg/baserepo"
)

// Repository provides database operations for tenancy management
type Repository struct {
	*baserepo.BaseRepository[tenancyModel, int] // embedded generic CRUD for tenancyModel
	db *gorm.DB
}

// NewRepository creates a new Repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		BaseRepository: baserepo.New[tenancyModel, int](db, "id"),
		db:             db,
	}
}

// ExistsByName checks if a tenancy with the given name already exists
func (r *Repository) ExistsByName(name string) (bool, error) {
	var count int64
	err := r.db.Model(&tenancyModel{}).Where("license_name = ?", name).Count(&count).Error
	return count > 0, err
}

// ExistsByNameExcluding checks if a tenancy with the given name exists, excluding a specific ID
func (r *Repository) ExistsByNameExcluding(name string, excludeID int) (bool, error) {
	var count int64
	err := r.db.Model(&tenancyModel{}).Where("license_name = ? AND id != ?", name, excludeID).Count(&count).Error
	return count > 0, err
}

// List returns paginated tenancies with optional name filter
func (r *Repository) List(nameFilter string, page, pageSize int) ([]tenancyModel, int64, error) {
	query := r.DB.Model(&tenancyModel{})
	if nameFilter != "" {
		query = query.Where("license_name LIKE ?", fmt.Sprintf("%%%s%%", nameFilter))
	}

	offset := (page - 1) * pageSize
	result, err := r.FindPage(query, "id ASC", offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	return result.Items, result.Total, nil
}

// strPtr returns a pointer to the given string
func strPtr(s string) *string {
	return &s
}

// timeFromMillis converts a millisecond timestamp to time.Time
func timeFromMillis(ms int64) *time.Time {
	t := time.UnixMilli(ms)
	return &t
}

// strOrEmpty safely dereferences a string pointer
func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// millisFromTime converts a time.Time to milliseconds
func millisFromTime(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.UnixMilli()
}
