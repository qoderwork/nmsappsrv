package snmp

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
)

// Worker polls the Redis SNMP queue and sends traps
type Worker struct {
	mu      sync.Mutex
	running bool
}

// NewWorker creates a new SNMP worker
func NewWorker() *Worker {
	return &Worker{}
}

// Start begins polling the Redis SNMP queue in a background goroutine
func (w *Worker) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	logger.Info("SNMP worker starting")

	utils.SafeGo("snmp-worker", func() {
		w.pollLoop()
	})
}

// Stop stops the worker
func (w *Worker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.running = false
	logger.Info("SNMP worker stopped")
}

// IsRunning returns whether the worker is currently running
func (w *Worker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// pollLoop continuously polls the Redis queue for SNMP messages
func (w *Worker) pollLoop() {
	for w.IsRunning() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		result, err := redis.BRPop(ctx, 1*time.Second, SnmpQueueName)
		cancel()

		if err != nil {
			// Timeout or queue empty is normal, just continue
			if err.Error() == "redis: nil" {
				continue
			}
			// Check if we should stop
			if !w.IsRunning() {
				return
			}
			logger.Debugf("SNMP worker queue poll error (may be timeout): %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if len(result) < 2 {
			continue
		}

		// result[0] is the queue name, result[1] is the message
		msgJSON := result[1]

		var msg SnmpMessage
		if err := json.Unmarshal([]byte(msgJSON), &msg); err != nil {
			logger.Errorf("SNMP worker failed to unmarshal message: %v, data: %s", err, msgJSON)
			continue
		}

		switch msg.OperationType {
		case OperationTrap:
			if err := SendTrap(msg.ConnectionInfo, msg.Payload); err != nil {
				logger.Errorf("SNMP worker failed to send trap: %v", err)
			}
		default:
			logger.Warnf("SNMP worker: unsupported operation type: %d", msg.OperationType)
		}
	}
}
