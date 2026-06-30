package snmp

import (
	"context"
	"encoding/json"
	"sync"

	"gorm.io/gorm"

	"nmsappsrv/internal/mq"
	"nmsappsrv/pkg/logger"
	redisclient "nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
)

// VIPChangePayload mirrors the payload published by the HA VIP monitor.
type VIPChangePayload struct {
	OldVIP    string `json:"old_vip"`
	NewVIP    string `json:"new_vip"`
	Timestamp string `json:"timestamp"`
}

// VIPSubscriber listens for HA VIP change events on Redis pub/sub and updates
// SNMP trap target addresses in the database accordingly.
type VIPSubscriber struct {
	db     *gorm.DB
	mu     sync.Mutex
	running bool
	stopCh  chan struct{}
}

// NewVIPSubscriber creates a new VIPSubscriber.
func NewVIPSubscriber(db *gorm.DB) *VIPSubscriber {
	return &VIPSubscriber{
		db: db,
	}
}

// Start subscribes to the VIP change channel and processes messages.
func (s *VIPSubscriber) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})

	utils.SafeGo("snmp-vip-subscriber", func() {
		s.run()
	})

	logger.Info("SNMP VIP subscriber started")
}

// Stop unsubscribes and cleans up.
func (s *VIPSubscriber) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
	logger.Info("SNMP VIP subscriber stopped")
}

// IsRunning returns whether the subscriber is currently active.
func (s *VIPSubscriber) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// run is the main loop that listens for VIP change notifications.
func (s *VIPSubscriber) run() {
	ctx := context.Background()
	pubsub := redisclient.Subscribe(ctx, mq.ChannelVIPChange)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-s.stopCh:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			s.handleMessage(msg.Payload)
		}
	}
}

// handleMessage parses the VIP change payload and triggers trap target updates.
func (s *VIPSubscriber) handleMessage(payload string) {
	var change VIPChangePayload
	if err := json.Unmarshal([]byte(payload), &change); err != nil {
		logger.Errorf("SNMP VIP subscriber: failed to parse payload: %v", err)
		return
	}

	if change.OldVIP == "" || change.NewVIP == "" {
		logger.Warn("SNMP VIP subscriber: received VIP change with empty old/new VIP, skipping")
		return
	}

	logger.Infof("SNMP VIP subscriber: processing VIP change %s -> %s", change.OldVIP, change.NewVIP)

	if err := s.updateTrapTargets(change.OldVIP, change.NewVIP); err != nil {
		logger.Errorf("SNMP VIP subscriber: failed to update trap targets: %v", err)
	}
}

// updateTrapTargets replaces the old VIP with the new VIP in all SNMP trap
// target connection URLs stored in the device table.
func (s *VIPSubscriber) updateTrapTargets(oldVIP, newVIP string) error {
	if s.db == nil {
		return nil
	}

	// Update snmp_connection_url in the device table, replacing old VIP with new VIP.
	// The URL format is like: snmp://<ip>:<port>?community=public&version=2c
	result := s.db.Table("device").
		Where("snmp_connection_url LIKE ?", "%"+oldVIP+"%").
		Update("snmp_connection_url",
			gorm.Expr("REPLACE(snmp_connection_url, ?, ?)", oldVIP, newVIP))

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		logger.Infof("SNMP VIP subscriber: updated %d trap target(s) from %s to %s",
			result.RowsAffected, oldVIP, newVIP)
	}

	// Also update any device records where the device_ip matches the old VIP,
	// in case the VIP is used as a direct device address.
	ipResult := s.db.Table("cpe_element").
		Where("device_ip = ?", oldVIP).
		Update("device_ip", newVIP)

	if ipResult.Error != nil {
		logger.Errorf("SNMP VIP subscriber: failed to update cpe_element device_ip: %v", ipResult.Error)
	} else if ipResult.RowsAffected > 0 {
		logger.Infof("SNMP VIP subscriber: updated %d cpe_element device_ip(s) from %s to %s",
			ipResult.RowsAffected, oldVIP, newVIP)
	}

	// Update the Redis key so the VIP monitor and other components see the new VIP.
	ctx := context.Background()
	_ = redisclient.Set(ctx, "ha:vip:current", newVIP, 0)

	logger.Infof("SNMP VIP subscriber: VIP change processing complete (%s -> %s)", oldVIP, newVIP)

	return nil
}
