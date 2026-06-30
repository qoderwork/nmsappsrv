package heartbeat

import "time"

// HeartbeatRecord stores the history of heartbeat exchanges with SAS/CBSD devices.
type HeartbeatRecord struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	DeviceSN     string    `gorm:"index;size:128;not null" json:"device_sn"`
	Timestamp    time.Time `gorm:"not null" json:"timestamp"`
	Status       string    `gorm:"size:32;not null" json:"status"`
	ResponseTime int       `json:"response_time"` // milliseconds
	GrantInfo    string    `gorm:"size:256" json:"grant_info"`
}

func (HeartbeatRecord) TableName() string {
	return "heartbeat_records"
}

// HeartbeatConfig is the persisted configuration for the heartbeat subsystem,
// stored in the system_config table under key = "heartbeat_config".
type HeartbeatConfig struct {
	Enabled         bool   `json:"enabled"`
	IntervalSeconds int    `json:"interval_seconds"`
	SASEndpoint     string `json:"sas_endpoint"`
}

// HeartbeatStatus is the DTO returned by the list API.
type HeartbeatStatus struct {
	DeviceSN      string    `json:"device_sn"`
	DeviceName    string    `json:"device_name"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Status        string    `json:"status"` // "online" | "offline" | "unknown"
	SpectrumGrant string    `json:"spectrum_grant"`
}
