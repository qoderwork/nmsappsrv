package tr069

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"nmsappsrv/internal/device"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/systemsettings"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/constants"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// buildParamMap converts a ParameterValueStruct slice to a name->value map for quick lookup.
func buildParamMap(params []soap.ParameterValueStruct) map[string]string {
	m := make(map[string]string, len(params))
	for _, p := range params {
		if p.Value != "" {
			m[p.Name] = p.Value
		}
	}
	return m
}

// updateDeviceOnlineStatus updates the online status of a device in Redis.
func (ep *EventProcessor) updateDeviceOnlineStatus(ctx context.Context, sn string, online bool) {
	key := constants.RedisKeyDeviceOnline + sn
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

// handleOpenStationDone updates BatchProcessFileSendLog status when open station completes.
// Java: OpenStationPostprocessor.openStationDonePostprocessor
// Finds the in-progress log (status=1) for the given element and updates it to
// status=2 (success) or status=3 (failure), setting fault_info accordingly.
func (ep *EventProcessor) handleOpenStationDone(elementId int64, success bool, faultInfo string) {
	var log misc.BatchProcessFileSendLog
	err := ep.db.Where("element_id = ? AND status = ?", elementId, 1).First(&log).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			logger.Errorf("handleOpenStationDone: failed to query log for element %d: %v", elementId, err)
		}
		return
	}

	var newStatus int
	if success {
		newStatus = 2 // success
	} else {
		newStatus = 3 // failure
	}

	updates := map[string]interface{}{
		"status":    newStatus,
		"fault_info": faultInfo,
	}
	if err := ep.db.Model(&log).Updates(updates).Error; err != nil {
		logger.Errorf("handleOpenStationDone: failed to update log %d for element %d: %v", log.Id, elementId, err)
	} else {
		logger.Infof("handleOpenStationDone: element %d, log %d updated to status=%d", elementId, log.Id, newStatus)
	}
}

// handleOpenStationCheckDone updates BatchProcessFileSendLog status when open station check completes.
// Java: OpenStationPostprocessor.checkDonePostprocessor
// Finds the in-progress log (status=1, check_file=true) for the given element and updates it to
// status=4 (check success) or status=5 (check failure), setting fault_info accordingly.
func (ep *EventProcessor) handleOpenStationCheckDone(elementId int64, success bool, faultInfo string) {
	var log misc.BatchProcessFileSendLog
	err := ep.db.Where("element_id = ? AND status = ? AND check_file = ?", elementId, 1, true).First(&log).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			logger.Errorf("handleOpenStationCheckDone: failed to query log for element %d: %v", elementId, err)
		}
		return
	}

	var newStatus int
	if success {
		newStatus = 4 // check success
	} else {
		newStatus = 5 // check failure
	}

	updates := map[string]interface{}{
		"status":    newStatus,
		"fault_info": faultInfo,
	}
	if err := ep.db.Model(&log).Updates(updates).Error; err != nil {
		logger.Errorf("handleOpenStationCheckDone: failed to update log %d for element %d: %v", log.Id, elementId, err)
	} else {
		logger.Infof("handleOpenStationCheckDone: element %d, log %d updated to status=%d", elementId, log.Id, newStatus)
	}
}
