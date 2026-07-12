package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
)

const upgradeQueue = "queue:upgrade"

// UpgradeMessage is the JSON payload pushed to the upgrade queue.
type UpgradeMessage struct {
	TaskId        int    `json:"task_id"`
	ElementId     int64  `json:"element_id"`
	UpgradeFileId int    `json:"upgrade_file_id"`
	OperationType string `json:"operation_type"` // UPGRADE | ROLLBACK | REBOOT
	LogUuid       string `json:"log_uuid"`
}

// UpgradeWorker polls the upgrade queue and dispatches TR-069 commands to devices.
type UpgradeWorker struct {
	db       *gorm.DB
	opSender *tr069.OperationSender
	repo     Repository

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

// NewUpgradeWorker creates a new UpgradeWorker.
func NewUpgradeWorker(db *gorm.DB, opSender *tr069.OperationSender) *UpgradeWorker {
	return &UpgradeWorker{
		db:       db,
		opSender: opSender,
		repo:     NewRepository(db),
	}
}

// Start launches the background poll loop.
func (w *UpgradeWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		return
	}
	w.running = true
	w.stopCh = make(chan struct{})
	utils.SafeGo("upgrade-worker", func() {
		w.pollLoop()
	})
	logger.Infof("upgrade worker started")
}

// Stop signals the poll loop to exit.
func (w *UpgradeWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	close(w.stopCh)
	w.running = false
	logger.Infof("upgrade worker stopped")
}

// IsRunning returns true if the worker is active.
func (w *UpgradeWorker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// pollLoop continuously pops messages from the upgrade queue.
func (w *UpgradeWorker) pollLoop() {
	for {
		select {
		case <-w.stopCh:
			return
		default:
		}

		ctx := context.Background()
		result, err := redis.BRPop(ctx, 5*time.Second, upgradeQueue)
		if err != nil {
			// Timeout or error - just loop again.
			continue
		}
		// result[0] = key, result[1] = value
		if len(result) < 2 {
			continue
		}

		var msg UpgradeMessage
		if err := json.Unmarshal([]byte(result[1]), &msg); err != nil {
			logger.Errorf("upgrade worker: failed to parse message: %v", err)
			continue
		}

		w.processMessage(ctx, &msg)
	}
}

// processMessage handles a single upgrade/rollback/reboot dispatch.
func (w *UpgradeWorker) processMessage(ctx context.Context, msg *UpgradeMessage) {
	// Look up device serial number
	sn, _, err := w.repo.FindElementInfo(msg.ElementId)
	if err != nil {
		logger.Errorf("upgrade worker: find element %d: %v", msg.ElementId, err)
		w.updateLogFailure(msg.LogUuid, fmt.Sprintf("element %d not found", msg.ElementId))
		return
	}

	// Load the upgrade log
	logEntry, err := w.findLogByUUID(msg.LogUuid)
	if err != nil {
		logger.Errorf("upgrade worker: find log %s: %v", msg.LogUuid, err)
		return
	}

	switch msg.OperationType {
	case "UPGRADE":
		w.handleUpgrade(ctx, msg, sn, logEntry)
	case "ROLLBACK":
		w.handleRollback(ctx, msg, sn, logEntry)
	case "REBOOT":
		w.handleReboot(ctx, msg, sn, logEntry)
	default:
		logger.Errorf("upgrade worker: unknown operation type %s", msg.OperationType)
		w.updateLogFailure(msg.LogUuid, fmt.Sprintf("unknown operation type: %s", msg.OperationType))
	}
}

// handleUpgrade sends a TR-069 Download request for the upgrade file.
func (w *UpgradeWorker) handleUpgrade(ctx context.Context, msg *UpgradeMessage, sn string, logEntry *UpgradeLog) {
	upFile, err := w.repo.FindByID(msg.UpgradeFileId)
	if err != nil {
		logger.Errorf("upgrade worker: find upgrade file %d: %v", msg.UpgradeFileId, err)
		w.updateLogFailure(msg.LogUuid, fmt.Sprintf("upgrade file %d not found", msg.UpgradeFileId))
		return
	}

	filePath := ""
	if upFile.FilePath != nil {
		filePath = *upFile.FilePath
	}
	fileSize := 0
	if upFile.FileSize != nil {
		fileSize = int(*upFile.FileSize)
	}
	version := ""
	if upFile.Version != nil {
		version = *upFile.Version
	}

	// Build Download request
	commandKey := fmt.Sprintf("upgrade_%d_%d_%d", msg.TaskId, msg.ElementId, time.Now().Unix())
	dl := &soap.Download{
		CommandKey: commandKey,
		FileType:   "1 Firmware Upgrade Image",
		URL:        filePath,
		FileSize:   fileSize,
	}

	operationId := fmt.Sprintf("upgrade_%d_%d", msg.TaskId, msg.ElementId)
	if err := w.opSender.SendDownload(sn, dl, operationId); err != nil {
		logger.Errorf("upgrade worker: SendDownload to %s failed: %v", sn, err)
		w.updateLogFailure(msg.LogUuid, fmt.Sprintf("SendDownload failed: %v", err))
		return
	}

	// Mark log as downloaded
	now := time.Now()
	logEntry.IsDownloaded = boolPtrVal(true)
	logEntry.DownloadedTime = &now
	logEntry.NewVersion = &version
	if err := w.repo.UpdateUpgradeLog(logEntry); err != nil {
		logger.Errorf("upgrade worker: update log after download %s: %v", msg.LogUuid, err)
	}

	logger.Infof("upgrade worker: download dispatched to %s for task %d", sn, msg.TaskId)
}

// handleRollback sends a TR-069 Download request with rollback-specific metadata.
func (w *UpgradeWorker) handleRollback(ctx context.Context, msg *UpgradeMessage, sn string, logEntry *UpgradeLog) {
	upFile, err := w.repo.FindByID(msg.UpgradeFileId)
	if err != nil {
		logger.Errorf("upgrade worker: find rollback file %d: %v", msg.UpgradeFileId, err)
		w.updateLogFailure(msg.LogUuid, fmt.Sprintf("rollback file %d not found", msg.UpgradeFileId))
		return
	}

	filePath := ""
	if upFile.FilePath != nil {
		filePath = *upFile.FilePath
	}
	fileSize := 0
	if upFile.FileSize != nil {
		fileSize = int(*upFile.FileSize)
	}
	version := ""
	if upFile.Version != nil {
		version = *upFile.Version
	}

	commandKey := fmt.Sprintf("rollback_%d_%d_%d", msg.TaskId, msg.ElementId, time.Now().Unix())
	dl := &soap.Download{
		CommandKey: commandKey,
		FileType:   "1 Firmware Upgrade Image",
		URL:        filePath,
		FileSize:   fileSize,
	}

	operationId := fmt.Sprintf("rollback_%d_%d", msg.TaskId, msg.ElementId)
	if err := w.opSender.SendDownload(sn, dl, operationId); err != nil {
		logger.Errorf("upgrade worker: SendDownload (rollback) to %s failed: %v", sn, err)
		w.updateLogFailure(msg.LogUuid, fmt.Sprintf("rollback SendDownload failed: %v", err))
		return
	}

	now := time.Now()
	logEntry.IsDownloaded = boolPtrVal(true)
	logEntry.DownloadedTime = &now
	logEntry.OldVersion = &version
	logEntry.Upgrade = boolPtrVal(false) // rollback
	if err := w.repo.UpdateUpgradeLog(logEntry); err != nil {
		logger.Errorf("upgrade worker: update log after rollback download %s: %v", msg.LogUuid, err)
	}

	logger.Infof("upgrade worker: rollback download dispatched to %s for task %d", sn, msg.TaskId)
}

// handleReboot sends a TR-069 Reboot request.
func (w *UpgradeWorker) handleReboot(ctx context.Context, msg *UpgradeMessage, sn string, logEntry *UpgradeLog) {
	operationId := fmt.Sprintf("upgrade_reboot_%d_%d", msg.TaskId, msg.ElementId)
	if err := w.opSender.SendReboot(sn, operationId); err != nil {
		logger.Errorf("upgrade worker: SendReboot to %s failed: %v", sn, err)
		w.updateLogFailure(msg.LogUuid, fmt.Sprintf("SendReboot failed: %v", err))
		return
	}

	now := time.Now()
	logEntry.IsDone = boolPtrVal(true)
	logEntry.DoneTime = &now
	logEntry.Success = boolPtrVal(true)
	if err := w.repo.UpdateUpgradeLog(logEntry); err != nil {
		logger.Errorf("upgrade worker: update log after reboot %s: %v", msg.LogUuid, err)
	}

	logger.Infof("upgrade worker: reboot dispatched to %s for task %d", sn, msg.TaskId)
}

// findLogByUUID loads an UpgradeLog by its string primary key.
func (w *UpgradeWorker) findLogByUUID(uuid string) (*UpgradeLog, error) {
	var log UpgradeLog
	if err := w.db.Where("id = ?", uuid).First(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

// updateLogFailure marks an upgrade log as failed.
func (w *UpgradeWorker) updateLogFailure(logUuid string, message string) {
	log, err := w.findLogByUUID(logUuid)
	if err != nil {
		logger.Errorf("upgrade worker: cannot find log %s for failure update: %v", logUuid, err)
		return
	}
	now := time.Now()
	log.IsDone = boolPtrVal(true)
	log.DoneTime = &now
	log.Success = boolPtrVal(false)
	log.Message = &message
	if err := w.repo.UpdateUpgradeLog(log); err != nil {
		logger.Errorf("upgrade worker: failed to update log %s: %v", logUuid, err)
	}
}
