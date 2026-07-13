package tr069

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nmsappsrv/internal/device"
	"nmsappsrv/internal/eventlog"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

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

	// If this GPV was issued by the monitor collector, persist the samples into
	// monitor_data via the registered callback (set by the monitor package).
	if opID := extractOperationID(eventLog); opID != "" && strings.HasPrefix(opID, "monitor:") {
		if MonitorGPVCallback != nil {
			MonitorGPVCallback(sn, opID, params)
		}
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

// updateParameterAttributesAfterSet and createParameterLog are in event_helpers.go.

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
					IsDownloaded   *bool      `gorm:"column:is_downloaded"`
					DownloadedTime *time.Time `gorm:"column:downloaded_time"`
					IsDone         *bool      `gorm:"column:is_done"`
					DoneTime       *time.Time `gorm:"column:done_time"`
					Success        *bool      `gorm:"column:success"`
					Message        *string    `gorm:"column:message"`
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

// extractOperationID returns the operation_id embedded in an event_log's
// command_track_data JSON, used to correlate GPV responses back to their issuer.
func extractOperationID(eventLog *eventlog.EventLog) string {
	if eventLog == nil || eventLog.CommandTrackData == nil {
		return ""
	}
	var track struct {
		OperationID string `json:"operation_id"`
	}
	if err := json.Unmarshal([]byte(*eventLog.CommandTrackData), &track); err != nil {
		return ""
	}
	return track.OperationID
}
