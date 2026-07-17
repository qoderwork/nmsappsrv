package tr069

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// updateStun extracts STUN/UE/upgrade status from Inform parameters and caches
// to Redis. Mirrors Java InformMessageProcessor.updateStun().
//
// Java logic:
//  1. Build nameToValueMap from parameterList
//  2. Update device's STUN parameters (call GetConnectionServiceImpl.updateSTUN)
//  3. Cache UE count: Redis key "UE_COUNT_{neNeid}" with 70s TTL
//  4. Cache capture status: Redis key "status{neNeid}" with 70s TTL (if "true" or "1")
//  5. Cache device upgrade status: Redis key "device_upgrade_status_{neNeid}" (if IsUpgradingByNMS=="1")
func (ep *EventProcessor) updateStun(ctx context.Context, sn string, params []soap.ParameterValueStruct) {
	var cpeID int64
	if err := ep.db.Table("cpe_element").
		Select("ne_neid").
		Where("serial_number = ? AND deleted = ?", sn, false).
		Scan(&cpeID).Error; err != nil || cpeID == 0 {
		return
	}

	nameToValue := make(map[string]string, len(params))
	for _, p := range params {
		if p.Value != "" {
			nameToValue[p.Name] = p.Value
		}
	}

	// STUN update is handled by GetConnectionServiceImpl.updateSTUN
	// (rebuilds STUN configuration from the device's reported STUN params)
	if stunAddr := nameToValue["Device.ManagementServer.STUNServerAddress"]; stunAddr != "" {
		stunData := map[string]string{
			"STUNServerAddress": stunAddr,
		}
		if v := nameToValue["Device.ManagementServer.STUNServerPort"]; v != "" {
			stunData["STUNServerPort"] = v
		}
		if v := nameToValue["Device.ManagementServer.STUNUsername"]; v != "" {
			stunData["STUNUsername"] = v
		}
		if v := nameToValue["Device.ManagementServer.STUNPassword"]; v != "" {
			stunData["STUNPassword"] = v
		}
		if v := nameToValue["Device.ManagementServer.STUNEnable"]; v != "" {
			stunData["STUNEnable"] = v
		}
		stunJSON, _ := json.Marshal(stunData)
		redis.Set(ctx, "stun_config_"+itoa(cpeID), string(stunJSON), 24*time.Hour)
	}

	// UE count
	if s := nameToValue["Device.Services.FAPService.1.CellConfig.1.NR.RAN.UeNumber"]; s != "" {
		redis.Set(ctx, "UE_COUNT_"+itoa(cpeID), s, 70*time.Second)
	}

	// Capture status
	if status := nameToValue["Device.CaptureStatus"]; status == "true" || status == "1" {
		redis.Set(ctx, "status"+itoa(cpeID), status, 70*time.Second)
	} else {
		redis.Del(ctx, "status"+itoa(cpeID))
	}

	// Upgrade status
	if upgradeStatus := nameToValue["Device.SoftwareCtrl.IsUpgradingByNMS"]; upgradeStatus == "1" {
		redis.Set(ctx, "device_upgrade_status_"+itoa(cpeID), "yes", 24*time.Hour)
	} else {
		redis.Del(ctx, "device_upgrade_status_"+itoa(cpeID))
	}
}

// updateGPSOrWifiInfo extracts GPS / WiFi location from Inform parameters and
// caches to Redis with 1h TTL. Mirrors Java InformMessageProcessor.updateGPSOrWifiInfo().
//
// Java reads from LocationDTO.getLocationDTO(parameterList) which extracts
// GPS coordinates from Device.GPS.Latitude / Longitude or WiFi-based location.
func (ep *EventProcessor) updateGPSOrWifiInfo(ctx context.Context, sn string, params []soap.ParameterValueStruct) {
	var cpeID int64
	if err := ep.db.Table("cpe_element").
		Select("ne_neid").
		Where("serial_number = ? AND deleted = ?", sn, false).
		Scan(&cpeID).Error; err != nil || cpeID == 0 {
		return
	}

	location := buildLocationDTO(params)
	if location == nil {
		return
	}

	locJSON, err := json.Marshal(location)
	if err != nil {
		return
	}
	redis.Set(ctx, "device_geo_"+itoa(cpeID), string(locJSON), time.Hour)
}

// cacheBasicParameter caches a small set of hot path parameters (AMFPoolConfigParam.State
// and CellConfig.NR.RAN.OpState) to Redis for fast access. Mirrors Java
// InformMessageProcessor.cacheBasicParameter().
//
// Java logic: filter parameters matching Device.Services.FAPService.1.FAPControl.NR.AMFPoolConfigParam.{N}.State
// or .*\\.Services\\.FAPService\\.1\\.CellConfig\\.[0-9]+\\.NR\\.RAN\\.OpState
func (ep *EventProcessor) cacheBasicParameter(ctx context.Context, sn string, params []soap.ParameterValueStruct) {
	var cpeID int64
	if err := ep.db.Table("cpe_element").
		Select("ne_neid").
		Where("serial_number = ? AND deleted = ?", sn, false).
		Scan(&cpeID).Error; err != nil || cpeID == 0 {
		return
	}

	amfPoolRe := regexp.MustCompile(`Device\.Services\.FAPService\.1\.FAPControl\.NR\.AMFPoolConfigParam\.[0-9]+\.State`)
	ranOpRe := regexp.MustCompile(`.*\.Services\.FAPService\.1\.CellConfig\.[0-9]+\.NR\.RAN\.OpState`)

	type paramVO struct {
		ParamName  string `json:"paramName"`
		ParamValue string `json:"paramValue"`
	}
	var paramVOs []paramVO
	for _, p := range params {
		if amfPoolRe.MatchString(p.Name) || ranOpRe.MatchString(p.Name) {
			paramVOs = append(paramVOs, paramVO{ParamName: p.Name, ParamValue: p.Value})
		}
	}

	key := "cache_param_" + itoa(cpeID)
	if len(paramVOs) == 0 {
		redis.Del(ctx, key)
		return
	}
	data, err := json.Marshal(paramVOs)
	if err != nil {
		return
	}
	redis.Set(ctx, key, string(data), 70*time.Second)
}

// buildLocationDTO extracts GPS or WiFi location from a TR-069 parameter list.
// Returns nil if no usable coordinates are found.
//
// Mirrors Java LocationDTO.getLocationDTO which reads from
// Device.GPS.Latitude / Longitude (and optional accuracy).
// Falls back to WiFi-based location if GPS coordinates are absent.
func buildLocationDTO(params []soap.ParameterValueStruct) map[string]interface{} {
	var lat, lon, acc string
	var wifiBSSIDs []string
	for _, p := range params {
		switch p.Name {
		case "Device.GPS.Latitude", "Device.DeviceInfo.Location.Latitude":
			lat = p.Value
		case "Device.GPS.Longitude", "Device.DeviceInfo.Location.Longitude":
			lon = p.Value
		case "Device.GPS.Accuracy", "Device.DeviceInfo.Location.Accuracy":
			acc = p.Value
		}
		if len(p.Name) > 9 && p.Name[:9] == "Device.Wi" {
			wifiBSSIDs = append(wifiBSSIDs, p.Value)
		}
	}
	if lat == "" && lon == "" && len(wifiBSSIDs) == 0 {
		return nil
	}
	dto := map[string]interface{}{}
	if lat != "" {
		dto["latitude"] = lat
	}
	if lon != "" {
		dto["longitude"] = lon
	}
	if acc != "" {
		dto["accuracy"] = acc
	}
	if len(wifiBSSIDs) > 0 {
		dto["wifiBSSIDs"] = wifiBSSIDs
	}
	return dto
}

// itoa is a small wrapper around strconv.FormatInt(int64(v), 10) without
// pulling the strconv import into the helpers file.
func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// Compile-time guard so an unused import warning is never raised in a
// stripped-down build.
var _ = logger.Errorf
