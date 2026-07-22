package restapi

import (
	"nmsappsrv/internal/device"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"

	"github.com/gin-gonic/gin"
)

// ============================
// Device operations
// ============================

func (s *service) ListDevices(c *gin.Context, offset, limit int) ([]RestDeviceVo, int64, error) {
	tenantId := middleware.GetTenantId(c)

	devices, total, err := s.repo.ListDevices(tenantId, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	var result []RestDeviceVo
	for _, d := range devices {
		vo := RestDeviceVo{
			Id:              d.NeNeid,
			SerialNumber:    derefStr(d.SerialNumber),
			DeviceName:      derefStr(d.DeviceName),
			DeviceType:      derefStr(d.DeviceType),
			Product:         derefStr(d.Product),
			SoftwareVersion: derefStr(d.SoftwareVersion),
			Manufacturer:    derefStr(d.Manufacturer),
			TenantId:       derefIntPtr(d.TenantId),
		}
		// Determine status from device state
		if d.Deleted {
			vo.Status = "deleted"
		} else {
			vo.Status = "online"
		}
		result = append(result, vo)
	}

	return result, total, nil
}

func (s *service) GetDevice(c *gin.Context, id int64) (*RestDeviceVo, error) {
	tenantId := middleware.GetTenantId(c)

	d, err := s.repo.GetDeviceById(id, tenantId)
	if err != nil {
		return nil, apperror.ErrDeviceNotFound
	}

	vo := &RestDeviceVo{
		Id:              d.NeNeid,
		SerialNumber:    derefStr(d.SerialNumber),
		DeviceName:      derefStr(d.DeviceName),
		DeviceType:      derefStr(d.DeviceType),
		Product:         derefStr(d.Product),
		SoftwareVersion: derefStr(d.SoftwareVersion),
		Manufacturer:    derefStr(d.Manufacturer),
		TenantId:       derefIntPtr(d.TenantId),
		Status:          "online",
	}

	return vo, nil
}

func (s *service) AddDevice(c *gin.Context, req *AddRestDeviceRequest) (*RestDeviceVo, error) {
	tenantId := middleware.GetTenantId(c)
	username := middleware.GetUsername(c)

	// Check for duplicate serial number
	existing, _ := s.repo.GetDeviceBySN(req.SerialNumber, tenantId)
	if existing != nil {
		return nil, apperror.ErrDeviceAlreadyExist
	}

	// Check device limit (default max 10000)
	count, err := s.repo.CountDevices(tenantId)
	if err != nil {
		return nil, apperror.Wrap(err, "CHECK_DEVICE_COUNT_FAILED", 500, "failed to check device count")
	}
	if count >= 10000 {
		return nil, apperror.ErrQuotaExceeded.WithMessage("device limit reached (max 10000)")
	}

	sn := req.SerialNumber
	d := &device.CpeElement{
		SerialNumber: &sn,
		TenantId:    &tenantId,
	}
	if req.DeviceName != "" {
		d.DeviceName = &req.DeviceName
	}
	if req.DeviceType != "" {
		d.DeviceType = &req.DeviceType
	}
	if req.Product != "" {
		d.Product = &req.Product
	}

	if err := s.repo.CreateDevice(d); err != nil {
		logger.Errorf("Failed to create device: %v", err)
		return nil, apperror.Wrap(err, "CREATE_DEVICE_FAILED", 500, "failed to create device")
	}

	logger.Infof("Device added: SN=%s by user %s", req.SerialNumber, username)

	vo := &RestDeviceVo{
		Id:           d.NeNeid,
		SerialNumber: req.SerialNumber,
		DeviceName:   req.DeviceName,
		DeviceType:   req.DeviceType,
		Product:      req.Product,
		TenantId:    tenantId,
		Status:       "online",
	}
	return vo, nil
}

func (s *service) ModifyDeviceById(c *gin.Context, id int64, req *ModifyRestDeviceRequest) error {
	tenantId := middleware.GetTenantId(c)

	d, err := s.repo.GetDeviceById(id, tenantId)
	if err != nil {
		return apperror.ErrDeviceNotFound
	}

	if req.DeviceName != nil {
		d.DeviceName = req.DeviceName
	}
	if req.DeviceType != nil {
		d.DeviceType = req.DeviceType
	}
	if req.Product != nil {
		d.Product = req.Product
	}

	if err := s.repo.UpdateDevice(d); err != nil {
		logger.Errorf("Failed to modify device %d: %v", id, err)
		return apperror.Wrap(err, "MODIFY_DEVICE_FAILED", 500, "failed to modify device")
	}

	return nil
}

func (s *service) ModifyDeviceBySN(c *gin.Context, req *ModifyRestDeviceBySNRequest) error {
	tenantId := middleware.GetTenantId(c)

	d, err := s.repo.GetDeviceBySN(req.SerialNumber, tenantId)
	if err != nil {
		return apperror.ErrDeviceNotFound
	}

	if req.DeviceName != nil {
		d.DeviceName = req.DeviceName
	}
	if req.DeviceType != nil {
		d.DeviceType = req.DeviceType
	}

	if err := s.repo.UpdateDevice(d); err != nil {
		logger.Errorf("Failed to modify device by SN %s: %v", req.SerialNumber, err)
		return apperror.Wrap(err, "MODIFY_DEVICE_FAILED", 500, "failed to modify device")
	}

	return nil
}

func (s *service) DeleteDevice(c *gin.Context, id int64) error {
	tenantId := middleware.GetTenantId(c)

	if err := s.repo.SoftDeleteDevice(id, tenantId); err != nil {
		logger.Errorf("Failed to delete device %d: %v", id, err)
		return apperror.Wrap(err, "DELETE_DEVICE_FAILED", 500, "failed to delete device")
	}

	return nil
}
