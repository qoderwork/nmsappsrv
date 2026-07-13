package tr069

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nmsappsrv/internal/alarm"
	"nmsappsrv/internal/device"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/constants"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

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

// handleUpgradeFinish and handleUnitUpgradeResult are in event_helpers.go.

// handleStartupStage processes "105 STARTUP STAGE REPORT" event.
// Java: maps stage value to progress (1->3, 2->4, else->5), updates ZTP log.
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
		// Success - atomic: update ZTP log + create retry log
		if err := ep.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&ztpLog).Updates(map[string]interface{}{
				"progress": 6,
				"done":     true,
				"end_time": &now,
				"info":     info,
			}).Error; err != nil {
				return err
			}
			return tx.Create(&misc.ZTPRetryLog{
				ElementId: &cpe.NeNeid,
				RetryTime: &now,
				Info:      stringPtr("ZTP completed successfully"),
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
				ElementId: &cpe.NeNeid,
				RetryTime: &now,
				Info:      stringPtr(fmt.Sprintf("ZTP failed: %s", info)),
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

	// CPELockPCIPostprocessor: enforce PCI lock if configured
	pciLockKey := fmt.Sprintf("pci_lock_%s", sn)
	var lockedPCI string
	if err := ep.db.Table("system_config").Where("config_key = ?", pciLockKey).Limit(1).Scan(&lockedPCI).Error; err == nil && lockedPCI != "" {
		if lockedPCI != pci {
			logger.Warnf("device %s: PCI lock enforced - reported=%s, locked=%s, reverting via SetParameterValues", sn, pci, lockedPCI)
			utils.SafeGo("PCILock-"+sn, func() {
				msgMgr := NewMessageManager()
				opSender := NewOperationSender(ep.db, msgMgr)
				revertParams := []soap.ParameterValueStruct{
					{
						Name:  "Device.Services.FAPService.1.CellConfig.1.NR.RAN.RF.PhyCellID",
						Value: lockedPCI,
						Type:  "string",
					},
				}
				if err := opSender.SendSetParameterValues(sn, revertParams, "pci_lock_revert", "pci_lock_revert_"+sn); err != nil {
					logger.Errorf("device %s: failed to send PCI lock revert SPV: %v", sn, err)
				}
			})
		} else {
			logger.Infof("device %s: PCI lock check passed - reported PCI matches locked value %s", sn, lockedPCI)
		}
	}
}

// handleMMLReport processes "X 681D64 MMLREPORT" event.
// Java: matches Device.mml.CMDUID (uid) and the parameter whose NAME ends with
// "MMLReport" (result text), then fills result + result_returned_time.
// Per Java, the result-return uplink does NOT change status (status=3 is set by
// the SPV-response handler when the device ACKs the command).
func (ep *EventProcessor) handleMMLReport(ctx context.Context, sn string, paramMap map[string]string) {
	uid := paramMap["Device.mml.CMDUID"]
	if uid == "" {
		return
	}

	// The result text arrives on a parameter whose name ends with "MMLReport"
	// (e.g. Device.mml.MTN.MMLReport). Fall back to Device.mml.ReportValue.
	reportValue := ""
	for k, v := range paramMap {
		if strings.HasSuffix(k, "MMLReport") {
			reportValue = v
			break
		}
	}
	if reportValue == "" {
		reportValue = paramMap["Device.mml.ReportValue"]
	}
	logger.Infof("device %s: MML report uid=%s", sn, uid)

	// Update mml_execute_result by uid (result text + returned time only).
	now := time.Now()
	result := struct {
		ResultReturnedTime *time.Time `gorm:"column:result_returned_time"`
		Result             *string    `gorm:"column:result"`
	}{
		ResultReturnedTime: &now,
		Result:             stringPtr(reportValue),
	}

	if err := ep.db.Table("mml_execute_result").
		Where("uid = ?", uid).
		Updates(&result).Error; err != nil {
		logger.Warnf("device %s: failed to update MML result for uid=%s: %v", sn, uid, err)
	}

	// Send web callback for MML result
	callback := map[string]interface{}{
		"type":      "mml_result_returned",
		"sn":        sn,
		"uid":       uid,
		"timestamp": time.Now().Unix(),
	}
	if cbJson, err := json.Marshal(callback); err == nil {
		redis.Publish(ctx, "web_callback", string(cbJson))
	}

	// Clean up Redis key for this UID
	redis.Del(ctx, "mml:uid:"+uid)
}

// handleBatchResult processes "X E01CEE BATCHRESULT" event.
// Java: extracts batch status/failureCause, delegates to OpenStationPostprocessor.openStationDonePostprocessor.
func (ep *EventProcessor) handleBatchResult(ctx context.Context, sn string, paramMap map[string]string) {
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		return
	}

	status := paramMap["Device.Services.FAPService.1.FAPControl.SelfConfig.X_BBU_batch.Status"]
	failureCause := paramMap["Device.Services.FAPService.1.FAPControl.SelfConfig.X_BBU_batch.FailureCause"]

	success := status == "1"
	ep.handleOpenStationDone(cpe.NeNeid, success, failureCause)

	logger.Infof("device %s: batch result status=%s, success=%v", sn, status, success)
}

// handleBatchCheckResult processes "X E01CEE BATCHCHECKRESULT" event.
// Java: extracts check status/failureCause, delegates to OpenStationPostprocessor.checkDonePostprocessor.
func (ep *EventProcessor) handleBatchCheckResult(ctx context.Context, sn string, paramMap map[string]string) {
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		return
	}

	status := paramMap["Device.Services.FAPService.1.FAPControl.SelfConfig.X_BBU_batchCheck.Status"]
	failureCause := paramMap["Device.Services.FAPService.1.FAPControl.SelfConfig.X_BBU_batchCheck.FailureCause"]

	success := status == "1"
	ep.handleOpenStationCheckDone(cpe.NeNeid, success, failureCause)

	logger.Infof("device %s: batch check result status=%s, success=%v", sn, status, success)
}

// handleAlarmGenerated processes "101 ALARM" event (Java: AlarmGeneratedPostprocessor).
// Parses alarm fields from Inform parameters, creates or clears alarm records,
// and publishes alarm notifications via Redis pub/sub.
func (ep *EventProcessor) handleAlarmGenerated(ctx context.Context, sn string, paramMap map[string]string) {
	// Parse alarm fields from Inform parameter map
	alarmIdentifier := paramMap["Device.FaultMgmt.AlarmGenerated.AlarmIdentifier"]
	probableCause := paramMap["Device.FaultMgmt.AlarmGenerated.ProbableCause"]
	specificProblem := paramMap["Device.FaultMgmt.AlarmGenerated.SpecificProblem"]
	perceivedSeverity := paramMap["Device.FaultMgmt.AlarmGenerated.PerceivedSeverity"]
	eventTimeStr := paramMap["Device.FaultMgmt.AlarmGenerated.EventTime"]
	additionalText := paramMap["Device.FaultMgmt.AlarmGenerated.AdditionalText"]

	if alarmIdentifier == "" {
		logger.Warnf("device %s: ALARM event missing AlarmIdentifier, skipping", sn)
		return
	}

	// Map PerceivedSeverity (TR-069 integer) to severity string
	severity := mapPerceivedSeverity(perceivedSeverity)

	// Look up device by serial_number to get element_id and license_id
	var cpe device.CpeElement
	if err := ep.db.Where("serial_number = ? AND deleted = ?", sn, false).Limit(1).Find(&cpe).Error; err != nil || cpe.NeNeid == 0 {
		logger.Warnf("device %s: ALARM event device not found, skipping", sn)
		return
	}

	// Parse event_time from Inform parameter
	var eventTime *time.Time
	if eventTimeStr != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", eventTimeStr); err == nil {
			eventTime = &t
		} else if t, err := time.Parse("2006-01-02 15:04:05", eventTimeStr); err == nil {
			eventTime = &t
		}
	}

	now := time.Now()

	// Check if an active alarm already exists with same element_id + alarm_identifier + alarm_type=ACTIVE
	var existingAlarm alarm.Alarm
	activeType := alarm.AlarmTypeActive
	err := ep.db.Where("element_id = ? AND alarm_identifier = ? AND alarm_type = ?",
		cpe.NeNeid, alarmIdentifier, activeType).Limit(1).Find(&existingAlarm).Error

	isCleared := perceivedSeverity == "6"

	if err == nil && existingAlarm.Id != 0 {
		// Active alarm exists
		if isCleared {
			// Severity is "cleared" (6): update alarm_status to HISTORY, set cleared_time
			historyType := alarm.AlarmTypeHistory
			historyStatus := alarm.AlarmStatusHistoryUnconfirmed
			ep.db.Model(&existingAlarm).Updates(map[string]interface{}{
				"alarm_status": historyStatus,
				"alarm_type":   historyType,
				"cleared_time": &now,
				"update_time":  &now,
			})
			logger.Infof("device %s: alarm cleared, alarm_identifier=%s, alarm_id=%d", sn, alarmIdentifier, existingAlarm.Id)

			// Publish alarm clear notification via Redis pub/sub
			ep.publishAlarmNotify(ctx, existingAlarm.Id, cpe.NeNeid, severity, "cleared")
		} else {
			// Active alarm exists and not cleared: skip (dedup)
			logger.Debugf("device %s: duplicate alarm ignored, alarm_identifier=%s, severity=%s", sn, alarmIdentifier, severity)
		}
	} else if !isCleared {
		// No active alarm exists and not cleared: create new alarm record
		newAlarm := alarm.Alarm{
			ElementId:             &cpe.NeNeid,
			LicenseId:             cpe.LicenseId,
			AlarmIdentifier:       stringPtr(alarmIdentifier),
			ProbableCause:         stringPtr(probableCause),
			SpecificProblem:       stringPtr(specificProblem),
			Severity:              stringPtr(severity),
			AlarmId:               stringPtr(additionalText),
			AdditionalInformation: stringPtr(additionalText),
			AlarmSource:           stringPtr("TR069"),
			EventType:             stringPtr("COMMUNICATION_ALARM"),
			AlarmStatus:           intPtr(alarm.AlarmStatusActiveUnconfirmed),
			AlarmType:             intPtr(alarm.AlarmTypeActive),
			EventTime:             eventTime,
			CreateTime:            &now,
			UpdateTime:            &now,
		}

		if err := ep.db.Create(&newAlarm).Error; err != nil {
			logger.Errorf("device %s: failed to create alarm record: %v", sn, err)
			return
		}

		logger.Infof("device %s: new alarm created, alarm_identifier=%s, severity=%s, alarm_id=%d",
			sn, alarmIdentifier, severity, newAlarm.Id)

		// Publish alarm notify via Redis pub/sub
		ep.publishAlarmNotify(ctx, newAlarm.Id, cpe.NeNeid, severity, "raised")
	}
}

// publishAlarmNotify publishes an alarm event to Redis channel "alarm:notify"
// for the alarm notifier service to process.
func (ep *EventProcessor) publishAlarmNotify(ctx context.Context, alarmId int64, elementId int64, severity string, action string) {
	payload := map[string]interface{}{
		"alarm_id":   alarmId,
		"element_id": elementId,
		"severity":   severity,
		"action":     action,
		"timestamp":  time.Now().Unix(),
	}
	if payloadJson, err := json.Marshal(payload); err == nil {
		redis.Publish(ctx, "alarm:notify", string(payloadJson))
	} else {
		logger.Errorf("failed to marshal alarm notify payload: %v", err)
	}
}

// mapPerceivedSeverity maps TR-069 PerceivedSeverity integer string to severity name.
// 1=critical, 2=major, 3=minor, 4=warning, 5=indeterminate, 6=cleared
func mapPerceivedSeverity(code string) string {
	switch code {
	case "1":
		return "critical"
	case "2":
		return "major"
	case "3":
		return "minor"
	case "4":
		return "warning"
	case "5":
		return "indeterminate"
	case "6":
		return "cleared"
	default:
		return "indeterminate"
	}
}

// handleDeviceReconnected processes device reconnection (Inform after being offline).
// Java: DeviceReconnectedPostprocessor - clears stale state and triggers targeted parameter refresh.
func (ep *EventProcessor) handleDeviceReconnected(ctx context.Context, sn string, cpe *device.CpeElement) {
	// Check if the device was previously offline
	onlineKey := constants.RedisKeyDeviceOnline + sn
	wasOnline := false
	if val, err := redis.Get(ctx, onlineKey); err == nil && val == "1" {
		wasOnline = true
	}

	if wasOnline {
		// Device was already online, not a reconnection
		return
	}

	logger.Infof("device %s: reconnected from offline state (neId=%d)", sn, cpe.NeNeid)

	// (a) Clear stale parameter cache in Redis (keys matching param_cache:{neId}:*)
	cachePattern := fmt.Sprintf("param_cache:%d:*", cpe.NeNeid)
	if keys, err := redis.RDB.Keys(ctx, cachePattern).Result(); err == nil && len(keys) > 0 {
		if err := redis.Del(ctx, keys...); err != nil {
			logger.Warnf("device %s: failed to clear stale param cache: %v", sn, err)
		} else {
			logger.Infof("device %s: cleared %d stale param cache keys", sn, len(keys))
		}
	}

	// (b) Trigger targeted GPV for critical parameters
	utils.SafeGo("DeviceReconnected-"+sn, func() {
		msgMgr := NewMessageManager()
		opSender := NewOperationSender(ep.db, msgMgr)
		criticalParams := []string{
			"Device.DeviceInfo.SoftwareVersion",
			"Device.DeviceInfo.HardwareVersion",
			"Device.Services.FAPService.1.FAPControl.NR.CellConfig.",
		}
		if err := opSender.SendGetParameterValues(sn, criticalParams, "reconnect_gpv_"+sn); err != nil {
			logger.Errorf("device %s: failed to send reconnect GPV: %v", sn, err)
		}
	})

	// (c) Log the reconnection event
	logger.Infof("device %s: reconnection postprocessing complete, targeted GPV triggered", sn)
}
