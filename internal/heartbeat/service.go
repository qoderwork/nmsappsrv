package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/internal/config"
	"nmsappsrv/pkg/constants"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// HeartbeatService implements the SAS/CBSD heartbeat protocol logic.
type HeartbeatService struct {
	cfg  *config.Config
	repo *Repository
}

// NewHeartbeatService creates a HeartbeatService.
func NewHeartbeatService(db *gorm.DB, cfg *config.Config) *HeartbeatService {
	return &HeartbeatService{
		cfg:  cfg,
		repo: NewRepository(db),
	}
}

// ProcessHeartbeat processes an incoming heartbeat from a SAS/CBSD device.
// It updates the last-seen timestamp in Redis and records the exchange.
func (s *HeartbeatService) ProcessHeartbeat(deviceSN string, payload map[string]interface{}) error {
	ctx := context.Background()
	now := time.Now()

	// Update last-seen timestamp in Redis (key expires after 3x interval)
	ttl := time.Duration(s.cfg.Heartbeat.IntervalSeconds*3) * time.Second
	redisKey := constants.RedisKeyHeartbeat + deviceSN
	if err := redis.Set(ctx, redisKey, now.Format(time.RFC3339), ttl); err != nil {
		logger.Errorf("heartbeat: failed to update redis for %s: %v", deviceSN, err)
	}

	// Extract grant info from payload if present
	grantInfo := ""
	if g, ok := payload["spectrum_grant"]; ok {
		if gs, ok := g.(string); ok {
			grantInfo = gs
		}
	}

	// Record the heartbeat exchange
	record := &HeartbeatRecord{
		DeviceSN:     deviceSN,
		Timestamp:    now,
		Status:       "online",
		ResponseTime: 0,
		GrantInfo:    grantInfo,
	}
	if err := s.repo.Create(record); err != nil {
		logger.Errorf("heartbeat: failed to save record for %s: %v", deviceSN, err)
		return fmt.Errorf("failed to save heartbeat record: %w", err)
	}

	logger.Infof("heartbeat: processed heartbeat from %s", deviceSN)
	return nil
}

// SendHeartbeatRequest sends a heartbeat request to a device via TR-069 SPV.
// It pushes a message to the device's TR-069 queue.
func (s *HeartbeatService) SendHeartbeatRequest(deviceSN string) error {
	ctx := context.Background()

	msg := map[string]interface{}{
		"type":      "heartbeat_request",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat request: %w", err)
	}

	queueKey := fmt.Sprintf("tr069:queue:%s", deviceSN)
	if err := redis.LPush(ctx, queueKey, string(msgBytes)); err != nil {
		return fmt.Errorf("failed to push heartbeat request to queue: %w", err)
	}

	logger.Infof("heartbeat: sent heartbeat request to %s", deviceSN)
	return nil
}

// ListHeartbeatStatus returns the heartbeat status for spectrum-managed devices.
// It queries the cbsd_infos table for registered devices and enriches with Redis data.
func (s *HeartbeatService) ListHeartbeatStatus(query string, page, pageSize int) ([]HeartbeatStatus, int64, error) {
	ctx := context.Background()

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	// Query CBSD devices via repository
	offset := (page - 1) * pageSize
	rows, total, err := s.repo.FindCBSDDevices(query, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	// Build status list with Redis last-seen data
	statuses := make([]HeartbeatStatus, 0, len(rows))
	for _, row := range rows {
		status := HeartbeatStatus{
			DeviceSN:   row.SerialNumber,
			DeviceName: row.DeviceName,
			Status:     "unknown",
		}

		redisKey := constants.RedisKeyHeartbeat + row.SerialNumber
		val, err := redis.Get(ctx, redisKey)
		if err == nil && val != "" {
			t, err := time.Parse(time.RFC3339, val)
			if err == nil {
				status.LastHeartbeat = t
				// Consider online if seen within 2x interval
				threshold := time.Duration(s.cfg.Heartbeat.IntervalSeconds*2) * time.Second
				if time.Since(t) < threshold {
					status.Status = "online"
				} else {
					status.Status = "offline"
				}
			}
		}

		statuses = append(statuses, status)
	}

	return statuses, total, nil
}
