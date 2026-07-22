package northinterfacelog

import (
	"time"

	"gorm.io/gorm"
)

// Repository persists and queries NorthInterfaceLog records.
type Repository struct {
	db *gorm.DB
}

// NewRepository builds a Repository from a *gorm.DB.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Save inserts a new audit log row.
func (r *Repository) Save(log *NorthInterfaceLog) error {
	return r.db.Create(log).Error
}

// ListFilter narrows a log query.
type ListFilter struct {
	TenancyID  int
	SearchText string
	StartTime  *time.Time
	EndTime    *time.Time
}

// List returns a page of logs ordered by operationTime descending, matching the
// Java NorthInterfaceLogManagementServiceImpl ordering. searchText matches
// log_name OR the device identified by element_id (device_name / serial_number
// in cpe_element), mirroring Java's Specification.
func (r *Repository) List(f ListFilter, page, pageSize int) ([]NorthInterfaceLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	q := r.db.Model(&NorthInterfaceLog{})
	if f.TenancyID != 0 {
		q = q.Where("tenant_id = ?", f.TenancyID)
	}
	if f.StartTime != nil {
		q = q.Where("operation_time >= ?", f.StartTime)
	}
	if f.EndTime != nil {
		q = q.Where("operation_time <= ?", f.EndTime)
	}
	if f.SearchText != "" {
		like := "%" + f.SearchText + "%"
		q = q.Where("log_name LIKE ? OR element_id IN (SELECT ne_neid FROM cpe_element WHERE device_name LIKE ? OR serial_number LIKE ?)",
			like, like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []NorthInterfaceLog
	offset := (page - 1) * pageSize
	err := q.Order("operation_time DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

// GetByID fetches a single log by primary key.
func (r *Repository) GetByID(id uint) (*NorthInterfaceLog, error) {
	var l NorthInterfaceLog
	err := r.db.First(&l, id).Error
	return &l, err
}
