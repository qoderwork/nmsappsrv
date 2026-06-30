package parameter

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// Scheduler manages periodic parameter collection and deployment tasks.
type Scheduler struct {
	db      *gorm.DB
	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

// NewScheduler creates a new parameter Scheduler.
func NewScheduler(db *gorm.DB) *Scheduler {
	return &Scheduler{
		db: db,
	}
}

// Start begins the scheduler loop. It checks every minute if any scheduled
// parameter collection task is due.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	logger.Info("parameter scheduler starting")

	go s.loop()
}

// Stop stops the scheduler gracefully.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
	logger.Info("parameter scheduler stopped")
}

// IsRunning returns whether the scheduler is currently running.
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// scheduleConfig is the JSON structure stored in system_config under
// the key "parameter_schedule_config".
type scheduleConfig struct {
	// Enable turns the periodic collection on/off.
	Enable bool `json:"enable"`
	// IntervalMinutes is how often to run GPV collection.
	IntervalMinutes int `json:"intervalMinutes"`
	// ElementIds are the devices to collect from. If empty, all online devices.
	ElementIds []int64 `json:"elementIds"`
	// ParamPaths are the parameter paths to collect. If empty, uses basic params.
	ParamPaths []string `json:"paramPaths"`
	// LastRunTime is the last time the scheduler ran (ISO 8601).
	LastRunTime string `json:"lastRunTime"`
}

// loop is the main scheduler loop. It ticks every minute and checks if any
// scheduled task is due.
func (s *Scheduler) loop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

// tick checks if a scheduled parameter collection is due and executes it.
func (s *Scheduler) tick() {
	cfg, err := s.loadScheduleConfig()
	if err != nil {
		logger.Warnf("parameter scheduler: failed to load config: %v", err)
		return
	}

	if cfg == nil || !cfg.Enable {
		return
	}

	interval := cfg.IntervalMinutes
	if interval <= 0 {
		interval = 60 // default: every hour
	}

	// Check if it's time to run
	if cfg.LastRunTime != "" {
		lastRun, err := time.Parse(time.RFC3339, cfg.LastRunTime)
		if err == nil {
			nextRun := lastRun.Add(time.Duration(interval) * time.Minute)
			if time.Now().Before(nextRun) {
				return // not yet due
			}
		}
	}

	// Execute the collection
	s.executeCollection(cfg)
}

// executeCollection performs the scheduled GPV collection.
func (s *Scheduler) executeCollection(cfg *scheduleConfig) {
	ctx := context.Background()
	now := time.Now()

	// Determine target devices
	elementIds := cfg.ElementIds
	if len(elementIds) == 0 {
		// Collect from all online devices
		s.db.Table("cpe_element").
			Select("ne_neid").
			Where("deleted = ? AND serial_number IS NOT NULL AND serial_number != ''", false).
			Scan(&elementIds)
	}

	if len(elementIds) == 0 {
		logger.Debug("parameter scheduler: no target devices")
		s.updateLastRunTime(cfg, now)
		return
	}

	// Determine parameter paths
	paramPaths := cfg.ParamPaths
	if len(paramPaths) == 0 {
		paramPaths = getBasicParamPathsHelper("")
	}

	dispatched := 0
	for _, elementId := range elementIds {
		// Resolve device SN
		var deviceInfo struct {
			SerialNumber string `gorm:"column:serial_number"`
		}
		if err := s.db.Table("cpe_element").
			Select("serial_number").
			Where("ne_neid = ? AND deleted = ?", elementId, false).
			Scan(&deviceInfo).Error; err != nil {
			continue
		}
		if deviceInfo.SerialNumber == "" {
			continue
		}

		// Check if device is online
		onlineKey := fmt.Sprintf("device:online:%s", deviceInfo.SerialNumber)
		onlineVal, _ := redis.Get(ctx, onlineKey)
		if onlineVal != "1" {
			continue
		}

		// Build GPV SOAP XML
		headerId := soap.GenerateHeaderID()
		soapXml := soap.BuildGetParameterValues(headerId, paramPaths)

		// Create event_log for tracking
		eventType := "GET_PARAMETER_VALUES"
		trackData, _ := json.Marshal(map[string]interface{}{
			"header_id":      headerId,
			"serial_number":  deviceInfo.SerialNumber,
			"operation_type": eventType,
			"is_scheduled":   true,
			"issue_time":     now.Format(time.RFC3339),
		})

		row := struct {
			Id               int64     `gorm:"primaryKey;autoIncrement"`
			EventType        string    `gorm:"column:event_type;type:varchar(255)"`
			OperationTime    time.Time `gorm:"column:operation_time"`
			User             string    `gorm:"column:user;type:varchar(255)"`
			ElementId        int64     `gorm:"column:element_id"`
			Status           int       `gorm:"column:status"`
			CommandTrackData string    `gorm:"column:command_track_data;type:longtext"`
		}{
			EventType:        eventType,
			OperationTime:    now,
			User:             "scheduler",
			ElementId:        elementId,
			Status:           1,
			CommandTrackData: string(trackData),
		}
		if err := s.db.Table("event_log").Create(&row).Error; err != nil {
			logger.Warnf("parameter scheduler: create event_log for device %d: %v", elementId, err)
			continue
		}

		// Cache track data in Redis
		trackKey := fmt.Sprintf("tr069:track:%s", headerId)
		trackJson, _ := json.Marshal(map[string]interface{}{
			"header_id":      headerId,
			"sn":             deviceInfo.SerialNumber,
			"operation_type": eventType,
			"event_log_id":   row.Id,
			"is_scheduled":   true,
		})
		redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

		// Push to device queue
		queueKey := fmt.Sprintf("tr069:queue:%s", deviceInfo.SerialNumber)
		if err := redis.LPush(ctx, queueKey, soapXml); err != nil {
			logger.Warnf("parameter scheduler: push GPV to device %s: %v", deviceInfo.SerialNumber, err)
			continue
		}
		redis.Expire(ctx, queueKey, 24*time.Hour)
		dispatched++
	}

	logger.Infof("parameter scheduler: dispatched GPV to %d/%d devices", dispatched, len(elementIds))

	// Update last run time
	s.updateLastRunTime(cfg, now)
}

// loadScheduleConfig reads the schedule config from system_config.
func (s *Scheduler) loadScheduleConfig() (*scheduleConfig, error) {
	var row struct {
		Value *string `gorm:"column:value"`
	}
	err := s.db.Table("system_config").
		Select("value").
		Where("config_key = ?", "parameter_schedule_config").
		First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	if row.Value == nil || *row.Value == "" {
		return nil, nil
	}
	var cfg scheduleConfig
	if err := json.Unmarshal([]byte(*row.Value), &cfg); err != nil {
		return nil, fmt.Errorf("invalid schedule config: %w", err)
	}
	return &cfg, nil
}

// updateLastRunTime saves the last run time back to system_config.
func (s *Scheduler) updateLastRunTime(cfg *scheduleConfig, now time.Time) {
	cfg.LastRunTime = now.Format(time.RFC3339)
	cfgJson, _ := json.Marshal(cfg)
	cfgStr := string(cfgJson)

	s.db.Table("system_config").
		Where("config_key = ?", "parameter_schedule_config").
		Update("value", cfgStr)
}
