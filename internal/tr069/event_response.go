package tr069

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nmsappsrv/internal/device"
	"nmsappsrv/internal/eventlog"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
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

		// If this SPV carried an MML command, mark the execution result delivered
		// with has_fault (对齐 Java MmlMessageProcessor on SOAP fault path).
		if opID := extractOperationID(eventLog); opID != "" && strings.HasPrefix(opID, "mml:") {
			if MMLResponseCallback != nil {
				MMLResponseCallback(parseMMLResultID(opID), false, fmt.Sprintf("SPV failed with status %d", status))
			}
		}
		return
	}

	eventLog.Status = intPtr(0) // success

	// If this SPV carried an MML command, mark the execution result delivered
	// (status=3). Success is encoded by has_fault=false (对齐 Java MmlMessageProcessor).
	if opID := extractOperationID(eventLog); opID != "" && strings.HasPrefix(opID, "mml:") {
		if MMLResponseCallback != nil {
			MMLResponseCallback(parseMMLResultID(opID), true, "")
		}
	}

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

				// Java: License download success triggers auto SOFT_REBOOT
				// (PositiveMessageProcessor.downloadLicenseFileAfter).
				if success && originEvent.CommandTrackData != nil {
					var trackData map[string]interface{}
					if json.Unmarshal([]byte(*originEvent.CommandTrackData), &trackData) == nil {
						if ft, ok := trackData["file_type"].(string); ok && ft == "102 License File" {
							logger.Infof("License download success for SN=%s, triggering auto SOFT_REBOOT", sn)
							utils.SafeGo("LicenseReboot-"+sn, func() {
								msgMgr := NewMessageManager()
								opSender := NewOperationSender(ep.db, msgMgr)
								opSender.SendSoftReboot(sn, "license_auto_reboot_"+strconv.FormatInt(now.Unix(), 10), "", 0)
							})
						}
					}
				}
			}

			// Check if this is a reboot operation
			if originEvent.EventType != nil && *originEvent.EventType == "REBOOT" && originEvent.ElementId != nil {
				// Clear rebooting flag
				rebootKey := fmt.Sprintf("rebooting_%d", *originEvent.ElementId)
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

				// Device log upload completion: CommandKey format is "log_{neLogId}".
				// Mirrors Java PositiveMessageProcessor.uploadLogSuccessfully (success)
				// and the TYPE_LOG failure branch. On success, retrieves the file name
				// from Redis (stored by the file upload endpoint) and updates ne_log
				// with log_name/generated_time/status=1/progress="100%". On failure,
				// sets status=3 and failure_reason.
				if strings.HasPrefix(tc.CommandKey, "log_") {
					neLogId, parseErr := strconv.ParseInt(tc.CommandKey[4:], 10, 64)
					if parseErr == nil {
						if success {
							var neLog struct {
								RequestId *string `gorm:"column:request_id"`
							}
							if dbErr := ep.db.Table("ne_log").Where("id = ?", neLogId).First(&neLog).Error; dbErr == nil {
								updates := map[string]interface{}{
									"generated_time": &now,
									"status":         1,
									"progress":       "100%",
								}
								if neLog.RequestId != nil && *neLog.RequestId != "" {
									fileName, _ := redis.Get(ctx, "LogFileName_"+*neLog.RequestId)
									if fileName != "" {
										updates["log_name"] = fileName
									}
								}
								ep.db.Table("ne_log").Where("id = ?", neLogId).Updates(updates)
							}
						} else {
							ep.db.Table("ne_log").Where("id = ?", neLogId).Updates(map[string]interface{}{
								"status":         3,
								"failure_reason": tc.FaultString,
							})
						}
					}
				}
			}

			// Check if this is a CONFIG upload/download operation.
			// Mirrors Java PositiveMessageProcessor.transferComplete.TYPE_CONFIG:
			//   - Download success: fire web callback CONFIG_DOWNLOADED
			//   - Upload success:  save file name to element.config_file,
			//                     delete Redis key ConfigFileName_{licenseId}_{neId},
			//                     fire web callback RECEIVE_CONFIG
			//   - OpenStation config success: mark element.open_station_config_status="success"
			if originEvent.EventType != nil {
				var trackData map[string]interface{}
				if originEvent.CommandTrackData != nil {
					_ = json.Unmarshal([]byte(*originEvent.CommandTrackData), &trackData)
				}

				isConfigOp := false
				isUpload := false
				var cpe device.CpeElement
				cpeOK := false
				if originEvent.ElementId != nil {
					cpeOK = ep.db.Where("ne_neid = ?", *originEvent.ElementId).First(&cpe).Error == nil
				}

				if originEvent.EventType != nil && *originEvent.EventType == "UPLOAD" {
					if v, ok := trackData["upload_type"].(string); ok && v == "config" {
						isConfigOp = true
						isUpload = true
					}
					if v, ok := trackData["file_type"].(string); ok && strings.Contains(strings.ToLower(v), "config") {
						isConfigOp = true
						isUpload = true
					}
				} else if originEvent.EventType != nil && *originEvent.EventType == "DOWNLOAD" {
					if v, ok := trackData["file_type"].(string); ok && strings.Contains(strings.ToLower(v), "config") {
						isConfigOp = true
					}
				}

				if isConfigOp && success && cpeOK {
					if isUpload {
						// Java: uploadConfigSuccessfully
						licenseId := 0
						if cpe.LicenseId != nil {
							licenseId = *cpe.LicenseId
						}
						redisKey := fmt.Sprintf("ConfigFileName_%d_%d", licenseId, cpe.NeNeid)
						fileName, _ := redis.Get(ctx, redisKey)
						if fileName != "" {
							ep.db.Model(&device.CpeElement{}).
								Where("ne_neid = ?", cpe.NeNeid).
								Updates(map[string]interface{}{
									"config_file":             fileName,
									"config_file_upload_time": &now,
								})
							redis.Del(ctx, redisKey)
						}

						// OpenStation config success
						if openStation, _ := trackData["open_station_config"].(bool); openStation {
							ep.db.Model(&device.CpeElement{}).
								Where("ne_neid = ?", cpe.NeNeid).
								Update("open_station_config_status", "success")
						}

						// Web callback RECEIVE_CONFIG (fire-and-forget).
						ep.sendWebCallback(ctx, 0, map[string]interface{}{
							"sn":          sn,
							"element_id":  cpe.NeNeid,
							"callback":    "RECEIVE_CONFIG",
							"event_log":   originEvent,
						})
					} else {
						// Java: downloadConfigSuccessfully
						ep.sendWebCallback(ctx, 0, map[string]interface{}{
							"sn":          sn,
							"element_id":  cpe.NeNeid,
							"callback":    "CONFIG_DOWNLOADED",
							"event_log":   originEvent,
						})
						if openStation, _ := trackData["open_station_config"].(bool); openStation {
							ep.db.Model(&device.CpeElement{}).
								Where("ne_neid = ?", cpe.NeNeid).
								Update("open_station_config_status", "success")
						}
					}
				}
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
	if eventLog.ElementId != nil {
		rebootKey := fmt.Sprintf("rebooting_%d", *eventLog.ElementId)
		if err := redis.Del(ctx, rebootKey); err != nil {
			logger.Warnf("failed to clear rebooting flag for %s: %v", sn, err)
		}
	}
}

// processReportTransmissionProgress handles an incoming
// ReportTransmissionProgress (CPE -> ACS) notification.
//
// Mirrors Java PositiveMessageProcessor.REPORT_TRANSMISSION_PROGRESS:
//   1. Parse CommandKey (format: "log_{eventLogId}") and ProgressPercentage.
//   2. If commandKey is non-empty and parses to an eventLogId, update
//      progress on the matching ne_log (DeviceLogFileLog) row.
//   3. Propagate the progress message to upgrade_log (via
//      event_log_id), manual_upgrade_log and capture_log tables
//      so that the operator UI displays real-time progress.
//
// The CPE expects an empty <ReportTransmissionProgressResponse/> body
// (already returned by soap.BuildReportTransmissionProgressResponse).
func (ep *EventProcessor) processReportTransmissionProgress(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing ReportTransmissionProgress for SN=%s", sn)

	eventLog.EventType = stringPtr("REPORT_TRANSMISSION_PROGRESS")
	eventLog.Status = intPtr(0)

	rtp, err := soap.ParseReportTransmissionProgress(soapXml)
	if err != nil {
		logger.Warnf("failed to parse ReportTransmissionProgress for SN=%s: %v", sn, err)
		eventLog.Status = intPtr(-1)
		eventLog.FaultInfo = stringPtr(err.Error())
		return
	}

	if rtp.CommandKey == "" {
		logger.Debugf("ReportTransmissionProgress: empty CommandKey for SN=%s", sn)
		return
	}

	// CommandKey format: "log_{eventLogId}" — match Java PositiveMessageProcessor.
	// split[0] = "log", split[1] = eventLogId.
	parts := strings.SplitN(rtp.CommandKey, "_", 2)
	if len(parts) != 2 {
		logger.Warnf("ReportTransmissionProgress: unexpected CommandKey format %q for SN=%s", rtp.CommandKey, sn)
		return
	}
	eventLogId, parseErr := strconv.ParseInt(parts[1], 10, 64)
	if parseErr != nil {
		logger.Warnf("ReportTransmissionProgress: failed to parse eventLogId from %q: %v", rtp.CommandKey, parseErr)
		return
	}

	progress := rtp.ProgressPercentage + "%"

	// 1. Update ne_log.progress (DeviceLogFileLog in Java).
	res := ep.db.Table("ne_log").
		Where("command_track_id = ?", eventLogId).
		Update("progress", progress)
	if res.Error != nil {
		logger.Warnf("ReportTransmissionProgress: failed to update ne_log progress for eventLogId=%d: %v", eventLogId, res.Error)
	}

	// 2. Propagate to upgrade_log.message via event_log_id.
	upgradeMsg := "download progress:" + progress
	ep.db.Table("upgrade_log").
		Where("event_log_id = ?", eventLogId).
		Update("message", upgradeMsg)

	// 3. Propagate to manual_upgrade_log.progress via event_log_id.
	ep.db.Table("manual_upgrade_log").
		Where("event_log_id = ?", eventLogId).
		Update("progress", progress)

	// 4. Propagate to capture_log.progress via event_log_id.
	ep.db.Table("capture_log").
		Where("event_log_id = ?", eventLogId).
		Update("progress", progress)
}

// processUpdateCBSDStatusResponse handles the response to UpdateCBSDStatus.
// It mirrors Java UpdateCBSDStatusProcessor.processResult — for each fault entry:
//   - FaultCode 9020 (bandwidth mismatch): disable the CBSD, store the preferred
//     bandwidth, set a Redis flag "cbsd_bandwidth_not_match_{id}", and trigger
//     relinquishment of the current grant.
//   - FaultCode 9021: reserved (no-op in Java as well).
func (ep *EventProcessor) processUpdateCBSDStatusResponse(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing UpdateCBSDStatusResponse for SN=%s", sn)

	eventLog.EventType = stringPtr("UPDATE_CBSD_STATUS_RESPONSE")
	eventLog.Status = intPtr(0)

	resp, err := soap.ParseUpdateCBSDStatusResponse(soapXml)
	if err != nil {
		logger.Warnf("failed to parse UpdateCBSDStatusResponse for SN=%s: %v", sn, err)
		eventLog.Status = intPtr(3)
		return
	}

	for _, fi := range resp.CBSDFaultInfos {
		switch fi.FaultCode {
		case 9020:
			logger.Warnf("UpdateCBSDStatus: bandwidth mismatch for CBSD %s (SN=%s), bw=%d",
				fi.CBSDSerialNumber, sn, fi.Bandwidth)

			var cpe device.CpeElement
			if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).
				Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
				logger.Warnf("UpdateCBSDStatus: cannot find CPE for SN=%s", sn)
				continue
			}

			var info struct {
				Id                 string  `gorm:"column:id"`
				Enable             *bool   `gorm:"column:enable"`
				CbsdID             *string `gorm:"column:cbsd_id"`
				GrantID            *string `gorm:"column:grant_id"`
				PreferredBandwidth *int    `gorm:"column:preferred_bandwidth"`
			}
			if err := ep.db.Table("cbsd_info").
				Where("serial_number = ? AND cbsd_serial_number = ? AND license_id = ?",
					sn, fi.CBSDSerialNumber, cpe.LicenseId).
				Take(&info).Error; err != nil {
				logger.Warnf("UpdateCBSDStatus: no CBSD info for SN=%s, cbsdSN=%s: %v",
					sn, fi.CBSDSerialNumber, err)
				continue
			}

			if info.Enable != nil && *info.Enable {
				falseVal := false
				bw := fi.Bandwidth
				ep.db.Table("cbsd_info").
					Where("id = ?", info.Id).
					Updates(map[string]interface{}{
						"enable":              &falseVal,
						"preferred_bandwidth": &bw,
					})

				redis.Set(ctx, "cbsd_bandwidth_not_match_"+info.Id, "yes", 0)

				if info.CbsdID != nil && info.GrantID != nil {
					logger.Infof("UpdateCBSDStatus: triggering relinquishment for CBSD %s (grant %s)",
						*info.CbsdID, *info.GrantID)
					utils.SafeGo("CbsdRelinquish-"+info.Id, func() {
						triggerCbsdRelinquishment(info.Id, *info.CbsdID, *info.GrantID)
					})
				}
			}

		case 9021:
			logger.Debugf("UpdateCBSDStatus: fault 9021 for CBSD %s (SN=%s)",
				fi.CBSDSerialNumber, sn)

		default:
			if fi.FaultCode != 0 {
				logger.Warnf("UpdateCBSDStatus: fault code %d for CBSD %s (SN=%s)",
					fi.FaultCode, fi.CBSDSerialNumber, sn)
			}
		}
	}
}

// triggerCbsdRelinquishment is a placeholder that will be wired to the CBSD
// service's Relinquishment method in a subsequent refactor. For now it logs
// the intent so operators know a relinquishment should follow.
func triggerCbsdRelinquishment(cbsdInfoId, cbsdId, grantId string) {
	logger.Infof("CBSD relinquishment triggered: id=%s cbsdId=%s grantId=%s "+
		"(wired to cbsd.Service.Relinquishment pending)", cbsdInfoId, cbsdId, grantId)
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

// parseMMLResultID extracts the numeric MML result id from an "mml:<id>" operation id.
func parseMMLResultID(opID string) int {
	if !strings.HasPrefix(opID, "mml:") {
		return 0
	}
	if n, err := strconv.Atoi(strings.TrimPrefix(opID, "mml:")); err == nil {
		return n
	}
	return 0
}
