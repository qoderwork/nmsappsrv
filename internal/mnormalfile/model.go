package mnormalfile

import (
	"time"
)

// MNormalFile 对应 mnormal_file 表 — tracks a file uploaded for device download.
type MNormalFile struct {
	Id             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	FileId         string     `gorm:"column:file_id;type:varchar(64);uniqueIndex" json:"fileId"`
	FileName       *string    `gorm:"column:file_name;type:varchar(255)" json:"fileName"`
	FileSize       *int64     `gorm:"column:file_size" json:"fileSize"`
	FileMd5        *string    `gorm:"column:file_md5;type:varchar(64)" json:"fileMd5"`
	ChunkCount     *int       `gorm:"column:chunk_count" json:"chunkCount"`
	Status         *int       `gorm:"column:status" json:"status"` // 1=uploading, 2=assembled, 3=complete
	TenantId      *int       `gorm:"column:tenant_id" json:"tenantId"`
	CreateTime     *time.Time `gorm:"column:create_time" json:"createTime"`
	UpdateTime     *time.Time `gorm:"column:update_time" json:"updateTime"`
	Username       *string    `gorm:"column:username;type:varchar(255)" json:"username"`
	OriginalName   *string    `gorm:"column:original_name;type:varchar(255)" json:"originalName"`
}

func (MNormalFile) TableName() string { return "mnormal_file" }

// MNormalFileChunk 对应 mnormal_file_chunk 表 — tracks individual uploaded chunks.
type MNormalFileChunk struct {
	Id         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	FileId     string     `gorm:"column:file_id;type:varchar(64);index" json:"fileId"`
	ChunkIndex int        `gorm:"column:chunk_index" json:"chunkIndex"`
	ChunkMd5   *string    `gorm:"column:chunk_md5;type:varchar(64)" json:"chunkMd5"`
	ChunkSize  *int64     `gorm:"column:chunk_size" json:"chunkSize"`
	CreateTime *time.Time `gorm:"column:create_time" json:"createTime"`
}

func (MNormalFileChunk) TableName() string { return "mnormal_file_chunk" }

// MNormalFileDownloadLog 对应 mnormal_file_download_log 表 — tracks download-to-device operations.
type MNormalFileDownloadLog struct {
	Id             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	FileId         string     `gorm:"column:file_id;type:varchar(64);index" json:"fileId"`
	ElementId      *int64     `gorm:"column:element_id" json:"elementId"`
	SerialNumber   *string    `gorm:"column:serial_number;type:varchar(255)" json:"serialNumber"`
	CommandTrackId *int64     `gorm:"column:command_track_id" json:"commandTrackId"`
	Status         *int       `gorm:"column:status" json:"status"` // 1=pending, 2=downloading, 3=success, 4=failed
	FaultInfo      *string    `gorm:"column:fault_info;type:text" json:"faultInfo"`
	CreateTime     *time.Time `gorm:"column:create_time" json:"createTime"`
	UpdateTime     *time.Time `gorm:"column:update_time" json:"updateTime"`
}

func (MNormalFileDownloadLog) TableName() string { return "mnormal_file_download_log" }

// DeviceMNormalFile 对应 device_m_normal_file 表 (Java: DeviceMNormalFile entity)
type DeviceMNormalFile struct {
	Id             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	FileName       *string    `gorm:"column:file_name;type:varchar(255)" json:"fileName"`
	OriginalFileName *string `gorm:"column:original_file_name;type:varchar(255)" json:"originalFileName"`
	FilePath       *string    `gorm:"column:file_path;type:varchar(255)" json:"filePath"`
	FileSize       *int64     `gorm:"column:file_size" json:"fileSize"`
	FileMd5        *string    `gorm:"column:file_md5;type:varchar(64)" json:"fileMd5"`
	FileType       *string    `gorm:"column:file_type;type:varchar(50)" json:"fileType"`
	TenantId      *int       `gorm:"column:tenant_id" json:"tenantId"`
	UploadTime     *time.Time `gorm:"column:upload_time" json:"uploadTime"`
	UploadUser     *string    `gorm:"column:upload_user;type:varchar(255)" json:"uploadUser"`
	Deleted        bool       `gorm:"column:deleted;default:false" json:"deleted"`
}

func (DeviceMNormalFile) TableName() string { return "device_m_normal_file" }

// ---------- DTOs ----------

// InitUploadRequest is the JSON body for POST /mnormal-file/init-upload.
type InitUploadRequest struct {
	FileName   string `json:"fileName" binding:"required"`
	FileSize   int64  `json:"fileSize" binding:"required"`
	ChunkCount int    `json:"chunkCount" binding:"required"`
	FileMd5    string `json:"fileMd5"`
}

// InitUploadResponse is the response for init-upload.
type InitUploadResponse struct {
	FileId string `json:"fileId"`
}

// UploadChunkRequest captures the chunk metadata for upload-chunk.
type UploadChunkRequest struct {
	FileId     string `form:"fileId" binding:"required"`
	ChunkIndex int    `form:"chunkIndex" binding:"required"`
	ChunkMd5   string `form:"chunkMd5"`
}

// CheckChunkRequest is the JSON body for POST /mnormal-file/check-chunk.
type CheckChunkRequest struct {
	FileId     string `json:"fileId" binding:"required"`
	ChunkIndex int    `json:"chunkIndex" binding:"required"`
}

// CheckChunkResponse is the response for check-chunk.
type CheckChunkResponse struct {
	Uploaded bool `json:"uploaded"`
}

// AssembleRequest is the JSON body for POST /mnormal-file/assemble.
type AssembleRequest struct {
	FileId string `json:"fileId" binding:"required"`
}

// DeleteRequest is the JSON body for POST /mnormal-file/delete.
type DeleteRequest struct {
	FileId string `json:"fileId" binding:"required"`
}

// ListRequest is the JSON body for POST /mnormal-file/list.
type ListRequest struct {
	FileName string `json:"fileName"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

// DetailRequest is the JSON body for POST /mnormal-file/detail.
type DetailRequest struct {
	FileId string `json:"fileId" binding:"required"`
}

// DownloadToDeviceRequest is the JSON body for POST /mnormal-file/download-to-device.
type DownloadToDeviceRequest struct {
	FileId      string  `json:"fileId" binding:"required"`
	ElementIds  []int64 `json:"elementIds" binding:"required"`
}

// DownloadResultsRequest is the JSON body for POST /mnormal-file/download-results.
type DownloadResultsRequest struct {
	FileId string `json:"fileId"`
	Page   int    `json:"page"`
	PageSize int  `json:"pageSize"`
}
