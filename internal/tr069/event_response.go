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

	// Java: WebMessageProcessor.dealWebGetParamValues → VALUE_GET callback
	// (cpe already declared above in this function)
	ep.sendWebCallback(ctx, 0, map[string]interface{}{
		"sn":         sn,
		"element_id": cpe.NeNeid,
		"callback":   "VALUE_GET",
		"event_log":  eventLog,
	})
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

	// Java: WebMessageProcessor.dealWebSetParamValues → VALUE_SET callback
	var cpe device.CpeElement
	if eventLog.ElementId != nil {
		if ep.db.Where("ne_neid = ?", *eventLog.ElementId).First(&cpe).Error == nil {
			ep.sendWebCallback(ctx, 0, map[string]interface{}{
				"sn":         sn,
				"element_id": cpe.NeNeid,
				"callback":   "VALUE_SET",
				"event_log":  eventLog,
			})
		}
	}

	// Java: WebMessageProcessor.dealWebSetParamValues → clean up PresetParametersTask
	if eventLog.CommandTrackData != nil {
		var trackData map[string]interface{}
		if json.Unmarshal([]byte(*eventLog.CommandTrackData), &trackData) == nil {
			if cti, ok := trackData["command_track_id"].(float64); ok {
				ep.db.Table("preset_parameters_task").
					Where("event_log_id = ?", int64(cti)).
					Updates(map[string]interface{}{
						"status": 2,
					})
			}
		}
	}

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

				// Java: transferUncompleted - TYPE_UPGRADE failure updates manual_upgrade_log
				if !success {
					faultString := fmt.Sprintf("Fault Code:%d, Fault Info:%s", tc.FaultCode, tc.FaultString)
					ep.db.Table("manual_upgrade_log").
						Where("event_log_id = ?", originEvent.Id).
						Updates(map[string]interface{}{
							"success":  false,
							"end_time": &now,
							"info":     faultString,
						})
				}

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

				// Java: transferUncompleted - faultCode 9003/9001 → changeFileVersion
				if !success && (tc.FaultCode == 9003 || tc.FaultCode == 9001) {
					if originEvent.CommandTrackData != nil {
						var trackData map[string]interface{}
						if json.Unmarshal([]byte(*originEvent.CommandTrackData), &trackData) == nil {
							if fileType, ok := trackData["file_type"].(string); ok {
								ep.changeFileVersion(ctx, sn, fileType)
							}
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

				if isConfigOp {
					if success && cpeOK {
						if isUpload {
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

							if openStation, _ := trackData["open_station_config"].(bool); openStation {
								ep.db.Model(&device.CpeElement{}).
									Where("ne_neid = ?", cpe.NeNeid).
									Update("open_station_config_status", "success")
							}

							ep.sendWebCallback(ctx, 0, map[string]interface{}{
								"sn":          sn,
								"element_id":  cpe.NeNeid,
								"callback":    "RECEIVE_CONFIG",
								"event_log":   originEvent,
							})
						} else {
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
					} else {
						// Java: transferUncompleted - CONFIG failure
						if isUpload {
							ep.sendWebCallback(ctx, 0, map[string]interface{}{
								"sn":          sn,
								"element_id":  cpe.NeNeid,
								"callback":    "UPLOAD_CONFIG_FAILED",
								"event_log":   originEvent,
							})
						} else {
							ep.sendWebCallback(ctx, 0, map[string]interface{}{
								"sn":          sn,
								"element_id":  cpe.NeNeid,
								"callback":    "DOWNLOAD_CONFIG_FAILED",
								"event_log":   originEvent,
							})
							if openStation, _ := trackData["open_station_config"].(bool); openStation {
								ep.db.Model(&device.CpeElement{}).
									Where("ne_neid = ?", cpe.NeNeid).
									Update("open_station_config_status", "failed")
							}
						}
					}
				}

				// Java: PositiveMessageProcessor.transferComplete - additional file types
				if fileType, ok := trackData["file_type"].(string); ok {
					if success {
						switch fileType {
						case "CERT_FILE":
							ep.db.Table("device_send_ca_log").
								Where("event_log_id = ?", originEvent.Id).
								Update("result", 1)
						case "ZTP_FILE":
							ep.db.Table("ztp_log").
								Where("event_log_id = ?", originEvent.Id).
								Update("progress", 2)
						case "BATCH_UPGRADE_FILE":
							ep.db.Table("eu_and_ru_batch_upgrade_log").
								Where("event_log_id = ?", originEvent.Id).
								Update("downloaded_time", &now)
						case "BATCH_PROCESS_FILE":
							ep.db.Table("batch_process_file_send_log").
								Where("command_track_id = ?", originEvent.Id).
								Updates(map[string]interface{}{
									"download_time": &now,
									"status":        1,
								})
						case "RRU_SOFTWARE":
							if cpeOK {
								ep.sendWebCallback(ctx, 0, map[string]interface{}{
									"sn":          sn,
									"element_id":  cpe.NeNeid,
									"callback":    "RRU_SOFTWARE_DOWNLOADED",
									"event_log":   originEvent,
								})
							}
						case "CBSD_CERT_FILE":
							ep.db.Table("send_cbsd_cert_file_log").
								Where("event_log_id = ?", originEvent.Id).
								Updates(map[string]interface{}{
									"status":    1,
									"end_time":  &now,
								})
						}
					} else {
						// Failure branches for each file type
						faultString := fmt.Sprintf("Fault Code:%d, Fault Info:%s", tc.FaultCode, tc.FaultString)
						switch fileType {
						case "CERT_FILE":
							ep.db.Table("device_send_ca_log").
								Where("event_log_id = ?", originEvent.Id).
								Updates(map[string]interface{}{
									"result": 2,
									"info":   faultString,
								})
						case "ZTP_FILE":
							// Java: ZTP failure (non-9004) → retry log + cleanup
							if tc.FaultCode != 9004 && cpeOK {
								ep.db.Table("ztp_retry_log").Create(map[string]interface{}{
									"element_id": cpe.NeNeid,
									"retry_time": &now,
									"info":       "Failed to download",
								})
								utils.SafeGo("ZTPFailedCleanup-"+sn, func() {
									ep.ztpFailedCleanup(ctx, sn, cpe.NeNeid)
								})
							}
						case "BATCH_UPGRADE_FILE":
							ep.db.Table("eu_and_ru_batch_upgrade_log").
								Where("event_log_id = ?", originEvent.Id).
								Updates(map[string]interface{}{
									"result":     2,
									"fault_info": faultString,
								})
						case "BATCH_PROCESS_FILE":
							ep.db.Table("batch_process_file_send_log").
								Where("command_track_id = ?", originEvent.Id).
								Updates(map[string]interface{}{
									"status":   3,
									"fault_info": faultString,
								})
						case "CBSD_CERT_FILE":
							ep.db.Table("send_cbsd_cert_file_log").
								Where("event_log_id = ?", originEvent.Id).
								Updates(map[string]interface{}{
									"status":   2,
									"fault_info": faultString,
								})
						case "M_NORMAL_FILE":
							ep.db.Table("m_normal_file_send_log").
								Where("command_track_id = ?", originEvent.Id).
								Updates(map[string]interface{}{
									"status":   2,
									"fault_info": faultString,
								})
						}
					}
				}
			}
		} else {
			logger.Debugf("TransferComplete CommandKey=%s: no matching origin event_log for SN=%s", tc.CommandKey, sn)
		}
	}
}

// processDownloadResponse handles DownloadResponse from CPE.
// Mirrors Java WebMessageProcessor.dealDownload:
//   - status 0: download accepted → CONFIG_DOWNLOAD or UPGRADE_DOWNLOAD callback
//   - status 1: download in progress → CONFIG_DOWNLOADING or UPGRADE_DOWNLOADING callback
//   - other: download failed → CONFIG_DOWNLOAD_EXPIRE or UPGRADE_DOWNLOAD_EXPIRE callback
//     + ZTP_FILE failure triggers retry log and cleanup
func (ep *EventProcessor) processDownloadResponse(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing DownloadResponse for SN=%s", sn)

	eventLog.EventType = stringPtr("DOWNLOAD_RESPONSE")

	resp, err := soap.ParseDownloadResponse(soapXml)
	if err != nil {
		logger.Warnf("failed to parse DownloadResponse for SN=%s: %v", sn, err)
		eventLog.Status = intPtr(-1)
		eventLog.FaultInfo = stringPtr(err.Error())
		return
	}

	eventLog.Status = intPtr(resp.Status)

	// Look up device and track data to determine callback type
	var trackData map[string]interface{}
	if eventLog.CommandTrackData != nil {
		_ = json.Unmarshal([]byte(*eventLog.CommandTrackData), &trackData)
	}

	var cpe device.CpeElement
	cpeOK := false
	if eventLog.ElementId != nil {
		cpeOK = ep.db.Where("ne_neid = ?", *eventLog.ElementId).First(&cpe).Error == nil
	}

	// Determine file type from track data
	fileType := ""
	if trackData != nil {
		if ft, ok := trackData["file_type"].(string); ok {
			fileType = ft
		}
	}

	// Determine if this is a config or upgrade download
	isConfig := strings.Contains(strings.ToLower(fileType), "config")
	isUpgrade := strings.Contains(strings.ToLower(fileType), "upgrade") || (!isConfig && fileType != "")
	isZTP := fileType == "ZTP_FILE"

	var callbackType string
	switch resp.Status {
	case 0: // accepted / not started yet
		if isConfig {
			callbackType = "CONFIG_DOWNLOAD"
		} else if isUpgrade {
			callbackType = "UPGRADE_DOWNLOAD"
		}
	case 1: // in progress
		if isConfig {
			callbackType = "CONFIG_DOWNLOADING"
		} else if isUpgrade {
			callbackType = "UPGRADE_DOWNLOADING"
		}
	default: // failed
		if isConfig {
			callbackType = "CONFIG_DOWNLOAD_EXPIRE"
		} else if isUpgrade {
			callbackType = "UPGRADE_DOWNLOAD_EXPIRE"
		}
		// Java: ZTP_FILE download failure → add retry log + delete ZTP file
		if isZTP && cpeOK {
			now := time.Now()
			ep.db.Table("ztp_retry_log").Create(map[string]interface{}{
				"element_id": cpe.NeNeid,
				"retry_time": &now,
				"info":       "Failed to download",
			})
			// Trigger ZTP failed cleanup (delete AOS file, geo cache, ZTP log)
			utils.SafeGo("ZTPFailedCleanup-"+sn, func() {
				ep.ztpFailedCleanup(ctx, sn, cpe.NeNeid)
			})
		}
	}

	if callbackType != "" && cpeOK {
		ep.sendWebCallback(ctx, 0, map[string]interface{}{
			"sn":          sn,
			"element_id":  cpe.NeNeid,
			"callback":    callbackType,
			"status":      resp.Status,
			"event_log":   eventLog,
		})
	}
}

// processUploadResponse handles UploadResponse from CPE.
// Mirrors Java WebMessageProcessor.dealWebUpload:
//   - TYPE_LOG → UPLOAD_LOG_SENT callback + CaptureLog.status=1
//   - TYPE_CONFIG → UPLOAD_CONFIG_SENT callback + CaptureLog.status=1
func (ep *EventProcessor) processUploadResponse(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing UploadResponse for SN=%s", sn)

	eventLog.EventType = stringPtr("UPLOAD_RESPONSE")
	eventLog.Status = intPtr(0)

	resp, err := soap.ParseUploadResponse(soapXml)
	if err != nil {
		logger.Warnf("failed to parse UploadResponse for SN=%s: %v", sn, err)
		eventLog.Status = intPtr(-1)
		eventLog.FaultInfo = stringPtr(err.Error())
		return
	}

	eventLog.Status = intPtr(resp.Status)

	// Look up device and track data to determine callback type
	var trackData map[string]interface{}
	if eventLog.CommandTrackData != nil {
		_ = json.Unmarshal([]byte(*eventLog.CommandTrackData), &trackData)
	}

	var cpe device.CpeElement
	cpeOK := false
	if eventLog.ElementId != nil {
		cpeOK = ep.db.Where("ne_neid = ?", *eventLog.ElementId).First(&cpe).Error == nil
	}

	// Determine upload type from track data
	var callbackType string
	if trackData != nil {
		uploadType := ""
		if ut, ok := trackData["upload_type"].(string); ok {
			uploadType = ut
		}
		fileType := ""
		if ft, ok := trackData["file_type"].(string); ok {
			fileType = ft
		}

		switch {
		case uploadType == "log" || strings.Contains(strings.ToLower(fileType), "log"):
			callbackType = "UPLOAD_LOG_SENT"
		case uploadType == "config" || strings.Contains(strings.ToLower(fileType), "config"):
			callbackType = "UPLOAD_CONFIG_SENT"
		}
	}

	if callbackType != "" && cpeOK {
		ep.sendWebCallback(ctx, 0, map[string]interface{}{
			"sn":         sn,
			"element_id": cpe.NeNeid,
			"callback":   callbackType,
			"event_log":  eventLog,
		})
	}

	// Java: dealWebUpload also updates CaptureLog.status=1 if found by event_log_id
	if eventLog.Id != 0 {
		result := ep.db.Table("capture_log").
			Where("event_log_id = ?", eventLog.Id).
			Update("status", 1)
		if result.RowsAffected > 0 {
			logger.Infof("UploadResponse: updated capture_log status for event_log_id=%d", eventLog.Id)
		}
	}
}

// processBatchUpgradeResponse handles BatchUpgradeResponse from CPE.
// Mirrors Java WebMessageProcessor.dealBatchUpgrade:
//   - On failure (status != 0), updates eu_and_ru_batch_upgrade_log.result=2 and fault_info
func (ep *EventProcessor) processBatchUpgradeResponse(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing BatchUpgradeResponse for SN=%s", sn)

	eventLog.EventType = stringPtr("BATCH_UPGRADE_RESPONSE")
	eventLog.Status = intPtr(0)

	resp, err := soap.ParseBatchUpgradeResponse(soapXml)
	if err != nil {
		logger.Warnf("failed to parse BatchUpgradeResponse for SN=%s: %v", sn, err)
		eventLog.Status = intPtr(-1)
		return
	}

	if resp.Status != 0 {
		eventLog.Status = intPtr(resp.Status)
		eventLog.FaultInfo = stringPtr(resp.FailCase)

		// Update batch upgrade log with failure info
		var trackData map[string]interface{}
		if eventLog.CommandTrackData != nil {
			_ = json.Unmarshal([]byte(*eventLog.CommandTrackData), &trackData)
		}
		cti := int64(0)
		if trackData != nil {
			if c, ok := trackData["command_track_id"].(float64); ok {
				cti = int64(c)
			}
		}
		if cti == 0 {
			cti = int64(eventLog.Id)
		}
		ep.db.Table("eu_and_ru_batch_upgrade_log").
			Where("event_log_id = ?", cti).
			Updates(map[string]interface{}{
				"result":    2,
				"fault_info": resp.FailCase,
			})
		// Also update event_log fault string
		ep.db.Table("event_log").
			Where("id = ?", eventLog.Id).
			Update("fault_info", resp.FailCase)
	}
}

// processCancelFutureUpgradeResponse handles CancelFutureUpgradeResponse from CPE.
// Mirrors Java WebMessageProcessor.dealUpgradeCancel:
//   - Parses CommandKey (format: "{taskId}_{eventLogId}")
//   - Updates the corresponding upgrade_log to cancelled state
func (ep *EventProcessor) processCancelFutureUpgradeResponse(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing CancelFutureUpgradeResponse for SN=%s", sn)

	eventLog.EventType = stringPtr("CANCEL_FUTURE_UPGRADE_RESPONSE")
	eventLog.Status = intPtr(0)

	resp, err := soap.ParseCancelFutureUpgradeResponse(soapXml)
	if err != nil {
		logger.Warnf("failed to parse CancelFutureUpgradeResponse for SN=%s: %v", sn, err)
		eventLog.Status = intPtr(-1)
		return
	}

	if resp.CommandKey == "" {
		return
	}

	// CommandKey format: "{taskId}_{eventLogId}" — parse the event_log_id part
	parts := strings.SplitN(resp.CommandKey, "_", 2)
	if len(parts) != 2 {
		logger.Warnf("CancelFutureUpgradeResponse: unexpected CommandKey format %q", resp.CommandKey)
		return
	}
	originEventLogId, parseErr := strconv.ParseInt(parts[1], 10, 64)
	if parseErr != nil {
		logger.Warnf("CancelFutureUpgradeResponse: failed to parse eventLogId from %q: %v", resp.CommandKey, parseErr)
		return
	}

	// Update upgrade_log to cancelled state
	now := time.Now()
	done := true
	success := false
	ep.db.Table("upgrade_log").
		Where("command_track_id = ?", originEventLogId).
		Updates(map[string]interface{}{
			"is_done":   &done,
			"done_time": &now,
			"success":   &success,
			"message":   "Upgrade canceled",
		})

	logger.Infof("CancelFutureUpgradeResponse: cancelled upgrade_log for command_track_id=%d", originEventLogId)
}

// processCaptureResponse handles CaptureResponse from CPE.
// Mirrors Java WebMessageProcessor.dealCapture:
//   - Updates capture_log.status=1 if found by event_log_id
func (ep *EventProcessor) processCaptureResponse(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing CaptureResponse for SN=%s", sn)

	eventLog.EventType = stringPtr("CAPTURE_RESPONSE")
	eventLog.Status = intPtr(0)

	// Look up the originating event_log_id from track data
	var trackData map[string]interface{}
	if eventLog.CommandTrackData != nil {
		_ = json.Unmarshal([]byte(*eventLog.CommandTrackData), &trackData)
	}
	cti := int64(0)
	if trackData != nil {
		if c, ok := trackData["command_track_id"].(float64); ok {
			cti = int64(c)
		}
	}
	if cti == 0 {
		cti = int64(eventLog.Id)
	}

	ep.db.Table("capture_log").
		Where("event_log_id = ?", cti).
		Update("status", 1)
}

// processFactoryResetResponse handles FactoryResetResponse from CPE.
// Mirrors Java WebMessageProcessor.dealWebFactoryReset → DEVICE_RESET callback.
func (ep *EventProcessor) processFactoryResetResponse(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing FactoryResetResponse for SN=%s", sn)

	eventLog.EventType = stringPtr("FACTORY_RESET_RESPONSE")
	eventLog.Status = intPtr(0)

	var cpe device.CpeElement
	if eventLog.ElementId != nil {
		if ep.db.Where("ne_neid = ?", *eventLog.ElementId).First(&cpe).Error == nil {
			ep.sendWebCallback(ctx, 0, map[string]interface{}{
				"sn":         sn,
				"element_id": cpe.NeNeid,
				"callback":   "DEVICE_RESET",
				"event_log":  eventLog,
			})
		}
	}
}

// ztpFailedCleanup performs ZTP failure cleanup: delete AOS file, geo cache, ZTP log.
// Mirrors Java ZTPFailedPostProcessor.deleteZTPFile.
func (ep *EventProcessor) ztpFailedCleanup(ctx context.Context, sn string, neId int64) {
	logger.Infof("ZTP failed cleanup for SN=%s neId=%d", sn, neId)
	// Clear ZTP-related Redis keys
	redis.Del(ctx, fmt.Sprintf("ztp_status_%d", neId))
	redis.Del(ctx, fmt.Sprintf("ztp_progress_%d", neId))
	// Update ztp_log to failed
	ep.db.Table("ztp_log").
		Where("element_id = ? AND done = ?", neId, false).
		Updates(map[string]interface{}{
			"done":      true,
			"has_fault": true,
			"info":      "Failed to download",
		})
}

// processHttpRequestProxyResponse handles HttpRequestProxyResponse from CPE.
// Mirrors Java WebMessageProcessor.dealHttpRequestProxyResonse:
//   - Parses HTTP responses from CPE proxy
//   - Updates core_network_data fields based on requestId prefix and type
//   - Updates core_network_operation_log result and response_time
func (ep *EventProcessor) processHttpRequestProxyResponse(ctx context.Context, soapXml string, sn string, eventLog *eventlog.EventLog) {
	logger.Infof("processing HttpRequestProxyResponse for SN=%s", sn)

	eventLog.EventType = stringPtr("HTTP_REQUEST_PROXY_RESPONSE")
	eventLog.Status = intPtr(0)

	resp, err := soap.ParseHttpRequestProxyResponse(soapXml)
	if err != nil {
		logger.Warnf("failed to parse HttpRequestProxyResponse for SN=%s: %v", sn, err)
		eventLog.Status = intPtr(-1)
		return
	}

	// Look up device and CoreNetwork data
	var cpe device.CpeElement
	if eventLog.ElementId == nil {
		return
	}
	if ep.db.Where("ne_neid = ?", *eventLog.ElementId).First(&cpe).Error != nil {
		logger.Warnf("HttpRequestProxyResponse: device not found for SN=%s", sn)
		return
	}

	// Find CoreNetwork entry for this device
	var coreNet struct {
		Id int `gorm:"column:id"`
	}
	if ep.db.Table("core_network").
		Where("element_id = ? AND deleted = ?", cpe.NeNeid, false).
		Take(&coreNet).Error != nil {
		logger.Debugf("HttpRequestProxyResponse: no CoreNetwork for element_id=%d", cpe.NeNeid)
		return
	}

	// Find CoreNetworkData
	var cnd struct {
		Id int `gorm:"column:id"`
	}
	if ep.db.Table("core_network_data").
		Where("core_network_id = ?", coreNet.Id).
		Take(&cnd).Error != nil {
		logger.Debugf("HttpRequestProxyResponse: no CoreNetworkData for core_network_id=%d", coreNet.Id)
		return
	}

	// Process each HTTP response
	hasFaultInWholeMessage := false
	updates := map[string]interface{}{}
	now := time.Now()

	for _, httpResp := range resp.Responses {
		hasFault := httpResp.Status == 1 || !strings.HasPrefix(httpResp.ResponseCode, "2")
		if hasFault {
			hasFaultInWholeMessage = true
		}

		requestId := httpResp.RequestId
		if strings.HasPrefix(requestId, "query:") {
			// Format: "query:{TYPE}" — map type to core_network_data column
			parts := strings.SplitN(requestId, ":", 2)
			if len(parts) == 2 {
				colName := coreNetworkDataTypeToColumn(parts[1])
				if colName != "" {
					updates[colName] = httpResp.Body
				}
			}
		}

		// Update core_network_operation_log
		result := 1
		if hasFault {
			result = 2
		}
		ep.db.Table("core_network_operation_log").
			Where("event_log_id = ? AND request_id = ?", eventLog.Id, requestId).
			Updates(map[string]interface{}{
				"result":        result,
				"response_time": &now,
			})
	}

	// Apply updates to core_network_data
	if len(updates) > 0 {
		ep.db.Table("core_network_data").
			Where("id = ?", cnd.Id).
			Updates(updates)
	}

	// Check for PCF UE operations and update core_network_operation_log result
	var opLog struct {
		Id      int64  `gorm:"column:id"`
		LogType string `gorm:"column:log_type"`
	}
	ep.db.Table("core_network_operation_log").
		Where("event_log_id = ?", eventLog.Id).
		Take(&opLog)
	if opLog.Id != 0 {
		switch opLog.LogType {
		case "Add PCF UE", "Modify PCF UE", "Delete PCF UE":
			result := 1
			if hasFaultInWholeMessage {
				result = 2
			}
			ep.db.Table("core_network_operation_log").
				Where("id = ?", opLog.Id).
				Updates(map[string]interface{}{
					"result":        result,
					"response_time": &now,
				})
		}
	}

	if hasFaultInWholeMessage {
		eventLog.Status = intPtr(3)
	}
}

// changeFileVersion mirrors Java PositiveMessageProcessor.changeFileVersion:
// toggles the device's new_version flag and sends a CHANGE_FILE_VERSION callback.
func (ep *EventProcessor) changeFileVersion(ctx context.Context, sn string, fileType string) {
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).First(&cpe).Error; err != nil {
		logger.Warnf("changeFileVersion: device not found for SN=%s: %v", sn, err)
		return
	}

	newVersion := !cpe.IsNewVersion

	ep.db.Model(&device.CpeElement{}).
		Where("ne_neid = ?", cpe.NeNeid).
		Update("is_new_version", newVersion)

	logger.Infof("changeFileVersion: toggled new_version to %v for SN=%s", newVersion, sn)

	ep.sendWebCallback(ctx, 0, map[string]interface{}{
		"sn":          sn,
		"element_id":  cpe.NeNeid,
		"callback":    "CHANGE_FILE_VERSION",
		"file_type":   fileType,
	})
}

// coreNetworkDataTypeToColumn maps Java CoreNetworkRequestDataType enum values
// to Go core_network_data table column names.
func coreNetworkDataTypeToColumn(dataType string) string {
	mapping := map[string]string{
		"AMF_INFO":              "amf_info",
		"IMS_INFO":              "ims_info",
		"PCF_INFO":              "pcf_info",
		"SMF_INFO":              "smf_info",
		"UDM_INFO":              "udm_info",
		"UPF_INFO":              "upf_info",
		"AUSF_INFO":             "ausf_info",
		"SMF_ALARM":             "smf_alarm",
		"UDM_ALARM":             "udm_alarm",
		"IMS_UE_INFO":           "ims_ue_info",
		"PCF_UE_INFO":           "pcf_ue_info",
		"SMF_UE_INFO":           "smf_ue_info",
		"IMS_UE_NUMBER":         "ims_ue_number",
		"SMF_UE_NUMBER":         "smf_ue_number",
		"CONFIG_AMF_TAI":        "config_amf_tai",
		"CONFIG_AMF_SYSTEM":     "config_amf_system",
		"CONFIG_IMS_SYSTEM":     "config_ims_system",
		"CONFIG_AMF_SLICE":      "config_amf_slice",
		"CONFIG_UDM_SYSTEM":     "config_udm_system",
		"CONFIG_AUSF_SYSTEM":    "config_ausf_system",
		"CONFIG_PCF_SYSTEM":     "config_pcf_system",
		"AMF_BASE_STATION_INFO": "amf_base_station_info",
	}
	if col, ok := mapping[dataType]; ok {
		return col
	}
	return ""
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

		// Java: WebMessageProcessor.dealWebReboot → REBOOT callback
		var cpe device.CpeElement
		if ep.db.Where("ne_neid = ?", *eventLog.ElementId).First(&cpe).Error == nil {
			ep.sendWebCallback(ctx, 0, map[string]interface{}{
				"sn":         sn,
				"element_id": cpe.NeNeid,
				"callback":   "REBOOT",
				"event_log":  eventLog,
			})
		}

		// Java: after license update + soft reboot → check and send DEVICE_UPDATE_LICENSE callback
		// if trackData contains "license_update" flag
		if eventLog.CommandTrackData != nil {
			var trackData map[string]interface{}
			if json.Unmarshal([]byte(*eventLog.CommandTrackData), &trackData) == nil {
				if licenseUpdate, _ := trackData["license_update"].(bool); licenseUpdate {
					ep.sendWebCallback(ctx, 0, map[string]interface{}{
						"sn":         sn,
						"element_id": cpe.NeNeid,
						"callback":   "DEVICE_UPDATE_LICENSE",
						"event_log":  eventLog,
					})
				}
			}
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

	// Java: MessageTrackPostProcessor.faultMessageReceivedAfterPostProcessor
	// Save ParameterLog for SPV failures and send MESSAGE_RECEIVED callback
	if operationType == "SET_PARAMETER_VALUES" && trackData != nil {
		if paramsJson, ok := trackData["parameters"].(string); ok && paramsJson != "" {
			var paramList []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			}
			if err := json.Unmarshal([]byte(paramsJson), &paramList); err == nil {
				var cpe device.CpeElement
				if ep.db.Where("serial_number = ? AND deleted = ?", sn, false).First(&cpe).Error == nil {
					for _, p := range paramList {
						ep.db.Table("parameter_log").Create(map[string]interface{}{
							"parameter_name": p.Name,
							"element_id":     cpe.NeNeid,
							"new_value":      p.Value,
							"change_time":    time.Now(),
							"has_fault":      true,
							"fault_msg":      faultDesc,
						})
					}
				}
			}
		}
	}

	// Send MESSAGE_RECEIVED callback for fault
	var cpe device.CpeElement
	if ep.db.Where("serial_number = ? AND deleted = ?", sn, false).First(&cpe).Error == nil {
		callbackData := map[string]interface{}{
			"sn":         sn,
			"element_id": cpe.NeNeid,
			"callback":   "MESSAGE_RECEIVED",
			"has_fault":  true,
			"fault_code": resp.FaultCode,
			"message":    faultDesc,
			"event_log":  eventLog,
		}
		if operationType != "" {
			callbackData["operation_type"] = operationType
		}
		ep.sendWebCallback(ctx, 0, callbackData)
	}

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
