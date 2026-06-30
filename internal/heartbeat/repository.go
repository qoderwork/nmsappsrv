package heartbeat

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

// Repository handles persistence for heartbeat records and configuration.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository backed by the given database connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// SaveHeartbeatRecord persists a heartbeat history record.
func (r *Repository) SaveHeartbeatRecord(record *HeartbeatRecord) error {
	return r.db.Create(record).Error
}

// GetHeartbeatConfig reads the heartbeat configuration from the system_config table.
// Returns a default config if the key is not found.
func (r *Repository) GetHeartbeatConfig() (*HeartbeatConfig, error) {
	var value string
	err := r.db.Table("system_configs").
		Where("`key` = ?", "heartbeat_config").
		Pluck("value", &value).Error
	if err != nil {
		return defaultHeartbeatConfig(), nil
	}
	if value == "" {
		return defaultHeartbeatConfig(), nil
	}

	var cfg HeartbeatConfig
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse heartbeat config: %w", err)
	}
	return &cfg, nil
}

func defaultHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		Enabled:         false,
		IntervalSeconds: 60,
		SASEndpoint:     "",
	}
}
