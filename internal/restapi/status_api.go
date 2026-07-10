package restapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"github.com/gin-gonic/gin"
)

// ============================
// Request status operations
// ============================

func (s *Service) GetRequestStatus(requestId string) (*RequestStatusVo, error) {
	status, err := s.repo.GetRequestStatus(requestId)
	if err != nil {
		return nil, fmt.Errorf("request not found")
	}
	return status, nil
}

// ============================
// Device Online Status (Task 6.2)
// ============================

// ListDeviceOnlineStatus returns real-time online status for all devices
func (s *Service) ListDeviceOnlineStatus(c *gin.Context) ([]DeviceOnlineStatusVo, error) {
	licenseId := middleware.GetLicenseId(c)

	devices, err := s.repo.ListAllNonDeletedDevices(licenseId)
	if err != nil {
		logger.Errorf("Failed to list devices for online status: %v", err)
		return nil, fmt.Errorf("failed to list devices")
	}

	if len(devices) == 0 {
		return []DeviceOnlineStatusVo{}, nil
	}

	// Build Redis keys for online status check
	keys := make([]string, len(devices))
	for i, d := range devices {
		keys[i] = fmt.Sprintf("online_%d", d.NeNeid)
	}

	// Batch check online status from Redis
	ctx := c.Request.Context()
	values, _ := redis.MGet(ctx, keys...)

	var result []DeviceOnlineStatusVo
	for i, d := range devices {
		online := false
		if i < len(values) && values[i] != nil {
			if val, ok := values[i].(string); ok && strings.ToLower(val) == "yes" {
				online = true
			}
		}

		vo := DeviceOnlineStatusVo{
			ElementId:    d.NeNeid,
			SerialNumber: derefStr(d.SerialNumber),
			DeviceName:   derefStr(d.DeviceName),
			Online:       online,
		}
		result = append(result, vo)
	}

	return result, nil
}

// GetDeviceOnlineStatus returns real-time online status for a single device
func (s *Service) GetDeviceOnlineStatus(c *gin.Context, elementId int64) (*DeviceOnlineStatusVo, error) {
	d, err := s.repo.GetDeviceByElementId(elementId)
	if err != nil {
		return nil, fmt.Errorf("device not found")
	}

	// Check Redis for online status
	ctx := c.Request.Context()
	key := fmt.Sprintf("online_%d", elementId)
	val, _ := redis.Get(ctx, key)

	online := strings.ToLower(val) == "yes"

	return &DeviceOnlineStatusVo{
		ElementId:    d.NeNeid,
		SerialNumber: derefStr(d.SerialNumber),
		DeviceName:   derefStr(d.DeviceName),
		Online:       online,
	}, nil
}

// ============================
// ACS Settings (Task 6.3)
// ============================

// GetACSSettings returns the ACS configuration via REST API
func (s *Service) GetACSSettings(c *gin.Context) (*RestACSConfigVo, error) {
	configJSON, err := s.repo.GetACSConfig()
	if err != nil {
		logger.Errorf("Failed to get ACS config: %v", err)
		return nil, fmt.Errorf("failed to get ACS config")
	}

	if configJSON == "" {
		return &RestACSConfigVo{}, nil
	}

	// Parse the full ACS config from DB
	var fullCfg struct {
		AcsUrl            *string `json:"acsUrl"`
		AcsUsername       *string `json:"acsUsername"`
		AcsPassword       *string `json:"acsPassword"`
		ConnectionTimeout *int    `json:"connectionTimeout"`
		InformInterval    *int    `json:"informInterval"`
		UdpPort           *int    `json:"udpPort"`
		TR069Enabled      *bool   `json:"tr069Enabled"`
	}
	if err := json.Unmarshal([]byte(configJSON), &fullCfg); err != nil {
		logger.Errorf("Failed to unmarshal ACS config: %v", err)
		return nil, fmt.Errorf("failed to parse ACS config")
	}

	// Return without password for security
	return &RestACSConfigVo{
		AcsUrl:            fullCfg.AcsUrl,
		AcsUsername:       fullCfg.AcsUsername,
		ConnectionTimeout: fullCfg.ConnectionTimeout,
		InformInterval:    fullCfg.InformInterval,
		UdpPort:           fullCfg.UdpPort,
		TR069Enabled:      fullCfg.TR069Enabled,
	}, nil
}

// UpdateACSSettings updates the ACS configuration via REST API
func (s *Service) UpdateACSSettings(c *gin.Context, req *RestUpdateACSConfigRequest) error {
	// Get existing config
	configJSON, err := s.repo.GetACSConfig()
	if err != nil {
		logger.Errorf("Failed to get existing ACS config: %v", err)
		return fmt.Errorf("failed to get ACS config")
	}

	// Parse existing config
	var existing struct {
		AcsUrl            *string `json:"acsUrl"`
		AcsUsername       *string `json:"acsUsername"`
		AcsPassword       *string `json:"acsPassword"`
		ConnectionTimeout *int    `json:"connectionTimeout"`
		InformInterval    *int    `json:"informInterval"`
		UdpPort           *int    `json:"udpPort"`
		TR069Enabled      *bool   `json:"tr069Enabled"`
	}
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &existing); err != nil {
			logger.Errorf("Failed to unmarshal existing ACS config: %v", err)
			return fmt.Errorf("failed to parse existing ACS config")
		}
	}

	// Merge with request
	if req.AcsUrl != nil {
		existing.AcsUrl = req.AcsUrl
	}
	if req.AcsUsername != nil {
		existing.AcsUsername = req.AcsUsername
	}
	if req.AcsPassword != nil {
		existing.AcsPassword = req.AcsPassword
	}
	if req.ConnectionTimeout != nil {
		existing.ConnectionTimeout = req.ConnectionTimeout
	}
	if req.InformInterval != nil {
		existing.InformInterval = req.InformInterval
	}
	if req.UdpPort != nil {
		existing.UdpPort = req.UdpPort
	}
	if req.TR069Enabled != nil {
		existing.TR069Enabled = req.TR069Enabled
	}

	// Marshal updated config
	data, err := json.Marshal(existing)
	if err != nil {
		return fmt.Errorf("failed to marshal ACS config")
	}

	// Save to DB
	if err := s.repo.UpdateACSConfig(string(data)); err != nil {
		logger.Errorf("Failed to update ACS config: %v", err)
		return fmt.Errorf("failed to update ACS config")
	}

	// Also update Redis cache
	ctx := c.Request.Context()
	redis.Set(ctx, "nms_config", string(data), 0)

	return nil
}
