package tr069

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nmsappsrv/internal/config"
	"nmsappsrv/internal/device"
	"nmsappsrv/internal/eventlog"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// fetchChangedParameters sends a GPV for basic param paths after a VALUE CHANGE event.
func (ep *EventProcessor) fetchChangedParameters(sn string) {
	ctx := context.Background()

	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).First(&cpe).Error; err != nil {
		logger.Warnf("VALUE CHANGE: device %s not found: %v", sn, err)
		return
	}

	deviceType := ""
	if cpe.DeviceType != nil {
		deviceType = *cpe.DeviceType
	}
	paramPaths := GetBasicParamPaths(deviceType)
	if len(paramPaths) == 0 {
		return
	}

	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetParameterValues(headerId, paramPaths)

	// Save tracking data
	now := time.Now()
	eventType := "GET_PARAMETER_VALUES"
	trackData, _ := json.Marshal(map[string]interface{}{
		"header_id":       headerId,
		"serial_number":   sn,
		"operation_type":  eventType,
		"is_value_change": true,
		"issue_time":      now.Format(time.RFC3339),
	})
	evLog := eventlog.EventLog{
		EventType:        &eventType,
		OperationTime:    &now,
		CommandIssueTime: &now,
		ElementId:        &cpe.NeNeid,
		Status:           intPtr(1),
		CommandTrackData: stringPtr(string(trackData)),
	}
	ep.db.Create(&evLog)

	// Cache in Redis
	trackKey := fmt.Sprintf("tr069:track:%s", headerId)
	trackJson, _ := json.Marshal(map[string]interface{}{
		"header_id":       headerId,
		"sn":              sn,
		"operation_type":  eventType,
		"event_log_id":    evLog.Id,
		"is_value_change": true,
	})
	redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

	// Push to device queue
	msgMgr := NewMessageManager()
	if err := msgMgr.PutMessage(sn, soapXml); err != nil {
		logger.Errorf("VALUE CHANGE: failed to push GPV to device %s: %v", sn, err)
		return
	}
	logger.Infof("VALUE CHANGE: GPV sent to device %s for %d params", sn, len(paramPaths))
}

// fetchChangedParametersDynamic sends a GPV for the parameter names reported in the VALUE CHANGE event.
// This replaces the hardcoded GetBasicParamPaths approach with dynamic discovery from event params.
func (ep *EventProcessor) fetchChangedParametersDynamic(sn string, eventParams []soap.ParameterValueStruct) {
	if len(eventParams) == 0 {
		return
	}

	ctx := context.Background()

	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		logger.Warnf("VALUE CHANGE dynamic: device %s not found: %v", sn, err)
		return
	}

	// Extract parameter names from the event params
	paramPaths := make([]string, 0, len(eventParams))
	for _, p := range eventParams {
		if p.Name != "" {
			paramPaths = append(paramPaths, p.Name)
		}
	}
	if len(paramPaths) == 0 {
		return
	}

	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetParameterValues(headerId, paramPaths)

	// Save tracking data
	now := time.Now()
	eventType := "GET_PARAMETER_VALUES"
	trackData, _ := json.Marshal(map[string]interface{}{
		"header_id":       headerId,
		"serial_number":   sn,
		"operation_type":  eventType,
		"is_value_change": true,
		"issue_time":      now.Format(time.RFC3339),
	})
	evLog := eventlog.EventLog{
		EventType:        &eventType,
		OperationTime:    &now,
		CommandIssueTime: &now,
		ElementId:        &cpe.NeNeid,
		Status:           intPtr(1),
		CommandTrackData: stringPtr(string(trackData)),
	}
	ep.db.Create(&evLog)

	// Cache in Redis
	trackKey := fmt.Sprintf("tr069:track:%s", headerId)
	trackJson, _ := json.Marshal(map[string]interface{}{
		"header_id":       headerId,
		"sn":              sn,
		"operation_type":  eventType,
		"event_log_id":    evLog.Id,
		"is_value_change": true,
	})
	redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

	// Push to device queue
	msgMgr := NewMessageManager()
	if err := msgMgr.PutMessage(sn, soapXml); err != nil {
		logger.Errorf("VALUE CHANGE dynamic: failed to push GPV to device %s: %v", sn, err)
		return
	}
	logger.Infof("VALUE CHANGE dynamic: GPV sent to device %s for %d event params", sn, len(paramPaths))
}

// triggerDeviceInit sends GetParameterValues for device-type-specific basic parameters
// when a device first connects (IsInitialized=false). After parameters are loaded,
// the device will be marked as initialized.
func (ep *EventProcessor) triggerDeviceInit(sn string, neId int64, deviceType string) {
	ctx := context.Background()

	// Acquire init lock to prevent concurrent init for the same device
	initLockKey := fmt.Sprintf("device:init_lock:%d", neId)
	if !redis.Lock(ctx, initLockKey, 60*time.Second) {
		logger.Infof("device %d already being initialized, skipping", neId)
		return
	}
	defer redis.Unlock(ctx, initLockKey)

	logger.Infof("starting device initialization for SN=%s (neId=%d, type=%s)", sn, neId, deviceType)

	// Look up rootNode from device record (default to "Device")
	var cpe device.CpeElement
	rootNode := "Device"
	if err := ep.db.Where("ne_neid = ?", neId).First(&cpe).Error; err == nil && cpe.RootNode != nil && *cpe.RootNode != "" {
		rootNode = *cpe.RootNode
	}

	// Stage 1: Send GPN with root path to discover all parameters dynamically
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetParameterNames(headerId, rootNode+".", false)

	// Save tracking data to event_log
	now := time.Now()
	eventType := "INIT_GPN"
	trackData, _ := json.Marshal(map[string]interface{}{
		"header_id":      headerId,
		"serial_number":  sn,
		"operation_type": eventType,
		"is_init":        true,
		"stage":          "gpn",
		"root_node":      rootNode,
		"issue_time":     now.Format(time.RFC3339),
	})
	eventLog := eventlog.EventLog{
		EventType:        &eventType,
		OperationTime:    &now,
		CommandIssueTime: &now,
		ElementId:        &neId,
		Status:           intPtr(1), // pending
		CommandTrackData: stringPtr(string(trackData)),
	}
	// Atomic: create event_log + mark device as initialized
	if err := ep.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&eventLog).Error; err != nil {
			return fmt.Errorf("failed to create init event_log: %w", err)
		}
		return tx.Model(&device.CpeElement{}).Where("ne_neid = ?", neId).Update("is_initialized", true).Error
	}); err != nil {
		logger.Errorf("device %d: init transaction failed: %v", neId, err)
		return
	}

	// Cache track data in Redis for quick lookup during response processing
	trackKey := fmt.Sprintf("tr069:track:%s", headerId)
	trackJson, _ := json.Marshal(map[string]interface{}{
		"header_id":      headerId,
		"sn":             sn,
		"operation_type": eventType,
		"event_log_id":   eventLog.Id,
		"is_init":        true,
		"stage":          "gpn",
		"root_node":      rootNode,
	})
	redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

	// Push GPN SOAP to device queue
	msgMgr := NewMessageManager()
	if err := msgMgr.PutMessage(sn, soapXml); err != nil {
		logger.Errorf("failed to push init GPN to device %s queue: %v", sn, err)
		return
	}

	logger.Infof("device init GPN sent for SN=%s, root=%s.", sn, rootNode)
}

// checkAndPresetParameters checks if preset parameters are configured for a newly
// created device and dispatches SPV commands. It looks at the device's ztp_parameters
// field and the global preset config in system_config.
func (ep *EventProcessor) checkAndPresetParameters(ctx context.Context, cpe *device.CpeElement, sn string) {
	// Collect preset parameters from device's ztp_parameters
	var presets map[string]string

	if cpe.ZtpParameters != nil && *cpe.ZtpParameters != "" {
		var ztpParams map[string]interface{}
		if err := json.Unmarshal([]byte(*cpe.ZtpParameters), &ztpParams); err == nil {
			presets = make(map[string]string, len(ztpParams))
			for k, v := range ztpParams {
				presets[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	// Also check global preset config from system_config
	var globalPreset struct {
		Config *string `gorm:"column:config"`
	}
	if err := ep.db.Table("system_config").
		Select("config").
		Where("id = ?", "parameter_preset_config").
		First(&globalPreset).Error; err == nil && globalPreset.Config != nil && *globalPreset.Config != "" {
		var globalParams map[string]interface{}
		if err := json.Unmarshal([]byte(*globalPreset.Config), &globalParams); err == nil {
			if presets == nil {
				presets = make(map[string]string)
			}
			for k, v := range globalParams {
				// Device-specific presets take precedence
				if _, exists := presets[k]; !exists {
					presets[k] = fmt.Sprintf("%v", v)
				}
			}
		}
	}

	if len(presets) == 0 {
		return
	}

	// Dispatch preset parameters asynchronously
	go func() {
		logger.Infof("presetting %d parameters for new device %s (neId=%d)", len(presets), sn, cpe.NeNeid)

		// Build SPV SOAP XML
		spvParams := make([]soap.ParameterValueStruct, 0, len(presets))
		for paramName, paramValue := range presets {
			spvParams = append(spvParams, soap.ParameterValueStruct{
				Name:  paramName,
				Value: paramValue,
			})
		}

		headerId := soap.GenerateHeaderID()
		soapXml := soap.BuildSetParameterValues(headerId, spvParams, "")

		// Create event_log for tracking
		now := time.Now()
		eventType := "SET_PARAMETER_VALUES"
		trackData, _ := json.Marshal(map[string]interface{}{
			"header_id":      headerId,
			"serial_number":  sn,
			"operation_type": eventType,
			"is_preset":      true,
			"issue_time":     now.Format(time.RFC3339),
		})
		evLog := eventlog.EventLog{
			EventType:        &eventType,
			OperationTime:    &now,
			CommandIssueTime: &now,
			ElementId:        &cpe.NeNeid,
			Status:           intPtr(1),
			CommandTrackData: stringPtr(string(trackData)),
		}
		ep.db.Create(&evLog)

		// Cache track data in Redis
		trackKey := fmt.Sprintf("tr069:track:%s", headerId)
		trackJson, _ := json.Marshal(map[string]interface{}{
			"header_id":      headerId,
			"sn":             sn,
			"operation_type": eventType,
			"event_log_id":   evLog.Id,
			"is_preset":      true,
		})
		redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

		// Push to device queue
		msgMgr := NewMessageManager()
		if err := msgMgr.PutMessage(sn, soapXml); err != nil {
			logger.Errorf("preset parameters: failed to push SPV to device %s: %v", sn, err)
			return
		}
		logger.Infof("preset parameters: SPV sent to device %s with %d params", sn, len(presets))
	}()
}

// askRebootConfig represents the runtime configuration stored in system_config.
type askRebootConfig struct {
	Enabled    bool     `json:"enabled"`
	Conditions []string `json:"conditions"`
}

// checkAndTriggerAskReboot evaluates whether a device should be rebooted after an Inform.
// It checks three conditions: version_mismatch, failed_state, and ztp_pending.
// Configuration is read from system_config table (key: ask_reboot_config) first,
// falling back to the static config.yaml enable_ask_reboot flag.
func (ep *EventProcessor) checkAndTriggerAskReboot(ctx context.Context, sn string, cpe *device.CpeElement, inform *soap.Inform) {
	// --- Determine if AskReboot is enabled and which conditions to check ---
	enabled := false
	conditions := map[string]bool{
		"version_mismatch": true,
		"failed_state":     true,
		"ztp_pending":      true,
	}

	// Try reading runtime config from system_config table first
	var sysCfg misc.SystemConfig
	if err := ep.db.Where("id = ?", "ask_reboot_config").First(&sysCfg).Error; err == nil && sysCfg.Config != nil && *sysCfg.Config != "" {
		var arc askRebootConfig
		if err := json.Unmarshal([]byte(*sysCfg.Config), &arc); err == nil {
			enabled = arc.Enabled
			// Override default conditions if specified
			if len(arc.Conditions) > 0 {
				conditions = make(map[string]bool, len(arc.Conditions))
				for _, c := range arc.Conditions {
					conditions[c] = true
				}
			}
		} else {
			logger.Warnf("AskReboot: failed to parse ask_reboot_config for SN=%s: %v", sn, err)
		}
	} else {
		// Fall back to static config.yaml
		if config.Cfg != nil {
			enabled = config.Cfg.TR069.EnableAskReboot
		}
	}

	if !enabled {
		return
	}

	// --- Evaluate conditions ---
	var reasons []string

	// Condition 1: version_mismatch -- target_version is set and differs from current software_version
	if conditions["version_mismatch"] {
		if cpe.TargetVersion != nil && *cpe.TargetVersion != "" {
			currentVersion := ""
			if cpe.SoftwareVersion != nil {
				currentVersion = *cpe.SoftwareVersion
			}
			if currentVersion != *cpe.TargetVersion {
				reasons = append(reasons, fmt.Sprintf(
					"version_mismatch: current=%s, target=%s", currentVersion, *cpe.TargetVersion))
			}
		}
	}

	// Condition 2: failed_state -- device status contains "error" or "failed"
	if conditions["failed_state"] {
		if cpe.Status != nil {
			statusLower := strings.ToLower(*cpe.Status)
			if strings.Contains(statusLower, "error") || strings.Contains(statusLower, "failed") {
				reasons = append(reasons, fmt.Sprintf("failed_state: status=%s", *cpe.Status))
			}
		}
	}

	// Condition 3: ztp_pending -- ReadyToZTP flag is set but device is not yet initialized
	if conditions["ztp_pending"] {
		if cpe.ReadyToZTP != nil && *cpe.ReadyToZTP && !cpe.IsInitialized {
			reasons = append(reasons, "ztp_pending: ReadyToZTP=true but IsInitialized=false")
		}
	}

	if len(reasons) == 0 {
		return
	}

	// --- Trigger reboot ---
	reasonStr := strings.Join(reasons, "; ")
	logger.Infof("AskReboot: triggering reboot for device %s (neId=%d), reasons: %s", sn, cpe.NeNeid, reasonStr)

	// Create an OperationSender to dispatch the reboot
	msgMgr := NewMessageManager()
	opSender := NewOperationSender(ep.db, msgMgr)
	operationId := fmt.Sprintf("ask_reboot_%s_%d", sn, time.Now().Unix())

	if err := opSender.SendReboot(sn, operationId); err != nil {
		logger.Errorf("AskReboot: failed to send reboot to device %s: %v", sn, err)
		return
	}

	// Create event_log entry for the automatic reboot
	now := time.Now()
	eventType := "ASK_REBOOT"
	trackData, _ := json.Marshal(map[string]interface{}{
		"serial_number": sn,
		"operation_id":  operationId,
		"reasons":       reasons,
		"trigger":       "inform_postprocessor",
		"issue_time":    now.Format(time.RFC3339),
	})
	evLog := eventlog.EventLog{
		EventType:        &eventType,
		OperationTime:    &now,
		CommandIssueTime: &now,
		ElementId:        &cpe.NeNeid,
		Status:           intPtr(1), // pending
		FaultInfo:        stringPtr(reasonStr),
		CommandTrackData: stringPtr(string(trackData)),
	}
	if err := ep.db.Create(&evLog).Error; err != nil {
		logger.Errorf("AskReboot: failed to create event_log for device %s: %v", sn, err)
	}

	logger.Infof("AskReboot: reboot command sent to device %s, event_log id=%d", sn, evLog.Id)
}

// sendWebCallback sends a web callback via Redis pub/sub.
func (ep *EventProcessor) sendWebCallback(ctx context.Context, callbackType soap.MessageType, data map[string]interface{}) {
	callback := map[string]interface{}{
		"type":      callbackType,
		"timestamp": time.Now().Unix(),
		"data":      data,
	}

	jsonData, err := json.Marshal(callback)
	if err != nil {
		logger.Errorf("failed to marshal callback data: %v", err)
		return
	}

	if err := redis.Publish(ctx, "web_callback", string(jsonData)); err != nil {
		logger.Errorf("failed to publish web callback: %v", err)
	}
}
