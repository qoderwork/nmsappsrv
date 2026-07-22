package mnormalfile

import (
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for MNormal file operations.
type Repository interface {
	CreateFile(f *MNormalFile) error
	FindFileByFileId(fileId string) (*MNormalFile, error)
	UpdateFile(f *MNormalFile) error
	DeleteFile(fileId string) error
	FindFiles(licenseId int, fileName string, offset, limit int) ([]MNormalFile, int64, error)

	CreateChunk(chunk *MNormalFileChunk) error
	FindChunk(fileId string, chunkIndex int) (*MNormalFileChunk, error)
	FindChunksByFileId(fileId string) ([]MNormalFileChunk, error)

	CreateDownloadLog(log *MNormalFileDownloadLog) error
	FindDownloadLogs(fileId string, offset, limit int) ([]MNormalFileDownloadLog, int64, error)
	UpdateDownloadLog(log *MNormalFileDownloadLog) error
	FindDownloadLogByTrackId(trackId int64) (*MNormalFileDownloadLog, error)

	DB() *gorm.DB
}

// repository is the concrete GORM-backed implementation.
type repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// DB returns the underlying *gorm.DB.
func (r *repository) DB() *gorm.DB {
	return r.db
}

// CreateFile inserts a new MNormal file record.
func (r *repository) CreateFile(f *MNormalFile) error {
	return r.db.Create(f).Error
}

// FindFileByFileId loads a file by its fileId.
func (r *repository) FindFileByFileId(fileId string) (*MNormalFile, error) {
	var f MNormalFile
	if err := r.db.Where("file_id = ?", fileId).First(&f).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

// UpdateFile saves changes to an existing MNormal file record.
func (r *repository) UpdateFile(f *MNormalFile) error {
	return r.db.Save(f).Error
}

// DeleteFile removes a file record by fileId.
func (r *repository) DeleteFile(fileId string) error {
	return r.db.Where("file_id = ?", fileId).Delete(&MNormalFile{}).Error
}

// FindFiles returns a paginated list of MNormal file records.
func (r *repository) FindFiles(licenseId int, fileName string, offset, limit int) ([]MNormalFile, int64, error) {
	var list []MNormalFile
	var total int64
	query := r.db.Model(&MNormalFile{})
	if licenseId > 0 {
		query = query.Where("license_id = ?", licenseId)
	}
	if fileName != "" {
		query = query.Where("file_name LIKE ?", "%"+fileName+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindFiles count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		logger.Errorf("FindFiles query error: %v", err)
		return nil, 0, err
	}
	return list, total, nil
}

// CreateChunk inserts a chunk record.
func (r *repository) CreateChunk(chunk *MNormalFileChunk) error {
	return r.db.Create(chunk).Error
}

// FindChunk finds a chunk by fileId and chunkIndex.
func (r *repository) FindChunk(fileId string, chunkIndex int) (*MNormalFileChunk, error) {
	var chunk MNormalFileChunk
	if err := r.db.Where("file_id = ? AND chunk_index = ?", fileId, chunkIndex).First(&chunk).Error; err != nil {
		return nil, err
	}
	return &chunk, nil
}

// FindChunksByFileId returns all chunks for a given fileId.
func (r *repository) FindChunksByFileId(fileId string) ([]MNormalFileChunk, error) {
	var chunks []MNormalFileChunk
	if err := r.db.Where("file_id = ?", fileId).Order("chunk_index ASC").Find(&chunks).Error; err != nil {
		return nil, err
	}
	return chunks, nil
}

// CreateDownloadLog inserts a download log record.
func (r *repository) CreateDownloadLog(log *MNormalFileDownloadLog) error {
	return r.db.Create(log).Error
}

// FindDownloadLogs returns paginated download logs.
func (r *repository) FindDownloadLogs(fileId string, offset, limit int) ([]MNormalFileDownloadLog, int64, error) {
	var list []MNormalFileDownloadLog
	var total int64
	query := r.db.Model(&MNormalFileDownloadLog{})
	if fileId != "" {
		query = query.Where("file_id = ?", fileId)
	}
	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindDownloadLogs count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		logger.Errorf("FindDownloadLogs query error: %v", err)
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateDownloadLog saves changes to a download log record.
func (r *repository) UpdateDownloadLog(log *MNormalFileDownloadLog) error {
	return r.db.Save(log).Error
}

// FindDownloadLogByTrackId finds a download log by command track ID.
func (r *repository) FindDownloadLogByTrackId(trackId int64) (*MNormalFileDownloadLog, error) {
	var log MNormalFileDownloadLog
	if err := r.db.Where("command_track_id = ?", trackId).First(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}
