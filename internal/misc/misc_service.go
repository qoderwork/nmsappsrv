package misc

import "time"

// ---------------------------------------------------------------------------
// BatchConfigurationLog
// ---------------------------------------------------------------------------

// ListBatchConfigLogs returns a paginated list of batch configuration logs.
func (s *service) ListBatchConfigLogs(tenantId int, page, pageSize int) ([]BatchConfigurationLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindBatchConfigLogs(tenantId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// MRData
// ---------------------------------------------------------------------------

// ListMRData returns a paginated list of MR data records.
func (s *service) ListMRData(elementId int64, page, pageSize int) ([]MRData, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindMRData(elementId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// NorthReport
// ---------------------------------------------------------------------------

// ListNorthReports returns all north reports for the given license.
func (s *service) ListNorthReports(tenantId int) ([]NorthReport, error) {
	return s.repo.FindNorthReports(tenantId)
}

// CreateNorthReport persists a new north report.
func (s *service) CreateNorthReport(r *NorthReport) error {
	return s.repo.CreateNorthReport(r)
}

// UpdateNorthReport persists changes to an existing north report.
func (s *service) UpdateNorthReport(r *NorthReport) error {
	return s.repo.UpdateNorthReport(r)
}

// DeleteNorthReport removes a north report by ID.
func (s *service) DeleteNorthReport(id int) error {
	return s.repo.DeleteNorthReport(id)
}

// ---------------------------------------------------------------------------
// Radius
// ---------------------------------------------------------------------------

// ListRadius returns all RADIUS configurations for the given tenancy.
func (s *service) ListRadius(tenantId int) ([]Radius, error) {
	return s.repo.FindRadius(tenantId)
}

// SaveRadius inserts or updates a RADIUS configuration.
func (s *service) SaveRadius(r *Radius) error {
	return s.repo.SaveRadius(r)
}

// DeleteRadius removes a RADIUS configuration by ID.
func (s *service) DeleteRadius(id int) error {
	return s.repo.DeleteRadius(id)
}

// ---------------------------------------------------------------------------
// SystemOperatorLog
// ---------------------------------------------------------------------------

// ListOperatorLogs returns a paginated list of operator logs.
func (s *service) ListOperatorLogs(tenantId int, page, pageSize int) ([]SystemOperatorLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindOperatorLogs(tenantId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// UploadFile
// ---------------------------------------------------------------------------

// ListUploadFiles returns a paginated list of uploaded files.
func (s *service) ListUploadFiles(page, pageSize int) ([]UploadFile, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindUploadFiles(offset, pageSize)
}

// CreateUploadFile persists a new upload file record.
func (s *service) CreateUploadFile(f *UploadFile) error {
	return s.repo.CreateUploadFile(f)
}

// DeleteUploadFile removes an upload file by ID.
func (s *service) DeleteUploadFile(id string) error {
	return s.repo.DeleteUploadFile(id)
}

// ---------------------------------------------------------------------------
// AOS Management — TBG
// ---------------------------------------------------------------------------

// ListTBGs returns a paginated list of TBG records.
func (s *service) ListTBGs(tenantId int, req *ListTBGRequest) ([]TBG, int64, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindTBGs(tenantId, req.Name, offset, pageSize)
}

// AddTBG creates a new TBG record.
func (s *service) AddTBG(tenantId int, req *AddTBGRequest) (*TBG, error) {
	now := time.Now()
	tbg := &TBG{
		Name:      &req.Name,
		IP:        &req.IP,
		Port:      &req.Port,
		TenantId: &tenantId,
		CreateTime: &now,
		UpdateTime: &now,
	}
	if err := s.repo.CreateTBG(tbg); err != nil {
		return nil, err
	}
	return tbg, nil
}

// ModifyTBG updates an existing TBG record.
func (s *service) ModifyTBG(req *ModifyTBGRequest) error {
	now := time.Now()
	tbg := &TBG{
		Id:        req.Id,
		UpdateTime: &now,
	}
	if req.Name != "" {
		tbg.Name = &req.Name
	}
	if req.IP != "" {
		tbg.IP = &req.IP
	}
	if req.Port != nil {
		tbg.Port = req.Port
	}
	return s.repo.UpdateTBG(tbg)
}

// DeleteTBGs removes TBG records by IDs.
func (s *service) DeleteTBGs(ids []int64) error {
	return s.repo.DeleteTBGs(ids)
}

// ImportTBGs batch imports TBG records.
func (s *service) ImportTBGs(tenantId int, tbgs []TBG) (int, error) {
	now := time.Now()
	for i := range tbgs {
		tbgs[i].TenantId = &tenantId
		tbgs[i].CreateTime = &now
		tbgs[i].UpdateTime = &now
	}
	if err := s.repo.CreateTBGsFromRows(tbgs); err != nil {
		return 0, err
	}
	return len(tbgs), nil
}

// ---------------------------------------------------------------------------
// AOS Management — PSAPID
// ---------------------------------------------------------------------------

// ListPSAPIDs returns a paginated list of PSAP ID records.
func (s *service) ListPSAPIDs(tenantId int, req *ListPSAPIDRequest) ([]PSAPID, int64, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindPSAPIDs(tenantId, req.PsapId, offset, pageSize)
}

// SyncPSAPIDs synchronizes PSAP ID records from the external source.
func (s *service) SyncPSAPIDs(tenantId int, operator string) (int, error) {
	// In a full implementation this would call an external PSAP service.
	// For now, we create a sync log and return.
	now := time.Now()
	status := 1 // 1=success
	detail := "sync completed"
	log := &PSAPIDSyncLog{
		Operator:   &operator,
		Status:     &status,
		Detail:     &detail,
		CreateTime: &now,
	}
	if err := s.repo.CreatePSAPIDSyncLog(log); err != nil {
		return 0, err
	}
	return 0, nil
}

// ListPSAPIDSyncLogs returns a paginated list of PSAP ID sync logs.
func (s *service) ListPSAPIDSyncLogs(page, pageSize int) ([]PSAPIDSyncLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindPSAPIDSyncLogs(offset, pageSize)
}

// ---------------------------------------------------------------------------
// AOS Management — SpatialFile
// ---------------------------------------------------------------------------

// ListSpatialFileMarkets returns all spatial file markets for a license.
func (s *service) ListSpatialFileMarkets(tenantId int) ([]SpatialFileMarket, error) {
	return s.repo.FindSpatialFileMarkets(tenantId)
}

// GetMarketCoordinates returns PSAP ID coordinates for a given market.
func (s *service) GetMarketCoordinates(marketId int) ([]PSAPID, error) {
	return s.repo.FindMarketCoordinates(marketId)
}
