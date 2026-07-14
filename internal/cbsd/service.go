package cbsd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/pkg/logger"
)

// Service defines the business-logic contract for CBSD management.
type Service interface {
	ListCbsdInfos(licenseId int, page, pageSize int) ([]CbsdInfo, int64, error)
	GetCbsdInfo(sn string, licenseId int) (*CbsdInfo, error)
	RegisterCbsd(info *CbsdInfo, licenseId int) error
	UpdateCbsdInfo(info *CbsdInfo) error
	DeregisterCbsd(id string) error
	ListCbrsLogs(cbsdId string, logType string, page, pageSize int) ([]CbrsLog, int64, error)
	CreateCertFileSendTask(t *CBSDCertFileSendTask) error
	ListCertFileSendTasks(tenancyId int, page, pageSize int) ([]CBSDCertFileSendTask, int64, error)

	// CBSD lifecycle
	EnableCBSD(id string) error
	DisableCBSD(id string) error
	DeleteCBSD(id string) error

	// SAS protocol
	SpectrumInquiry(licenseId int, req *SpectrumInquiryRequest) (map[string]interface{}, error)
	Grant(cbsdId string, req *GrantRequest) (map[string]interface{}, error)
	Relinquishment(cbsdId string, req *RelinquishmentRequest) (map[string]interface{}, error)
	SasHeartbeat(cbsdId string) (map[string]interface{}, error)

	// SAS operation-state state machine (mirrors Java OperationStateMaintainThread)
	MaintainOperationStates(ctx context.Context) (int, error)

	// Import
	ImportCBSDs(licenseId int, records [][]string) (int, error)

	// Statistics
	ListCBSDStatusCount(licenseId int) ([]CbsdStatusCountItem, error)

	// SAS config
	ListSasConfig(licenseId int) ([]SasConfig, error)
	UpdateSasConfig(cfg *SasConfig) error
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// ListCbsdInfos returns a paginated list of CBSD info records.
func (s *service) ListCbsdInfos(licenseId int, page, pageSize int) ([]CbsdInfo, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindCbsdInfos(licenseId, offset, pageSize)
}

// GetCbsdInfo returns a single CBSD info by serial number.
func (s *service) GetCbsdInfo(sn string, licenseId int) (*CbsdInfo, error) {
	return s.repo.FindCbsdInfoBySN(sn, licenseId)
}

// RegisterCbsd persists a new CBSD registration, stamping the tenant
// license id from the authenticated context so the record is isolated per
// tenant (mirrors Java nms-serv behaviour).
func (s *service) RegisterCbsd(info *CbsdInfo, licenseId int) error {
	if licenseId > 0 {
		info.LicenseId = &licenseId
	}
	return s.repo.Create(info)
}

// UpdateCbsdInfo persists changes to an existing CBSD info.
func (s *service) UpdateCbsdInfo(info *CbsdInfo) error {
	return s.repo.Save(info)
}

// DeregisterCbsd removes a CBSD info by ID.
func (s *service) DeregisterCbsd(id string) error {
	return s.repo.DeleteByID(id)
}

// ListCbrsLogs returns a paginated list of CBRS logs.
func (s *service) ListCbrsLogs(cbsdId string, logType string, page, pageSize int) ([]CbrsLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindCbrsLogs(cbsdId, logType, offset, pageSize)
}

// CreateCertFileSendTask persists a new cert file send task.
func (s *service) CreateCertFileSendTask(t *CBSDCertFileSendTask) error {
	return s.repo.CreateCertFileSendTask(t)
}

// ListCertFileSendTasks returns a paginated list of cert file send tasks.
func (s *service) ListCertFileSendTasks(tenancyId int, page, pageSize int) ([]CBSDCertFileSendTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindCertFileSendTasks(tenancyId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// CBSD lifecycle
// ---------------------------------------------------------------------------

// EnableCBSD enables a CBSD device.
func (s *service) EnableCBSD(id string) error {
	info, err := s.repo.FindCbsdInfoByID(id)
	if err != nil {
		return fmt.Errorf("CBSD not found: %w", err)
	}
	if info.Enable != nil && *info.Enable {
		return fmt.Errorf("CBSD is already enabled")
	}
	return s.repo.UpdateCbsdEnable(id, true)
}

// DisableCBSD disables a CBSD device.
func (s *service) DisableCBSD(id string) error {
	info, err := s.repo.FindCbsdInfoByID(id)
	if err != nil {
		return fmt.Errorf("CBSD not found: %w", err)
	}
	if info.Enable != nil && !*info.Enable {
		return fmt.Errorf("CBSD is already disabled")
	}
	return s.repo.UpdateCbsdEnable(id, false)
}

// DeleteCBSD removes a CBSD record by ID.
func (s *service) DeleteCBSD(id string) error {
	return s.repo.DeleteByID(id)
}

// ---------------------------------------------------------------------------
// SAS protocol operations
// ---------------------------------------------------------------------------

// getSasUrl resolves the active SAS config URL for the given license.
func (s *service) getSasUrl(licenseId int) (string, error) {
	configs, err := s.repo.FindSasConfigs(licenseId)
	if err != nil {
		return "", fmt.Errorf("failed to load SAS config: %w", err)
	}
	for _, cfg := range configs {
		if cfg.Enabled {
			return cfg.SasUrl, nil
		}
	}
	return "", fmt.Errorf("no enabled SAS config found for license %d", licenseId)
}

// callSasApi sends a POST request to the SAS API and returns the parsed response.
func (s *service) callSasApi(sasUrl, path string, body interface{}) (map[string]interface{}, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SAS request: %w", err)
	}

	url := sasUrl + path
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("SAS API call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read SAS response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse SAS response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("SAS API %s returned status %d: %s", path, resp.StatusCode, string(respBody))
		return result, fmt.Errorf("SAS API returned status %d", resp.StatusCode)
	}

	return result, nil
}

// SpectrumInquiry sends a spectrum inquiry request to the SAS.
func (s *service) SpectrumInquiry(licenseId int, req *SpectrumInquiryRequest) (map[string]interface{}, error) {
	sasUrl, err := s.getSasUrl(licenseId)
	if err != nil {
		return nil, err
	}
	return s.callSasApi(sasUrl, "/v1.2/spectrumInquiry", req)
}

// Grant sends a grant request to the SAS for a specific CBSD.
func (s *service) Grant(cbsdId string, req *GrantRequest) (map[string]interface{}, error) {
	info, err := s.repo.FindCbsdInfoByID(cbsdId)
	if err != nil {
		return nil, fmt.Errorf("CBSD not found: %w", err)
	}
	licenseId := 0
	if info.LicenseId != nil {
		licenseId = *info.LicenseId
	}
	sasUrl, err := s.getSasUrl(licenseId)
	if err != nil {
		return nil, err
	}

	grantReq := map[string]interface{}{
		"cbsdId":        derefString(info.CbsdID),
		"lowFrequency":  req.LowFrequency,
		"highFrequency": req.HighFrequency,
		"maxEirp":       req.MaxEirp,
	}
	result, err := s.callSasApi(sasUrl, "/v1.2/grant", grantReq)
	if err != nil {
		return nil, err
	}

	// Update grant ID and advance operation-state to GRANTED on success,
	// persisting the SAS-returned expire times so the maintainer thread can
	// drive future transitions (mirrors Java grant flow).
	if grantId, ok := result["grantId"].(string); ok && grantId != "" {
		now := time.Now()
		info.GrantID = &grantId
		info.LastGrantTime = &now
		info.OperationState = strPtr(OpStateGranted)
		if gexp := sasTimeString(result["grantExpireTime"]); gexp != "" {
			info.GrantExpireTime = &gexp
		}
		if texp := sasTimeString(result["transmitExpireTime"]); texp != "" {
			info.TransmitExpireTime = &texp
		}
		if err := s.repo.Save(info); err != nil {
			return nil, fmt.Errorf("failed to persist grant state: %w", err)
		}
	}

	return result, nil
}

// Relinquishment sends a relinquishment request to the SAS for a specific CBSD.
func (s *service) Relinquishment(cbsdId string, req *RelinquishmentRequest) (map[string]interface{}, error) {
	info, err := s.repo.FindCbsdInfoByID(cbsdId)
	if err != nil {
		return nil, fmt.Errorf("CBSD not found: %w", err)
	}
	licenseId := 0
	if info.LicenseId != nil {
		licenseId = *info.LicenseId
	}
	sasUrl, err := s.getSasUrl(licenseId)
	if err != nil {
		return nil, err
	}

	relReq := map[string]interface{}{
		"cbsdId":  derefString(info.CbsdID),
		"grantId": req.GrantId,
	}
	result, err := s.callSasApi(sasUrl, "/v1.2/relinquishment", relReq)
	if err != nil {
		return nil, err
	}

	// Clear grant and revert operation-state to REGISTERED after relinquishment.
	emptyGrant := ""
	info.GrantID = &emptyGrant
	info.OperationState = strPtr(OpStateRegistered)
	info.GrantExpireTime = nil
	info.TransmitExpireTime = nil
	s.repo.Save(info)

	return result, nil
}

// ---------------------------------------------------------------------------
// SAS heartbeat (DP -> SAS) and operation-state state machine
// ---------------------------------------------------------------------------

// SasHeartbeat sends a DP->SAS heartbeat for a specific CBSD and advances its
// operation-state based on the SAS response code (mirrors Java
// HeartbeatRequestService): respCode 0 => AUTHORIZED, 501 => SUSPENDED,
// 500 => REGISTERED (and an inquiry/registration is implied).
func (s *service) SasHeartbeat(cbsdId string) (map[string]interface{}, error) {
	info, err := s.repo.FindCbsdInfoByID(cbsdId)
	if err != nil {
		return nil, fmt.Errorf("CBSD not found: %w", err)
	}
	licenseId := 0
	if info.LicenseId != nil {
		licenseId = *info.LicenseId
	}
	sasUrl, err := s.getSasUrl(licenseId)
	if err != nil {
		return nil, err
	}

	hbReq := map[string]interface{}{
		"cbsdId":  derefString(info.CbsdID),
		"grantId": derefString(info.GrantID),
	}
	result, err := s.callSasApi(sasUrl, "/v1.2/heartbeat", hbReq)
	if err != nil {
		return nil, err
	}

	code := sasResponseCode(result)
	switch code {
	case 0:
		info.OperationState = strPtr(OpStateAuthorized)
	case 501:
		info.OperationState = strPtr(OpStateSuspended)
	case 500:
		info.OperationState = strPtr(OpStateRegistered)
	default:
		// Unknown code: leave state unchanged but still persist.
		logger.Warnf("SasHeartbeat: unexpected response code %d for %s", code, cbsdId)
	}
	if err := s.repo.Save(info); err != nil {
		return nil, fmt.Errorf("failed to persist heartbeat state: %w", err)
	}

	return result, nil
}

// MaintainOperationStates scans CBSDs with active grants (GRANTED/AUTHORIZED)
// and advances their operation-state when grant/transmit expire times have
// elapsed. Expired grants revert to REGISTERED and trigger a re-grant; expired
// transmits (while AUTHORIZED) revert to GRANTED and trigger a re-heartbeat.
// Mirrors Java OperationStateMaintainThread. Returns the number of transitions
// applied. It is safe to call on a schedule (idempotent).
func (s *service) MaintainOperationStates(ctx context.Context) (int, error) {
	cbsds, err := s.repo.FindCbsdInfosByStates([]string{OpStateGranted, OpStateAuthorized})
	if err != nil {
		return 0, fmt.Errorf("failed to list CBSDs for state maintenance: %w", err)
	}

	now := time.Now()
	transitioned := 0
	for i := range cbsds {
		if ctx.Err() != nil {
			return transitioned, ctx.Err()
		}
		c := cbsds[i]
		plan := decideTransition(&c, now)
		if plan.newState == "" {
			continue
		}

		fields := map[string]interface{}{"operation_state": plan.newState}
		if plan.clearGrant {
			fields["grant_id"] = ""
			fields["grant_expire_time"] = nil
		}
		if plan.clearTransmit {
			fields["transmit_expire_time"] = nil
		}
		if err := s.repo.UpdateFields(c.Id, fields); err != nil {
			logger.Errorf("maintain: failed to update state for %s: %v", c.Id, err)
			continue
		}

		s.writeCbrsLog(c.CbsdID, plan.logType, plan.logStatus)

		if plan.reGrant {
			s.retryGrant(&c)
		}
		if plan.reHeartbeat {
			if _, err := s.SasHeartbeat(c.Id); err != nil {
				logger.Errorf("maintain: re-heartbeat failed for %s: %v", c.Id, err)
			}
		}
		transitioned++
	}
	return transitioned, nil
}

// retryGrant re-issues a grant using the CBSD's stored frequency/EIRP params.
func (s *service) retryGrant(c *CbsdInfo) {
	if c.LowFrequency == nil {
		logger.Warnf("maintain: skip re-grant for %s (no low frequency)", c.Id)
		return
	}
	req := &GrantRequest{
		LowFrequency:  derefInt64(c.LowFrequency),
		HighFrequency: derefInt64(c.HighFrequency),
		MaxEirp:       derefFloat64(c.MaxEirp),
	}
	if _, err := s.Grant(c.Id, req); err != nil {
		logger.Errorf("maintain: re-grant failed for %s: %v", c.Id, err)
	}
}

// writeCbrsLog appends a cbrs_log row (heartbeat/grant lifecycle events).
func (s *service) writeCbrsLog(cbsdID *string, logType, status string) {
	id := ""
	if cbsdID != nil {
		id = *cbsdID
	}
	log := &CbrsLog{
		LogType:  &logType,
		Status:   &status,
		CbsdId:   &id,
		LogTime:  timePtr(time.Now()),
	}
	if err := s.repo.CreateCbrsLog(log); err != nil {
		logger.Errorf("maintain: failed to write cbrs log: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Import
// ---------------------------------------------------------------------------

// ImportCBSDs imports CBSD records from parsed CSV data.
// Each record is expected as [serial_number, cbsd_category, latitude, longitude, height, vendor, model].
func (s *service) ImportCBSDs(licenseId int, records [][]string) (int, error) {
	if len(records) == 0 {
		return 0, fmt.Errorf("no records to import")
	}

	now := time.Now()
	var infos []CbsdInfo
	for _, row := range records {
		if len(row) < 2 {
			continue
		}
		info := CbsdInfo{
			CbsdSerialNumber: strPtr(row[0]),
			LicenseId:        &licenseId,
			LastRegistrationTime: &now,
		}
		if len(row) > 1 {
			info.CbsdCategory = strPtr(row[1])
		}
		if len(row) > 2 {
			info.Latitude = strPtr(row[2])
		}
		if len(row) > 3 {
			info.Longitude = strPtr(row[3])
		}
		if len(row) > 4 {
			info.Height = strPtr(row[4])
		}
		if len(row) > 5 {
			info.Vendor = strPtr(row[5])
		}
		if len(row) > 6 {
			info.Model = strPtr(row[6])
		}
		infos = append(infos, info)
	}

	if len(infos) == 0 {
		return 0, fmt.Errorf("no valid records found in import data")
	}

	if err := s.repo.BulkCreateCbsdInfos(infos); err != nil {
		return 0, fmt.Errorf("failed to import CBSD records: %w", err)
	}

	return len(infos), nil
}

// ---------------------------------------------------------------------------
// Statistics
// ---------------------------------------------------------------------------

// ListCBSDStatusCount returns per-operation_state counts of CBSD records.
func (s *service) ListCBSDStatusCount(licenseId int) ([]CbsdStatusCountItem, error) {
	return s.repo.CountCbsdByStatus(licenseId)
}

// ---------------------------------------------------------------------------
// SAS config
// ---------------------------------------------------------------------------

// ListSasConfig returns all SAS configs for a license.
func (s *service) ListSasConfig(licenseId int) ([]SasConfig, error) {
	return s.repo.FindSasConfigs(licenseId)
}

// UpdateSasConfig persists changes to a SAS config.
func (s *service) UpdateSasConfig(cfg *SasConfig) error {
	existing, err := s.repo.FindSasConfigByID(cfg.Id)
	if err != nil {
		return fmt.Errorf("SAS config not found: %w", err)
	}
	cfg.CreateTime = existing.CreateTime
	cfg.UpdateTime = time.Now()
	return s.repo.UpdateSasConfig(cfg)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func strPtr(s string) *string {
	return &s
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func derefFloat64(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func timePtr(t time.Time) *time.Time {
	return &t
}
