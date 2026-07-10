package restapi

import (
	"fmt"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/logger"

	"github.com/gin-gonic/gin"
)

// ============================
// TBG (femtocell) operations
// ============================

func (s *Service) ListTBGs(c *gin.Context, offset, limit int) ([]TBGVo, int64, error) {
	licenseId := middleware.GetLicenseId(c)

	tbgs, total, err := s.repo.ListTBGs(licenseId, offset, limit)
	if err != nil {
		return nil, 0, err
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

func (s *Service) GetTBGBySN(sn string) (*TBGVo, error) {
	tbg, err := s.repo.GetTBGBySN(sn)
	if err != nil {
		return nil, fmt.Errorf("TBG device not found")
	}

	return &TBGVo{
		Id:            tbg.Id,
		SerialNumber:  derefStr(tbg.SerialNumber),
		Band:          derefStr(tbg.Band),
		Address:       derefStr(tbg.Address),
		WanMacAddress: derefStr(tbg.WanMacAddress),
	}, nil
}

func (s *Service) GetTBGByWanMac(mac string) (*TBGVo, error) {
	tbg, err := s.repo.GetTBGByWanMac(mac)
	if err != nil {
		return nil, fmt.Errorf("TBG device not found")
	}

	return &TBGVo{
		Id:            tbg.Id,
		SerialNumber:  derefStr(tbg.SerialNumber),
		Band:          derefStr(tbg.Band),
		Address:       derefStr(tbg.Address),
		WanMacAddress: derefStr(tbg.WanMacAddress),
	}, nil
}

func (s *Service) AddTBGs(c *gin.Context, reqs []AddTBGRequest) ([]TBGVo, error) {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)

	if len(reqs) > 100 {
		return nil, fmt.Errorf("batch size exceeds maximum of 100")
	}

	var tbgs []TBGDevice
	for _, req := range reqs {
		// Check for duplicate SN
		existing, _ := s.repo.GetTBGBySN(req.SerialNumber)
		if existing != nil {
			return nil, fmt.Errorf("TBG with serial number %s already exists", req.SerialNumber)
		}

		sn := req.SerialNumber
		tbg := TBGDevice{
			SerialNumber: &sn,
			LicenseId:    &licenseId,
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
		return nil, fmt.Errorf("failed to create TBG devices")
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

func (s *Service) ModifyTBGs(c *gin.Context, reqs []ModifyTBGRequest) error {
	username := middleware.GetUsername(c)

	for _, req := range reqs {
		tbg, err := s.repo.GetTBGBySN(req.SerialNumber)
		if err != nil {
			return fmt.Errorf("TBG with serial number %s not found", req.SerialNumber)
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

		if err := s.repo.UpdateTBG(tbg); err != nil {
			logger.Errorf("Failed to update TBG %s: %v", req.SerialNumber, err)
			return fmt.Errorf("failed to update TBG device %s", req.SerialNumber)
		}
	}

	logger.Infof("Modified %d TBG devices by user %s", len(reqs), username)
	return nil
}

func (s *Service) DeleteTBGs(c *gin.Context, req *DeleteTBGRequest) error {
	username := middleware.GetUsername(c)

	if len(req.SerialNumbers) > 100 {
		return fmt.Errorf("batch size exceeds maximum of 100")
	}

	if err := s.repo.DeleteTBGsBySNs(req.SerialNumbers); err != nil {
		logger.Errorf("Failed to delete TBG devices: %v", err)
		return fmt.Errorf("failed to delete TBG devices")
	}

	logger.Infof("Deleted %d TBG devices by user %s", len(req.SerialNumbers), username)
	return nil
}
