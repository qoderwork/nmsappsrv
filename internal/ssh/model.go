package ssh

import "time"

// SystemConfig mirrors system_config table for key-value config storage.
type SystemConfig struct {
	Id    int     `gorm:"primaryKey;autoIncrement" json:"id"`
	Key   *string `gorm:"column:config_key;type:varchar(255);uniqueIndex" json:"key"`
	Value *string `gorm:"column:config_value;type:longtext" json:"value"`
}

func (SystemConfig) TableName() string { return "system_config" }

// ---------- SSH Label ----------

// SSHLabel mirrors ssh_label table.
type SSHLabel struct {
	Id        int     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      *string `gorm:"column:name;type:varchar(255)" json:"name"`
	Content   *string `gorm:"column:content;type:text" json:"content"`
	TenantId *int    `gorm:"column:tenant_id" json:"tenant_id"`
}

func (SSHLabel) TableName() string { return "ssh_label" }

// AddSSHLabelRequest is the JSON body for POST /addSSHLabel.
type AddSSHLabelRequest struct {
	Name    string `json:"name" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// UpdateSSHLabelRequest is the JSON body for POST /updateSSHLabel.
type UpdateSSHLabelRequest struct {
	Id      int    `json:"id" binding:"required"`
	Name    string `json:"name" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// DeleteSSHLabelRequest is the JSON body for POST /deleteSSHLabel.
type DeleteSSHLabelRequest struct {
	Id int `json:"id" binding:"required"`
}

// ---------- SSH Access Timer ----------

// SSHAccessTimerTask mirrors ssh_access_timer_task table.
type SSHAccessTimerTask struct {
	Id               int        `gorm:"primaryKey;autoIncrement" json:"id"`
	TenancyName      *string    `gorm:"column:tenancy_name;type:varchar(255)" json:"tenancy_name"`
	TenantId        *int       `gorm:"column:tenant_id" json:"tenant_id"`
	ElementId        *int64     `gorm:"column:element_id" json:"element_id"`
	SshStatus        *string    `gorm:"column:ssh_status;type:varchar(10)" json:"ssh_status"`
	DeviceName       *string    `gorm:"column:device_name;type:varchar(255)" json:"device_name"`
	SerialNumber     *string    `gorm:"column:serial_number;type:varchar(255)" json:"serial_number"`
	Deadline         *time.Time `gorm:"column:deadline" json:"deadline"`
	LatestModifyTime *time.Time `gorm:"column:latest_modify_time" json:"latest_modify_time"`
}

func (SSHAccessTimerTask) TableName() string { return "ssh_access_timer_task" }

// SSHAccessTimerRequest is the JSON body for POST /sshAccessTimer.
type SSHAccessTimerRequest struct {
	Deadline       int64    `json:"deadline"`
	ElementIds     []int64  `json:"elementIds"`
	DeviceGroupIds []string `json:"deviceGroupIds"`
}

// ListSSHAccessTimerRequest is the JSON body for POST /listSSHAccessTimer.
type ListSSHAccessTimerRequest struct {
	ElementId *int64 `json:"elementId"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
}

// SSHAccessTimerVO is the response item for SSH access timer listing.
type SSHAccessTimerVO struct {
	Id           int        `json:"id"`
	ElementId    int64      `json:"elementId"`
	DeviceName   string     `json:"deviceName"`
	SerialNumber string     `json:"serialNumber"`
	SshStatus    string     `json:"sshStatus"`
	Deadline     *time.Time `json:"deadline"`
	TenancyName  string     `json:"tenancyName"`
}

// ---------- Internal ----------

type operationMessage struct {
	EventType      string `json:"eventType"`
	NeNeid         int64  `json:"neNeid"`
	Operation      string `json:"operation"`
	OperationParam string `json:"operationParam"`
	OperationUser  string `json:"operationUser"`
	CommandTrackId int64  `json:"commandTrackId"`
	ExpiredAt      int64  `json:"expiredAt"`
}
