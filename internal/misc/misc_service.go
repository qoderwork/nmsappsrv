package misc

// ---------------------------------------------------------------------------
// BatchConfigurationLog
// ---------------------------------------------------------------------------

// ListBatchConfigLogs returns a paginated list of batch configuration logs.
func (s *Service) ListBatchConfigLogs(tenancyId int, page, pageSize int) ([]BatchConfigurationLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindBatchConfigLogs(tenancyId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// MRData
// ---------------------------------------------------------------------------

// ListMRData returns a paginated list of MR data records.
func (s *Service) ListMRData(elementId int64, page, pageSize int) ([]MRData, int64, error) {
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
func (s *Service) ListNorthReports(licenseId int) ([]NorthReport, error) {
	return s.repo.FindNorthReports(licenseId)
}

// CreateNorthReport persists a new north report.
func (s *Service) CreateNorthReport(r *NorthReport) error {
	return s.repo.CreateNorthReport(r)
}

// UpdateNorthReport persists changes to an existing north report.
func (s *Service) UpdateNorthReport(r *NorthReport) error {
	return s.repo.UpdateNorthReport(r)
}

// DeleteNorthReport removes a north report by ID.
func (s *Service) DeleteNorthReport(id int) error {
	return s.repo.DeleteNorthReport(id)
}

// ---------------------------------------------------------------------------
// Radius
// ---------------------------------------------------------------------------

// ListRadius returns all RADIUS configurations for the given tenancy.
func (s *Service) ListRadius(tenancyId int) ([]Radius, error) {
	return s.repo.FindRadius(tenancyId)
}

// SaveRadius inserts or updates a RADIUS configuration.
func (s *Service) SaveRadius(r *Radius) error {
	return s.repo.SaveRadius(r)
}

// DeleteRadius removes a RADIUS configuration by ID.
func (s *Service) DeleteRadius(id int) error {
	return s.repo.DeleteRadius(id)
}

// ---------------------------------------------------------------------------
// SystemOperatorLog
// ---------------------------------------------------------------------------

// ListOperatorLogs returns a paginated list of operator logs.
func (s *Service) ListOperatorLogs(tenancyId int, page, pageSize int) ([]SystemOperatorLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindOperatorLogs(tenancyId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// UploadFile
// ---------------------------------------------------------------------------

// ListUploadFiles returns a paginated list of uploaded files.
func (s *Service) ListUploadFiles(page, pageSize int) ([]UploadFile, int64, error) {
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
func (s *Service) CreateUploadFile(f *UploadFile) error {
	return s.repo.CreateUploadFile(f)
}

// DeleteUploadFile removes an upload file by ID.
func (s *Service) DeleteUploadFile(id string) error {
	return s.repo.DeleteUploadFile(id)
}
