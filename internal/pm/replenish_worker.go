package pm

import (
	"sync"
	"time"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"
)

// ReplenishWorker simulates the Java replenish worker. It periodically
// scans pm_replenish_task rows in status=1 (Waiting) or status=2
// (Executing), and for each task marks every device Done=true (in
// memory; the service holds the per-(task, elementId) Done state).
// When all devices are Done, the task status is bumped to 3 (Executed).
//
// The Go side does NOT actually pull PM data from the device -- that
// would require a real TR-069 PM collector which is out of scope. The
// worker exists so the listDeviceReplenish endpoint can return real
// Done values and the task lifecycle can be exercised end-to-end.
type ReplenishWorker struct {
	svc      Service
	repo     Repository
	interval time.Duration

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

// NewReplenishWorker creates a new worker. interval is the tick period
// (e.g. 30*time.Second for demo; the Java side runs on a similar cadence).
func NewReplenishWorker(svc Service, repo Repository, interval time.Duration) *ReplenishWorker {
	return &ReplenishWorker{
		svc:      svc,
		repo:     repo,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the worker loop. Idempotent.
func (w *ReplenishWorker) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	stopCh := w.stopCh
	w.mu.Unlock()

	logger.Infof("pm replenish-worker: starting (interval=%s)", w.interval)
	utils.SafeGo("pm-replenish-worker", func() {
		w.run(stopCh)
	})
}

// Stop signals the loop to exit.
func (w *ReplenishWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	w.running = false
	close(w.stopCh)
}

func (w *ReplenishWorker) run(stopCh chan struct{}) {
	w.runOnce()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			logger.Info("pm replenish-worker: stopped")
			return
		case <-ticker.C:
			w.runOnce()
		}
	}
}

// runOnce processes every pending task once.
func (w *ReplenishWorker) runOnce() {
	tasks, err := w.repo.FindPendingReplenishTasks(100)
	if err != nil {
		logger.Errorf("pm replenish-worker: load pending tasks: %v", err)
		return
	}
	for _, t := range tasks {
		if t.Id == 0 {
			continue
		}
		// Bump to Executing (2) on first sight. Mirrors the Java
		// replenish worker that flips Waiting->Executing on dispatch.
		if t.Status != nil && *t.Status == 1 {
			if err := w.repo.UpdateReplenishTaskStatus(t.Id, 2); err != nil {
				logger.Errorf("pm replenish-worker: mark task %d Executing: %v", t.Id, err)
			}
		}
		// Pull the device list (cpe_element rows from element_ids).
		devices, err := w.repo.FindReplenishTaskDevices(t.Id)
		if err != nil {
			logger.Errorf("pm replenish-worker: load devices for task %d: %v", t.Id, err)
			continue
		}
		if len(devices) == 0 {
			// No devices: mark task Executed straight away.
			_ = w.repo.UpdateReplenishTaskStatus(t.Id, 3)
			continue
		}
		// Mark each device Done. The Go worker doesn't actually pull
		// data; it just sets the flag so the listDeviceReplenish
		// endpoint returns a useful response.
		for _, d := range devices {
			w.svc.MarkReplenishDeviceDone(t.Id, d.NeNeid)
		}
		// All devices done -> bump to Executed (3).
		if err := w.repo.UpdateReplenishTaskStatus(t.Id, 3); err != nil {
			logger.Errorf("pm replenish-worker: mark task %d Executed: %v", t.Id, err)
		}
		logger.Infof("pm replenish-worker: task %d replenished %d devices", t.Id, len(devices))
	}
}
