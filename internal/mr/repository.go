package mr

import (
	"time"

	"gorm.io/gorm"
)

// Repository persists MR measurement rows and upload logs.
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) SaveBatch(rows []MRData) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.CreateInBatches(rows, 500).Error
}

// ListByElementTime returns MR rows for an element within [start,end].
func (r *Repository) ListByElementTime(elementID int64, start, end time.Time) ([]MRData, error) {
	var rows []MRData
	err := r.db.Where("element_id = ? AND start_time >= ? AND end_time <= ?", elementID, start, end).
		Find(&rows).Error
	return rows, err
}

// ListCellIDs returns distinct cellIds for an element within [start,end].
func (r *Repository) ListCellIDs(elementID int64, start, end time.Time) ([]string, error) {
	var ids []string
	err := r.db.Model(&MRData{}).
		Where("element_id = ? AND start_time >= ? AND end_time <= ?", elementID, start, end).
		Distinct("cell_id").Pluck("cell_id", &ids).Error
	return ids, err
}

func (r *Repository) CreateLog(log *MRFileLog) error {
	return r.db.Create(log).Error
}

func (r *Repository) ListLogs(page, pageSize int) ([]MRFileLog, int64, error) {
	var total int64
	r.db.Model(&MRFileLog{}).Count(&total)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	var logs []MRFileLog
	err := r.db.Order("upload_time DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

func (r *Repository) GetLogByID(id int64) (MRFileLog, error) {
	var log MRFileLog
	err := r.db.First(&log, "id = ?", id).Error
	return log, err
}
