package snmp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
)

// SNMPPoller periodically polls SNMP-capable devices for configured OIDs
// and stores the results in element_basic_info_parameter.
type SNMPPoller struct {
	db      *gorm.DB
	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

// NewSNMPPoller creates a new SNMP poller.
func NewSNMPPoller(db *gorm.DB) *SNMPPoller {
	return &SNMPPoller{db: db}
}

// Start begins the SNMP poller loop in a background goroutine.
func (p *SNMPPoller) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.stopCh = make(chan struct{})
	p.mu.Unlock()

	logger.Info("SNMP poller starting")

	utils.SafeGo("snmp-poller", func() {
		p.pollLoop()
	})
}

// Stop stops the SNMP poller gracefully.
func (p *SNMPPoller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return
	}
	p.running = false
	close(p.stopCh)
	logger.Info("SNMP poller stopped")
}

// IsRunning returns whether the poller is currently running.
func (p *SNMPPoller) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// snmpPollConfig is the JSON structure stored in system_config under
// the key "snmp_poll_config".
type snmpPollConfig struct {
	Enabled         bool     `json:"enabled"`
	IntervalMinutes int      `json:"interval_minutes"`
	OIDs            []string `json:"oids"`
	DeviceType      string   `json:"device_type"`
	LastRunTime     string   `json:"last_run_time"`
}

// defaultPollOIDs are the OIDs polled when no OIDs are configured.
var defaultPollOIDs = []string{
	"1.3.6.1.2.1.1.1.0", // sysDescr
	"1.3.6.1.2.1.1.3.0", // sysUpTime
	"1.3.6.1.2.1.2.1.0", // ifNumber
}

// pollLoop is the main poller loop. It ticks at the configured interval.
func (p *SNMPPoller) pollLoop() {
	// Initial tick after a short delay to let the system settle
	select {
	case <-p.stopCh:
		return
	case <-time.After(30 * time.Second):
	}

	p.tick()

	// Use a 1-minute ticker and check interval on each tick
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.tick()
		}
	}
}

// tick checks if a polling cycle is due and executes it.
func (p *SNMPPoller) tick() {
	cfg, err := p.loadPollConfig()
	if err != nil {
		logger.Warnf("SNMP poller: failed to load config: %v", err)
		return
	}

	if cfg == nil || !cfg.Enabled {
		return
	}

	interval := cfg.IntervalMinutes
	if interval <= 0 {
		interval = 5
	}

	// Check if it's time to run based on last run time
	if cfg.LastRunTime != "" {
		lastRun, parseErr := time.Parse(time.RFC3339, cfg.LastRunTime)
		if parseErr == nil {
			nextRun := lastRun.Add(time.Duration(interval) * time.Minute)
			if time.Now().Before(nextRun) {
				return // not yet due
			}
		}
	}

	// Execute the polling
	p.executePoll(cfg)
}

// snmpDeviceTarget holds the minimal device info needed for polling.
type snmpDeviceTarget struct {
	ElementID int64  `gorm:"column:ne_neid"`
	DeviceIP  string `gorm:"column:device_ip"`
}

// executePoll performs SNMP GET on all target devices and stores results.
func (p *SNMPPoller) executePoll(cfg *snmpPollConfig) {
	now := time.Now()

	oids := cfg.OIDs
	if len(oids) == 0 {
		oids = defaultPollOIDs
	}

	// Query all SNMP-capable devices (devices with an IP address)
	var targets []snmpDeviceTarget
	query := p.db.Table("cpe_element").
		Select("ne_neid, device_ip").
		Where("deleted = ? AND device_ip IS NOT NULL AND device_ip != ''", false)

	if cfg.DeviceType != "" && cfg.DeviceType != "all" {
		query = query.Where("device_type = ?", cfg.DeviceType)
	}

	if err := query.Scan(&targets).Error; err != nil {
		logger.Errorf("SNMP poller: failed to query devices: %v", err)
		p.updateLastRunTime(cfg, now)
		return
	}

	if len(targets) == 0 {
		logger.Debug("SNMP poller: no target devices found")
		p.updateLastRunTime(cfg, now)
		return
	}

	successCount := 0
	failCount := 0

	for _, target := range targets {
		// Check if we should stop
		if !p.IsRunning() {
			return
		}

		connInfo := SnmpConnectionInfo{
			IP:        target.DeviceIP,
			Port:      161,
			Version:   2,
			Community: "public",
		}

		results, err := SendGet(p.db, connInfo, oids)
		if err != nil {
			logger.Errorf("SNMP poller: GET failed for device %d (%s): %v", target.ElementID, target.DeviceIP, err)
			failCount++
			continue
		}

		// Store results in element_basic_info_parameter
		p.saveResults(target.ElementID, results, now)
		successCount++
	}

	logger.Infof("SNMP poller: completed poll of %d devices (success=%d, failed=%d, oids=%d)",
		len(targets), successCount, failCount, len(oids))

	// Update last run time
	p.updateLastRunTime(cfg, now)
}

// saveResults stores SNMP GET results into element_basic_info_parameter.
func (p *SNMPPoller) saveResults(elementID int64, results []SnmpParameter, now time.Time) {
	ctx := context.Background()

	for _, param := range results {
		if param.OID == "" {
			continue
		}

		// Use upsert pattern: INSERT ... ON DUPLICATE KEY UPDATE
		rawSQL := `INSERT INTO element_basic_info_parameter (element_id, param_name, param_value, update_time)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE param_value = VALUES(param_value), update_time = VALUES(update_time)`
		if err := p.db.Exec(rawSQL, elementID, param.OID, param.Value, now).Error; err != nil {
			logger.Errorf("SNMP poller: failed to upsert param %s for device %d: %v", param.OID, elementID, err)
		}
	}

	// Cache a freshness timestamp in Redis for quick lookups
	freshnessKey := fmt.Sprintf("snmp:poll:freshness:%d", elementID)
	redis.Set(ctx, freshnessKey, now.Format(time.RFC3339), 24*time.Hour)
}

// loadPollConfig reads the SNMP poll config from system_config.
func (p *SNMPPoller) loadPollConfig() (*snmpPollConfig, error) {
	var row struct {
		Config *string `gorm:"column:config"`
	}
	err := p.db.Table("system_config").
		Select("config").
		Where("id = ?", "snmp_poll_config").
		Limit(1).
		Find(&row).Error
	if err != nil {
		return nil, err
	}
	if row.Config == nil || *row.Config == "" {
		return nil, nil
	}
	var cfg snmpPollConfig
	if err := json.Unmarshal([]byte(*row.Config), &cfg); err != nil {
		return nil, fmt.Errorf("invalid snmp_poll_config: %w", err)
	}
	return &cfg, nil
}

// updateLastRunTime saves the last run time back to system_config.
func (p *SNMPPoller) updateLastRunTime(cfg *snmpPollConfig, now time.Time) {
	cfg.LastRunTime = now.Format(time.RFC3339)
	cfgJson, _ := json.Marshal(cfg)
	cfgStr := string(cfgJson)

	p.db.Table("system_config").
		Where("id = ?", "snmp_poll_config").
		Update("config", cfgStr)
}
