package tr069

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"nmsappsrv/internal/device"
	"nmsappsrv/internal/eventlog"
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
		// DeviceReconnectedPostprocessor: check before updating online status
		ep.handleDeviceReconnected(ctx, sn, &cpe)

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
		ep.handleAlarmGenerated(ctx, sn, paramMap)
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
		var cpe device.CpeElement
		if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err == nil && cpe.NeNeid != 0 {
			rebootKey := fmt.Sprintf("rebooting_%d", cpe.NeNeid)
			if err := redis.Del(ctx, rebootKey); err != nil {
				logger.Warnf("failed to clear rebooting flag for %s: %v", sn, err)
			}
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

	case soap.MsgTransferComplete, soap.MsgAutonomousTransferComplete, soap.MsgFragmentTransferComplete, soap.MsgAutonomousFragmentTransferComplete:
		ep.processTransferComplete(ctx, soapXml, sn, &eventLog)

	case soap.MsgDownloadResponse:
		ep.processDownloadResponse(ctx, soapXml, sn, &eventLog)

	case soap.MsgRebootResponse:
		ep.processRebootResponse(ctx, sn, &eventLog)

	case soap.MsgReportTransmissionProgress:
		ep.processReportTransmissionProgress(ctx, soapXml, sn, &eventLog)

	case soap.MsgFault:
		ep.processFault(ctx, soapXml, sn, headerId, &eventLog)

	case soap.MsgUpdateCBSDStatusResponse:
		ep.processUpdateCBSDStatusResponse(ctx, soapXml, sn, &eventLog)

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
