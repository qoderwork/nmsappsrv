package misc

// ---------------------------------------------------------------------------
// BatchConfigurationLog
// ---------------------------------------------------------------------------

// ListBatchConfigLogs returns a paginated list of batch configuration logs.
func (s *service) ListBatchConfigLogs(tenancyId int, page, pageSize int) ([]BatchConfigurationLog, int64, error) {
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
func (s *service) ListNorthReports(licenseId int) ([]NorthReport, error) {
	return s.repo.FindNorthReports(licenseId)
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
func (s *service) ListRadius(tenancyId int) ([]Radius, error) {
	return s.repo.FindRadius(tenancyId)
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
func (s *service) ListOperatorLogs(tenancyId int, page, pageSize int) ([]SystemOperatorLog, int64, error) {
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
