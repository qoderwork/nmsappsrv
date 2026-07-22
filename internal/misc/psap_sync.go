package misc

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"nmsappsrv/pkg/logger"
)

// ---------------------------------------------------------------------------
// PSAP sync (mirrors Java AOSManagementServiceImpl.syncPSADID)
//
// This lives in the misc package deliberately: misc cannot import
// ztp/external (that would create an import cycle — external already imports
// misc). The Spectrum reverse-geocode and GMLC modify clients below are
// self-contained net/http implementations that read the ZTP setting via
// GetZTPSetting, exactly like the engine in ztp/external.
// ---------------------------------------------------------------------------

// e911CancelDTO mirrors external.CancelDTO — the per-device registration
// record persisted in cpe_element.e911_data. Declared locally to avoid the
// misc→ztp/external import cycle. Only the field we need (gmlcIdentifier) is
// read here.
type e911CancelDTO struct {
	GmlcIdentifier string `json:"gmlcIdentifier"`
}

// psapSyncElement is the cpe_element projection needed for PSAP sync.
type psapSyncElement struct {
	NeNeid    int64   `gorm:"column:ne_neid"`
	Latitude  *string `gorm:"column:latitude"`
	Longitude *string `gorm:"column:longitude"`
	PsapId    *string `gorm:"column:psap_id"`
	E911Data  *string `gorm:"column:e911_data"`
}

func (psapSyncElement) TableName() string { return "cpe_element" }

// SyncPSAPIDs reverse-geocodes each candidate device via Spectrum Spatial,
// updates its psap_id on cpe_element, and pushes the new psap_id to GMLC for
// devices that were previously registered (progress >= 1 + e911_data present).
// It mirrors Java's syncPSADID result codes: a PSAPIDSyncLog with status 1
// (success) or 2 (failure) is always written. The returned count is the number
// of devices whose psap_id actually changed.
func (s *service) SyncPSAPIDs(req *SyncPSAPIDRequest, operator string) (count int, err error) {
	ctx := context.Background()
	var (
		status = 1
		detail string
	)
	defer func() {
		now := time.Now()
		op := operator
		st := status
		dt := detail
		if dt == "" {
			dt = "sync completed"
		}
		log := &PSAPIDSyncLog{Operator: &op, Status: &st, Detail: &dt, CreateTime: &now}
		if lgErr := s.repo.CreatePSAPIDSyncLog(log); lgErr != nil {
			logger.Warnf("psap sync: failed to write sync log: %v", lgErr)
		}
	}()

	setting, gErr := s.GetZTPSetting()
	if gErr != nil {
		status, detail, err = 2, gErr.Error(), gErr
		return
	}
	if setting == nil {
		detail = "ztp_config not found; nothing to sync"
		return
	}

	candidates, rErr := s.resolvePSAPSyncElements(req)
	if rErr != nil {
		status, detail, err = 2, rErr.Error(), rErr
		return
	}
	if len(candidates) == 0 {
		detail = "no devices to synchronize"
		return
	}

	spectrum := newPsapSpectrumClient(setting.SpectrumSpatial)
	gmlc := newPsapGmlcClient(setting.GMLC)

	updatePsadId := 0
	updateGMLC := 0
	gmlcQueue := make([]psapSyncElement, 0, len(candidates))

	// Step 1: reverse-geocode + update psap_id.
	if spectrum.Enabled() {
		for i := range candidates {
			dev := &candidates[i]
			lat, lng := parseCoordPair(dev.Latitude, dev.Longitude)
			if lat == 0 && lng == 0 {
				continue
			}
			psapID, rgErr := spectrum.ReverseGeocode(ctx, lat, lng)
			if rgErr != nil {
				status, detail, err = 2, "Failed to reache Spectrum Interface: " + rgErr.Error(), rgErr
				return
			}
			if psapID == "" {
				status, detail, err = 2, "Failed to reache Spectrum Interface: empty psapId", fmt.Errorf("empty psapId")
				return
			}
			if dev.PsapId != nil && *dev.PsapId == psapID {
				continue // unchanged
			}
			if uErr := s.repo.DB().Table("cpe_element").
				Where("ne_neid = ?", dev.NeNeid).
				Update("psap_id", psapID).Error; uErr != nil {
				status, detail, err = 2, uErr.Error(), uErr
				return
			}
			dev.PsapId = &psapID
			updatePsadId++
			gmlcQueue = append(gmlcQueue, *dev)
		}
	} else {
		logger.Infof("psap sync: spectrum spatial not configured; skipping reverse-geocode")
	}

	// Step 2: push the new psap_id to GMLC for previously-registered cells.
	if gmlc.Enabled() && len(gmlcQueue) > 0 {
		for _, dev := range gmlcQueue {
			progress, pErr := s.findZTPLogProgress(dev.NeNeid)
			if pErr != nil {
				logger.Warnf("psap sync: element %d ztp_log query failed: %v", dev.NeNeid, pErr)
				continue
			}
			if progress == nil || *progress < 1 {
				continue
			}
			if dev.E911Data == nil || *dev.E911Data == "" {
				continue
			}
			var cancel e911CancelDTO
			if jErr := json.Unmarshal([]byte(*dev.E911Data), &cancel); jErr != nil {
				continue
			}
			if cancel.GmlcIdentifier == "" {
				continue
			}
			if uErr := gmlc.UpdateCell(ctx, cancel.GmlcIdentifier, strOrEmpty(dev.PsapId)); uErr != nil {
				status, detail, err = 2, "Failed to reache GMLC: " + uErr.Error(), uErr
				return
			}
			updateGMLC++
		}
	}

	count = updatePsadId
	detail = fmt.Sprintf("The number of device needs to be synchronized: %d; PSAPID changes the number of devices: %d; Successfully synchronized to the number of GMLC devices: %d",
		len(candidates), updatePsadId, updateGMLC)
	return
}

// resolvePSAPSyncElements collects the devices to sync by scope.
func (s *service) resolvePSAPSyncElements(req *SyncPSAPIDRequest) ([]psapSyncElement, error) {
	q := s.repo.DB().Table("cpe_element").Where("deleted = ?", false)
	switch req.Scope {
	case "deviceGroup":
		if len(req.DeviceGroupIds) == 0 {
			return nil, nil
		}
		q = q.Where("device_group_id IN (?)", req.DeviceGroupIds)
	case "element":
		if len(req.ElementIds) == 0 {
			return nil, nil
		}
		q = q.Where("ne_neid IN (?)", req.ElementIds)
	case "market":
		if len(req.Markets) == 0 {
			return nil, nil
		}
		q = q.Where("market IN (?)", req.Markets)
	default:
		// empty scope → all non-deleted devices (see SyncPSAPIDRequest doc).
	}
	if req.TenantId != 0 {
		q = q.Where("tenant_id = ?", req.TenantId)
	}
	var rows []psapSyncElement
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// findZTPLogProgress returns the latest ztp_log progress for an element, or
// nil if the device has no log yet.
func (s *service) findZTPLogProgress(elementId int64) (*int, error) {
	var progs []int
	if err := s.repo.DB().Table("ztp_log").
		Where("element_id = ?", elementId).
		Order("id DESC").Limit(1).
		Pluck("progress", &progs).Error; err != nil {
		return nil, err
	}
	if len(progs) == 0 {
		return nil, nil
	}
	p := progs[0]
	return &p, nil
}

// parseCoordPair converts the (string) latitude/longitude columns to floats.
func parseCoordPair(lat, lng *string) (float64, float64) {
	var outLat, outLng float64
	if lat != nil {
		if v, err := strconv.ParseFloat(strings.TrimSpace(*lat), 64); err == nil {
			outLat = v
		}
	}
	if lng != nil {
		if v, err := strconv.ParseFloat(strings.TrimSpace(*lng), 64); err == nil {
			outLng = v
		}
	}
	return outLat, outLng
}

// ---------------------------------------------------------------------------
// Spectrum Spatial reverse-geocode client (self-contained)
// ---------------------------------------------------------------------------

type psapSpectrumClient struct {
	setting *SpectrumSpatialSetting
	http    *http.Client
}

func newPsapSpectrumClient(s *SpectrumSpatialSetting) *psapSpectrumClient {
	return &psapSpectrumClient{setting: s, http: &http.Client{Timeout: 5 * time.Second}}
}

func (c *psapSpectrumClient) Enabled() bool {
	return c.setting != nil && c.setting.ReverseGeoCodeURL != nil && *c.setting.ReverseGeoCodeURL != ""
}

type psapSpectrumLocation struct {
	PsapID string `json:"psapId"`
}

// ReverseGeocode returns the PSAP id for a GPS fix. Mirrors Java's
// SpectrumSpatialInterfaceHelper.getAddressResponse: GET
// {reverseGeoCodeUrl}/CovCheck_FEMTO_Call/results.json?Data.Latitude&Data.Longitude,
// retry deleteRetryTimes (default 1).
func (c *psapSpectrumClient) ReverseGeocode(ctx context.Context, lat, lng float64) (string, error) {
	base := strings.TrimRight(*c.setting.ReverseGeoCodeURL, "/")
	u := fmt.Sprintf("%s/CovCheck_FEMTO_Call/results.json?Data.Latitude=%v&Data.Longitude=%v", base, lat, lng)

	retry := 1
	if c.setting.DeleteRetryTimes != nil && *c.setting.DeleteRetryTimes > 0 {
		retry = *c.setting.DeleteRetryTimes
	}

	var lastErr error
	for i := 0; i < retry; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return "", err
		}
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		var env struct {
			Output     []psapSpectrumLocation `json:"Output"`
			OutputPort []psapSpectrumLocation `json:"output_port"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			return "", fmt.Errorf("parse spectrum response: %w", err)
		}
		if len(env.Output) > 0 {
			return env.Output[0].PsapID, nil
		}
		if len(env.OutputPort) > 0 {
			return env.OutputPort[0].PsapID, nil
		}
		return "", fmt.Errorf("spectrum returned no location")
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("spectrum reverse-geocode failed after %d attempt(s)", retry)
	}
	return "", lastErr
}

// ---------------------------------------------------------------------------
// GMLC modify client (self-contained) — pushes the updated psap_id to an
// already-registered cell, mirroring Java GMLCHelper.updateCell.
// ---------------------------------------------------------------------------

type psapGmlcClient struct {
	setting *ExternalEndpointSetting
	http    *http.Client
}

func newPsapGmlcClient(s *ExternalEndpointSetting) *psapGmlcClient {
	return &psapGmlcClient{setting: s, http: &http.Client{Timeout: 5 * time.Second}}
}

func (c *psapGmlcClient) Enabled() bool {
	return c.setting != nil && strOrEmpty(c.setting.URL) != "" &&
		strOrEmpty(c.setting.Username) != "" && strOrEmpty(c.setting.Password) != ""
}

// GMLC modifyRequest envelope (SPML, GMLC_NSR namespace). Mirrors Java
// GMLCUpdateDTO.createXML. No SOAP envelope is used, matching Go's existing
// GMLC addRequest client in ztp/external (the GMLC endpoint reads the bare
// spml document).
type psapGmlcModifyRequest struct {
	XMLName               xml.Name             `xml:"spml:modifyRequest"`
	XmlnsSpml             string               `xml:"xmlns:spml,attr"`
	XmlnsGmlc             string               `xml:"xmlns:gmlc,attr"`
	XmlnsXsi              string               `xml:"xmlns:xsi,attr"`
	RequestID             string               `xml:"requestID,attr"`
	ReturnResultingObject string               `xml:"returnResultingObject,attr"`
	Version               string               `xml:"version"`
	Identifier            string               `xml:"identifier"`
	Modification          psapGmlcModification `xml:"modification"`
}

type psapGmlcModification struct {
	Operation   string                 `xml:"operation,attr"`
	ValueObject psapGmlcValueObject    `xml:"valueObject"`
}

type psapGmlcValueObject struct {
	XsiType    string `xml:"xsi:type,attr"`
	Identifier string `xml:"identifier"`
	PsapID     string `xml:"psapId"`
}

// UpdateCell pushes the new psap_id to the cell identified by gmlcIdentifier.
// Returns nil on HTTP 2xx; otherwise an error (Java throws RuntimeException
// "Failed to reache GMLC").
func (c *psapGmlcClient) UpdateCell(ctx context.Context, identifier, psapID string) error {
	if !c.Enabled() {
		return nil
	}
	doc := psapGmlcModifyRequest{
		XmlnsSpml:             "urn:siemens:names:prov:gw:SPML:2:0",
		XmlnsGmlc:             "urn:siemens:names:prov:gw:GMLC_NSR:1:0",
		XmlnsXsi:              "http://www.w3.org/2001/XMLSchema-instance",
		RequestID:             uuid.New().String(),
		ReturnResultingObject: "full",
		Version:               "GMLC_NSR_v10",
		Identifier:            identifier,
		Modification: psapGmlcModification{
			Operation: "setoradd",
			ValueObject: psapGmlcValueObject{
				XsiType:    "gmlc:GMLCCell",
				Identifier: identifier,
				PsapID:     psapID,
			},
		},
	}
	raw, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal gmlc modify: %w", err)
	}
	body := append([]byte(xml.Header), raw...)

	retry := 1
	if c.setting.DeleteRetryTimes != nil && *c.setting.DeleteRetryTimes > 0 {
		retry = *c.setting.DeleteRetryTimes
	}

	u := strings.TrimRight(strOrEmpty(c.setting.URL), "/")
	var lastErr error
	for i := 0; i < retry; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/xml")
		req.Header.Set("SOAPAction", "\"\"")
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		rb, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode/100 == 2 {
			return nil
		}
		lastErr = fmt.Errorf("gmlc updateCell status %d: %s", resp.StatusCode, string(rb))
	}
	return lastErr
}
