package snmp

import "time"

// SnmpTrapLog records incoming SNMP traps received from network devices.
type SnmpTrapLog struct {
	Id          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId   *int64    `gorm:"column:element_id;index" json:"element_id"`
	SourceIP    string    `gorm:"column:source_ip;type:varchar(64)" json:"source_ip"`
	TrapOID     string    `gorm:"column:trap_oid;type:varchar(255)" json:"trap_oid"`
	VarBinds    string    `gorm:"column:var_binds;type:text" json:"var_binds"` // JSON
	ReceiveTime time.Time `gorm:"column:receive_time" json:"receive_time"`
}

func (SnmpTrapLog) TableName() string { return "snmp_trap_log" }

// SnmpOperationLog records SNMP GET/SET/TRAP operations for audit purposes.
type SnmpOperationLog struct {
	Id          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId   *int64    `gorm:"column:element_id;index" json:"element_id"`
	Operation   string    `gorm:"column:operation;type:varchar(32)" json:"operation"`   // GET, SET, TRAP
	OID         string    `gorm:"column:oid;type:varchar(512)" json:"oid"`
	Value       string    `gorm:"column:value;type:text" json:"value"`
	Status      string    `gorm:"column:status;type:varchar(32)" json:"status"` // SUCCESS, FAILED
	ErrorMsg    string    `gorm:"column:error_msg;type:text" json:"error_msg"`
	Operator    string    `gorm:"column:operator;type:varchar(128)" json:"operator"`
	OperateTime time.Time `gorm:"column:operate_time" json:"operate_time"`
}

func (SnmpOperationLog) TableName() string { return "snmp_operation_log" }
