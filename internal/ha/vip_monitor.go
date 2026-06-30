package ha

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"gorm.io/gorm"
	goredis "github.com/go-redis/redis/v8"

	"nmsappsrv/internal/config"
	"nmsappsrv/internal/mq"
	"nmsappsrv/pkg/logger"
	redisclient "nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
)

const (
	// redisKeyVIPCurrent is the Redis key that holds the current VIP address.
	redisKeyVIPCurrent = "ha:vip:current"

	// defaultMonitorInterval is the fallback check interval in seconds.
	defaultMonitorInterval = 10
)

// VIPChangePayload is the JSON structure published on VIP change.
type VIPChangePayload struct {
	OldVIP    string `json:"old_vip"`
	NewVIP    string `json:"new_vip"`
	Timestamp string `json:"timestamp"`
}

// VIPMonitor periodically checks the current VIP address (from Redis or config)
// and publishes a notification when a change is detected.
type VIPMonitor struct {
	db  *gorm.DB
	rdb *goredis.Client
	cfg *config.Config

	mu        sync.Mutex
	running   bool
	stopCh    chan struct{}
	cachedVIP string
}

// NewVIPMonitor creates a new VIPMonitor.
func NewVIPMonitor(db *gorm.DB, rdb *goredis.Client, cfg *config.Config) *VIPMonitor {
	return &VIPMonitor{
		db:  db,
		rdb: rdb,
		cfg: cfg,
	}
}

// Start begins periodic VIP monitoring in a background goroutine.
func (m *VIPMonitor) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})

	// Initialize cached VIP from config as the starting value.
	m.cachedVIP = m.cfg.HA.CurrentVIP

	utils.SafeGo("ha-vip-monitor", func() {
		m.monitorLoop()
	})

	logger.Info("HA VIP monitor started")
}

// Stop stops the VIP monitor gracefully.
func (m *VIPMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
	logger.Info("HA VIP monitor stopped")
}

// IsRunning returns whether the monitor is currently active.
func (m *VIPMonitor) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// monitorLoop periodically checks for VIP changes.
func (m *VIPMonitor) monitorLoop() {
	interval := m.cfg.HA.VIPMonitorInterval
	if interval <= 0 {
		interval = defaultMonitorInterval
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			oldVIP, newVIP, changed := m.detectVIPChange()
			if changed {
				logger.Infof("HA VIP change detected: %s -> %s", oldVIP, newVIP)
				m.publishVIPChange(oldVIP, newVIP)

				m.mu.Lock()
				m.cachedVIP = newVIP
				m.mu.Unlock()
			}
		}
	}
}

// detectVIPChange compares the current VIP (from Redis or config) with the
// cached value and returns whether a change occurred.
func (m *VIPMonitor) detectVIPChange() (oldVIP, newVIP string, changed bool) {
	ctx := context.Background()

	// Try reading the current VIP from Redis first.
	currentVIP, err := redisclient.Get(ctx, redisKeyVIPCurrent)
	if err != nil || currentVIP == "" {
		// Fall back to config value.
		currentVIP = m.cfg.HA.CurrentVIP
	}

	if currentVIP == "" {
		return "", "", false
	}

	m.mu.Lock()
	cached := m.cachedVIP
	m.mu.Unlock()

	if cached == "" {
		// First run — seed the cache without triggering a change event.
		m.mu.Lock()
		m.cachedVIP = currentVIP
		m.mu.Unlock()
		return "", currentVIP, false
	}

	if currentVIP != cached {
		return cached, currentVIP, true
	}

	return "", "", false
}

// publishVIPChange publishes a VIP change event to the Redis pub/sub channel.
func (m *VIPMonitor) publishVIPChange(oldVIP, newVIP string) {
	payload := VIPChangePayload{
		OldVIP:    oldVIP,
		NewVIP:    newVIP,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if err := mq.PublishEvent(context.Background(), mq.ChannelVIPChange, payload); err != nil {
		logger.Errorf("HA VIP monitor: failed to publish VIP change: %v", err)
	} else {
		data, _ := json.Marshal(payload)
		logger.Infof("HA VIP monitor: published VIP change event: %s", string(data))
	}
}
