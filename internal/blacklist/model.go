package blacklist

import "time"

// ---------- Database entities ----------

// ElementBlackList corresponds to the element_black_list table.
type ElementBlackList struct {
	Id         int       `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	SN         string    `gorm:"column:sn;type:varchar(255);uniqueIndex" json:"sn"`
	Username   string    `gorm:"column:username;type:varchar(255)" json:"username"`
	AddTime    time.Time `gorm:"column:add_time" json:"addTime"`
	TenantId  int       `gorm:"column:tenant_id" json:"tenantId"`
	DeviceType string    `gorm:"column:device_type;type:varchar(50)" json:"deviceType"`
	Reason     string    `gorm:"column:reason;type:varchar(1024)" json:"reason"`
}

func (ElementBlackList) TableName() string { return "element_black_list" }

// BlackListOperationLog corresponds to the black_list_operation_log table.
type BlackListOperationLog struct {
	Id               int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	DeviceSN         string    `gorm:"column:device_sn;type:varchar(255)" json:"deviceSn"`
	DeviceType       string    `gorm:"column:device_type;type:varchar(50)" json:"deviceType"`
	OperationType    string    `gorm:"column:operation_type;type:varchar(20)" json:"operationType"` // ADD / REMOVE
	OperatorUsername string    `gorm:"column:operator_username;type:varchar(255)" json:"operatorUsername"`
	OperationTime    time.Time `gorm:"column:operation_time" json:"operationTime"`
	OperationReason  string    `gorm:"column:operation_reason;type:varchar(1024)" json:"operationReason"`
	TenantId        int       `gorm:"column:tenant_id" json:"tenantId"`
}

func (BlackListOperationLog) TableName() string { return "black_list_operation_log" }

// ---------- Request DTOs ----------

type AddDeviceToBlackListRequest struct {
	SN         string `json:"sn" binding:"required"`
	DeviceType string `json:"deviceType" binding:"required"`
	Reason     string `json:"reason"`
}

type ListBlackListQuery struct {
	SN       string `json:"sn"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

type ListBlackListOperationLogQuery struct {
	SearchText    string  `json:"searchText"`
	DeviceSN      string  `json:"deviceSn"`
	OperationType string  `json:"operationType"`
	DeviceType    string  `json:"deviceType"`
	StartTime     *string `json:"startTime"`
	EndTime       *string `json:"endTime"`
	Page          int     `json:"page"`
	PageSize      int     `json:"pageSize"`
}

type BatchDeleteRequest struct {
	Ids []int `json:"ids" binding:"required"`
}

// ---------- Response VOs ----------

type ListDeviceBlackListVO struct {
	Id          int       `json:"id"`
	SN          string    `json:"sn"`
	Username    string    `json:"username"`
	AddTime     time.Time `json:"addTime"`
	DeviceType  string    `json:"deviceType"`
	TenancyName string    `json:"tenancyName"`
	Reason      string    `json:"reason"`
}

type ListBlackListOperationLogVO struct {
	Id               int64     `json:"id"`
	DeviceSN         string    `json:"deviceSn"`
	DeviceType       string    `json:"deviceType"`
	OperationType    string    `json:"operationType"`
	OperatorUsername string    `json:"operatorUsername"`
	OperationTime    time.Time `json:"operationTime"`
	OperationReason  string    `json:"operationReason"`
	TenancyName      string    `json:"tenancyName"`
}
