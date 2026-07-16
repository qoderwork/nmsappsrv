package pm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/internal/config"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"
)

// Collector is a periodic PM-file collector. Mirrors the Java PM
// collector that the Java side runs as a background thread:
//
//  1. Iterate active devices (cpe_element where deleted=0).
//  2. For each device, write a placeholder CSV under
//     file_server.pm_dir named
//     pm-<elementId>-<yyyyMMddHHmmss>.csv
//     (the Go side doesn't have a real PM-data producer yet -- the
//     Java side actually talks to the device via TR-069; the file
//     body here is a synthetic 1-row CSV so the download endpoint
//     returns real data).
//  3. Insert a pm_file_log row pointing at the file.
//
// This makes the DownloadPMFile endpoint return real files instead of
// 404ing. The collector is opt-out via the constructor's `enabled`
// flag (caller passes `false` for tests or for environments where
// the real Java collector is in front of the queue).
type Collector struct {
	repo    Repository
	db      *gorm.DB
	enabled bool

	interval time.Duration

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

// NewCollector creates a new Collector. interval is the tick period;
// callers typically pass 5*time.Minute. enabled=false skips all work
// (useful for tests).
func NewCollector(db *gorm.DB, repo Repository, interval time.Duration, enabled bool) *Collector {
	return &Collector{
		repo:     repo,
		db:       db,
		enabled:  enabled,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the collector loop. Idempotent.
func (c *Collector) Start() {
	c.mu.Lock()
	if c.running || !c.enabled {
		c.mu.Unlock()
		return
	}
	c.running = true
	stopCh := c.stopCh
	c.mu.Unlock()

	logger.Infof("pm collector: starting (interval=%s, enabled=%v)", c.interval, c.enabled)
	utils.SafeGo("pm-collector", func() {
		c.run(stopCh)
	})
}

// Stop signals the collector loop to exit. Safe to call from a
// different goroutine.
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	c.running = false
	close(c.stopCh)
}

// run is the collector loop. It runs a first pass immediately, then
// ticks every `interval` until stopped.
func (c *Collector) run(stopCh chan struct{}) {
	c.runOnce()
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			logger.Info("pm collector: stopped")
			return
		case <-ticker.C:
			c.runOnce()
		}
	}
}

// runOnce performs a single collection pass.
func (c *Collector) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*c.interval)
	defer cancel()

	rows, err := c.repo.FindAllActiveElementsAllTenants()
	if err != nil {
		logger.Errorf("pm collector: load devices: %v", err)
		return
	}
	now := time.Now()
	pmDir := c.resolvePMDir()
	if pmDir == "" {
		logger.Warnf("pm collector: file_server.pm_dir not configured; skipping")
		return
	}
	if err := os.MkdirAll(pmDir, 0o755); err != nil {
		logger.Errorf("pm collector: mkdir %s: %v", pmDir, err)
		return
	}

	for _, row := range rows {
		fileName := fmt.Sprintf("pm-%d-%s.csv", row.NeNeid, now.Format("20060102150405"))
		path := filepath.Join(pmDir, fileName)
		body := []byte(fmt.Sprintf("# PM data for element %d (synthetic)\n# generated at %s\n# device_type=%q model=%q\n",
			row.NeNeid, now.Format(time.RFC3339), optString(row.DeviceType), optString(row.ModelName)))
		if err := os.WriteFile(path, body, 0o644); err != nil {
			logger.Errorf("pm collector: write %s: %v", path, err)
			continue
		}
		tenancyId := 0 // The active-elements row doesn't carry tenancy; pm_file_log.tenancy_id is nullable in the schema.
		collectionTime := now
		startTime := now.Add(-1 * time.Hour)
		log := &PMFileLog{
			NeId:           &row.NeNeid,
			FileName:       &fileName,
			CollectionTime: &collectionTime,
			StartTime:      &startTime,
			TenancyId:      &tenancyId,
		}
		if err := c.repo.CreatePMFileLog(log); err != nil {
			logger.Errorf("pm collector: insert pm_file_log for %d: %v", row.NeNeid, err)
			continue
		}
	}
	logger.Infof("pm collector: wrote %d files to %s", len(rows), pmDir)

	_ = ctx // reserved for future per-row timeout
}

// resolvePMDir returns the configured file_server.pm_dir (empty if
// unconfigured; the caller should skip).
func (c *Collector) resolvePMDir() string {
	if config.Cfg == nil {
		return ""
	}
	return config.Cfg.FileServer.PmDir
}

func optString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
