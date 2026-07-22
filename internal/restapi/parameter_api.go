package restapi

import (
	"nmsappsrv/internal/middleware"
	"nmsappsrv/internal/misc"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"

	"github.com/gin-gonic/gin"
)

// ============================
// Parameter operations
// ============================

func (s *service) GetDeviceParams(c *gin.Context, elementId int64) ([]RestParameterVo, error) {
	tenantId := middleware.GetTenantId(c)

	// Verify device exists and belongs to this license
	_, err := s.repo.GetDeviceById(elementId, tenantId)
	if err != nil {
		return nil, apperror.ErrDeviceNotFound
	}

	params, err := s.repo.GetDeviceParams(elementId)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_DEVICE_PARAMS_FAILED", 500, "failed to get device parameters")
	}

	return params, nil
}

func (s *service) SetDeviceParams(c *gin.Context, elementId int64, req *SetRestParameterRequest) error {
	tenantId := middleware.GetTenantId(c)
	username := middleware.GetUsername(c)

	// Verify device exists
	_, err := s.repo.GetDeviceById(elementId, tenantId)
	if err != nil {
		return apperror.ErrDeviceNotFound
	}

	if err := s.repo.SetDeviceParams(elementId, req.Parameters); err != nil {
		logger.Errorf("Failed to set device params for element %d: %v", elementId, err)
		return apperror.Wrap(err, "SET_DEVICE_PARAMS_FAILED", 500, "failed to set device parameters")
	}

	logger.Infof("Device params set for element %d by user %s", elementId, username)
	return nil
}

func (s *service) PresetDeviceParams(c *gin.Context, elementId int64, req *PresetParameterRequest) (*RequestStatusVo, error) {
	tenantId := middleware.GetTenantId(c)
	username := middleware.GetUsername(c)

	// Verify device exists
	_, err := s.repo.GetDeviceById(elementId, tenantId)
	if err != nil {
		return nil, apperror.ErrDeviceNotFound
	}

	// Create preset parameters task record
	task := &misc.PresetParametersTask{
		ElementId: &elementId,
	}
	_ = task // Task record creation would use its own repository in production

	requestId, err := s.repo.PresetDeviceParams(elementId, req.Parameters)
	if err != nil {
		logger.Errorf("Failed to preset device params for element %d: %v", elementId, err)
		return nil, apperror.Wrap(err, "PRESET_DEVICE_PARAMS_FAILED", 500, "failed to preset device parameters")
	}

	logger.Infof("Preset params queued for element %d, requestId=%s, by user %s", elementId, requestId, username)

	return &RequestStatusVo{
		RequestId: requestId,
		Status:    "pending",
	}, nil
}
