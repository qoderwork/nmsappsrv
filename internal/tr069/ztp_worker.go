package tr069

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"nmsappsrv/internal/device"
	"nmsappsrv/internal/eventlog"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/mq"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// ZTPMessage is the JSON payload pushed to the queue:ztp Redis queue.
type ZTPMessage struct {
	ElementId     int64  `json:"elementId"`
	SerialNumber  string `json:"serialNumber"`
	OperationType string `json:"operationType"` // "provision", "reztp"
	OperationUser string `json:"operationUser"`
}

// ZTPWorker consumes messages from the ZTP queue and orchestrates
// zero-touch provisioning by sending SPV commands to devices.
type ZTPWorker struct {
	mu        sync.Mutex
	running   bool
	db        *gorm.DB
	opSender  *OperationSender
}

// NewZTPWorker creates a new ZTPWorker.
func NewZTPWorker(db *gorm.DB, msgManager *MessageManager) *ZTPWorker {
	return &ZTPWorker{
		db:       db,
		opSender: NewOperationSender(db, msgManager),
	}
}

// Start begins the ZTP provisioning loop.
func (w *ZTPWorker) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	logger.Info("ZTP provisioning worker starting")

	utils.SafeGo("ztp-worker", func() {
		w.pollLoop()
	})
}

// Stop stops the worker gracefully.
func (w *ZTPWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.running = false
	logger.Info("ZTP provisioning worker stopped")
}

// IsRunning returns whether the worker is currently running.
func (w *ZTPWorker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// pollLoop continuously polls the ZTP Redis queue for provisioning messages.
func (w *ZTPWorker) pollLoop() {
	for w.IsRunning() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		result, err := redis.BRPop(ctx, 1*time.Second, mq.ZTPQueue)
		cancel()

		if err != nil {
			if err.Error() == "redis: nil" {
				continue
			}
			if !w.IsRunning() {
				return
			}
			logger.Debugf("ZTP worker queue poll error (may be timeout): %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if len(result) < 2 {
			continue
		}

		// result[0] is the queue name, result[1] is the message
		msgJSON := result[1]

		var msg ZTPMessage
		if err := json.Unmarshal([]byte(msgJSON), &msg); err != nil {
			logger.Errorf("ZTP worker failed to unmarshal message: %v, data: %s", err, msgJSON)
			continue
		}

		logger.Infof("ZTP worker: processing %s for device %d (SN=%s)", msg.OperationType, msg.ElementId, msg.SerialNumber)

		switch msg.OperationType {
		case "provision", "reztp":
			w.handleProvision(&msg)
		default:
			logger.Warnf("ZTP worker: unsupported operation type: %s", msg.OperationType)
		}
	}
}

// handleProvision executes the ZTP provisioning flow for a single device.
func (w *ZTPWorker) handleProvision(msg *ZTPMessage) {
	now := time.Now()

	// 1. Look up the device
	var dev device.CpeElement
	if err := w.db.Where("ne_neid = ? AND deleted = ?", msg.ElementId, false).First(&dev).Error; err != nil {
		logger.Errorf("ZTP worker: device %d not found: %v", msg.ElementId, err)
		w.createRetryLog(msg.ElementId, &now, fmt.Sprintf("device not found: %v", err))
		return
	}

	if dev.SerialNumber == nil || *dev.SerialNumber == "" {
		logger.Errorf("ZTP worker: device %d has no serial number", msg.ElementId)
		w.createRetryLog(msg.ElementId, &now, "device has no serial number")
		return
	}
	sn := *dev.SerialNumber

	// 2. Load ZTP settings from system_config
	ztpSetting, err := w.loadZTPSetting()
	if err != nil {
		logger.Errorf("ZTP worker: failed to load ZTP setting: %v", err)
		w.createRetryLog(msg.ElementId, &now, fmt.Sprintf("failed to load ZTP setting: %v", err))
		return
	}

	// 3. Parse ZTP parameters from the device's ztp_parameters field
	var ztpParams map[string]interface{}
	if dev.ZtpParameters != nil && *dev.ZtpParameters != "" {
		if err := json.Unmarshal([]byte(*dev.ZtpParameters), &ztpParams); err != nil {
			logger.Errorf("ZTP worker: failed to parse ztp_parameters for device %d: %v", msg.ElementId, err)
			w.createRetryLog(msg.ElementId, &now, fmt.Sprintf("invalid ztp_parameters: %v", err))
			return
		}
	} else {
		logger.Warnf("ZTP worker: device %d has no ztp_parameters", msg.ElementId)
		w.createRetryLog(msg.ElementId, &now, "device has no ztp_parameters configured")
		return
	}

	// 4. Create ZTP log entry
	ztpLog := &misc.ZTPLog{
		ElementId: intPtr64(msg.ElementId),
		Progress:  intPtr(0), // 0 = started
		Done:      boolPtr(false),
		StartTime: &now,
		HasFault:  boolPtr(false),
	}
	if err := w.db.Create(ztpLog).Error; err != nil {
		logger.Errorf("ZTP worker: failed to create ztp_log for device %d: %v", msg.ElementId, err)
	}

	// 5. Create event_log for tracking
	operationId := fmt.Sprintf("ztp_%d_%d", msg.ElementId, now.Unix())
	eventLogEntry := eventlog.EventLog{
		EventType:        stringPtr("ZTP_PROVISION"),
		OperationTime:    &now,
		CommandIssueTime: &now,
		ElementId:        intPtr64(msg.ElementId),
		Status:           intPtr(1), // pending
	}

	trackData := map[string]interface{}{
		"operation_id":   operationId,
		"serial_number":  sn,
		"operation_type": "ZTP_PROVISION",
		"ztp_log_id":     ztpLog.Id,
		"issue_time":     now.Format(time.RFC3339),
	}
	if trackJson, err := json.Marshal(trackData); err == nil {
		eventLogEntry.CommandTrackData = stringPtr(string(trackJson))
	}

	if err := w.db.Create(&eventLogEntry).Error; err != nil {
		logger.Errorf("ZTP worker: failed to create event_log for device %d: %v", msg.ElementId, err)
	}

	// Update ZTP log with event_log_id
	w.db.Model(ztpLog).Update("event_log_id", eventLogEntry.Id)

	// 6. Build SPV parameters from ztp_parameters and ZTP settings
	spvParams := w.buildZTPParameterValues(ztpParams, ztpSetting)

	if len(spvParams) == 0 {
		info := "no ZTP parameters to send"
		logger.Warnf("ZTP worker: %s for device %d", info, msg.ElementId)
		w.updateZTPLogDone(ztpLog.Id, false, info, &now)
		w.createRetryLog(msg.ElementId, &now, info)
		return
	}

	// 7. Send SPV to device via OperationSender
	paramKey := fmt.Sprintf("ztp_%d", msg.ElementId)
	if err := w.opSender.SendSetParameterValues(sn, spvParams, paramKey, operationId); err != nil {
		logger.Errorf("ZTP worker: failed to send SPV for device %d (SN=%s): %v", msg.ElementId, sn, err)
		info := fmt.Sprintf("SPV send failed: %v", err)
		w.updateZTPLogDone(ztpLog.Id, false, info, &now)
		w.createRetryLog(msg.ElementId, &now, info)
		return
	}

	// 8. Update ZTP log progress (SPV sent successfully, waiting for response)
	w.db.Model(ztpLog).Updates(map[string]interface{}{
		"progress": 3, // 3 = SPV sent, waiting for response
		"info":     "SPV sent, waiting for device response",
	})

	logger.Infof("ZTP worker: SPV sent for device %d (SN=%s), operation=%s", msg.ElementId, sn, operationId)
}

// buildZTPParameterValues converts the device's ztp_parameters JSON and ZTP settings
// into SOAP ParameterValueStruct entries for the SPV command.
func (w *ZTPWorker) buildZTPParameterValues(ztpParams map[string]interface{}, setting *misc.ZTPSetting) []soap.ParameterValueStruct {
	var params []soap.ParameterValueStruct

	// Map ZTP parameters from the device to TR-069 parameter paths.
	// The ztp_parameters JSON contains key-value pairs where keys are
	// TR-069 parameter names and values are the values to set.
	for paramName, paramValue := range ztpParams {
		valueStr := fmt.Sprintf("%v", paramValue)
		params = append(params, soap.ParameterValueStruct{
			Name:  paramName,
			Value: valueStr,
		})
	}

	// Add ZTP setting parameters that apply to all devices
	if setting != nil {
		if setting.GoogleAPIKey != nil && *setting.GoogleAPIKey != "" {
			params = append(params, soap.ParameterValueStruct{
				Name:  "Device.Services.FAPService.1.FAPControl.NR.SelfConfigParams.GoogleAPIKey",
				Value: *setting.GoogleAPIKey,
			})
		}
		if setting.SpectrumSpatialURL != nil && *setting.SpectrumSpatialURL != "" {
			params = append(params, soap.ParameterValueStruct{
				Name:  "Device.Services.FAPService.1.FAPControl.NR.SelfConfigParams.SpectrumSpatialURL",
				Value: *setting.SpectrumSpatialURL,
			})
		}
		if setting.BMCUrl != nil && *setting.BMCUrl != "" {
			params = append(params, soap.ParameterValueStruct{
				Name:  "Device.Services.FAPService.1.FAPControl.NR.SelfConfigParams.BMCUrl",
				Value: *setting.BMCUrl,
			})
		}
		if setting.GMLCUrl != nil && *setting.GMLCUrl != "" {
			params = append(params, soap.ParameterValueStruct{
				Name:  "Device.Services.FAPService.1.FAPControl.NR.SelfConfigParams.GMLCUrl",
				Value: *setting.GMLCUrl,
			})
		}
		if setting.MSAGUrl != nil && *setting.MSAGUrl != "" {
			params = append(params, soap.ParameterValueStruct{
				Name:  "Device.Services.FAPService.1.FAPControl.NR.SelfConfigParams.MSAGUrl",
				Value: *setting.MSAGUrl,
			})
		}
		if setting.TPlatformUrl != nil && *setting.TPlatformUrl != "" {
			params = append(params, soap.ParameterValueStruct{
				Name:  "Device.Services.FAPService.1.FAPControl.NR.SelfConfigParams.TPlatformUrl",
				Value: *setting.TPlatformUrl,
			})
		}
		if setting.PTPEnable != nil {
			ptpVal := "0"
			if *setting.PTPEnable {
				ptpVal = "1"
			}
			params = append(params, soap.ParameterValueStruct{
				Name:  "Device.Services.FAPService.1.FAPControl.NR.SelfConfigParams.PTPEnable",
				Value: ptpVal,
			})
		}
		if setting.WifiPositioning != nil {
			wifiVal := "0"
			if *setting.WifiPositioning {
				wifiVal = "1"
			}
			params = append(params, soap.ParameterValueStruct{
				Name:  "Device.Services.FAPService.1.FAPControl.NR.SelfConfigParams.WifiPositioning",
				Value: wifiVal,
			})
		}
	}

	return params
}

// loadZTPSetting loads the ZTP configuration from system_config.
func (w *ZTPWorker) loadZTPSetting() (*misc.ZTPSetting, error) {
	var cfg misc.SystemConfig
	err := w.db.Where("id = ?", "ztp_config").First(&cfg).Error
	if err != nil {
		// No config yet, return empty defaults
		return &misc.ZTPSetting{}, nil
	}
	if cfg.Config == nil || *cfg.Config == "" {
		return &misc.ZTPSetting{}, nil
	}
	var setting misc.ZTPSetting
	if err := json.Unmarshal([]byte(*cfg.Config), &setting); err != nil {
		return nil, fmt.Errorf("invalid ztp_config: %w", err)
	}
	return &setting, nil
}

// updateZTPLogDone marks a ZTP log entry as completed.
func (w *ZTPWorker) updateZTPLogDone(logId int64, success bool, info string, endTime *time.Time) {
	progress := 6 // 6 = complete
	hasFault := !success
	updates := map[string]interface{}{
		"progress":  progress,
		"done":       true,
		"info":       info,
		"end_time":   endTime,
		"has_fault":  hasFault,
	}
	if err := w.db.Model(&misc.ZTPLog{}).Where("id = ?", logId).Updates(updates).Error; err != nil {
		logger.Errorf("ZTP worker: failed to update ztp_log %d: %v", logId, err)
	}
}

// createRetryLog creates a ZTP retry log entry for a failed provisioning attempt.
func (w *ZTPWorker) createRetryLog(elementId int64, retryTime *time.Time, info string) {
	retryLog := &misc.ZTPRetryLog{
		ElementId: intPtr64(elementId),
		RetryTime: retryTime,
		Info:      stringPtr(info),
	}
	if err := w.db.Create(retryLog).Error; err != nil {
		logger.Errorf("ZTP worker: failed to create retry log for device %d: %v", elementId, err)
	}
}

// EnqueueZTPProvision enqueues a ZTP provisioning message to the Redis queue.
// Called by the HTTP handler when POST /ztp/provision is invoked.
func EnqueueZTPProvision(ctx context.Context, elementId int64, serialNumber, operationType, operationUser string) error {
	msg := ZTPMessage{
		ElementId:     elementId,
		SerialNumber:  serialNumber,
		OperationType: operationType,
		OperationUser: operationUser,
	}
	return mq.Enqueue(ctx, mq.ZTPQueue, msg)
}

// ---------- pointer helpers ----------

func intPtr64(v int64) *int64 {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}
