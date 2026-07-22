package misc

import (
	"sync"
	"time"

	"nmsappsrv/pkg/logger"
)

// AuditLogEntry represents a single operator audit log entry captured by middleware.
type AuditLogEntry struct {
	Username           string
	IPAddress          string
	LogName            string
	RecordDetail       string
	Results            int    // 1=success, 2=failure
	FailureReason      string
	OperationStartTime time.Time
	OperationEndTime   time.Time
	TenantID           int
}

// AuditLogBatchWriter receives audit log entries and persists them in batches.
// This mirrors the Java OperatorLogSaveThread pattern: entries are accumulated
// in a channel and flushed every 5s or when the batch size reaches 1000.
type AuditLogBatchWriter struct {
	repo   Repository
	ch     chan *AuditLogEntry
	done   chan struct{}
	wg     sync.WaitGroup
	batch  []SystemOperatorLog
	mu     sync.Mutex
	ticker *time.Ticker
}

const maxBatchSize = 1000
const flushInterval = 5 * time.Second

// NewAuditLogBatchWriter creates and starts a background goroutine that consumes
// audit log entries and persists them in batches.
func NewAuditLogBatchWriter(repo Repository) *AuditLogBatchWriter {
	w := &AuditLogBatchWriter{
		repo:   repo,
		ch:     make(chan *AuditLogEntry, 5000),
		done:   make(chan struct{}),
		ticker: time.NewTicker(flushInterval),
	}
	w.wg.Add(1)
	go w.run()
	logger.Infof("[audit-log] batch writer started (max_batch=%d, flush_interval=%v)", maxBatchSize, flushInterval)
	return w
}

// Write enqueues an audit log entry for asynchronous persistence.
// It never blocks: if the channel is full the entry is dropped.
func (w *AuditLogBatchWriter) Write(entry *AuditLogEntry) {
	select {
	case w.ch <- entry:
	default:
		// Channel full — drop entry to avoid blocking the HTTP handler.
		logger.Warnf("[audit-log] buffer full, dropping entry: %s", entry.LogName)
	}
}

func (w *AuditLogBatchWriter) run() {
	defer w.wg.Done()
	for {
		select {
		case entry := <-w.ch:
			w.addToBatch(entry)
		case <-w.ticker.C:
			w.flush()
		case <-w.done:
			// Drain remaining entries before shutting down.
			for {
				select {
				case entry := <-w.ch:
					w.addToBatch(entry)
				default:
					w.flush()
					return
				}
			}
		}
	}
}

func (w *AuditLogBatchWriter) addToBatch(entry *AuditLogEntry) {
	w.mu.Lock()
	log := SystemOperatorLog{
		Username:  &entry.Username,
		IpAddress: &entry.IPAddress,
		LogName:   &entry.LogName,
		Results:   &entry.Results,
		TenantId:  &entry.TenantID,
	}
	if entry.RecordDetail != "" {
		log.RecordDetail = &entry.RecordDetail
	}
	if entry.FailureReason != "" {
		log.FailureReason = &entry.FailureReason
	}
	if !entry.OperationStartTime.IsZero() {
		log.OperationStartTime = &entry.OperationStartTime
	}
	if !entry.OperationEndTime.IsZero() {
		log.OperationEndTime = &entry.OperationEndTime
	}
	w.batch = append(w.batch, log)

	shouldFlush := len(w.batch) >= maxBatchSize
	w.mu.Unlock()

	if shouldFlush {
		w.flush()
	}
}

func (w *AuditLogBatchWriter) flush() {
	w.mu.Lock()
	if len(w.batch) == 0 {
		w.mu.Unlock()
		return
	}
	batch := make([]SystemOperatorLog, len(w.batch))
	copy(batch, w.batch)
	w.batch = w.batch[:0]
	w.mu.Unlock()

	if err := w.repo.BatchCreateOperatorLogs(batch); err != nil {
		logger.Errorf("[audit-log] batch insert failed (count=%d): %v", len(batch), err)
	}
}

// Shutdown stops the background goroutine and drains the remaining batch.
func (w *AuditLogBatchWriter) Shutdown() {
	close(w.done)
	w.wg.Wait()
	w.ticker.Stop()
	logger.Infof("[audit-log] batch writer shutdown complete")
}
