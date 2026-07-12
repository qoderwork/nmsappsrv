package pmfile

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"nmsappsrv/internal/mq"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Processor is a background worker that consumes PM file messages from the
// queue:pm Redis queue and parses uploaded PM files asynchronously.
type Processor struct {
	db      *gorm.DB
	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

// NewProcessor creates a new Processor backed by db.
func NewProcessor(db *gorm.DB) *Processor {
	return &Processor{db: db}
}

// Start begins the PM file processing loop.
func (p *Processor) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.stopCh = make(chan struct{})
	p.mu.Unlock()

	logger.Info("pmfile: processor starting")

	utils.SafeGo("pmfile-processor", func() {
		p.pollLoop()
	})
}

// Stop stops the processor gracefully.
func (p *Processor) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return
	}
	p.running = false
	close(p.stopCh)
	logger.Info("pmfile: processor stopped")
}

// IsRunning returns whether the processor is currently running.
func (p *Processor) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// pollLoop continuously polls the PM Redis queue for file processing messages.
func (p *Processor) pollLoop() {
	for p.IsRunning() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		result, err := redis.BRPop(ctx, 5*time.Second, mq.PMQueue)
		cancel()

		if err != nil {
			if err.Error() == "redis: nil" {
				continue
			}
			if !p.IsRunning() {
				return
			}
			logger.Debugf("pmfile: queue poll error (may be timeout): %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if len(result) < 2 {
			continue
		}

		// result[0] is the queue name, result[1] is the message JSON
		msgJSON := result[1]

		var msg PMQueueMessage
		if err := json.Unmarshal([]byte(msgJSON), &msg); err != nil {
			logger.Errorf("pmfile: failed to unmarshal message: %v, data: %s", err, msgJSON)
			continue
		}

		logger.Infof("pmfile: processing file id=%d, name=%s, element=%d", msg.PMFileId, msg.FileName, msg.ElementId)
		p.processFile(&msg)
	}
}

// processFile handles a single PM file message: updates status, parses the XML,
// and writes KPI measurements to the database.
func (p *Processor) processFile(msg *PMQueueMessage) {
	// Mark as processing
	now := time.Now()
	p.db.Model(&PMFile{}).Where("id = ?", msg.PMFileId).Updates(map[string]interface{}{
		"status":       StatusProcessing,
		"process_time": now,
	})

	// Parse the PM file
	parseResult, err := ParseXMLFileV2(msg.FilePath)
	if err != nil {
		p.markFailed(msg.PMFileId, fmt.Sprintf("parse error: %v", err))
		return
	}

	// Parse the begin time for measurements
	measureTime, _ := time.Parse("2006-01-02T15:04:05", parseResult.BeginTime)
	if measureTime.IsZero() {
		measureTime = time.Now()
	}

	// Convert KPI values to measurements and batch insert
	if len(parseResult.KPIs) > 0 {
		measurements := make([]PMKPIMeasurement, 0, len(parseResult.KPIs))
		for _, kpi := range parseResult.KPIs {
			val, parseErr := strconv.ParseFloat(kpi.Value, 64)
			if parseErr != nil {
				// Skip non-numeric values
				continue
			}
			measurements = append(measurements, PMKPIMeasurement{
				ElementId:     msg.ElementId,
				KPIName:       kpi.KpiName,
				MeasuredValue: val,
				MeasObjLdn:    kpi.MeasObjLdn,
				CellIdentity:  kpi.CellIdentity,
				MeasureTime:   measureTime,
				PMFileId:      msg.PMFileId,
				CreateTime:    time.Now(),
			})
		}

		// Batch insert in chunks to avoid oversized queries
		batchSize := 500
		for i := 0; i < len(measurements); i += batchSize {
			end := i + batchSize
			if end > len(measurements) {
				end = len(measurements)
			}
			batch := measurements[i:end]
			if err := p.db.CreateInBatches(batch, len(batch)).Error; err != nil {
				p.markFailed(msg.PMFileId, fmt.Sprintf("batch insert error: %v", err))
				return
			}
		}

		logger.Infof("pmfile: inserted %d KPI measurements for file id=%d", len(measurements), msg.PMFileId)
	}

	// Mark as done
	p.db.Model(&PMFile{}).Where("id = ?", msg.PMFileId).Update("status", StatusDone)
	logger.Infof("pmfile: file id=%d processed successfully", msg.PMFileId)
}

// markFailed updates the PM file status to failed with the given error message.
func (p *Processor) markFailed(fileId int64, errMsg string) {
	p.db.Model(&PMFile{}).Where("id = ?", fileId).Updates(map[string]interface{}{
		"status":      StatusFailed,
		"parse_error": errMsg,
	})
	logger.Errorf("pmfile: file id=%d failed: %s", fileId, errMsg)
}
