package operation

import (
	"context"
	"sync"
	"time"

	"nmsappsrv/internal/mq"
	"nmsappsrv/internal/opmsg"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
)

// Worker is the single-goroutine consumer of `mq.OperationQueue` (Redis LIST
// "operation_queue") that mirrors Java's `@RabbitListener(queues = "operation_queue")`
// thread on `Receiver`.
//
// Concurrency model:
//   - **One** BRPop loop. Java Spring AMQP defaults a single consumer thread
//     per @RabbitListener; the 200 ops/s rate limiter is shared across all
//     operation types and enforces that budget uniformly.
//   - The `Dispatcher` is called synchronously inside the loop. To scale
//     beyond 200 ops/s, the natural extension is N parallel `Worker`
//     instances each calling `BRPop` (Java's analogue: `concurrency = "N"`
//     on the listener). Today we keep parity with Java's single-thread
//     design.
type Worker struct {
	dispatcher *Dispatcher

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

// NewWorker wires the worker. Both `dispatcher` and its `OperationSender`
// must be non-nil before `Start()`.
func NewWorker(dispatcher *Dispatcher) *Worker {
	return &Worker{dispatcher: dispatcher}
}

// Start begins the BRPop loop in a background goroutine. Idempotent.
func (w *Worker) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.mu.Unlock()

	logger.Info("operation worker: starting")
	utils.SafeGo("operation-worker", func() {
		w.run()
	})
}

// Stop signals the loop to exit and waits for the goroutine to finish. Safe
// to call from a different goroutine.
func (w *Worker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	w.running = false
	close(w.stopCh)
	logger.Info("operation worker: stopped")
}

// IsRunning reports whether the BRPop loop is currently active.
func (w *Worker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

func (w *Worker) run() {
	for w.IsRunning() {
		// 5s timeout aligns with `mml/worker`/`upgrade/worker`/`snmp/worker`
		// — long enough to avoid hot-looping the Redis client, short enough
		// to honour Stop() quickly.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result, err := redis.BRPop(ctx, 5*time.Second, mq.OperationQueue)
		cancel()

		if err != nil {
			if err.Error() == "redis: nil" {
				// BRPop timeout: normal idle.
				continue
			}
			if !w.IsRunning() {
				return
			}
			logger.Debugf("operation worker: BRPop idle/timeout: %v", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if len(result) < 2 {
			continue
		}

		payload := result[1]

		msg, err := opmsg.Unmarshal([]byte(payload))
		if err != nil {
			logger.Errorf("operation worker: unmarshal failed: %v, payload=%s", err, payload)
			continue
		}

		// Apply the global 200 ops/s rate limit. This blocks until a token
		// is available; on Stop() the context cancels and we abort cleanly.
		waitCtx, waitCancel := context.WithCancel(context.Background())
		go func() {
			w.mu.Lock()
			defer w.mu.Unlock()
			if !w.running {
				waitCancel()
			}
		}()
		if err := waitRateLimit(waitCtx); err != nil {
			waitCancel()
			if !w.IsRunning() {
				return
			}
			logger.Warnf("operation worker: rate-limit wait interrupted: %v", err)
			continue
		}
		waitCancel()

		// Dispatch synchronously. The dispatcher logs and continues on
		// per-message failure so a single bad message does not stall the
		// queue.
		if err := w.dispatcher.Dispatch(context.Background(), msg); err != nil {
			logger.Errorf("operation worker: dispatch failed: %v (operation=%s neNeid=%d)",
				err, msg.Operation, msg.NeNeid)
		}
	}
}
