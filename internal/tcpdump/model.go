package tcpdump

import "time"

// Task status constants
const (
	StatusRunning = 1
	StatusDone    = 2
	StatusFailed  = 3
)

// TcpdumpTask represents a tcpdump capture task in the database.
type TcpdumpTask struct {
	Id           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId    int64      `gorm:"column:element_id;index" json:"element_id"`
	Interface    string     `gorm:"column:interface_name;type:varchar(50)" json:"interface_name"`
	Filter       string     `gorm:"column:filter;type:varchar(500)" json:"filter"`
	Duration     int        `gorm:"column:duration" json:"duration"` // seconds
	PacketCount  int        `gorm:"column:packet_count" json:"packet_count"`
	Status       int        `gorm:"column:status" json:"status"` // 1=running, 2=done, 3=failed
	FilePath     string     `gorm:"column:file_path;type:varchar(500)" json:"file_path"`
	FileSize     int64      `gorm:"column:file_size" json:"file_size"`
	ErrorMessage string     `gorm:"column:error_message;type:text" json:"error_message"`
	CreateTime   time.Time  `gorm:"column:create_time" json:"create_time"`
	EndTime      *time.Time `gorm:"column:end_time" json:"end_time"`
}

func (TcpdumpTask) TableName() string { return "tcpdump_task" }

// ---------- Request DTOs ----------

// StartRequest is the JSON body for POST /tcpdump/start.
type StartRequest struct {
	ElementId   int64  `json:"elementId" binding:"required"`
	Interface   string `json:"interface"`                  // default "eth0"
	Filter      string `json:"filter"`                     // pcap filter expression
	Duration    int    `json:"duration" binding:"required"` // seconds, required
	PacketCount int    `json:"packetCount"`                // max packets (0 = unlimited)
}

// StartResponse is returned after a capture task is created.
type StartResponse struct {
	TaskId int64 `json:"taskId"`
}
