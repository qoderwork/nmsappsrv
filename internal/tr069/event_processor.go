package tr069

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"time"

	"nmsappsrv/internal/device"
	"nmsappsrv/internal/eventlog"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

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
		logger.Infof("device %s not found, auto-creating", sn)
		cpe = device.CpeElement{
			SerialNumber:    stringPtr(sn),
			Status:          stringPtr("online"),
			CreationTime:    &now,
			Manufacturer:    stringPtr(inform.DeviceId.Manufacturer),
			Oui:             stringPtr(inform.DeviceId.OUI),
			ModelName:       stringPtr(inform.DeviceId.ProductClass),
			Generation:      stringPtr(generation),
			DeviceType:      stringPtr(deviceType),
			LoadedBasicInfo: false,
			IsInitialized:   false,
			Deleted:         false,
		}

		// Extract basic info from Inform parameters
		for _, param := range inform.ParameterList {
			switch param.Name {
			case "InternetGatewayDevice.DeviceInfo.SoftwareVersion":
				cpe.SoftwareVersion = stringPtr(param.Value)
			case "InternetGatewayDevice.DeviceInfo.HardwareVersion":
				cpe.HardwareVersion = stringPtr(param.Value)
			case "InternetGatewayDevice.DeviceInfo.Manufacturer":
				cpe.Manufacturer = stringPtr(param.Value)
			case "InternetGatewayDevice.DeviceInfo.ModelName":
				cpe.ModelName = stringPtr(param.Value)
			}
		}

		if err := ep.db.Create(&cpe).Error; err != nil {
			logger.Errorf("failed to create device %s: %v", sn, err)
			return
		}

		// Set online status in Redis for new device
		ep.updateDeviceOnlineStatus(ctx, sn, true)

		logger.Infof("device %s created successfully", sn)
	} else {
		logger.Errorf("failed to query device %s: %v", sn, err)
		return
	}

	// Process each event code
	for _, evt := range inform.EventList {
		ep.processEventCode(ctx, evt, sn)
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
}

// processEventCode handles individual TR069 event codes.
func (ep *EventProcessor) processEventCode(ctx context.Context, evt soap.EventStruct, sn string) {
	switch evt.Code {
	case "0 BOOTSTRAP":
		logger.Infof("device %s: BOOTSTRAP event", sn)

	case "1 BOOT":
		logger.Infof("device %s: BOOT event", sn)
		// Clear rebooting flag in Redis
		rebootKey := fmt.Sprintf("device:rebooting:%s", sn)
		if err := redis.Del(ctx, rebootKey); err != nil {
			logger.Warnf("failed to clear rebooting flag for %s: %v", sn, err)
		}

	case "2 PERIODIC":
		// No action needed for periodic inform
		logger.Debugf("device %s: PERIODIC event", sn)

	case "4 VALUE CHANGE":
		logger.Infof("device %s: VALUE CHANGE event - parameter changed", sn)

	case "M Reboot":
		logger.Infof("device %s: M Reboot event", sn)
		// Clear rebooting flag in Redis
		rebootKey := fmt.Sprintf("device:rebooting:%s", sn)
		if err := redis.Del(ctx, rebootKey); err != nil {
			logger.Warnf("failed to clear rebooting flag for %s: %v", sn, err)
		}

	case "101 ALARM":
		logger.Infof("device %s: ALARM event - command key: %s", sn, evt.CommandKey)

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
		ep.processFault(ctx, soapXml, sn, &eventLog)

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

// processGetParameterValuesResponse extracts parameter values and saves to element_basic_info_parameter table.
func (ep *EventProcessor) processGetParameterValuesResponse(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing GetParameterValuesResponse for SN=%s", sn)

	// TODO: Parse actual parameter values from response and save to element_basic_info_parameter
	// For now, we just log that we received it

	// Update event log status
	eventLog.EventType = stringPtr("GET_PARAMETER_VALUES_RESPONSE")
	eventLog.Status = intPtr(0) // success
}

// processSetParameterValuesResponse updates parameter log status.
func (ep *EventProcessor) processSetParameterValuesResponse(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing SetParameterValuesResponse for SN=%s", sn)

	eventLog.EventType = stringPtr("SET_PARAMETER_VALUES_RESPONSE")
	eventLog.Status = intPtr(0) // success
}

// processTransferComplete updates download/upgrade status.
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

// processFault logs fault and updates task status as failed.
func (ep *EventProcessor) processFault(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Warnf("processing Fault for SN=%s", sn)

	resp, err := soap.ParseGenericResponse(soapXml)
	if err != nil {
		logger.Errorf("failed to parse Fault response: %v", err)
		return
	}

	eventLog.EventType = stringPtr("FAULT")
	eventLog.Status = intPtr(resp.FaultCode)
	eventLog.FaultInfo = stringPtr(resp.FaultString)

	logger.Warnf("device %s fault: code=%d, string=%s", sn, resp.FaultCode, resp.FaultString)
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
	for _, param := range params {
		switch param.Name {
		case "InternetGatewayDevice.DeviceInfo.SoftwareVersion":
			cpe.SoftwareVersion = stringPtr(param.Value)
		case "InternetGatewayDevice.DeviceInfo.HardwareVersion":
			cpe.HardwareVersion = stringPtr(param.Value)
		case "InternetGatewayDevice.DeviceInfo.Manufacturer":
			cpe.Manufacturer = stringPtr(param.Value)
		case "InternetGatewayDevice.DeviceInfo.ModelName":
			cpe.ModelName = stringPtr(param.Value)
		case "InternetGatewayDevice.WANDevice.1.WANConnectionDevice.1.WANIPConnection.1.ExternalIPAddress":
			cpe.DeviceIp = stringPtr(param.Value)
		}
	}
	cpe.LoadedBasicInfo = true
}

// saveParameterValues saves parameter values to element_basic_info_parameter table.
func (ep *EventProcessor) saveParameterValues(ctx context.Context, sn string, params []soap.ParameterValueStruct) {
	// First, find the device to get element_id
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ?", sn).First(&cpe).Error; err != nil {
		logger.Errorf("failed to find device %s for saving parameters: %v", sn, err)
		return
	}

	now := time.Now()

	for _, param := range params {
		// Check if parameter already exists
		var existing device.ElementBasicInfoParameter
		err := ep.db.Where("element_id = ? AND param_name = ?", cpe.NeNeid, param.Name).First(&existing).Error

		if err == nil {
			// Update existing
			existing.ParamValue = stringPtr(param.Value)
			existing.UpdateTime = &now
			if err := ep.db.Save(&existing).Error; err != nil {
				logger.Errorf("failed to update parameter %s for %s: %v", param.Name, sn, err)
			}
		} else if err == gorm.ErrRecordNotFound {
			// Create new
			newParam := device.ElementBasicInfoParameter{
				ElementId:  &cpe.NeNeid,
				ParamName:  stringPtr(param.Name),
				ParamValue: stringPtr(param.Value),
				UpdateTime: &now,
			}
			if err := ep.db.Create(&newParam).Error; err != nil {
				logger.Errorf("failed to create parameter %s for %s: %v", param.Name, sn, err)
			}
		} else {
			logger.Errorf("failed to query parameter %s for %s: %v", param.Name, sn, err)
		}
	}
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
