package restapi

import (
	"fmt"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/internal/misc"
	"nmsappsrv/pkg/logger"

	"github.com/gin-gonic/gin"
)

// ============================
// Parameter operations
// ============================

func (s *Service) GetDeviceParams(c *gin.Context, elementId int64) ([]RestParameterVo, error) {
	licenseId := middleware.GetLicenseId(c)

	// Verify device exists and belongs to this license
	_, err := s.repo.GetDeviceById(elementId, licenseId)
	if err != nil {
		return nil, fmt.Errorf("device not found")
	}

	params, err := s.repo.GetDeviceParams(elementId)
	if err != nil {
		return nil, fmt.Errorf("failed to get device parameters")
	}

	return params, nil
}

func (s *Service) SetDeviceParams(c *gin.Context, elementId int64, req *SetRestParameterRequest) error {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)

	// Verify device exists
	_, err := s.repo.GetDeviceById(elementId, licenseId)
	if err != nil {
		return fmt.Errorf("device not found")
	}

	if err := s.repo.SetDeviceParams(elementId, req.Parameters); err != nil {
		logger.Errorf("Failed to set device params for element %d: %v", elementId, err)
		return fmt.Errorf("failed to set device parameters")
	}

	logger.Infof("Device params set for element %d by user %s", elementId, username)
	return nil
}

func (s *Service) PresetDeviceParams(c *gin.Context, elementId int64, req *PresetParameterRequest) (*RequestStatusVo, error) {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)

	// Verify device exists
	_, err := s.repo.GetDeviceById(elementId, licenseId)
	if err != nil {
		return nil, fmt.Errorf("device not found")
	}

	// Create preset parameters task record
	task := &misc.PresetParametersTask{
		ElementId: &elementId,
	}
	_ = task // Task record creation would use its own repository in production

	requestId, err := s.repo.PresetDeviceParams(elementId, req.Parameters)
	if err != nil {
		logger.Errorf("Failed to preset device params for element %d: %v", elementId, err)
		return nil, fmt.Errorf("failed to preset device parameters")
	}

	logger.Infof("Preset params queued for element %d, requestId=%s, by user %s", elementId, requestId, username)

	return &RequestStatusVo{
		RequestId: requestId,
		Status:    "pending",
	}, nil
}
