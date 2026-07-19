// Package ztp implements the ZTP (Zero Touch Provisioning) external-system
// registration orchestrator — the Go equivalent of Java's
// GenerateZTPFileThread. For each ready device it validates the ZTP setting,
// reverse-geocodes the device location (PSAP + geofence), allocates a gNB-ID
// and TAC, ensures the AOS pull-file exists, pushes the cell to the external
// E911 systems (MSAG / BMC old+new / LMF 1–4 / GMLC) via the external package,
// records the registration in cpe_element.e911_data, and rolls everything back
// on failure.
//
// Phase 2a scope: the full orchestration / state machine / allocation /
// geofence logic is implemented here. The external calls are gated by their
// enable-state and routed through external.Transport. Phase 2b Increment ①/②
// added the real mTLS wire transport (PKCS12 client certs, per-system
// transports, LMF X-Auth-Token session flow); Increment ③ wired the KML/CSV
// core-file parsing: the geofence polygon selects the market + ARFCN and the
// matching CORE_NR_FEMTO record supplies the PLMN (MCC/MNC) + PCI, replacing
// the Phase 2a hardcoded 310/260 default (which remains the safe fallback).
// With an empty ZTP setting the orchestrator refuses to start a device
// (validateZTPSettings fails) but the existing AOS-generation path
// (ztp-aos-gen cron + tr069 worker) continues unaffected.
package ztp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"nmsappsrv/internal/alarm"
	"nmsappsrv/internal/device"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/ztp/external"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// defaultMCC/defaultMNC are the fallback PLMN identity used when no core file
// is available (the directory is absent) or no CORE_NR_FEMTO.csv record
// matches the device's market. In Java these come from the CORE_NR_FEMTO.csv
// core file (market → plmn); Phase 2b (Increment ③) now resolves them from
// the parsed core data via resolveCoreValues, falling back to these defaults
// only when the core file is missing or unmatched.
const (
	defaultMCC = "310"
	defaultMNC = "260"
)

// errDeviceSkipped is returned by processElement when a device is excluded by
// a gate (e.g. empty ACS URL) rather than failing. ScanAndProcess treats it as
// a no-op skip: not logged as a failure and not counted as attempted.
var errDeviceSkipped = errors.New("ztp: device skipped by gate")

// Thread is the ZTP external-registration orchestrator.
type Thread struct {
	db         *gorm.DB
	svc        misc.Service // GetZTPSetting + GenerateAOSFile
	transports *external.Transports
	alarmSvc   alarm.Service // raises ztp_failed alarms on gate/skip conditions
	tplat      *external.TPlatformClient // T-Platform alert outcall (event-driven)
}

// NewThread builds an orchestrator. tc is the global mTLS transport config
// (cert paths + passwords); a nil tc uses the Java-default paths read from
// the environment. The per-system HTTP transports are built once here and
// reused across every device in a scan, matching Java's singleton helper
// beans (and preserving the LMF X-Auth-Token cache across devices). alarmSvc
// raises the per-element "ztp_failed" alarm used by the Java-equivalent gates.
func NewThread(db *gorm.DB, svc misc.Service, tc *external.TransportConfig, alarmSvc alarm.Service) *Thread {
	if tc == nil {
		tc = external.NewTransportConfig()
	}
	tr, err := external.NewTransports(tc)
	if err != nil {
		logger.Errorf("ztp: failed to build mTLS transports (%v); disabled systems no-op, enabled systems will fail at the wire", err)
		tr = &external.Transports{Shared: external.NotImplementedTransport{}, LMF: []external.Transport{external.NotImplementedTransport{}}}
	}
	return &Thread{db: db, svc: svc, transports: tr, alarmSvc: alarmSvc, tplat: external.NewTPlatformClient(tr, alarmSvc)}
}

// ScanAndProcess selects ready devices that have not yet been processed by
// this orchestrator (aos_file_name IS NULL, read_to_ztp = 1, not in ztp_log)
// and processes each. It mirrors Java's GenerateZTPFileThread.getNeedZTPElements
// + per-element loop. The ZTP setting, external config, and the client
// registry (with mTLS transports + LMF token cache) are built once here and
// shared across all devices in the scan. Returns the number of devices
// attempted.
func (t *Thread) ScanAndProcess(ctx context.Context) (int, error) {
	setting, err := t.svc.GetZTPSetting()
	if err != nil {
		return 0, fmt.Errorf("load ztp setting: %w", err)
	}
	// Java behavior (GenerateZTPFileThread.run): when validateZTPSettings
	// fails it generates a ZTP-setting alarm and `continue`s the loop
	// WITHOUT logging an error. Mirror that here — return (0, nil) so the
	// caller does not print an ERROR every tick (this job runs every 30s).
	// The detail is surfaced via debug logging only.
	if msg := validateZTPSettings(setting); msg != nil {
		logger.Debugf("ztp-external-gen: setting invalid, skipping scan: %s", *msg)
		return 0, nil
	}
	cfg := external.FromZTPSetting(setting)
	reg := external.NewRegistryWithTransports(cfg, t.transports)
	roller := external.NewRollback(reg)

	// Load the core files (CORE_NR_FEMTO csv + N41_NR_SC kml) once per scan.
	// A missing directory or parse error is non-fatal: the orchestrator keeps
	// using the hardcoded PLMN default and skips polygon geofence selection,
	// mirroring the Phase 2a behaviour.
	coreRecords, spatialFiles, err := LoadCoreData(CoreFileDir)
	if err != nil {
		logger.Warnf("ztp: failed to load core files from %s: %v (continuing with defaults)", CoreFileDir, err)
	}

	var ids []int64
	if err := t.db.Model(&device.CpeElement{}).
		Where("aos_file_name IS NULL AND read_to_ztp = ? AND deleted = ? AND ne_neid NOT IN (SELECT element_id FROM ztp_log)", true, false).
		Pluck("ne_neid", &ids).Error; err != nil {
		return 0, fmt.Errorf("select ready devices for ZTP: %w", err)
	}
	attempted := 0
	for _, id := range ids {
		if err := t.processElement(ctx, id, setting, cfg, reg, roller, coreRecords, spatialFiles); err != nil {
			if errors.Is(err, errDeviceSkipped) {
				continue // gate skip: not a failure, not counted as attempted
			}
			logger.Warnf("ztp: device %d processing failed: %v", id, err)
			continue
		}
		attempted++
	}
	return attempted, nil
}

// locationMode is the subset of wifi_or_gps_info we need.
type locationMode struct {
	Mode string `json:"mode"`
}

// processElement runs the full ZTP registration flow for one device. The
// setting / external config / registry are built once by ScanAndProcess and
// shared across devices (so the LMF token cache survives between devices).
func (t *Thread) processElement(ctx context.Context, elementId int64, setting *misc.ZTPSetting, cfg *external.ExternalConfig, reg *external.Registry, roller *external.Rollback, coreRecords []*CoreRecord, spatialFiles []*SpatialFileDTO) error {

	now := time.Now()
	var ztpLog misc.ZTPLog
	ztpLog.ElementId = &elementId
	ztpLog.Progress = intPtr(1)
	ztpLog.Done = boolPtr(false)
	ztpLog.StartTime = &now
	ztpLog.HasFault = boolPtr(false)
	if err := t.db.Create(&ztpLog).Error; err != nil {
		return fmt.Errorf("create ztp_log: %w", err)
	}
	setFault := func(info string) {
		t.db.Model(&ztpLog).Updates(map[string]interface{}{
			"progress": 6, "done": true, "has_fault": true, "info": info, "end_time": time.Now(),
		})
		t.db.Create(&misc.ZTPRetryLog{ElementId: &elementId, RetryTime: timePtr(time.Now()), Info: strPtr(info)})
	}

	// Load device.
	var dev device.CpeElement
	if err := t.db.Where("ne_neid = ? AND deleted = ?", elementId, false).First(&dev).Error; err != nil {
		setFault(fmt.Sprintf("device not found: %v", err))
		return err
	}
	if dev.SerialNumber == nil || *dev.SerialNumber == "" {
		setFault("device has no serial number")
		return errors.New("device has no serial number")
	}
	sn := *dev.SerialNumber

	// acs_url gating (Java GenerateZTPFileThread line 459): if the device's
	// TR-069 ACS URL (cpe_element.coon_req_url) is empty, raise the
	// "ztp_failed" alarm and skip the device WITHOUT faulting the ztp_log
	// (mirrors Java's triggerZTPAlarmForFemto + continue).
	if dev.CoonReqUrl == nil || strings.TrimSpace(*dev.CoonReqUrl) == "" {
		t.raiseZTPFailedAlarm(dev, "The ACS URL parameter is missing in Inform")
		logger.Warnf("ztp: device %d skipped — ACS URL (coon_req_url) is empty", elementId)
		return errDeviceSkipped
	}

	lat, lng, err := parseLatLng(dev.Latitude, dev.Longitude)
	if err != nil {
		setFault("device has no GPS coordinates")
		return err
	}

	mode := "GPS"
	if dev.WifiOrGpsInfo != nil {
		var lm locationMode
		if json.Unmarshal([]byte(*dev.WifiOrGpsInfo), &lm) == nil && lm.Mode != "" {
			mode = lm.Mode
		}
	}

	// Spectrum reverse-geocode → PSAP id + expected location (geofence).
	var psapID string
	if reg.Spectrum.Enabled() {
		loc, err := reg.Spectrum.ReverseGeocode(ctx, lat, lng)
		if err != nil {
			setFault(fmt.Sprintf("spectrum reverse-geocode failed: %v", err))
			return err
		}
		if loc == nil {
			setFault("Failed to get response from Spectrum Spatial System")
			return errors.New("no spectrum response")
		}
		psapID = loc.PsapID
		// Vincenty geofence: device GPS vs reverse-geocoded expected location.
		dist := vincentyDistance(lat, lng, loc.Latitude, loc.Longitude)
		if dist >= cfg.RadiusThreshold {
			setFault("Femto location outside designated range")
			return errors.New("geofence violation")
		}
	}

	market := strOrEmpty(dev.Market)

	// Core-file-driven values (Phase 2b Increment ③). The geofence polygon
	// (KML) selects the effective market + ARFCN; the matching CORE_NR_FEMTO
	// record supplies the PLMN (MCC/MNC) + PCI. Falls back to the default PLMN
	// and the device's own market when no polygon contains the point or no
	// core record matches.
	effMarket, dcMCC, dcMNC, dcNrPci, dcArfcnDl, dcArfcnUl, spatialHit, coreHit :=
		resolveCoreValues(lat, lng, market, coreRecords, spatialFiles)
	if !spatialHit && len(spatialFiles) > 0 {
		logger.Warnf("ztp: device %d latitude/longitude did not match to the market (falling back to device market %q)", elementId, market)
	}
	if !coreHit {
		logger.Warnf("ztp: device %d no CORE_NR_FEMTO record matched market %q; using default PLMN %s/%s", elementId, effMarket, dcMCC, dcMNC)
	}

	// gNB-ID allocation (reuse if still in range, else scan).
	gnbID, _, err := allocateGnbID(t.db, setting, elementId, market)
	if err != nil {
		setFault(fmt.Sprintf("failed to allocate gnb id: %v", err))
		return err
	}

	// TAC allocation (per-market cursor; needs TacStart/TacEnd from setting).
	finalTac, _, err := allocateTAC(t.db, cfg, market)
	if err != nil {
		setFault(fmt.Sprintf("failed to allocate tac: %v", err))
		return err
	}

	// Build the per-device context the registrars consume.
	dc := &external.DeviceContext{
		ElementID:    elementId,
		SerialNumber: sn,
		Market:       market,
		Mode:         mode,
		Latitude:     lat,
		Longitude:    lng,
		Altitude:     0,
		MCC:          dcMCC,
		MNC:          dcMNC,
		CellID:       1,
		TAC:          finalTac,
		GnbID:        gnbID,
		NrPci:        dcNrPci,
		ArfcnDl:      dcArfcnDl,
		ArfcnUl:      dcArfcnUl,
		PsapID:       psapID,
		Uncertainty:  0,
	}

	// Ensure the AOS pull-file exists.
	aosFile, err := t.svc.GenerateAOSFile(elementId)
	if err != nil {
		setFault(fmt.Sprintf("AOS generation failed: %v", err))
		return err
	}

	// External registration. Each failure triggers a rollback of whatever was
	// already pushed, then faults the device.
	cancel := &external.CancelDTO{
		GmlcIdentifier: fmt.Sprintf("%s_%s_%d", dc.MCC, dc.MNC, gnbID*4096+1),
		GndId:          gnbID,
		CellId:         1,
		Mcc:            dc.MCC,
		Mnc:            dc.MNC,
		Tac:            finalTac,
	}

	// Push the cell to every configured E911 system in order. On the first
	// failure RunRegistration returns the failing system's name; we fault the
	// device and roll back whatever was already pushed.
	step, err := external.RunRegistration(ctx, reg, dc, cancel)
	if err != nil {
		setFault(fmt.Sprintf("%s registration failed: %v", step, err))
		roller.DeleteInfoFromE911Components(ctx, cancel, dc)
		return err
	}

	// Success: persist the registration record, mark the device ready, and
	// record the gnbId usage.
	e911JSON, _ := json.Marshal(cancel)
	if err := t.db.Model(&device.CpeElement{}).Where("ne_neid = ?", elementId).Updates(map[string]interface{}{
		"e911_data":     string(e911JSON),
		"aos_file_name": aosFile,
		"read_to_ztp":   true,
	}).Error; err != nil {
		setFault(fmt.Sprintf("persist e911_data failed: %v", err))
		return err
	}
	t.db.Create(&misc.ZTPGnbIdUsed{
		Id:        uuid.New().String(),
		ElementId: &elementId,
		Market:    strPtr(market),
		GnbId:     &gnbID,
	})
	t.db.Model(&ztpLog).Updates(map[string]interface{}{
		"progress": 6, "done": true, "has_fault": false,
		"info": "ZTP provisioning completed", "end_time": time.Now(),
	})
	t.db.Create(&misc.ZTPFileSendLog{
		ElementId:    &elementId,
		FileName:     strPtr(aosFile),
		GenerateTime: timePtr(time.Now()),
	})

	logger.Infof("ztp: device %d provisioned (gnbId=%d, tac=%d, psap=%s)", elementId, gnbID, finalTac, psapID)
	return nil
}

// ---------------------------------------------------------------------------
// Allocation helpers
// ---------------------------------------------------------------------------

// allocateGnbID returns a gNB-ID for the device. If the device already has a
// ztp_gnbid_used row whose gnbId is still within [start,end], it is reused;
// otherwise the first free id in the range is allocated. A stale (out-of-range)
// row is deleted before scanning.
func allocateGnbID(db *gorm.DB, setting *misc.ZTPSetting, elementId int64, _ string) (int, bool, error) {
	if setting.GnbIdStart == nil || setting.GnbIdEnd == nil {
		return 0, false, errors.New("gnb id range not configured")
	}
	start, end := *setting.GnbIdStart, *setting.GnbIdEnd

	var existing misc.ZTPGnbIdUsed
	if err := db.Where("element_id = ?", elementId).First(&existing).Error; err == nil {
		if existing.GnbId != nil && *existing.GnbId >= start && *existing.GnbId <= end {
			return *existing.GnbId, true, nil
		}
		db.Where("element_id = ?", elementId).Delete(&misc.ZTPGnbIdUsed{})
	}

	var used []misc.ZTPGnbIdUsed
	if err := db.Find(&used).Error; err != nil {
		return 0, false, fmt.Errorf("load used gnb ids: %w", err)
	}
	usedSet := make(map[int]bool, len(used))
	for _, u := range used {
		if u.GnbId != nil {
			usedSet[*u.GnbId] = true
		}
	}
	for i := start; i <= end; i++ {
		if !usedSet[i] {
			return i, false, nil
		}
	}
	return 0, false, fmt.Errorf("no free gnb id in [%d,%d]", start, end)
}

// allocateTAC returns the final TAC for the device's market. The TAC range
// (tacStart/tacEnd) is sourced from the ZTP setting (Java reads it from the
// CORE_NR_FEMTO.csv core file). When the range is not configured, TAC
// allocation is skipped and (0, 0, nil) is returned (TODO: core-file parsing).
// It returns (finalTac, tacMid, error).
func allocateTAC(db *gorm.DB, cfg *external.ExternalConfig, market string) (int, int, error) {
	if cfg.TacStart == nil || cfg.TacEnd == nil {
		return 0, 0, nil
	}
	tacStartStr := strconv.Itoa(*cfg.TacStart)
	tacEndStr := strconv.Itoa(*cfg.TacEnd)
	start := getMbPart(tacStartStr)
	end := getMbPart(tacEndStr)
	if start < 0 || end < 0 {
		return 0, 0, fmt.Errorf("invalid tac range %d..%d", *cfg.TacStart, *cfg.TacEnd)
	}

	var used misc.ZTPTACUsed
	err := db.Where("market = ?", market).First(&used).Error
	var current int
	if err != nil || used.CurrentUsedTac == nil || *used.CurrentUsedTac < start || *used.CurrentUsedTac > end {
		current = start
	} else {
		current = *used.CurrentUsedTac + 1
		if current > end {
			current = start
		}
	}

	finalTac, err := reassembleTAC(tacStartStr, current)
	if err != nil {
		return 0, 0, err
	}

	// Persist the per-market cursor.
	if err := db.Save(&misc.ZTPTACUsed{
		Id:             uuid.New().String(),
		Market:         strPtr(market),
		CurrentUsedTac: &current,
	}).Error; err != nil {
		return 0, 0, fmt.Errorf("persist tac cursor: %w", err)
	}
	return finalTac, current, nil
}

// getMbPart extracts the middle two hex digits of a TAC string as a base-16
// int (mirrors Java's getMbPart).
func getMbPart(tac string) int {
	if len(tac) < 4 {
		return -1
	}
	mid := tac[2 : len(tac)-2]
	v, err := strconv.ParseInt(mid, 16, 64)
	if err != nil {
		return -1
	}
	return int(v)
}

// reassembleTAC rebuilds the final TAC hex string: tacStart[0:2] + hex(mid,
// zero-padded to the middle width) + tacStart[4:], parsed back as base-16.
func reassembleTAC(tacStart string, mid int) (int, error) {
	if len(tacStart) < 4 {
		return 0, fmt.Errorf("tacStart too short: %q", tacStart)
	}
	middleLen := len(tacStart) - 4
	midHex := fmt.Sprintf("%0*X", middleLen, mid)
	result := tacStart[:2] + midHex + tacStart[4:]
	v, err := strconv.ParseInt(result, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("reassemble tac %q: %w", result, err)
	}
	return int(v), nil
}

// ---------------------------------------------------------------------------
// Core-file value resolution (Phase 2b Increment ③)
// ---------------------------------------------------------------------------

// resolveCoreValues computes the PLMN, PCI and ARFCN values for a device from
// the parsed ZTP core files:
//   - The geofence polygon (N41_NR_SC KML) selects the effective market via
//     point-in-polygon, and supplies the ARFCN DL/UL (its nRARFCN). When no
//     polygon contains the device point, the device's own market is used and
//     spatialHit is false.
//   - The matching CORE_NR_FEMTO.csv record (by market) supplies the PLMN
//     (MCC/MNC) and the PCI pool. When none matches, the default PLMN
//     (310/260) is returned and coreHit is false.
//
// It is a pure function (no DB / network) so it is directly unit-testable.
func resolveCoreValues(lat, lng float64, deviceMarket string, records []*CoreRecord, spatial []*SpatialFileDTO) (
	effectiveMarket string, mcc, mnc string, nrPci, arfcnDl, arfcnUl int, spatialHit, coreHit bool,
) {
	effectiveMarket = deviceMarket
	var sp *SpatialFileDTO
	if len(spatial) > 0 {
		sp = selectSpatialFile(lng, lat, spatial)
	}
	if sp != nil {
		effectiveMarket = sp.Market
		arfcnDl = sp.NRARFCN
		arfcnUl = sp.NRARFCN
		spatialHit = true
	}
	core := matchCoreRecord(records, effectiveMarket)
	if core != nil {
		mcc, mnc = core.MCC, core.MNC
		nrPci = pickPCI(core.Pci)
		coreHit = true
	} else {
		mcc, mnc = defaultMCC, defaultMNC
	}
	return
}

// matchCoreRecord finds the CORE_NR_FEMTO.csv record whose market matches m
// (either the plain market name or the composite MarketKey). Returns the first
// match, or nil when nothing matches (caller keeps the default PLMN).
func matchCoreRecord(records []*CoreRecord, m string) *CoreRecord {
	if m == "" {
		return nil
	}
	for _, r := range records {
		if r.Market == m || r.MarketKey == m {
			return r
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// ZTP failure alarm (mirrors Java triggerZTPAlarmForFemto)
// ---------------------------------------------------------------------------

// ztpFailedAlarmID is the alarm_id Java raises for every ZTP failure. A single
// logical alarm is kept per element and updated in place when the fault detail
// changes, so repeated scans do not pile up duplicate alarms.
const ztpFailedAlarmID = "ztp_failed"

// raiseZTPFailedAlarm mirrors Java's triggerZTPAlarmForFemto: it raises (or
// updates) the per-element "ztp_failed" alarm carrying faultInfo in its
// additional_information, de-duplicating so repeated scans with the same
// detail do not re-raise. It never writes a ztp_log fault — the caller
// decides whether to skip (acs_url gate) or proceed (other faults use
// setFault). Errors are logged and swallowed so an alarm outage cannot break
// the orchestrator.
func (t *Thread) raiseZTPFailedAlarm(dev device.CpeElement, faultInfo string) {
	existing, err := t.alarmSvc.GetByElementTypeAlarmId(dev.NeNeid, alarm.AlarmTypeActive, ztpFailedAlarmID)
	if err != nil {
		logger.Warnf("ztp: lookup ztp_failed alarm for device %d failed: %v", dev.NeNeid, err)
		existing = nil
	}
	// De-dup: already raised with the same detail → do nothing.
	if existing != nil && strOrEmpty(existing.AdditionalInformation) == faultInfo {
		return
	}

	now := time.Now()
	if existing != nil {
		// Detail changed → update the existing alarm in place.
		if err := t.db.Model(&alarm.Alarm{}).Where("id = ?", existing.Id).Updates(map[string]interface{}{
			"additional_information": faultInfo,
			"update_time":            now,
		}).Error; err != nil {
			logger.Warnf("ztp: update ztp_failed alarm for device %d failed: %v", dev.NeNeid, err)
		}
		return
	}

	a := &alarm.Alarm{
		AlarmId:               strPtr(ztpFailedAlarmID),
		AlarmIdentifier:       strPtr(strconv.FormatInt(now.UnixMilli(), 10)),
		AlarmSource:           strPtr("OMC"),
		AlarmStatus:           intPtr(alarm.AlarmStatusActiveUnconfirmed),
		AlarmType:             intPtr(alarm.AlarmTypeActive),
		EventTime:             &now,
		EventType:             strPtr("ZTP Alarm"),
		NetworkElement:        strPtr("DN='OMC'"),
		ProbableCause:         strPtr("ZTP failed. Please check the detailed information"),
		Severity:              strPtr("Critical"),
		SpecificProblem:       strPtr("ZTP Failed"),
		ElementId:             &dev.NeNeid,
		LicenseId:             dev.LicenseId,
		UpdateTime:            &now,
		AdditionalInformation: strPtr(faultInfo),
	}
	if err := t.alarmSvc.CreateAlarm(a); err != nil {
		logger.Warnf("ztp: raise ztp_failed alarm for device %d failed: %v", dev.NeNeid, err)
	}
}

// NotifyTPlatform forwards an operational alert to the T-Mobile T-Platform
// alert-management API. It re-reads the live ZTPSetting (Java reads it from
// system_config on every notify), refreshes the client's endpoint config, and
// delegates to the TPlatformClient. This is an event-driven outcall — Java
// triggers it from TplatformMessageConsumer (RabbitMQ); the ZTP registration
// chain never calls it. Returns (accepted, err) — see
// TPlatformClient.Notify for semantics (a non-nil err is informational, not
// fatal; the t_platform_unavailable alarm already reflects the outage).
func (t *Thread) NotifyTPlatform(ctx context.Context, req *external.TPlatformAlertRequest) (bool, error) {
	setting, err := t.svc.GetZTPSetting()
	if err != nil {
		return false, fmt.Errorf("load ztp setting for t-platform notify: %w", err)
	}
	t.tplat.SetConfig(external.FromZTPSetting(setting).TPlatform)
	return t.tplat.Notify(ctx, req)
}

// ---------------------------------------------------------------------------
// Validation + geo helpers
// ---------------------------------------------------------------------------

// validateZTPSettings returns the first failing rule's message, or nil if the
// setting is valid (mirrors Java's ~20-rule validateZTPSettings).
func validateZTPSettings(s *misc.ZTPSetting) *string {
	if s == nil {
		return strPtr("ZTP Setting configuration is missing")
	}
	if s.GnbIdStart == nil {
		return strPtr("gNB ID Start is missing")
	}
	if s.GnbIdEnd == nil {
		return strPtr("gNB ID End is missing")
	}
	if strOrEmpty(s.GoogleAPIKey) == "" {
		return strPtr("Google API Key is missing")
	}
	if s.PTP == nil {
		return strPtr("PTP Setting is missing")
	}
	if s.PTP.ClockDomainNumber == nil {
		return strPtr("PTP Clock Domain Number is missing")
	}
	if strOrEmpty(s.PTP.ClockSyncMode) == "" {
		return strPtr("PTP Clock Sync Mode is missing")
	}
	if s.SpectrumSpatial == nil {
		return strPtr("Spectrum Spatial Setting is missing")
	}
	if strOrEmpty(s.SpectrumSpatial.GeoCodeURL) == "" {
		return strPtr("Spectrum Spatial GeoCode URL is missing")
	}
	if strOrEmpty(s.SpectrumSpatial.ReverseGeoCodeURL) == "" {
		return strPtr("Spectrum Spatial Reverse GeoCode URL is missing")
	}
	if s.MSAG == nil {
		return strPtr("MSAG Setting is missing")
	}
	if strOrEmpty(s.MSAG.URL) == "" {
		return strPtr("MSAG URL is missing")
	}
	if strOrEmpty(s.MSAG.Username) == "" {
		return strPtr("MSAG Username is missing")
	}
	if strOrEmpty(s.MSAG.Password) == "" {
		return strPtr("MSAG Password is missing")
	}
	if !bmcConfigValid(s.BMC, s.NewBMC) {
		return strPtr("BMC Setting is invalid (old or new BMC must be configured)")
	}
	if !lmfConfigValid(s.LMF, s.LMF2, s.LMF3, s.LMF4) {
		return strPtr("LMF Configuration is invalid (at least one LMF setting must be valid)")
	}
	if s.GMLC == nil {
		return strPtr("GMLC Setting is missing")
	}
	if strOrEmpty(s.GMLC.URL) == "" {
		return strPtr("GMLC URL is missing")
	}
	if strOrEmpty(s.GMLC.Username) == "" {
		return strPtr("GMLC Username is missing")
	}
	if strOrEmpty(s.GMLC.Password) == "" {
		return strPtr("GMLC Password is missing")
	}
	return nil
}

func bmcConfigValid(oldC, newC *misc.ExternalEndpointSetting) bool {
	oldOK := oldC != nil && strOrEmpty(oldC.URL) != "" && strOrEmpty(oldC.Username) != "" && strOrEmpty(oldC.Password) != ""
	newOK := newC != nil && strOrEmpty(newC.URL) != ""
	return oldOK || newOK
}

func lmfConfigValid(l ...*misc.ExternalEndpointSetting) bool {
	for _, x := range l {
		if x != nil && strOrEmpty(x.URL) != "" && strOrEmpty(x.Username) != "" && strOrEmpty(x.Password) != "" {
			return true
		}
	}
	return false
}

// parseLatLng parses the device's longitude/latitude string columns.
func parseLatLng(latP, lngP *string) (float64, float64, error) {
	if latP == nil || lngP == nil || *latP == "" || *lngP == "" {
		return 0, 0, errors.New("missing device GPS coordinates")
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(*latP), 64)
	lng, err2 := strconv.ParseFloat(strings.TrimSpace(*lngP), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("invalid device GPS coordinates: lat=%q lng=%q", strOrEmpty(latP), strOrEmpty(lngP))
	}
	return lat, lng, nil
}

// vincentyDistance returns the great-circle distance (metres) between two
// WGS84 points using the Vincenty inverse formula.
func vincentyDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const a = 6378137.0
	const f = 1 / 298.257223563
	b := (1 - f) * a
	rad := math.Pi / 180
	phi1 := lat1 * rad
	phi2 := lat2 * rad
	lam1 := lon1 * rad
	lam2 := lon2 * rad
	L := lam2 - lam1

	tanU1 := (1 - f) * math.Tan(phi1)
	cosU1 := 1 / math.Sqrt(1+tanU1*tanU1)
	sinU1 := tanU1 * cosU1
	tanU2 := (1 - f) * math.Tan(phi2)
	cosU2 := 1 / math.Sqrt(1+tanU2*tanU2)
	sinU2 := tanU2 * cosU2

	lambda := L
	var sinSigma, cosSigma, sigma, sinAlpha, cosSqAlpha, cos2SigmaM, C float64
	for i := 0; i < 1000; i++ {
		sinLam := math.Sin(lambda)
		cosLam := math.Cos(lambda)
		sinSigma = math.Sqrt((cosU2*sinLam)*(cosU2*sinLam) +
			(cosU1*sinU2-sinU1*cosU2*cosLam)*(cosU1*sinU2-sinU1*cosU2*cosLam))
		if sinSigma == 0 {
			return 0
		}
		cosSigma = sinU1*sinU2 + cosU1*cosU2*cosLam
		sigma = math.Atan2(sinSigma, cosSigma)
		sinAlpha = cosU1 * cosU2 * sinLam / sinSigma
		cosSqAlpha = 1 - sinAlpha*sinAlpha
		if cosSqAlpha == 0 {
			cos2SigmaM = 0
		} else {
			cos2SigmaM = cosSigma - 2*sinU1*sinU2/cosSqAlpha
		}
		C = f / 16 * cosSqAlpha * (4 + f*(4-3*cosSqAlpha))
		lambdaPrev := lambda
		lambda = L + (1-C)*f*sinAlpha*(sigma+C*sinSigma*(cos2SigmaM+C*cosSigma*(-1+2*cos2SigmaM*cos2SigmaM)))
		if math.Abs(lambda-lambdaPrev) < 1e-12 {
			break
		}
	}
	uSq := cosSqAlpha * (a*a - b*b) / (b * b)
	A := 1 + uSq/16384*(4096+uSq*(-768+uSq*(320-175*uSq)))
	B := uSq / 1024 * (256 + uSq*(-128+uSq*(74-47*uSq)))
	dSigma := B * sinSigma * (cos2SigmaM + B/4*(cosSigma*(-1+2*cos2SigmaM*cos2SigmaM)-
		B/6*cos2SigmaM*(-3+4*sinSigma*sinSigma)*(-3+4*cos2SigmaM*cos2SigmaM)))
	return b * A * (sigma - dSigma)
}

// ---------------------------------------------------------------------------
// pointer helpers
// ---------------------------------------------------------------------------

func intPtr(i int) *int    { return &i }
func boolPtr(b bool) *bool { return &b }
func strPtr(s string) *string { return &s }
func timePtr(t time.Time) *time.Time { return &t }

// strOrEmpty dereferences a *string, returning "" for nil.
func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
