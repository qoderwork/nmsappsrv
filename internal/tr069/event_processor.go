package tr069

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"nmsappsrv/internal/config"
	"nmsappsrv/internal/device"
	"nmsappsrv/internal/eventlog"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/systemsettings"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// EventProcessor handles processing of CPE events (Inform, TransferComplete, etc.)
type EventProcessor struct {
	db *gorm.DB
}

// NewEventProcessor creates a new EventProcessor.
func NewEventProcessor(db *gorm.DB) *EventProcessor {
	return &EventProcessor{db: db}
}

// ProcessInform processes an Inform message from CPE.
func (ep *EventProcessor) ProcessInform(inform *soap.Inform, sn string, deviceType string, generation string) {
	ctx := context.Background()

	if sn == "" {
		logger.Error("ProcessInform called with empty serial number")
		return
	}

	// Look up device by serial_number in DB
	var cpe device.CpeElement
	err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).First(&cpe).Error

	now := time.Now()

	if err == nil {
		// Device found: update online status in Redis
		ep.updateDeviceOnlineStatus(ctx, sn, true)

		// Update device basic info from Inform parameters
		ep.updateDeviceBasicInfo(ctx, &cpe, inform.ParameterList)

		// Save to DB
		if err := ep.db.Save(&cpe).Error; err != nil {
			logger.Errorf("failed to save device %s: %v", sn, err)
		}

		logger.Infof("device %s found and updated", sn)
	} else if err == gorm.ErrRecordNotFound {
		// Device NOT found: auto-create device record

		// Check license limit before creating new device
		if err := ep.checkDeviceLimit(); err != nil {
			logger.Warnf("device %s auto-registration blocked: %v", sn, err)
			return
		}

		// Auto-detect device type if not provided (e.g., from generic HandleACS)
		productClass := inform.DeviceId.ProductClass
		if deviceType == "" {
			deviceType, generation = detectDeviceType(productClass)
			logger.Infof("device %s: auto-detected type=%s, generation=%s from productClass=%s", sn, deviceType, generation, productClass)
		}

		logger.Infof("device %s not found, auto-creating", sn)

		// Resolve default license_id from system_config (aligns with Java auto-registration)
		var resolvedLicenseId *int
		var configValue string
		if err := ep.db.Table("system_config").Where("config_key = ?", "default_license").Limit(1).Scan(&configValue).Error; err == nil && configValue != "" {
			if lid, err := strconv.Atoi(configValue); err == nil && lid > 0 {
				resolvedLicenseId = &lid
			}
		}

		// Determine rootNode from first Inform parameter (e.g. "Device" or "InternetGatewayDevice")
		rootNode := "Device"
		if len(inform.ParameterList) > 0 {
			if idx := strings.Index(inform.ParameterList[0].Name, "."); idx > 0 {
				rootNode = inform.ParameterList[0].Name[:idx]
			}
		}

		cpe = device.CpeElement{
			SerialNumber:    stringPtr(sn),
			Status:          stringPtr("online"),
			CreationTime:    &now,
			Manufacturer:    stringPtr(inform.DeviceId.Manufacturer),
			Oui:             stringPtr(inform.DeviceId.OUI),
			ModelName:       stringPtr(inform.DeviceId.ProductClass),
			Generation:      stringPtr(generation),
			DeviceType:      stringPtr(deviceType),
			IsNewVersion:    deviceType != "cpe",
			RootNode:        stringPtr(rootNode),
			LoadedBasicInfo: false,
			IsInitialized:   false,
			Deleted:         false,
			LicenseId:       resolvedLicenseId,
		}

		// Extract basic info from Inform parameters (matches Java extraBasicInfo)
		macRegex := regexp.MustCompile(`Device\.Ethernet\.Interface\.\d+\.MACAddress`)
		var macs []string
		for _, param := range inform.ParameterList {
			switch {
			case param.Name == rootNode+".DeviceInfo.SoftwareVersion" ||
				param.Name == rootNode+".DeviceInfo.MU.1.SoftwareVersion" ||
				param.Name == rootNode+".DeviceInfo.MU.1.Slot.1.SoftwareVersion":
				cpe.SoftwareVersion = stringPtr(param.Value)
			case param.Name == rootNode+".DeviceInfo.HardwareVersion" ||
				param.Name == rootNode+".DeviceInfo.MU.1.HardwareVersion" ||
				param.Name == rootNode+".DeviceInfo.MU.1.Slot.1.HardwareVersion":
				cpe.HardwareVersion = stringPtr(param.Value)
			case param.Name == rootNode+".DeviceInfo.FirmwareVersion" ||
				param.Name == "Device.DeviceInfo.MU.1.FirmwareVersion":
				cpe.FirmwareVersion = stringPtr(param.Value)
			case param.Name == "Device.DeviceInfo.FullSoftwareVersion":
				cpe.FullSoftwareVersion = stringPtr(param.Value)
			case param.Name == "Device.DeviceInfo.StmVersion":
				cpe.StmVersion = stringPtr(param.Value)
			case param.Name == rootNode+".DeviceInfo.ModelName" ||
				param.Name == rootNode+".DeviceInfo.MU.1.ModelName" ||
				param.Name == rootNode+".DeviceInfo.MU.1.Slot.1.ModelName":
				cpe.ModelName = stringPtr(param.Value)
			case param.Name == rootNode+".DeviceInfo.Manufacturer":
				cpe.Manufacturer = stringPtr(param.Value)
			case param.Name == rootNode+".ManagementServer.URL":
				cpe.CoonReqUrl = stringPtr(param.Value)
			case macRegex.MatchString(param.Name):
				// Strip dashes, collect for sorting
				macs = append(macs, strings.ReplaceAll(param.Value, "-", ""))
			}
		}

		// Sort and join MAC addresses (matches Java behavior)
		if len(macs) > 0 {
			sort.Strings(macs)
			// Deduplicate
			unique := macs[:0]
			for i, m := range macs {
				if i == 0 || m != macs[i-1] {
					unique = append(unique, m)
				}
			}
			macStr := strings.Join(unique, ",")
			cpe.Mac = stringPtr(macStr)
		}

		// Fallback: modelName = productClass if not found in params
		if cpe.ModelName == nil || *cpe.ModelName == "" {
			cpe.ModelName = stringPtr(inform.DeviceId.ProductClass)
		}

		if err := ep.db.Create(&cpe).Error; err != nil {
			logger.Errorf("failed to create device %s: %v", sn, err)
			return
		}

		// Auto-assign to default device groups (2.7)
		ep.autoAssignToDefaultGroups(cpe.NeNeid, cpe.LicenseId)

		// Set online status in Redis for new device
		ep.updateDeviceOnlineStatus(ctx, sn, true)

		// Task 5.4: Check for preset parameters after new device onboarding
		ep.checkAndPresetParameters(ctx, &cpe, sn)

		logger.Infof("device %s created successfully", sn)
	} else {
		logger.Errorf("failed to query device %s: %v", sn, err)
		return
	}

	// Process each event code
	for _, evt := range inform.EventList {
		ep.processEventCode(ctx, evt, sn, inform.ParameterList)
	}

	// Trigger device initialization if not yet initialized (first Inform after boot)
	if !cpe.IsInitialized {
		go ep.triggerDeviceInit(sn, cpe.NeNeid, deviceType)
	}

	// Create event log entry in DB
	eventLog := eventlog.EventLog{
		EventType:     stringPtr("INFORM"),
		OperationTime: &now,
		ElementId:     &cpe.NeNeid,
		Status:        intPtr(0), // success
	}

	// Marshal inform data to JSON for tracking
	if trackData, err := json.Marshal(map[string]interface{}{
		"serial_number": sn,
		"events":        inform.EventList,
		"parameters":    inform.ParameterList,
	}); err == nil {
		eventLog.CommandTrackData = stringPtr(string(trackData))
	}

	if err := ep.db.Create(&eventLog).Error; err != nil {
		logger.Errorf("failed to create event log for %s: %v", sn, err)
	}

	// AskReboot post-processor: check if device needs a reboot after Inform
	utils.SafeGo("AskReboot-"+sn, func() {
		ep.checkAndTriggerAskReboot(ctx, sn, &cpe, inform)
	})
}

// processEventCode handles individual TR069 event codes.
// Aligned with Java InformMessageProcessor.processEvent (21 event codes).
func (ep *EventProcessor) processEventCode(ctx context.Context, evt soap.EventStruct, sn string, params []soap.ParameterValueStruct) {
	paramMap := buildParamMap(params)

	switch evt.Code {
	case "0 BOOTSTRAP":
		ep.handleBootstrap(ctx, sn, paramMap)

	case "1 BOOT":
		ep.handleBoot(ctx, sn, paramMap)

	case "2 PERIODIC":
		logger.Debugf("device %s: PERIODIC event", sn)

	case "4 VALUE CHANGE":
		logger.Infof("device %s: VALUE CHANGE event", sn)
		ep.handleValueChange(ctx, sn, params)

	case "6 CONNECTION REQUEST":
		logger.Infof("device %s: CONNECTION REQUEST event", sn)
		ep.handleConnectionRequest(ctx, sn)

	case "8 DIAGNOSTICS COMPLETE":
		logger.Infof("device %s: DIAGNOSTICS COMPLETE event", sn)
		ep.handleDiagnosticsComplete(ctx, sn)

	case "101 ALARM":
		logger.Infof("device %s: ALARM event - command key: %s", sn, evt.CommandKey)
		ep.handleValueChange(ctx, sn, params)

	case "102 UPGRADE FINISH":
		logger.Infof("device %s: UPGRADE FINISH event - command key: %s", sn, evt.CommandKey)
		ep.handleUpgradeFinish(ctx, sn, params, evt.CommandKey)

	case "103 ADD OBJECT":
		logger.Infof("device %s: ADD OBJECT event", sn)
		ep.handleAddObject(ctx, sn, paramMap)

	case "104 DELETE OBJECT":
		logger.Infof("device %s: DELETE OBJECT event", sn)
		ep.handleDeleteObject(ctx, sn, paramMap)

	case "105 STARTUP STAGE REPORT":
		logger.Infof("device %s: STARTUP STAGE REPORT event", sn)
		ep.handleStartupStage(ctx, sn, paramMap)

	case "106 STARTUP RESULT REPORT":
		logger.Infof("device %s: STARTUP RESULT REPORT event", sn)
		ep.handleStartupResult(ctx, sn, paramMap)

	case "107 UNIT UPGRADE RESULT":
		logger.Infof("device %s: UNIT UPGRADE RESULT event - command key: %s", sn, evt.CommandKey)
		ep.handleUnitUpgradeResult(ctx, sn, params, evt.CommandKey)

	case "M Reboot":
		logger.Infof("device %s: M Reboot event", sn)
		rebootKey := fmt.Sprintf("device:rebooting:%s", sn)
		if err := redis.Del(ctx, rebootKey); err != nil {
			logger.Warnf("failed to clear rebooting flag for %s: %v", sn, err)
		}

	case "M PCI CHANGE":
		logger.Infof("device %s: PCI CHANGE event", sn)
		ep.handlePCIChange(ctx, sn, paramMap)

	case "X Restore Complete":
		logger.Infof("device %s: Restore Complete event", sn)
		ep.handleRestoreComplete(ctx, sn)

	case "X ASK REBOOT":
		logger.Infof("device %s: ASK REBOOT event", sn)
		// Handled by checkAndTriggerAskReboot post-processor in ProcessInform

	case "X 681D64 MMLREPORT":
		logger.Infof("device %s: MML REPORT event", sn)
		ep.handleMMLReport(ctx, sn, paramMap)

	case "X E01CEE BATCHSTAGE":
		// No action needed (same as Java)

	case "X E01CEE BATCHRESULT":
		logger.Infof("device %s: BATCH RESULT event", sn)
		ep.handleBatchResult(ctx, sn, paramMap)

	case "X E01CEE BATCHCHECKRESULT":
		logger.Infof("device %s: BATCH CHECK RESULT event", sn)
		ep.handleBatchCheckResult(ctx, sn, paramMap)

	default:
		logger.Infof("device %s: vendor-specific event code: %s, command key: %s", sn, evt.Code, evt.CommandKey)
	}
}

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
		"header_id":      headerId,
		"serial_number":  sn,
		"operation_type": eventType,
		"is_value_change": true,
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

// buildParamMap converts a ParameterValueStruct slice to a name→value map for quick lookup.
func buildParamMap(params []soap.ParameterValueStruct) map[string]string {
	m := make(map[string]string, len(params))
	for _, p := range params {
		if p.Value != "" {
			m[p.Name] = p.Value
		}
	}
	return m
}

// handleBootstrap processes "0 BOOTSTRAP" event.
// Java: conditionally reloads all parameters when ACS URL contains gnbInitialAcs.
func (ep *EventProcessor) handleBootstrap(ctx context.Context, sn string, paramMap map[string]string) {
	logger.Infof("device %s: BOOTSTRAP event", sn)

	// Check if ACS URL contains gnbInitialAcs (ZTP initial provisioning)
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		return
	}

	// Read ACS URL from Inform parameters or from device record
	acsUrl := paramMap["InternetGatewayDevice.ManagementServer.URL"]
	if acsUrl == "" && cpe.DeviceIp != nil {
		// Fall back to checking if device was in initial ACS flow
		acsUrl = ""
	}

	if strings.Contains(acsUrl, "gnbInitialAcs") {
		// Clear geo data
		geoKey := fmt.Sprintf("device_geo_%d", cpe.NeNeid)
		redis.Del(ctx, geoKey)
		ep.db.Model(&cpe).Updates(map[string]interface{}{
			"longitude": nil,
			"latitude":  nil,
		})

		// Trigger full parameter reload (async)
		utils.SafeGo("BootstrapReload-"+sn, func() {
			msgMgr := NewMessageManager()
			opSender := NewOperationSender(ep.db, msgMgr)
			opSender.SendGetParameterNames(sn, "Device.", false, "bootstrap_reload_"+sn)
		})
	}
}

// handleBoot processes "1 BOOT" event.
// Java: clears geo data, sends reboot notification via Redis pub/sub, clears reboot user key.
func (ep *EventProcessor) handleBoot(ctx context.Context, sn string, paramMap map[string]string) {
	logger.Infof("device %s: BOOT event", sn)

	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		return
	}

	// Clear reboot user key
	rebootUserKey := fmt.Sprintf("rebootUser_%d", cpe.NeNeid)
	redis.Del(ctx, rebootUserKey)

	// Clear geo data
	geoKey := fmt.Sprintf("device_geo_%d", cpe.NeNeid)
	redis.Del(ctx, geoKey)

	// Publish reboot notification via Redis pub/sub
	notification := map[string]interface{}{
		"type":       "reboot_notification",
		"element_id": cpe.NeNeid,
		"timestamp":  time.Now().Unix(),
	}
	if notifyJson, err := json.Marshal(notification); err == nil {
		redis.Publish(ctx, "web_callback", string(notifyJson))
	}

	// Clear rebooting flag
	rebootKey := fmt.Sprintf("device:rebooting:%s", sn)
	redis.Del(ctx, rebootKey)
}

// handleConnectionRequest processes "6 CONNECTION REQUEST" event.
// Java: records the latest connection request timestamp in Redis.
func (ep *EventProcessor) handleConnectionRequest(ctx context.Context, sn string) {
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		return
	}

	connKey := fmt.Sprintf("device:conn_request_time:%d", cpe.NeNeid)
	redis.Set(ctx, connKey, fmt.Sprintf("%d", time.Now().UnixMilli()), 24*time.Hour)
}

// handleDiagnosticsComplete processes "8 DIAGNOSTICS COMPLETE" event.
// Java: sends GetParameterNames for TraceRouteDiagnostics.RouteHops.
func (ep *EventProcessor) handleDiagnosticsComplete(ctx context.Context, sn string) {
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		return
	}

	// Send GPN for diagnostics results
	msgMgr := NewMessageManager()
	opSender := NewOperationSender(ep.db, msgMgr)
	opSender.SendGetParameterNames(sn, "Device.TraceRouteDiagnostics.RouteHops.", false, "diagnostics_"+sn)

	logger.Infof("device %s: diagnostics complete, GPN sent for TraceRouteDiagnostics", sn)
}

// handleValueChange processes "4 VALUE CHANGE" and "101 ALARM" events.
// Java: runs valueChangeInformReceiveAfter + getParameterValuesAfter postprocessors.
// Saves Inform parameters to DB and triggers parameter refresh.
func (ep *EventProcessor) handleValueChange(ctx context.Context, sn string, params []soap.ParameterValueStruct) {
	// Save parameter values from the Inform to element_basic_info_parameter
	if len(params) > 0 {
		var cpe device.CpeElement
		if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err == nil && cpe.NeNeid != 0 {
			// Acquire Redis distributed lock (same as Java GetParametersPostprocessor)
			lockKey := fmt.Sprintf("red_lock_add_parameter_names_%d", cpe.NeNeid)
			if redis.Lock(ctx, lockKey, 30*time.Second) {
				defer redis.Unlock(ctx, lockKey)
				ep.saveParameterValues(ctx, sn, params)
			}
		}
	}

	// Use event params directly for GPV (dynamic discovery instead of hardcoded paths)
	go ep.fetchChangedParametersDynamic(sn, params)
}

// handleUpgradeFinish processes "102 UPGRADE FINISH" event.
// Java: parses eventLogId from commandKey (format: "X_{eventLogId}"), triggers batch upgrade postprocessor.
func (ep *EventProcessor) handleUpgradeFinish(ctx context.Context, sn string, params []soap.ParameterValueStruct, commandKey string) {
	if commandKey == "" {
		logger.Warnf("device %s: UPGRADE FINISH with empty command key", sn)
		return
	}

	parts := strings.SplitN(commandKey, "_", 2)
	if len(parts) < 2 {
		logger.Warnf("device %s: UPGRADE FINISH invalid command key format: %s", sn, commandKey)
		return
	}

	var eventLogId int64
	if _, err := fmt.Sscanf(parts[1], "%d", &eventLogId); err != nil {
		logger.Warnf("device %s: UPGRADE FINISH failed to parse eventLogId from commandKey=%s: %v", sn, commandKey, err)
		return
	}

	logger.Infof("device %s: UPGRADE FINISH eventLogId=%d", sn, eventLogId)

	// Update upgrade_log status to done
	now := time.Now()
	done := true
	ep.db.Table("upgrade_log").
		Where("command_track_id = ?", eventLogId).
		Updates(map[string]interface{}{
			"is_done":   &done,
			"done_time": &now,
			"success":   &done,
		})

	// Update software version from Inform parameters if available
	paramMap := buildParamMap(params)
	if swVer, ok := paramMap["Device.DeviceInfo.SoftwareVersion"]; ok {
		ep.db.Model(&device.CpeElement{}).
			Where("serial_number = ? AND deleted = ?", sn, false).
			Update("software_version", swVer)
	}
}

// handleUnitUpgradeResult processes "107 UNIT UPGRADE RESULT" event.
// Java: only processes for specific models (HCS-NW-FEMTO008, HCS-NW-FEMTO009).
func (ep *EventProcessor) handleUnitUpgradeResult(ctx context.Context, sn string, params []soap.ParameterValueStruct, commandKey string) {
	// Check device model
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		return
	}

	modelName := ""
	if cpe.ModelName != nil {
		modelName = *cpe.ModelName
	}

	// Java: only process for specific models
	supportedModels := map[string]bool{
		"HCS-NW-FEMTO008": true,
		"HCS-NW-FEMTO009": true,
	}
	if !supportedModels[modelName] {
		logger.Debugf("device %s: UNIT UPGRADE RESULT ignored for model %s", sn, modelName)
		return
	}

	var eventLogId int64
	if commandKey != "" {
		parts := strings.SplitN(commandKey, "_", 2)
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &eventLogId)
		}
	}

	logger.Infof("device %s: UNIT UPGRADE RESULT eventLogId=%d", sn, eventLogId)

	if eventLogId > 0 {
		now := time.Now()
		done := true
		ep.db.Table("upgrade_log").
			Where("command_track_id = ?", eventLogId).
			Updates(map[string]interface{}{
				"is_done":   &done,
				"done_time": &now,
				"success":   &done,
			})
	}
}

// handleStartupStage processes "105 STARTUP STAGE REPORT" event.
// Java: maps stage value to progress (1→3, 2→4, else→5), updates ZTP log.
func (ep *EventProcessor) handleStartupStage(ctx context.Context, sn string, paramMap map[string]string) {
	stageStr := paramMap["Device.Services.FAPService.1.FAPControl.SelfConfig.Startup.Stage"]
	if stageStr == "" {
		return
	}

	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		return
	}

	var progress int
	switch stageStr {
	case "1":
		progress = 3
	case "2":
		progress = 4
	default:
		progress = 5
	}

	// Update ZTP log progress
	ep.db.Model(&misc.ZTPLog{}).
		Where("element_id = ? AND done = ?", cpe.NeNeid, false).
		Update("progress", &progress)

	logger.Infof("device %s: startup stage=%s, progress=%d", sn, stageStr, progress)
}

// handleStartupResult processes "106 STARTUP RESULT REPORT" event.
// Java: checks startup status, updates ZTP log with success/failure, triggers sync on success.
func (ep *EventProcessor) handleStartupResult(ctx context.Context, sn string, paramMap map[string]string) {
	status := paramMap["Device.Services.FAPService.1.FAPControl.SelfConfig.Startup.Status"]
	info := paramMap["Device.Services.FAPService.1.FAPControl.SelfConfig.Startup.FailureCause"]

	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		return
	}

	// Look up active ZTP log
	var ztpLog misc.ZTPLog
	if err := ep.db.Where("element_id = ? AND done = ?", cpe.NeNeid, false).Limit(1).Find(&ztpLog).Error; err != nil {
		logger.Warnf("device %s: no active ZTP log found for startup result", sn)
		return
	}

	now := time.Now()
	if status == "1" {
		// Success â atomic: update ZTP log + create retry log
		if err := ep.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&ztpLog).Updates(map[string]interface{}{
				"progress":  6,
				"done":      true,
				"end_time":  &now,
				"info":      info,
			}).Error; err != nil {
				return err
			}
			return tx.Create(&misc.ZTPRetryLog{
				ElementId:  &cpe.NeNeid,
				RetryTime:  &now,
				Info:       stringPtr("ZTP completed successfully"),
			}).Error
		}); err != nil {
			logger.Errorf("device %s: ZTP success transaction failed: %v", sn, err)
		}

		// Trigger parameter and alarm sync
		utils.SafeGo("ZTPSync-"+sn, func() {
			msgMgr := NewMessageManager()
			opSender := NewOperationSender(ep.db, msgMgr)
			opSender.SendGetParameterNames(sn, "Device.", false, "ztp_param_sync_"+sn)
			opSender.SendGetParameterValues(sn, []string{"Device.FaultMgmt.CurrentAlarm."}, "ztp_alarm_sync_"+sn)
		})

		logger.Infof("device %s: ZTP startup success", sn)
	} else {
		// Failure
		if ztpLog.Progress != nil && *ztpLog.Progress == 5 {
			ep.db.Model(&ztpLog).Updates(map[string]interface{}{
				"done":      true,
				"end_time":  &now,
				"info":      info,
				"has_fault": true,
			})
		} else {
			ep.db.Model(&ztpLog).Updates(map[string]interface{}{
				"done":      true,
				"end_time":  &now,
				"info":      info,
				"has_fault": true,
			})
		}

		// Create fault retry log (atomic with the update above)
		ep.db.Transaction(func(tx *gorm.DB) error {
			return tx.Create(&misc.ZTPRetryLog{
				ElementId:  &cpe.NeNeid,
				RetryTime:  &now,
				Info:       stringPtr(fmt.Sprintf("ZTP failed: %s", info)),
			}).Error
		})

		logger.Warnf("device %s: ZTP startup failed: %s", sn, info)
	}
}

// handleAddObject processes "103 ADD OBJECT" event.
// Java: reads AddEndPoint from Inform params, syncs object additions.
func (ep *EventProcessor) handleAddObject(ctx context.Context, sn string, paramMap map[string]string) {
	addEndPoint := paramMap["Device.DeviceInfo.AddEndPoint"]
	if addEndPoint == "" {
		return
	}

	logger.Infof("device %s: ADD OBJECT endPoint=%s", sn, addEndPoint)

	// Java: addObjectAfter is currently commented out in the postprocessor.
	// We log the event for observability but take no action, matching Java behavior.
}

// handleDeleteObject processes "104 DELETE OBJECT" event.
// Java: reads DelEndPoint, deletes matching params from element_basic_info_parameter.
func (ep *EventProcessor) handleDeleteObject(ctx context.Context, sn string, paramMap map[string]string) {
	delEndPoint := paramMap["Device.DeviceInfo.DelEndPoint"]
	if delEndPoint == "" {
		return
	}

	logger.Infof("device %s: DELETE OBJECT delEndPoint=%s", sn, delEndPoint)

	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		return
	}

	// Delete matching parameters (Java: deleteByElementIdAndNameLike)
	paths := strings.Split(delEndPoint, ",")
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path != "" {
			ep.db.Where("element_id = ? AND param_name LIKE ?", cpe.NeNeid, path+"%").
				Delete(&device.ElementBasicInfoParameter{})
		}
	}
}

// handleRestoreComplete processes "X Restore Complete" event.
// Java: triggers full parameter reload via GetParameterNames for "Device.".
func (ep *EventProcessor) handleRestoreComplete(ctx context.Context, sn string) {
	utils.SafeGo("RestoreComplete-"+sn, func() {
		msgMgr := NewMessageManager()
		opSender := NewOperationSender(ep.db, msgMgr)
		opSender.SendGetParameterNames(sn, "Device.", false, "restore_complete_"+sn)
	})
	logger.Infof("device %s: restore complete, full parameter reload triggered", sn)
}

// handlePCIChange processes "M PCI CHANGE" event.
// Java: reads PhyCellID from Inform params, publishes PCI change notification.
func (ep *EventProcessor) handlePCIChange(ctx context.Context, sn string, paramMap map[string]string) {
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		return
	}

	pci := paramMap["Device.Services.FAPService.1.CellConfig.1.NR.RAN.RF.PhyCellID"]

	notification := map[string]interface{}{
		"type":       "pci_change",
		"element_id": cpe.NeNeid,
		"pci":        pci,
		"timestamp":  time.Now().Unix(),
	}
	if notifyJson, err := json.Marshal(notification); err == nil {
		redis.Publish(ctx, "web_callback", string(notifyJson))
	}

	logger.Infof("device %s: PCI changed to %s", sn, pci)
}

// handleMMLReport processes "X 681D64 MMLREPORT" event.
// Java: extracts CMDUID and result, updates mml_execute_result, sends web callback.
func (ep *EventProcessor) handleMMLReport(ctx context.Context, sn string, paramMap map[string]string) {
	uid := paramMap["Device.mml.CMDUID"]
	if uid == "" {
		return
	}

	reportValue := paramMap["Device.mml.ReportValue"]
	logger.Infof("device %s: MML report uid=%s", sn, uid)

	// Update mml_execute_result by uid
	now := time.Now()
	result := struct {
		ResultReturnedTime *time.Time `gorm:"column:result_returned_time"`
		Result             *string    `gorm:"column:result"`
		Status             *string    `gorm:"column:status"`
	}{
		ResultReturnedTime: &now,
		Result:             stringPtr(reportValue),
		Status:             stringPtr("completed"),
	}

	if err := ep.db.Table("mml_execute_result").
		Where("uid = ?", uid).
		Updates(&result).Error; err != nil {
		logger.Warnf("device %s: failed to update MML result for uid=%s: %v", sn, uid, err)
	}

	// Send web callback for MML result
	callback := map[string]interface{}{
		"type":       "mml_result_returned",
		"sn":         sn,
		"uid":        uid,
		"timestamp":  time.Now().Unix(),
	}
	if cbJson, err := json.Marshal(callback); err == nil {
		redis.Publish(ctx, "web_callback", string(cbJson))
	}

	// Clean up Redis key for this UID
	redis.Del(ctx, "mml:uid:"+uid)
}

// handleBatchResult processes "X E01CEE BATCHRESULT" event.
// Java: extracts batch status/failureCause, updates batch_process_file_send_log.
func (ep *EventProcessor) handleBatchResult(ctx context.Context, sn string, paramMap map[string]string) {
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		return
	}

	status := paramMap["Device.Services.FAPService.1.FAPControl.SelfConfig.X_BBU_batch.Status"]
	failureCause := paramMap["Device.Services.FAPService.1.FAPControl.SelfConfig.X_BBU_batch.FailureCause"]

	// Update batch_process_file_send_log: find pending record (status=1) for this element
	var newStatus int
	if status == "1" {
		newStatus = 2 // success
	} else {
		newStatus = 3 // failure
	}

	updates := map[string]interface{}{
		"status":     newStatus,
		"fault_info": failureCause,
	}
	ep.db.Table("batch_process_file_send_log").
		Where("element_id = ? AND status = ?", cpe.NeNeid, 1).
		Updates(updates)

	logger.Infof("device %s: batch result status=%s, newStatus=%d", sn, status, newStatus)
}

// handleBatchCheckResult processes "X E01CEE BATCHCHECKRESULT" event.
// Java: extracts check status/failureCause, updates batch_process_file_send_log.
func (ep *EventProcessor) handleBatchCheckResult(ctx context.Context, sn string, paramMap map[string]string) {
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		return
	}

	status := paramMap["Device.Services.FAPService.1.FAPControl.SelfConfig.X_BBU_batchCheck.Status"]
	failureCause := paramMap["Device.Services.FAPService.1.FAPControl.SelfConfig.X_BBU_batchCheck.FailureCause"]

	var newStatus int
	if status == "1" {
		newStatus = 4 // check success
	} else {
		newStatus = 5 // check failure
	}

	updates := map[string]interface{}{
		"status":     newStatus,
		"fault_info": failureCause,
	}
	ep.db.Table("batch_process_file_send_log").
		Where("element_id = ? AND status = ?", cpe.NeNeid, 1).
		Updates(updates)

	logger.Infof("device %s: batch check result status=%s, newStatus=%d", sn, status, newStatus)
}

// autoAssignToDefaultGroups assigns a newly created device to all default groups.
func (ep *EventProcessor) autoAssignToDefaultGroups(elementId int64, licenseId *int) {
	// Find platform-level default groups (licenseId=0)
	groups, err := ep.findDefaultGroupsHelper(0)
	if err != nil {
		logger.Warnf("failed to find platform default groups: %v", err)
	}

	// Find tenant-level default groups if applicable
	var tenantGroups []device.DeviceGroup
	if licenseId != nil && *licenseId > 0 {
		tenantGroups, err = ep.findDefaultGroupsHelper(*licenseId)
		if err != nil {
			logger.Warnf("failed to find tenant default groups: %v", err)
		}
	}

	// Atomic: assign all default groups in a single transaction
	allGroups := append(groups, tenantGroups...)
	if len(allGroups) == 0 {
		return
	}
	if err := ep.db.Transaction(func(tx *gorm.DB) error {
		for _, g := range allGroups {
			rel := device.GroupHasElement{GroupId: g.Id, ElementId: elementId}
			if err := tx.Where("group_id = ? AND element_id = ?", g.Id, elementId).First(&rel).Error; err != nil {
				if err := tx.Create(&rel).Error; err != nil {
					return fmt.Errorf("failed to assign group %s to device %d: %w", g.Id, elementId, err)
				}
			}
		}
		return nil
	}); err != nil {
		logger.Errorf("autoAssignToDefaultGroups failed for device %d: %v", elementId, err)
	}
}

// findDefaultGroupsHelper returns device groups marked as default for the given license scope.
func (ep *EventProcessor) findDefaultGroupsHelper(licenseId int) ([]device.DeviceGroup, error) {
	var groups []device.DeviceGroup
	q := ep.db.Where("default_group = ?", true)
	if licenseId > 0 {
		q = q.Where("license_id = ?", licenseId)
	} else {
		q = q.Where("license_id IS NULL")
	}
	if err := q.Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// ProcessResult processes CPE result messages (GetParameterValuesResponse, SetParameterValuesResponse, etc.)
func (ep *EventProcessor) ProcessResult(soapXml string, sn string, deviceType string, generation string) {
	ctx := context.Background()

	if soapXml == "" {
		logger.Warn("ProcessResult called with empty SOAP XML")
		return
	}

	// Parse SOAP header to get cwmp:ID (the tracking ID)
	headerId := extractHeaderIDFromXML(soapXml)
	if headerId == "" {
		logger.Warn("ProcessResult: failed to extract header ID from SOAP")
		return
	}

	// Detect message type
	msgType := soap.DetectMessageType(soapXml)
	logger.Infof("processing result for SN=%s, msgType=%d, headerId=%s", sn, msgType, headerId)

	// Look up command tracking data from event_log table by ID
	var eventLog eventlog.EventLog
	err := ep.db.Where("command_track_data LIKE ?", "%"+headerId+"%").First(&eventLog).Error
	if err != nil {
		logger.Warnf("no tracking data found for headerId=%s: %v", headerId, err)
		// Continue processing even without tracking data
	}

	now := time.Now()

	// Based on message type, process the response
	switch msgType {
	case soap.MsgGetParameterNamesResponse:
		ep.processGetParameterNamesResponse(ctx, soapXml, sn, &eventLog)

	case soap.MsgGetParameterValuesResponse:
		ep.processGetParameterValuesResponse(ctx, soapXml, sn, &eventLog)

	case soap.MsgSetParameterValuesResponse:
		ep.processSetParameterValuesResponse(ctx, soapXml, sn, &eventLog)

	case soap.MsgTransferComplete, soap.MsgAutonomousTransferComplete, soap.MsgFragmentTransferComplete:
		ep.processTransferComplete(ctx, soapXml, sn, &eventLog)

	case soap.MsgDownloadResponse:
		ep.processDownloadResponse(ctx, soapXml, sn, &eventLog)

	case soap.MsgRebootResponse:
		ep.processRebootResponse(ctx, sn, &eventLog)

	case soap.MsgFault:
		ep.processFault(ctx, soapXml, sn, headerId, &eventLog)

	default:
		logger.Infof("unhandled message type %d for SN=%s", msgType, sn)
	}

	// Update event_log record with response data
	eventLog.CommandResponseTime = &now
	if err := ep.db.Save(&eventLog).Error; err != nil {
		logger.Errorf("failed to update event_log for SN=%s: %v", sn, err)
	}

	// Send web callback via Redis pub/sub
	ep.sendWebCallback(ctx, msgType, map[string]interface{}{
		"sn":        sn,
		"header_id": headerId,
		"msg_type":  msgType,
		"event_log": eventLog,
	})
}

// processGetParameterNamesResponse handles GetParameterNamesResponse from CPE.
// During init flow (is_init=true), saves discovered parameter names and sends follow-up GPV for leaf parameters.
func (ep *EventProcessor) processGetParameterNamesResponse(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing GetParameterNamesResponse for SN=%s", sn)

	params, err := soap.ParseGetParameterNamesResponse(soapXml)
	if err != nil {
		logger.Errorf("failed to parse GPN response for SN=%s: %v", sn, err)
		eventLog.EventType = stringPtr("GET_PARAMETER_NAMES_RESPONSE")
		eventLog.Status = intPtr(-1)
		eventLog.FaultInfo = stringPtr(err.Error())
		return
	}

	if len(params) == 0 {
		logger.Warnf("GPN response for SN=%s contains no parameters", sn)
		eventLog.EventType = stringPtr("GET_PARAMETER_NAMES_RESPONSE")
		eventLog.Status = intPtr(0)
		return
	}

	eventLog.EventType = stringPtr("GET_PARAMETER_NAMES_RESPONSE")

	// Check if this is part of the init flow
	isInit := false
	if eventLog.CommandTrackData != nil {
		var trackData map[string]interface{}
		if err := json.Unmarshal([]byte(*eventLog.CommandTrackData), &trackData); err == nil {
			if init, ok := trackData["is_init"].(bool); ok {
				isInit = init
			}
		}
	}

	// Look up device
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).First(&cpe).Error; err != nil {
		logger.Errorf("failed to find device %s for GPN processing: %v", sn, err)
		eventLog.Status = intPtr(-1)
		eventLog.FaultInfo = stringPtr(fmt.Sprintf("device not found: %s", sn))
		return
	}

	// Save all discovered parameter names to element_basic_info_parameter (with empty value)
	now := time.Now()
	for _, p := range params {
		if p.Name == "" {
			continue
		}
		rawSQL := `INSERT INTO element_basic_info_parameter (element_id, param_name, param_value, update_time)
			VALUES (?, ?, '', ?)
			ON DUPLICATE KEY UPDATE update_time = VALUES(update_time)`
		ep.db.Exec(rawSQL, cpe.NeNeid, p.Name, now)
	}

	logger.Infof("GPN: saved %d parameter names for SN=%s (neId=%d)", len(params), sn, cpe.NeNeid)

	// For init flow: filter to leaf parameters and send GPV
	if isInit {
		var leafParams []string
		for _, p := range params {
			// Leaf parameters don't end with "." (object paths end with ".")
			if p.Name != "" && !strings.HasSuffix(p.Name, ".") {
				leafParams = append(leafParams, p.Name)
			}
		}

		if len(leafParams) == 0 {
			logger.Infof("GPN init: no leaf params found for SN=%s, all %d params are objects", sn, len(params))
			eventLog.Status = intPtr(0)
			return
		}

		// Send GPV for discovered leaf parameters (stage 2 of init)
		headerId := soap.GenerateHeaderID()
		soapXml := soap.BuildGetParameterValues(headerId, leafParams)

		// Save tracking data for the GPV stage
		gpvEventType := "INIT_GPV"
		gpvTrackData, _ := json.Marshal(map[string]interface{}{
			"header_id":      headerId,
			"serial_number":  sn,
			"operation_type": gpvEventType,
			"is_init":        true,
			"stage":          "gpv",
			"param_count":    len(leafParams),
			"issue_time":     time.Now().Format(time.RFC3339),
		})
		gpvEventLog := eventlog.EventLog{
			EventType:        &gpvEventType,
			OperationTime:    &now,
			CommandIssueTime: &now,
			ElementId:        &cpe.NeNeid,
			Status:           intPtr(1),
			CommandTrackData: stringPtr(string(gpvTrackData)),
		}
		ep.db.Create(&gpvEventLog)

		// Cache in Redis
		trackKey := fmt.Sprintf("tr069:track:%s", headerId)
		trackJson, _ := json.Marshal(map[string]interface{}{
			"header_id":      headerId,
			"sn":             sn,
			"operation_type": gpvEventType,
			"event_log_id":   gpvEventLog.Id,
			"is_init":        true,
			"stage":          "gpv",
		})
		redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

		// Push GPV to device queue
		msgMgr := NewMessageManager()
		if err := msgMgr.PutMessage(sn, soapXml); err != nil {
			logger.Errorf("GPN init: failed to push GPV to device %s: %v", sn, err)
			eventLog.Status = intPtr(-1)
			eventLog.FaultInfo = stringPtr(fmt.Sprintf("failed to push init GPV: %v", err))
			return
		}

		logger.Infof("GPN init: GPV sent for SN=%s, requesting %d leaf parameters", sn, len(leafParams))
	}

	eventLog.Status = intPtr(0)
}

// processGetParameterValuesResponse extracts parameter values and saves to element_basic_info_parameter table.
func (ep *EventProcessor) processGetParameterValuesResponse(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing GetParameterValuesResponse for SN=%s", sn)

	// Parse parameter values from SOAP response
	params, err := soap.ParseGetParameterValuesResponse(soapXml)
	if err != nil {
		logger.Errorf("failed to parse GPV response for SN=%s: %v", sn, err)
		eventLog.EventType = stringPtr("GET_PARAMETER_VALUES_RESPONSE")
		eventLog.Status = intPtr(-1)
		eventLog.FaultInfo = stringPtr(err.Error())
		return
	}

	if len(params) == 0 {
		logger.Warnf("GPV response for SN=%s contains no parameters", sn)
		eventLog.EventType = stringPtr("GET_PARAMETER_VALUES_RESPONSE")
		eventLog.Status = intPtr(0)
		return
	}

	// Look up device to get neId for lock key
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).First(&cpe).Error; err != nil {
		logger.Errorf("failed to find device %s for saving GPV params: %v", sn, err)
		eventLog.EventType = stringPtr("GET_PARAMETER_VALUES_RESPONSE")
		eventLog.Status = intPtr(-1)
		eventLog.FaultInfo = stringPtr(fmt.Sprintf("device not found: %s", sn))
		return
	}

	// Acquire Redis distributed lock to prevent concurrent param writes
	lockKey := fmt.Sprintf("red_lock_add_parameter_names_%d", cpe.NeNeid)
	if !redis.Lock(ctx, lockKey, 30*time.Second) {
		logger.Warnf("failed to acquire param save lock for device %d, will retry", cpe.NeNeid)
		eventLog.EventType = stringPtr("GET_PARAMETER_VALUES_RESPONSE")
		eventLog.Status = intPtr(-1)
		eventLog.FaultInfo = stringPtr("failed to acquire param save lock, retry later")
		return
	}
	defer redis.Unlock(ctx, lockKey)

	// Save parameter values to element_basic_info_parameter
	ep.saveParameterValues(ctx, sn, params)

	logger.Infof("saved %d parameter values for SN=%s (neId=%d)", len(params), sn, cpe.NeNeid)

	// Task 5.5: Publish GPV result notification to Redis pub/sub
	paramNames := make([]string, len(params))
	for i, p := range params {
		paramNames[i] = p.Name
	}
	gpvNotification := map[string]interface{}{
		"type":         "gpv_result",
		"sn":           sn,
		"element_id":   cpe.NeNeid,
		"param_names":  paramNames,
		"param_count":  len(params),
		"timestamp":    time.Now().Unix(),
		"event_log_id": eventLog.Id,
	}
	if notifyJson, err := json.Marshal(gpvNotification); err == nil {
		if err := redis.Publish(ctx, "parameter:change", string(notifyJson)); err != nil {
			logger.Warnf("failed to publish GPV notification for SN=%s: %v", sn, err)
		}
	}

	// Update event log status
	eventLog.EventType = stringPtr("GET_PARAMETER_VALUES_RESPONSE")
	eventLog.Status = intPtr(0) // success
}

// processSetParameterValuesResponse updates parameter log status and parameter_attributes on success.
func (ep *EventProcessor) processSetParameterValuesResponse(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing SetParameterValuesResponse for SN=%s", sn)

	status, err := soap.ParseSetParameterValuesResponse(soapXml)
	if err != nil {
		logger.Errorf("failed to parse SPV response for SN=%s: %v", sn, err)
		eventLog.EventType = stringPtr("SET_PARAMETER_VALUES_RESPONSE")
		eventLog.Status = intPtr(-1)
		eventLog.FaultInfo = stringPtr(err.Error())
		return
	}

	eventLog.EventType = stringPtr("SET_PARAMETER_VALUES_RESPONSE")

	if status != 0 {
		// SPV failed
		logger.Warnf("SPV response for SN=%s returned status=%d", sn, status)
		eventLog.Status = intPtr(status)
		eventLog.FaultInfo = stringPtr(fmt.Sprintf("SPV failed with status %d", status))
		return
	}

	eventLog.Status = intPtr(0) // success

	// Extract the parameter names that were set from the tracking data.
	// The command_track_data contains the operationParam JSON with paramName/paramValue.
	if eventLog.CommandTrackData != nil {
		var trackData map[string]interface{}
		if err := json.Unmarshal([]byte(*eventLog.CommandTrackData), &trackData); err == nil {
			if opParam, ok := trackData["operationParam"].(string); ok {
				var setParams []struct {
					ParamName  string `json:"paramName"`
					ParamValue string `json:"paramValue"`
				}
				if err := json.Unmarshal([]byte(opParam), &setParams); err == nil {
					ep.updateParameterAttributesAfterSet(ctx, sn, setParams)
				}
			}
		}
	}

	logger.Infof("SPV success for SN=%s, parameter attributes updated", sn)

	// Task 5.5: Publish parameter change notification to Redis pub/sub
	if eventLog.CommandTrackData != nil {
		var trackData map[string]interface{}
		if err := json.Unmarshal([]byte(*eventLog.CommandTrackData), &trackData); err == nil {
			paramNames := []string{}
			if opParam, ok := trackData["operationParam"].(string); ok {
				var setParams []struct {
					ParamName string `json:"paramName"`
				}
				if err := json.Unmarshal([]byte(opParam), &setParams); err == nil {
					for _, p := range setParams {
						paramNames = append(paramNames, p.ParamName)
					}
				}
			}
			changeNotification := map[string]interface{}{
				"type":         "spv_change",
				"sn":           sn,
				"param_names":  paramNames,
				"timestamp":    time.Now().Unix(),
				"event_log_id": eventLog.Id,
			}
			if notifyJson, err := json.Marshal(changeNotification); err == nil {
				if err := redis.Publish(ctx, "parameter:change", string(notifyJson)); err != nil {
					logger.Warnf("failed to publish parameter change notification for SN=%s: %v", sn, err)
				}
			}
		}
	}
}

// updateParameterAttributesAfterSet updates parameter_attributes and creates parameter_log entries
// after a successful SetParameterValues response.
func (ep *EventProcessor) updateParameterAttributesAfterSet(ctx context.Context, sn string, params []struct {
	ParamName  string `json:"paramName"`
	ParamValue string `json:"paramValue"`
}) {
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).First(&cpe).Error; err != nil {
		logger.Errorf("failed to find device %s for SPV post-processing: %v", sn, err)
		return
	}

	now := time.Now()
	for _, p := range params {
		// Update element_basic_info_parameter
		var existing device.ElementBasicInfoParameter
		err := ep.db.Where("element_id = ? AND param_name = ?", cpe.NeNeid, p.ParamName).First(&existing).Error
		if err == nil {
			oldValue := ""
			if existing.ParamValue != nil {
				oldValue = *existing.ParamValue
			}
			existing.ParamValue = stringPtr(p.ParamValue)
			existing.UpdateTime = &now
			ep.db.Save(&existing)

			// Create parameter_log
			ep.createParameterLog(ctx, &cpe, p.ParamName, oldValue, p.ParamValue, &now)
		} else if err == gorm.ErrRecordNotFound {
			newParam := device.ElementBasicInfoParameter{
				ElementId:  &cpe.NeNeid,
				ParamName:  stringPtr(p.ParamName),
				ParamValue: stringPtr(p.ParamValue),
				UpdateTime: &now,
			}
			ep.db.Create(&newParam)
			ep.createParameterLog(ctx, &cpe, p.ParamName, "", p.ParamValue, &now)
		}
	}
}

// createParameterLog creates a parameter_log entry recording the old and new values.
func (ep *EventProcessor) createParameterLog(ctx context.Context, cpe *device.CpeElement, paramName, oldValue, newValue string, changeTime *time.Time) {
	log := struct {
		ParameterName string     `gorm:"column:parameter_name;type:varchar(255)"`
		OldValue      string     `gorm:"column:old_value;type:mediumtext"`
		NewValue      string     `gorm:"column:new_value;type:mediumtext"`
		ChangeUser    string     `gorm:"column:change_user;type:varchar(255)"`
		ChangeTime    *time.Time `gorm:"column:change_time"`
		ElementId     int64      `gorm:"column:element_id"`
	}{
		ParameterName: paramName,
		OldValue:      oldValue,
		NewValue:      newValue,
		ChangeUser:    "tr069",
		ChangeTime:    changeTime,
		ElementId:     cpe.NeNeid,
	}
	if err := ep.db.Table("parameter_log").Create(&log).Error; err != nil {
		logger.Errorf("failed to create parameter_log for %s param %s: %v", *cpe.SerialNumber, paramName, err)
	}
}

// processTransferComplete updates download/upgrade status.
// Uses CommandKey to correlate back to the originating operation (Download/Upload/Reboot).
func (ep *EventProcessor) processTransferComplete(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing TransferComplete for SN=%s", sn)

	tc, err := soap.ParseTransferComplete(soapXml)
	if err != nil {
		logger.Errorf("failed to parse TransferComplete: %v", err)
		eventLog.Status = intPtr(-1)
		eventLog.FaultInfo = stringPtr(err.Error())
		return
	}

	eventLog.EventType = stringPtr("TRANSFER_COMPLETE")

	if tc.FaultCode != 0 {
		eventLog.Status = intPtr(tc.FaultCode)
		eventLog.FaultInfo = stringPtr(tc.FaultString)
		logger.Warnf("TransferComplete fault for SN=%s: code=%d, string=%s", sn, tc.FaultCode, tc.FaultString)
	} else {
		eventLog.Status = intPtr(0) // success
	}

	// Use CommandKey to find the originating operation's event_log
	if tc.CommandKey != "" {
		var originEvent eventlog.EventLog
		err := ep.db.Where("command_track_data LIKE ?", "%"+tc.CommandKey+"%").First(&originEvent).Error
		if err == nil {
			logger.Infof("TransferComplete CommandKey=%s matched event_log id=%d for SN=%s", tc.CommandKey, originEvent.Id, sn)

			// Update upgrade_log status via command_track_id
			now := time.Now()
			done := true
			success := tc.FaultCode == 0

			// Check if this is a download operation
			if originEvent.EventType != nil && *originEvent.EventType == "DOWNLOAD" {
				ep.db.Model(&struct {
					IsDownloaded *bool      `gorm:"column:is_downloaded"`
					DownloadedTime *time.Time `gorm:"column:downloaded_time"`
					IsDone       *bool      `gorm:"column:is_done"`
					DoneTime     *time.Time `gorm:"column:done_time"`
					Success      *bool      `gorm:"column:success"`
					Message      *string    `gorm:"column:message"`
				}{}).
					Table("upgrade_log").
					Where("command_track_id = ?", originEvent.Id).
					Updates(map[string]interface{}{
						"is_downloaded":   &done,
						"downloaded_time": &now,
						"is_done":         &done,
						"done_time":       &now,
						"success":         &success,
					})
			}

			// Check if this is a reboot operation
			if originEvent.EventType != nil && *originEvent.EventType == "REBOOT" {
				// Clear rebooting flag
				rebootKey := fmt.Sprintf("device:rebooting:%s", sn)
				redis.Del(ctx, rebootKey)
			}

			// Check if this is an upload operation
			if originEvent.EventType != nil && *originEvent.EventType == "UPLOAD" {
				ep.db.Table("upgrade_log").
					Where("command_track_id = ?", originEvent.Id).
					Updates(map[string]interface{}{
						"is_done":   &done,
						"done_time": &now,
						"success":   &success,
					})
			}
		} else {
			logger.Debugf("TransferComplete CommandKey=%s: no matching origin event_log for SN=%s", tc.CommandKey, sn)
		}
	}
}

// processDownloadResponse updates download task status.
func (ep *EventProcessor) processDownloadResponse(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing DownloadResponse for SN=%s", sn)

	eventLog.EventType = stringPtr("DOWNLOAD_RESPONSE")
	eventLog.Status = intPtr(0) // success
}

// processRebootResponse updates reboot task status.
func (ep *EventProcessor) processRebootResponse(ctx context.Context, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing RebootResponse for SN=%s", sn)

	eventLog.EventType = stringPtr("REBOOT_RESPONSE")
	eventLog.Status = intPtr(0) // success

	// Clear rebooting flag in Redis
	rebootKey := fmt.Sprintf("device:rebooting:%s", sn)
	if err := redis.Del(ctx, rebootKey); err != nil {
		logger.Warnf("failed to clear rebooting flag for %s: %v", sn, err)
	}
}

// processFault handles SOAP Fault responses from CPE.
// It looks up the tracking data from Redis, updates the event_log status to failed,
// and logs the fault with context about which operation failed.
func (ep *EventProcessor) processFault(ctx context.Context, soapXml string, sn string, headerId string, eventLog *eventlog.EventLog) {
	logger.Warnf("processing Fault for SN=%s, headerId=%s", sn, headerId)

	resp, err := soap.ParseGenericResponse(soapXml)
	if err != nil {
		logger.Errorf("failed to parse Fault response for SN=%s, headerId=%s: %v", sn, headerId, err)
		eventLog.EventType = stringPtr("FAULT")
		eventLog.Status = intPtr(3) // failed
		eventLog.FaultInfo = stringPtr(fmt.Sprintf("failed to parse fault response: %v", err))
		return
	}

	// Look up tracking data from Redis to determine which operation failed
	trackKey := fmt.Sprintf("tr069:track:%s", headerId)
	trackJson, trackErr := redis.Get(ctx, trackKey)

	operationType := "UNKNOWN"
	var trackData map[string]interface{}
	if trackErr == nil && trackJson != "" {
		if err := json.Unmarshal([]byte(trackJson), &trackData); err == nil {
			if opType, ok := trackData["operation_type"].(string); ok {
				operationType = opType
			}
		}
	}

	// Build fault description with CWMP fault code name
	faultDesc := fmt.Sprintf("code=%d, string=%s", resp.FaultCode, resp.FaultString)
	switch resp.FaultCode {
	case soap.FaultMethodNotSupported:
		faultDesc = fmt.Sprintf("MethodNotSupported(9000): %s", resp.FaultString)
	case soap.FaultRequestDenied:
		faultDesc = fmt.Sprintf("RequestDenied(9001): %s", resp.FaultString)
	case soap.FaultInternalError:
		faultDesc = fmt.Sprintf("InternalError(9002): %s", resp.FaultString)
	case soap.FaultInvalidArguments:
		faultDesc = fmt.Sprintf("InvalidArguments(9003): %s", resp.FaultString)
	case soap.FaultResourcesExceeded:
		faultDesc = fmt.Sprintf("ResourcesExceeded(9004): %s", resp.FaultString)
	case soap.FaultRetryRequest:
		faultDesc = fmt.Sprintf("RetryRequest(9005): %s", resp.FaultString)
	case soap.FaultTransferCompleteRetry:
		faultDesc = fmt.Sprintf("TransferCompleteRetry(9006): %s", resp.FaultString)
	case soap.FaultAuthenticationFailure:
		faultDesc = fmt.Sprintf("AuthenticationFailure(9007): %s", resp.FaultString)
	case soap.FaultUnsupportedProtocol:
		faultDesc = fmt.Sprintf("UnsupportedProtocol(9008): %s", resp.FaultString)
	}

	logger.Warnf("device %s fault on operation %s (headerId=%s): %s", sn, operationType, headerId, faultDesc)

	// Update event_log with fault information
	eventLog.EventType = stringPtr("FAULT")
	eventLog.Status = intPtr(3) // failed
	eventLog.FaultInfo = stringPtr(fmt.Sprintf("[%s] %s", operationType, faultDesc))

	// Update Redis track data to mark as failed
	if trackErr == nil && trackJson != "" {
		trackData["status"] = "failed"
		trackData["fault_code"] = resp.FaultCode
		trackData["fault_string"] = resp.FaultString
		if updatedJson, err := json.Marshal(trackData); err == nil {
			if err := redis.Set(ctx, trackKey, string(updatedJson), 24*time.Hour); err != nil {
				logger.Warnf("failed to update fault track data in Redis for headerId=%s: %v", headerId, err)
			}
		}
	}
}

// updateDeviceOnlineStatus updates the online status of a device in Redis.
func (ep *EventProcessor) updateDeviceOnlineStatus(ctx context.Context, sn string, online bool) {
	key := fmt.Sprintf("device:online:%s", sn)
	value := "0"
	if online {
		value = "1"
	}

	// Set with 5 minute TTL
	if err := redis.Set(ctx, key, value, 5*time.Minute); err != nil {
		logger.Errorf("failed to update online status for %s: %v", sn, err)
	}
}

// updateDeviceBasicInfo updates device basic information from Inform parameters.
func (ep *EventProcessor) updateDeviceBasicInfo(ctx context.Context, cpe *device.CpeElement, params []soap.ParameterValueStruct) {
	// Determine rootNode from CpeElement or first param
	rootNode := "Device"
	if cpe.RootNode != nil && *cpe.RootNode != "" {
		rootNode = *cpe.RootNode
	} else if len(params) > 0 {
		if idx := strings.Index(params[0].Name, "."); idx > 0 {
			rootNode = params[0].Name[:idx]
		}
	}

	macRegex := regexp.MustCompile(`Device\.Ethernet\.Interface\.\d+\.MACAddress`)
	var macs []string

	for _, param := range params {
		switch {
		case param.Name == rootNode+".DeviceInfo.SoftwareVersion" ||
			param.Name == rootNode+".DeviceInfo.MU.1.SoftwareVersion" ||
			param.Name == rootNode+".DeviceInfo.MU.1.Slot.1.SoftwareVersion":
			cpe.SoftwareVersion = stringPtr(param.Value)
		case param.Name == rootNode+".DeviceInfo.HardwareVersion" ||
			param.Name == rootNode+".DeviceInfo.MU.1.HardwareVersion" ||
			param.Name == rootNode+".DeviceInfo.MU.1.Slot.1.HardwareVersion":
			cpe.HardwareVersion = stringPtr(param.Value)
		case param.Name == rootNode+".DeviceInfo.FirmwareVersion" ||
			param.Name == "Device.DeviceInfo.MU.1.FirmwareVersion":
			cpe.FirmwareVersion = stringPtr(param.Value)
		case param.Name == "Device.DeviceInfo.FullSoftwareVersion":
			cpe.FullSoftwareVersion = stringPtr(param.Value)
		case param.Name == "Device.DeviceInfo.StmVersion":
			cpe.StmVersion = stringPtr(param.Value)
		case param.Name == rootNode+".DeviceInfo.ModelName" ||
			param.Name == rootNode+".DeviceInfo.MU.1.ModelName" ||
			param.Name == rootNode+".DeviceInfo.MU.1.Slot.1.ModelName":
			cpe.ModelName = stringPtr(param.Value)
		case param.Name == rootNode+".DeviceInfo.Manufacturer":
			cpe.Manufacturer = stringPtr(param.Value)
		case param.Name == rootNode+".ManagementServer.URL":
			cpe.CoonReqUrl = stringPtr(param.Value)
		case param.Name == "Device.SoftwareCtrl.ManualActivateTargetSoftVersion":
			cpe.TargetVersion = stringPtr(param.Value)
		case param.Name == "Device.SoftwareCtrl.ManualActivateTargetFwVersion":
			cpe.TargetHardwareVersion = stringPtr(param.Value)
		case param.Name == rootNode+".WANDevice.1.WANConnectionDevice.1.WANIPConnection.1.ExternalIPAddress":
			cpe.DeviceIp = stringPtr(param.Value)
		case macRegex.MatchString(param.Name):
			macs = append(macs, strings.ReplaceAll(param.Value, "-", ""))
		}
	}

	// Sort and join MAC addresses
	if len(macs) > 0 {
		sort.Strings(macs)
		unique := macs[:0]
		for i, m := range macs {
			if i == 0 || m != macs[i-1] {
				unique = append(unique, m)
			}
		}
		cpe.Mac = stringPtr(strings.Join(unique, ","))
	}

	cpe.LoadedBasicInfo = true
}

// saveParameterValues saves parameter values to element_basic_info_parameter table using batch upsert.
func (ep *EventProcessor) saveParameterValues(ctx context.Context, sn string, params []soap.ParameterValueStruct) {
	// First, find the device to get element_id
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ?", sn).First(&cpe).Error; err != nil {
		logger.Errorf("failed to find device %s for saving parameters: %v", sn, err)
		return
	}

	now := time.Now()

	// Build batch upsert data
	for _, param := range params {
		if param.Name == "" {
			continue
		}
		// Use GORM's upsert pattern: ON DUPLICATE KEY UPDATE
		rawSQL := `INSERT INTO element_basic_info_parameter (element_id, param_name, param_value, update_time)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE param_value = VALUES(param_value), update_time = VALUES(update_time)`
		if err := ep.db.Exec(rawSQL, cpe.NeNeid, param.Name, param.Value, now).Error; err != nil {
			logger.Errorf("failed to upsert parameter %s for %s: %v", param.Name, sn, err)
		}
	}

	// Mark device as having loaded basic info
	if !cpe.LoadedBasicInfo {
		ep.db.Model(&cpe).Update("loaded_basic_info", true)
	}
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

	// Condition 1: version_mismatch — target_version is set and differs from current software_version
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

	// Condition 2: failed_state — device status contains "error" or "failed"
	if conditions["failed_state"] {
		if cpe.Status != nil {
			statusLower := strings.ToLower(*cpe.Status)
			if strings.Contains(statusLower, "error") || strings.Contains(statusLower, "failed") {
				reasons = append(reasons, fmt.Sprintf("failed_state: status=%s", *cpe.Status))
			}
		}
	}

	// Condition 3: ztp_pending — ReadyToZTP flag is set but device is not yet initialized
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

// Helper functions

func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}


// extractHeaderIDFromXML extracts the cwmp:ID from a SOAP envelope XML string.
func extractHeaderIDFromXML(xmlStr string) string {
	type soapEnvelope struct {
		XMLName struct{} `xml:"Envelope"`
		Header  struct {
			ID string `xml:"ID"`
		} `xml:"Header"`
	}

	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return ""
	}
	return env.Header.ID
}

// checkDeviceLimit checks if the license allows creating more devices.
// It reads device_config from system_config table, compares current device count vs maxDeviceCount.
// Returns nil if creation is allowed, or an error if the limit has been reached.
func (ep *EventProcessor) checkDeviceLimit() error {
	// Read platform-level device config (tenancyId=0)
	var cfg misc.SystemConfig
	key := "device_config_0"
	err := ep.db.Where("id = ?", key).First(&cfg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// No config found, allow device creation (no limit set)
			return nil
		}
		return fmt.Errorf("failed to read device config: %w", err)
	}

	if cfg.Config == nil || *cfg.Config == "" {
		// No config value, allow device creation
		return nil
	}

	var deviceCfg systemsettings.DeviceConfig
	if err := json.Unmarshal([]byte(*cfg.Config), &deviceCfg); err != nil {
		logger.Warnf("failed to unmarshal device config: %v", err)
		return nil // Allow creation if config is malformed
	}

	// If maxDeviceCount is not set, allow unlimited devices
	if deviceCfg.MaxDeviceCount == nil {
		return nil
	}

	maxCount := *deviceCfg.MaxDeviceCount
	if maxCount <= 0 {
		return nil // No limit
	}

	// Count current non-deleted devices
	var currentCount int64
	if err := ep.db.Model(&device.CpeElement{}).Where("deleted = ?", false).Count(&currentCount).Error; err != nil {
		return fmt.Errorf("failed to count devices: %w", err)
	}

	if currentCount >= int64(maxCount) {
		return fmt.Errorf("device limit reached: current=%d, max=%d", currentCount, maxCount)
	}

	return nil
}

// detectDeviceType auto-detects device type and generation from the ProductClass field.
// Returns deviceType and generation strings.
func detectDeviceType(productClass string) (deviceType string, generation string) {
	if productClass == "" {
		return "cpe", ""
	}

	upper := strings.ToUpper(productClass)

	// Check for gNB/gNodeB (5G NR)
	if strings.Contains(upper, "GNB") || strings.Contains(upper, "GNODEB") {
		return "enb", "NR"
	}

	// Check for eNB/eNodeB (4G LTE)
	if strings.Contains(upper, "ENB") || strings.Contains(upper, "ENODEB") {
		return "enb", ""
	}

	// Check for CPE/IGD/Femto
	if strings.Contains(upper, "CPE") || strings.Contains(upper, "IGD") || strings.Contains(upper, "FEMTO") {
		return "cpe", ""
	}

	// Default to CPE
	return "cpe", ""
}
