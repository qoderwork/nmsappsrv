package restapi

import (
	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"

	"github.com/gin-gonic/gin"
)

// ============================
// TBG (femtocell) operations
// ============================

func (s *service) ListTBGs(c *gin.Context, offset, limit int) ([]TBGVo, int64, error) {
	tenantId := middleware.GetTenantId(c)

	tbgs, total, err := s.repo.ListTBGs(tenantId, offset, limit)
	if err != nil {
		return nil, 0, apperror.Wrap(err, "LIST_TBGS_FAILED", 500, "failed to list TBGs")
	}

	var result []TBGVo
	for _, t := range tbgs {
		vo := TBGVo{
			Id:            t.Id,
			SerialNumber:  derefStr(t.SerialNumber),
			Band:          derefStr(t.Band),
			Address:       derefStr(t.Address),
			WanMacAddress: derefStr(t.WanMacAddress),
		}
		result = append(result, vo)
	}

	return result, total, nil
}

func (s *service) GetTBGBySN(sn string) (*TBGVo, error) {
	tbg, err := s.repo.GetTBGBySN(sn)
	if err != nil {
		return nil, apperror.ErrNotFound.WithMessage("TBG device not found")
	}

	return &TBGVo{
		Id:            tbg.Id,
		SerialNumber:  derefStr(tbg.SerialNumber),
		Band:          derefStr(tbg.Band),
		Address:       derefStr(tbg.Address),
		WanMacAddress: derefStr(tbg.WanMacAddress),
	}, nil
}

func (s *service) GetTBGByWanMac(mac string) (*TBGVo, error) {
	tbg, err := s.repo.GetTBGByWanMac(mac)
	if err != nil {
		return nil, apperror.ErrNotFound.WithMessage("TBG device not found")
	}

	return &TBGVo{
		Id:            tbg.Id,
		SerialNumber:  derefStr(tbg.SerialNumber),
		Band:          derefStr(tbg.Band),
		Address:       derefStr(tbg.Address),
		WanMacAddress: derefStr(tbg.WanMacAddress),
	}, nil
}

func (s *service) AddTBGs(c *gin.Context, reqs []AddTBGRequest) ([]TBGVo, error) {
	tenantId := middleware.GetTenantId(c)
	username := middleware.GetUsername(c)

	if len(reqs) > 100 {
		return nil, apperror.ErrInvalidInput.WithMessage("batch size exceeds maximum of 100")
	}

	var tbgs []TBGDevice
	for _, req := range reqs {
		// Check for duplicate SN
		existing, _ := s.repo.GetTBGBySN(req.SerialNumber)
		if existing != nil {
			return nil, apperror.ErrConflict.WithMessage("TBG with serial number " + req.SerialNumber + " already exists")
		}

		sn := req.SerialNumber
		tbg := TBGDevice{
			SerialNumber: &sn,
			TenantId:    &tenantId,
		}
		if req.Band != "" {
			tbg.Band = &req.Band
		}
		if req.Address != "" {
			tbg.Address = &req.Address
		}
		if req.WanMacAddress != "" {
			tbg.WanMacAddress = &req.WanMacAddress
		}
		tbgs = append(tbgs, tbg)
	}

	if err := s.repo.CreateTBGs(tbgs); err != nil {
		logger.Errorf("Failed to create TBG devices: %v", err)
		return nil, apperror.Wrap(err, "CREATE_TBG_FAILED", 500, "failed to create TBG devices")
	}

	logger.Infof("Created %d TBG devices by user %s", len(tbgs), username)

	var result []TBGVo
	for _, t := range tbgs {
		vo := TBGVo{
			Id:            t.Id,
			SerialNumber:  derefStr(t.SerialNumber),
			Band:          derefStr(t.Band),
			Address:       derefStr(t.Address),
			WanMacAddress: derefStr(t.WanMacAddress),
		}
		result = append(result, vo)
	}

	return result, nil
}

func (s *service) ModifyTBGs(c *gin.Context, reqs []ModifyTBGRequest) error {
	username := middleware.GetUsername(c)

	for _, req := range reqs {
		tbg, err := s.repo.GetTBGBySN(req.SerialNumber)
		if err != nil {
			return apperror.ErrNotFound.WithMessage("TBG with serial number " + req.SerialNumber + " not found")
		}

		if req.Band != nil {
			tbg.Band = req.Band
		}
		if req.Address != nil {
			tbg.Address = req.Address
		}
		if req.WanMacAddress != nil {
			tbg.WanMacAddress = req.WanMacAddress
		}

		if err := s.repo.Save(tbg); err != nil {
			logger.Errorf("Failed to update TBG %s: %v", req.SerialNumber, err)
			return apperror.Wrap(err, "UPDATE_TBG_FAILED", 500, "failed to update TBG device " + req.SerialNumber)
		}
	}

	logger.Infof("Modified %d TBG devices by user %s", len(reqs), username)
	return nil
}

func (s *service) DeleteTBGs(c *gin.Context, req *DeleteTBGRequest) error {
	username := middleware.GetUsername(c)

	if len(req.SerialNumbers) > 100 {
		return apperror.ErrInvalidInput.WithMessage("batch size exceeds maximum of 100")
	}

	if err := s.repo.DeleteTBGsBySNs(req.SerialNumbers); err != nil {
		logger.Errorf("Failed to delete TBG devices: %v", err)
		return apperror.Wrap(err, "DELETE_TBG_FAILED", 500, "failed to delete TBG devices")
	}

	logger.Infof("Deleted %d TBG devices by user %s", len(req.SerialNumbers), username)
	return nil
}
