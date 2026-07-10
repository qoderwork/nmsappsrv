package misc

import (
	"encoding/json"
	"fmt"
)

// ---------------------------------------------------------------------------
// ZTPLog
// ---------------------------------------------------------------------------

// ListZTPLogs returns all ZTP logs for the given element.
func (s *Service) ListZTPLogs(elementId int64) ([]ZTPLog, error) {
	return s.repo.FindZTPLogs(elementId)
}

// ---------------------------------------------------------------------------
// ZTP
// ---------------------------------------------------------------------------

// GetZTPSetting loads and parses the ZTP configuration from system_config.
func (s *Service) GetZTPSetting() (*ZTPSetting, error) {
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
func (s *Service) SaveZTPSetting(setting *ZTPSetting) error {
	data, err := json.Marshal(setting)
	if err != nil {
		return fmt.Errorf("marshal ztp_config: %w", err)
	}
	return s.repo.SaveSystemConfigValue("ztp_config", string(data))
}

// ListZTPResults returns paginated ZTP provisioning results.
func (s *Service) ListZTPResults(req *ListZTPResultsRequest) ([]ZTPResultVo, int64, error) {
	return s.repo.FindZTPResults(req)
}

// ListZTPRetryLogs returns retry logs for a device.
func (s *Service) ListZTPRetryLogs(elementId int64) ([]ZTPRetryLogVo, error) {
	return s.repo.FindZTPRetryLogs(elementId)
}

// ListHistoryZTPFiles returns paginated ZTP file history.
func (s *Service) ListHistoryZTPFiles(elementId int64, page, pageSize int) ([]HistoryZTPFileVo, int64, error) {
	return s.repo.FindHistoryZTPFiles(elementId, page, pageSize)
}

// SetZTPStatus enables or disables ZTP for the given devices.
func (s *Service) SetZTPStatus(req *SetZTPStatusRequest) error {
	if req.Status == "enable" {
		// Reset aos_file_name to nil and read_to_ztp to 0 so the ZTP thread picks them up.
		return s.repo.ClearDeviceAOSFile(req.ElementIds)
	}
	// "disable": just clear the read_to_ztp flag.
	return s.repo.DB().Table("cpe_element").
		Where("ne_neid IN (?)", req.ElementIds).
		Update("read_to_ztp", 0).Error
}

// BatchReZTP triggers re-provisioning for a batch of devices.
func (s *Service) BatchReZTP(req *BatchReZTPRequest) error {
	elementIds := s.resolveReZTPDeviceIds(req)
	if len(elementIds) == 0 {
		return fmt.Errorf("no devices resolved for re-ZTP")
	}

	// Clean up old ZTP data for these devices.
	_ = s.repo.DeleteZTPLogsByElementIds(elementIds)
	_ = s.repo.DeleteZTPFileSendLogsByElementIds(elementIds)
	for _, id := range elementIds {
		_ = s.repo.DeleteGnbIdUsedByElementId(id)
	}

	// Reset device AOS file so the ZTP thread picks them up again.
	return s.repo.ClearDeviceAOSFile(elementIds)
}

// DeleteZTPFiles deletes ZTP files and related data for the given devices.
func (s *Service) DeleteZTPFiles(req *DeleteZTPFileRequest) error {
	_ = s.repo.DeleteZTPLogsByElementIds(req.ElementIds)
	_ = s.repo.DeleteZTPFileSendLogsByElementIds(req.ElementIds)
	for _, id := range req.ElementIds {
		_ = s.repo.DeleteGnbIdUsedByElementId(id)
	}
	return s.repo.ClearDeviceAOSFile(req.ElementIds)
}

// resolveReZTPDeviceIds extracts device IDs from the batch re-ZTP request.
func (s *Service) resolveReZTPDeviceIds(req *BatchReZTPRequest) []int64 {
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
