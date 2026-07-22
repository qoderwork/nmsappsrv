package northinterfacelog

import "time"

// NorthInterfaceLog is the audit record for inbound northbound (REST API) calls.
// Column names mirror the Java JPA entity (com.waveoss.stationapinew.dao.entity
// .NorthInterfaceLog) so a table created by the Java service stays compatible.
// Behaviour-aligned note: Java writes this only from a few endpoints
// (presetParameters, femto add/modify/delete, femto reads). The Go rewrite logs
// EVERY northbound call via the AuditMiddleware, which is strictly more complete.
type NorthInterfaceLog struct {
	ID            uint      `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	LogName       string    `gorm:"column:log_name;type:varchar(255)" json:"logName"`
	User          string    `gorm:"column:user;type:varchar(128)" json:"username"`
	OperationTime time.Time `gorm:"column:operation_time" json:"operationTime"`
	Result        int       `gorm:"column:result" json:"results"`
	RequestData   string    `gorm:"column:request_data;type:longtext" json:"requestData"`
	ElementID     int64     `gorm:"column:element_id" json:"elementId"`
	PresetTaskID  int64     `gorm:"column:preset_task_id" json:"presetTaskId"`
	Info          string    `gorm:"column:info;type:varchar(512)" json:"info"`
	TenancyID     int       `gorm:"column:tenant_id" json:"tenantId"`
}

// TableName matches the Java entity table.
func (NorthInterfaceLog) TableName() string { return "north_interface_log" }
