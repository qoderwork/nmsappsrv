package tenancy

import (
	"fmt"

	"gorm.io/gorm"
	"nmsappsrv/pkg/apperror"
)

// Service provides tenancy management operations
type Service struct {
	repo *Repository
}

// NewService creates a new Service
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// AddTenancy creates a new tenancy
func (s *Service) AddTenancy(req *AddTenancyRequest) (int, error) {
	if req.LicenseName == "" {
		return 0, apperror.ErrInvalidInput.WithMessage("tenancy name cannot be null")
	}

	exists, err := s.repo.ExistsByName(req.LicenseName)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, apperror.ErrConflict.WithMessage("the tenant name is already in use")
	}

	if req.ExpiryDate == nil {
		return 0, apperror.ErrInvalidInput.WithMessage("the expiration time cannot be null")
	}
	if req.UserQuantity < 0 {
		return 0, apperror.ErrInvalidInput.WithMessage("user quantity cannot be smaller than 0")
	}
	if req.EnbQuantity < 0 {
		return 0, apperror.ErrInvalidInput.WithMessage("device quantity cannot be smaller than 0")
	}

	t := &tenancyModel{
		LicenseName:          strPtr(req.LicenseName),
		ExpiryDate:           timeFromMillis(*req.ExpiryDate),
		EnbQuantity:          req.EnbQuantity,
		UserQuantity:         req.UserQuantity,
		ProvinceAbbreviation: strPtr(req.ProvinceAbbreviation),
		VendorCode:           strPtr(req.VendorCode),
		OmcName:              strPtr(req.OmcName),
		Timezone:             strPtr(req.Timezone),
		GnbQuantity:          req.GnbQuantity,
		CpeQuantity:          req.CpeQuantity,
	}

	if req.LogoBase64 != "" {
		t.LogoBase64 = strPtr(req.LogoBase64)
	}

	// First insert to get auto-generated ID
	if err := s.repo.Create(t); err != nil {
		return 0, err
	}

	// Set licenseId = string(id)
	idStr := fmt.Sprintf("%d", t.Id)
	t.LicenseId = &idStr
	if err := s.repo.Save(t); err != nil {
		return 0, err
	}

	return t.Id, nil
}

// UpdateTenancy updates an existing tenancy
func (s *Service) UpdateTenancy(req *UpdateTenancyRequest) (*tenancyModel, error) {
	if req.LicenseName == "" {
		return nil, apperror.ErrInvalidInput.WithMessage("tenancy name cannot be null")
	}
	if req.ExpiryDate == nil {
		return nil, apperror.ErrInvalidInput.WithMessage("the expiration time cannot be null")
	}
	if req.UserQuantity < 0 {
		return nil, apperror.ErrInvalidInput.WithMessage("user quantity cannot be smaller than 0")
	}
	if req.EnbQuantity < 0 {
		return nil, apperror.ErrInvalidInput.WithMessage("device quantity cannot be smaller than 0")
	}

	existing, err := s.repo.FindByID(req.Id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperror.ErrNotFound.WithMessage("user license not found")
		}
		return nil, err
	}

	// Check name uniqueness
	nameExists, err := s.repo.ExistsByNameExcluding(req.LicenseName, req.Id)
	if err != nil {
		return nil, err
	}
	if nameExists {
		return nil, apperror.ErrConflict.WithMessage("license name already exists")
	}

	// Update fields
	existing.LicenseName = strPtr(req.LicenseName)
	existing.ExpiryDate = timeFromMillis(*req.ExpiryDate)
	existing.EnbQuantity = req.EnbQuantity
	existing.UserQuantity = req.UserQuantity
	existing.ProvinceAbbreviation = strPtr(req.ProvinceAbbreviation)
	existing.VendorCode = strPtr(req.VendorCode)
	existing.OmcName = strPtr(req.OmcName)
	existing.Timezone = strPtr(req.Timezone)
	existing.GnbQuantity = req.GnbQuantity
	existing.CpeQuantity = req.CpeQuantity

	if req.LogoBase64 != "" {
		existing.LogoBase64 = strPtr(req.LogoBase64)
	}

	if err := s.repo.Save(existing); err != nil {
		return nil, err
	}
	return existing, nil
}

// ListTenancies returns paginated tenancies
func (s *Service) ListTenancies(query *ListTenancyQuery) ([]TenancyVO, int64, error) {
	page := query.Page
	pageSize := query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	items, total, err := s.repo.List(query.LicenseName, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var result []TenancyVO
	for _, t := range items {
		result = append(result, TenancyVO{
			Id:                   t.Id,
			LicenseName:          strOrEmpty(t.LicenseName),
			LicenseId:            strOrEmpty(t.LicenseId),
			ExpiryDate:           millisFromTime(t.ExpiryDate),
			EnbQuantity:          t.EnbQuantity,
			UserQuantity:         t.UserQuantity,
			ProvinceAbbreviation: strOrEmpty(t.ProvinceAbbreviation),
			VendorCode:           strOrEmpty(t.VendorCode),
			OmcName:              strOrEmpty(t.OmcName),
			Timezone:             strOrEmpty(t.Timezone),
			LogoBase64:           strOrEmpty(t.LogoBase64),
			GnbQuantity:          t.GnbQuantity,
			CpeQuantity:          t.CpeQuantity,
		})
	}

	return result, total, nil
}

// DeleteTenancy deletes a tenancy by ID
func (s *Service) DeleteTenancy(id int) error {
	if id == 0 {
		return apperror.ErrInvalidInput.WithMessage("default tenancy cannot be deleted")
	}
	return s.repo.DeleteByID(id)
}

// ViewTenancy returns a tenancy by ID
func (s *Service) ViewTenancy(id int) (*ViewTenancyResponse, error) {
	t, err := s.repo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperror.ErrNotFound.WithMessage("user license not found")
		}
		return nil, err
	}

	return &ViewTenancyResponse{
		Id:                   t.Id,
		LicenseName:          strOrEmpty(t.LicenseName),
		LicenseId:            strOrEmpty(t.LicenseId),
		ExpiryDate:           millisFromTime(t.ExpiryDate),
		EnbQuantity:          t.EnbQuantity,
		UserQuantity:         t.UserQuantity,
		ProvinceAbbreviation: strOrEmpty(t.ProvinceAbbreviation),
		VendorCode:           strOrEmpty(t.VendorCode),
		OmcName:              strOrEmpty(t.OmcName),
		Timezone:             strOrEmpty(t.Timezone),
		LogoBase64:           strOrEmpty(t.LogoBase64),
		GnbQuantity:          t.GnbQuantity,
		CpeQuantity:          t.CpeQuantity,
	}, nil
}
