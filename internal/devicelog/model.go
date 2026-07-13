package devicelog

import "time"

// NeLog 对应 ne_log 表 (device log file records).
// 列名对齐 Java DeviceLogFileLog（ne_log 表）：log_name / generated_time /
// ne_id / is_active_log，避免与 Java 共用同一张表时读写列错位。
type NeLog struct {
	Id             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId      *int64     `gorm:"column:ne_id;index" json:"element_id"`
	FileName       *string    `gorm:"column:log_name;type:varchar(255)" json:"file_name"`
	FilePath       *string    `gorm:"column:file_path;type:varchar(255)" json:"file_path"`
	FileSize       *int64     `gorm:"column:file_size" json:"file_size"`
	CollectionTime *time.Time `gorm:"column:generated_time" json:"collection_time"`
	EventLogId     *int64     `gorm:"column:event_log_id" json:"event_log_id"`
	Status         *int       `gorm:"column:status" json:"status"` // 1=pending, 2=collecting, 3=done, 4=failed
	FailureReason  *string    `gorm:"column:failure_reason;type:text" json:"failure_reason"`
	IsActiveLog    *bool      `gorm:"column:is_active_log" json:"is_active_log"`
	LicenseId      *int       `gorm:"column:license_id" json:"license_id"`
}

func (NeLog) TableName() string { return "ne_log" }

// --- DTOs ---

type AddLogCollectionRequest struct {
	ElementIds []int64 `json:"elementIds" binding:"required"`
	LogType    string  `json:"logType"` // "all", "syslog", "crash", etc.
}

type ListLogCollectionResultRequest struct {
	ElementId  *int64  `json:"elementId"`
	DeviceType *string `json:"deviceType"` // optional filter
	Status     *int    `json:"status"`
	Page       int     `json:"page"`
	PageSize   int     `json:"pageSize"`
}

type LogCollectionResultVo struct {
	Id             int64  `json:"id"`
	ElementId      int64  `json:"elementId"`
	DeviceName     string `json:"deviceName"`
	SerialNumber   string `json:"serialNumber"`
	FileName       string `json:"fileName"`
	FileSize       int64  `json:"fileSize"`
	CollectionTime string `json:"collectionTime"`
	Status         int    `json:"status"`
	FailureReason  string `json:"failureReason"`
}

type DeleteAllLogFileRequest struct {
	ElementId int64 `json:"elementId" binding:"required"`
}

type DeleteLogFileRequest struct {
	LogId int64 `json:"logId" binding:"required"`
}

type ListLogFileRequest struct {
	ElementId int64 `json:"elementId" binding:"required"`
	Page      int   `json:"page"`
	PageSize  int   `json:"pageSize"`
}

type LogFileVo struct {
	Id             int64  `json:"id"`
	FileName       string `json:"fileName"`
	FileSize       int64  `json:"fileSize"`
	CollectionTime string `json:"collectionTime"`
}

type EnablePeriodicUploadRequest struct {
	ElementId int64 `json:"elementId" binding:"required"`
	Interval  int   `json:"interval"` // seconds
}

type DisablePeriodicUploadRequest struct {
	ElementId int64 `json:"elementId" binding:"required"`
}
