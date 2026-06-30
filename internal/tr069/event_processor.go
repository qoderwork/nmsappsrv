package tr069

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"nmsappsrv/internal/device"
	"nmsappsrv/internal/eventlog"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/systemsettings"
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
		ep.processEventCode(ctx, evt, sn)
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
		logger.Infof("device %s: VALUE CHANGE event - triggering parameter refresh", sn)
		// Auto-fetch changed parameters by sending a GPV for basic param paths
		go ep.fetchChangedParameters(sn)

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

// autoAssignToDefaultGroups assigns a newly created device to all default groups.
func (ep *EventProcessor) autoAssignToDefaultGroups(elementId int64, licenseId *int) {
	// Find platform-level default groups (licenseId=0)
	groups, err := ep.findDefaultGroupsHelper(0)
	if err != nil {
		logger.Warnf("failed to find platform default groups: %v", err)
	}
	for _, g := range groups {
		rel := device.GroupHasElement{GroupId: g.Id, ElementId: elementId}
		if err := ep.db.Where("group_id = ? AND element_id = ?", g.Id, elementId).First(&rel).Error; err != nil {
			ep.db.Create(&rel)
		}
	}

	// Find tenant-level default groups if applicable
	if licenseId != nil && *licenseId > 0 {
		tenantGroups, err := ep.findDefaultGroupsHelper(*licenseId)
		if err != nil {
			logger.Warnf("failed to find tenant default groups: %v", err)
		}
		for _, g := range tenantGroups {
			rel := device.GroupHasElement{GroupId: g.Id, ElementId: elementId}
			if err := ep.db.Where("group_id = ? AND element_id = ?", g.Id, elementId).First(&rel).Error; err != nil {
				ep.db.Create(&rel)
			}
		}
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

	// Get the parameter paths to query based on device type
	paramPaths := GetBasicParamPaths(deviceType)
	if len(paramPaths) == 0 {
		logger.Warnf("no basic param paths for device type %s, marking as initialized", deviceType)
		ep.db.Model(&device.CpeElement{}).Where("ne_neid = ?", neId).Update("is_initialized", true)
		return
	}

	// Build GPV SOAP XML
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetParameterValues(headerId, paramPaths)

	// Save tracking data to event_log
	now := time.Now()
	eventType := "GET_PARAMETER_VALUES"
	trackData, _ := json.Marshal(map[string]interface{}{
		"header_id":      headerId,
		"serial_number":  sn,
		"operation_type": eventType,
		"is_init":        true,
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
	if err := ep.db.Create(&eventLog).Error; err != nil {
		logger.Errorf("failed to create init event_log for device %d: %v", neId, err)
	}

	// Cache track data in Redis for quick lookup during response processing
	trackKey := fmt.Sprintf("tr069:track:%s", headerId)
	trackJson, _ := json.Marshal(map[string]interface{}{
		"header_id":      headerId,
		"sn":             sn,
		"operation_type": eventType,
		"event_log_id":   eventLog.Id,
		"is_init":        true,
	})
	redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

	// Push SOAP to device queue via MessageManager
	msgMgr := NewMessageManager()
	if err := msgMgr.PutMessage(sn, soapXml); err != nil {
		logger.Errorf("failed to push init GPV to device %s queue: %v", sn, err)
		return
	}

	logger.Infof("device init GPV sent for SN=%s, requesting %d parameters", sn, len(paramPaths))

	// Mark device as initialized (the GPV response will be processed asynchronously)
	// We mark it here to prevent repeated init triggers on subsequent Informs
	ep.db.Model(&device.CpeElement{}).Where("ne_neid = ?", neId).Update("is_initialized", true)
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
		Value *string `gorm:"column:value"`
	}
	if err := ep.db.Table("system_config").
		Select("value").
		Where("config_key = ?", "parameter_preset_config").
		First(&globalPreset).Error; err == nil && globalPreset.Value != nil && *globalPreset.Value != "" {
		var globalParams map[string]interface{}
		if err := json.Unmarshal([]byte(*globalPreset.Value), &globalParams); err == nil {
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
	err := ep.db.Where("config_key = ?", key).First(&cfg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// No config found, allow device creation (no limit set)
			return nil
		}
		return fmt.Errorf("failed to read device config: %w", err)
	}

	if cfg.Value == nil || *cfg.Value == "" {
		// No config value, allow device creation
		return nil
	}

	var deviceCfg systemsettings.DeviceConfig
	if err := json.Unmarshal([]byte(*cfg.Value), &deviceCfg); err != nil {
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
