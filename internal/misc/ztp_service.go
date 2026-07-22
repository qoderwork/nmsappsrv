package misc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"nmsappsrv/pkg/redis"
)

// ---------------------------------------------------------------------------
// ZTPLog
// ---------------------------------------------------------------------------

// ListZTPLogs returns all ZTP logs for the given element.
func (s *service) ListZTPLogs(elementId int64) ([]ZTPLog, error) {
	return s.repo.FindZTPLogs(elementId)
}

// ---------------------------------------------------------------------------
// ZTP
// ---------------------------------------------------------------------------

// GetZTPSetting loads and parses the ZTP configuration from system_config.
func (s *service) GetZTPSetting() (*ZTPSetting, error) {
	val, err := s.repo.GetSystemConfigValue("ztp_config")
	if err != nil {
		// No config yet — return defaults.
		return &ZTPSetting{}, nil
	}
	var setting ZTPSetting
	if err := json.Unmarshal([]byte(val), &setting); err != nil {
		return nil, fmt.Errorf("invalid ztp_config: %w", err)
	}
	return &setting, nil
}

// SaveZTPSetting persists the ZTP configuration to system_config.
func (s *service) SaveZTPSetting(setting *ZTPSetting) error {
	data, err := json.Marshal(setting)
	if err != nil {
		return fmt.Errorf("marshal ztp_config: %w", err)
	}
	return s.repo.SaveSystemConfigValue("ztp_config", string(data))
}

// ListZTPResults returns paginated ZTP provisioning results.
func (s *service) ListZTPResults(req *ListZTPResultsRequest) ([]ZTPResultVo, int64, error) {
	return s.repo.FindZTPResults(req)
}

// ListZTPRetryLogs returns retry logs for a device.
func (s *service) ListZTPRetryLogs(elementId int64) ([]ZTPRetryLogVo, error) {
	return s.repo.FindZTPRetryLogs(elementId)
}

// ListHistoryZTPFiles returns paginated ZTP file history.
func (s *service) ListHistoryZTPFiles(elementId int64, page, pageSize int) ([]HistoryZTPFileVo, int64, error) {
	return s.repo.FindHistoryZTPFiles(elementId, page, pageSize)
}

// SetZTPStatus enables or disables ZTP for the given devices. Mirrors Java's
// setZTPStatus -> neElementService.updateReadyToZTP(elementId, status == 1):
// a pure ready_to_ztp toggle. Go's AOS-generation cron (ztp-aos-gen) is gated
// on (aos_file_name IS NULL AND read_to_ztp = 1), so an "enable" also clears
// aos_file_name so the engine regenerates + re-provisions the device (Java
// reaches the same re-provisioning because its regen is gated on
// aos_file_name IS NULL alone). "disable" only clears the ready flag.
func (s *service) SetZTPStatus(req *SetZTPStatusRequest) error {
	ready := 0
	if req.Status == "enable" {
		ready = 1
	}
	updates := map[string]interface{}{"read_to_ztp": ready}
	if ready == 1 {
		updates["aos_file_name"] = nil
	}
	return s.repo.DB().Table("cpe_element").
		Where("ne_neid IN (?)", req.ElementIds).
		Updates(updates).Error
}

// BatchReZTP triggers re-provisioning for a batch of devices. Mirrors Java's
// @Async batchReztp: it only re-triggers devices whose ztp_log progress == 6
// (completed), across the chosen scope (element / deviceGroup / market). Java's
// market scope additionally filters by point-in-polygon against the spatial
// KML; Go stores market as a flat cpe_element column, so we match by market and
// let the ztp orchestrator engine (internal/ztp) re-validate each device's
// polygon on the next scan.
func (s *service) BatchReZTP(req *BatchReZTPRequest) error {
	candidates := s.resolveReZTPDeviceIds(req)
	if len(candidates) == 0 {
		return fmt.Errorf("no devices resolved for re-ZTP")
	}

	// Only completed devices (progress == 6) are re-triggered, matching Java.
	var completed []int64
	if err := s.repo.DB().Table("ztp_log").
		Where("progress = ? AND element_id IN (?)", 6, candidates).
		Pluck("element_id", &completed).Error; err != nil {
		return err
	}
	if len(completed) == 0 {
		return fmt.Errorf("no completed (progress=6) devices found for re-ZTP")
	}

	for _, id := range completed {
		if err := s.RetryZtp(id); err != nil {
			return err
		}
	}
	return nil
}

// DeleteZTPFiles re-triggers ZTP for the given devices. Mirrors Java's
// deleteZTPFile -> retryZtp: the whole request fails if any device is currently
// upgrading, otherwise each device is retried.
func (s *service) DeleteZTPFiles(req *DeleteZTPFileRequest) error {
	ctx := context.Background()
	// Java fails the entire request if ANY device is upgrading.
	for _, id := range req.ElementIds {
		if isDeviceUpgrading(ctx, id) {
			return fmt.Errorf("During the device upgrade, this operation is prohibited")
		}
	}
	for _, id := range req.ElementIds {
		if err := s.RetryZtp(id); err != nil {
			return err
		}
	}
	return nil
}

// RetryZtp re-triggers ZTP for a single device, mirroring Java's retryZtp +
// ZTPFailedConsumer.ztpFailedNotify. Order follows Java:
//  1. clear the local E911 registration reference (Java also de-registers the
//     remote E911 systems via E911Helper; that remote rollback is owned by the
//     ztp orchestrator engine, internal/ztp/external, on the next failed scan)
//  2. delete the geo redis cache (Java key: "device_geo_<id>")
//  3. clear aos_file_name + set read_to_ztp=1 so the engine regenerates and
//     re-provisions the device (Java: updateAOSFileNameAndReadyToZTP(id, null,
//     FALSE) then the AOS-regen gate re-readies the device)
//  4. delete the ztp_log
//  5. record a ZTP retry log
func (s *service) RetryZtp(elementId int64) error {
	ctx := context.Background()

	// 1. clear local e911_data reference.
	s.repo.DB().Table("cpe_element").
		Where("ne_neid = ?", elementId).
		Update("e911_data", nil)

	// 2. delete geo redis cache.
	redis.Del(ctx, "device_geo_"+strconv.FormatInt(elementId, 10))

	// 3. clear AOS file + re-ready for regeneration.
	if err := s.repo.DB().Table("cpe_element").
		Where("ne_neid = ?", elementId).
		Updates(map[string]interface{}{"aos_file_name": nil, "read_to_ztp": 1}).Error; err != nil {
		return err
	}

	// 4. delete ztp_log.
	if err := s.repo.DeleteZTPLogsByElementIds([]int64{elementId}); err != nil {
		return err
	}

	// 5. record retry log.
	now := time.Now()
	if err := s.repo.DB().Create(&ZTPRetryLog{
		ElementId: &elementId,
		RetryTime: &now,
		Info:      strPtr("User-triggered ZTP retry"),
	}).Error; err != nil {
		return err
	}
	return nil
}

// isDeviceUpgrading reports whether a device is currently upgrading (Java:
// tr069Helper.isInUpgrade). Go records this in Redis as
// "device_upgrade_status_<id>" = "yes" from the TR-069 Inform handler.
func isDeviceUpgrading(ctx context.Context, elementId int64) bool {
	val, err := redis.Get(ctx, "device_upgrade_status_"+strconv.FormatInt(elementId, 10))
	if err != nil {
		return false
	}
	return val == "yes"
}

// resolveReZTPDeviceIds extracts device IDs from the batch re-ZTP request.
func (s *service) resolveReZTPDeviceIds(req *BatchReZTPRequest) []int64 {
	seen := make(map[int64]struct{})
	var result []int64

	addId := func(id int64) {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}

	for _, id := range req.ElementIds {
		addId(id)
	}

	if req.Scope == "deviceGroup" && len(req.DeviceGroupIds) > 0 {
		var fromGroups []int64
		s.repo.DB().Raw(`SELECT ne_neid FROM cpe_element WHERE device_group_id IN (?)`, req.DeviceGroupIds).Scan(&fromGroups)
		for _, id := range fromGroups {
			addId(id)
		}
	}

	if req.Scope == "market" && len(req.Markets) > 0 {
		var fromMarkets []int64
		s.repo.DB().Raw(`SELECT ne_neid FROM cpe_element WHERE market IN (?)`, req.Markets).Scan(&fromMarkets)
		for _, id := range fromMarkets {
			addId(id)
		}
	}

	return result
}
