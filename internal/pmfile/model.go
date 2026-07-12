package pmfile

import "time"

// File status constants
const (
	StatusUploaded   = 1
	StatusProcessing = 2
	StatusDone       = 3
	StatusFailed     = 4
)

// PMFile represents an uploaded Performance Management file record.
type PMFile struct {
	Id          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId   int64      `gorm:"column:element_id;index" json:"element_id"`
	FileName    string     `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	FilePath    string     `gorm:"column:file_path;type:varchar(500)" json:"file_path"`
	FileSize    int64      `gorm:"column:file_size" json:"file_size"`
	FileType    string     `gorm:"column:file_type;type:varchar(50)" json:"file_type"` // xml, csv
	Status      int        `gorm:"column:status" json:"status"`                        // 1=uploaded,2=processing,3=done,4=failed
	ParseError  string     `gorm:"column:parse_error;type:text" json:"parse_error"`
	CreateTime  time.Time  `gorm:"column:create_time" json:"create_time"`
	ProcessTime *time.Time `gorm:"column:process_time" json:"process_time"`
}

func (PMFile) TableName() string { return "pm_file" }

// PMKPIMeasurement stores individual KPI values extracted from PM files.
type PMKPIMeasurement struct {
	Id            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId     int64     `gorm:"column:element_id;index" json:"element_id"`
	KPIName       string    `gorm:"column:kpi_name;type:varchar(255);index" json:"kpi_name"`
	MeasuredValue float64   `gorm:"column:measured_value" json:"measured_value"`
	MeasObjLdn    string    `gorm:"column:meas_obj_ldn;type:varchar(500)" json:"meas_obj_ldn"`
	CellIdentity  string    `gorm:"column:cell_identity;type:varchar(255)" json:"cell_identity"`
	MeasureTime   time.Time `gorm:"column:measure_time;index" json:"measure_time"`
	PMFileId      int64     `gorm:"column:pm_file_id;index" json:"pm_file_id"`
	CreateTime    time.Time `gorm:"column:create_time" json:"create_time"`
}

func (PMKPIMeasurement) TableName() string { return "pm_kpi_measurement" }

// ---------- Request DTOs ----------

// UploadRequest is the form/multipart request for uploading a PM file.
type UploadRequest struct {
	ElementId int64 `form:"elementId" binding:"required"`
}

// PMQueueMessage is the JSON payload pushed to the queue:pm Redis queue
// when a PM file is uploaded and ready for processing.
type PMQueueMessage struct {
	PMFileId  int64 `json:"pm_file_id"`
	ElementId int64 `json:"element_id"`
	FileName  string `json:"file_name"`
	FilePath  string `json:"file_path"`
}

// KPIValue holds a single parsed KPI value from a PM file.
type KPIValue struct {
	KpiName      string `json:"kpi_name"`
	Value        string `json:"value"`
	MeasObjLdn   string `json:"meas_obj_ldn"`
	CellIdentity string `json:"cell_identity"`
}

// PMFileParseResult holds the full result of parsing a PM XML file.
type PMFileParseResult struct {
	BeginTime string     `json:"begin_time"`
	KPIs      []KPIValue `json:"kpis"`
}
