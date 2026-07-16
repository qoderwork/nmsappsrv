package external

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"
	"time"

	"nmsappsrv/internal/misc"
)

// ---------------------------------------------------------------------------
// Spectrum Spatial — reverse-geocode (PSAP id + expected location) & geocode
// ---------------------------------------------------------------------------

// LocationInfo is the normalized Spectrum Spatial response (PSAP id + geo).
type LocationInfo struct {
	PsapID         string
	Fips           string
	Latitude       float64
	Longitude      float64
	Distance       float64
	City           string
	TimeZone       string
	Dst            string
	StreetSide     string
	PercentGeocode float64
	AddressLine1   string
	AddressLine2   string
	LastLine       string
	PostalCode     string
	StateProvince  string
	County         string
	Status         string
}

// spectrumResponse models the JSON envelope returned by the Spectrum Spatial
// reverse-geocode / geocode endpoints (first element of the result array).
type spectrumResponse struct {
	Output []struct {
		PsapID         string  `json:"psapId"`
		Fips           string  `json:"fips"`
		Latitude       float64 `json:"latitude"`
		Longitude      float64 `json:"longitude"`
		Distance       float64 `json:"distance"`
		City           string  `json:"city"`
		TimeZone       string  `json:"timeZone"`
		Dst            string  `json:"dst"`
		StreetSide     string  `json:"streetSide"`
		PercentGeocode float64 `json:"percentGeocode"`
		AddressLine1   string  `json:"addressLine1"`
		AddressLine2   string  `json:"addressLine2"`
		LastLine       string  `json:"lastLine"`
		PostalCode     string  `json:"postalCode"`
		StateProvince  string  `json:"stateProvince"`
		County         string  `json:"county"`
		Status         string  `json:"status"`
	} `json:"Output"`
	// Geocode uses "output_port" instead of "Output".
	OutputPort []struct {
		PsapID         string  `json:"psapId"`
		Fips           string  `json:"fips"`
		Latitude       float64 `json:"latitude"`
		Longitude      float64 `json:"longitude"`
		Distance       float64 `json:"distance"`
		City           string  `json:"city"`
		TimeZone       string  `json:"timeZone"`
		Dst            string  `json:"dst"`
		StreetSide     string  `json:"streetSide"`
		PercentGeocode float64 `json:"percentGeocode"`
		AddressLine1   string  `json:"addressLine1"`
		AddressLine2   string  `json:"addressLine2"`
		LastLine       string  `json:"lastLine"`
		PostalCode     string  `json:"postalCode"`
		StateProvince  string  `json:"stateProvince"`
		County         string  `json:"county"`
		Status         string  `json:"status"`
	} `json:"output_port"`
}

// SpectrumClient wraps the Spectrum Spatial reverse-geocode (PSAP) and geocode
// lookups. It is not a Registrar — it is a lookup the orchestrator calls to
// obtain the PSAP id and the expected (reverse-geocoded) device location used
// for the Vincenty geofence check.
type SpectrumClient struct {
	cfg      *misc.SpectrumSpatialSetting
	transport Transport
}

// NewSpectrumClient builds a Spectrum client.
func NewSpectrumClient(cfg *misc.SpectrumSpatialSetting, t Transport) *SpectrumClient {
	if t == nil {
		t = NotImplementedTransport{}
	}
	return &SpectrumClient{cfg: cfg, transport: t}
}

// Enabled reports whether reverse-geocode is configured.
func (c *SpectrumClient) Enabled() bool {
	return c.cfg != nil && c.cfg.ReverseGeoCodeURL != nil && *c.cfg.ReverseGeoCodeURL != ""
}

// ReverseGeocode returns the PSAP id + expected location for a device GPS fix.
// Returns (nil, nil) when disabled (skipped).
func (c *SpectrumClient) ReverseGeocode(ctx context.Context, lat, lng float64) (*LocationInfo, error) {
	if !c.Enabled() {
		return nil, nil
	}
	base := *c.cfg.ReverseGeoCodeURL
	u := fmt.Sprintf("%s/CovCheck_FEMTO_Call/results.json?Data.Latitude=%v&Data.Longitude=%v",
		strings.TrimRight(base, "/"), lat, lng)
	resp, err := c.transport.RoundTrip(ctx, &TransportRequest{Method: "GET", URL: u})
	if err != nil {
		return nil, err
	}
	return parseSpectrum(resp.Body)
}

// Geocode returns the expected location for a street address (used when the
// device has no GPS fix). Returns (nil, nil) when disabled.
func (c *SpectrumClient) Geocode(ctx context.Context, address string) (*LocationInfo, error) {
	if c.cfg == nil || c.cfg.GeoCodeURL == nil || *c.cfg.GeoCodeURL == "" {
		return nil, nil
	}
	base := *c.cfg.GeoCodeURL
	// Java appends Lat/Lng headers when the URL contains "e911/spectrumSpatialInterface".
	u := fmt.Sprintf("%s/rest/GeocodeUSAddress/results.json?Data.AddressLine1=%s",
		strings.TrimRight(base, "/"), url.QueryEscape(address))
	resp, err := c.transport.RoundTrip(ctx, &TransportRequest{Method: "GET", URL: u})
	if err != nil {
		return nil, err
	}
	return parseSpectrum(resp.Body)
}

func parseSpectrum(body []byte) (*LocationInfo, error) {
	var sr spectrumResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("parse spectrum response: %w", err)
	}
	var src *struct {
		PsapID         string  `json:"psapId"`
		Fips           string  `json:"fips"`
		Latitude       float64 `json:"latitude"`
		Longitude      float64 `json:"longitude"`
		Distance       float64 `json:"distance"`
		City           string  `json:"city"`
		TimeZone       string  `json:"timeZone"`
		Dst            string  `json:"dst"`
		StreetSide     string  `json:"streetSide"`
		PercentGeocode float64 `json:"percentGeocode"`
		AddressLine1   string  `json:"addressLine1"`
		AddressLine2   string  `json:"addressLine2"`
		LastLine       string  `json:"lastLine"`
		PostalCode     string  `json:"postalCode"`
		StateProvince  string  `json:"stateProvince"`
		County         string  `json:"county"`
		Status         string  `json:"status"`
	}
	if len(sr.Output) > 0 {
		src = &sr.Output[0]
	} else if len(sr.OutputPort) > 0 {
		src = &sr.OutputPort[0]
	} else {
		return nil, fmt.Errorf("spectrum response contained no location")
	}
	return &LocationInfo{
		PsapID:         src.PsapID,
		Fips:           src.Fips,
		Latitude:       src.Latitude,
		Longitude:      src.Longitude,
		Distance:       src.Distance,
		City:           src.City,
		TimeZone:       src.TimeZone,
		Dst:            src.Dst,
		StreetSide:     src.StreetSide,
		PercentGeocode: src.PercentGeocode,
		AddressLine1:   src.AddressLine1,
		AddressLine2:   src.AddressLine2,
		LastLine:       src.LastLine,
		PostalCode:     src.PostalCode,
		StateProvince:  src.StateProvince,
		County:         src.County,
		Status:         src.Status,
	}, nil
}

// ---------------------------------------------------------------------------
// MSAG — address normalization (GET query + Basic auth)
// ---------------------------------------------------------------------------

// MSAGMatchResp is the parsed MSAG response.
type MSAGMatchResp struct {
	Code              string
	StreetName        string
	StateProvince     string
	StreetSuffix      string
	PrefixDirectional string
	PostDirectional   string
	MSAGCommunity     string
	MatchedHouseNum  string
}

type msagXML struct {
	XMLName xml.Name `xml:"response"`
	Status  struct {
		Code string `xml:"code"`
	} `xml:"status"`
	MSAG struct {
		StreetName        string `xml:"streetName"`
		StateProvince     string `xml:"stateProvince"`
		StreetSuffix      string `xml:"streetSuffix"`
		PrefixDirectional string `xml:"prefixDirectional"`
		PostDirectional   string `xml:"postDirectional"`
		MSAGCommunity     string `xml:"msagCommunity"`
		MatchedHouseNum   string `xml:"matchedHouseNum"`
	} `xml:"msag"`
}

// MSAGClient normalizes the device's civic address. Add performs the lookup
// and, on a "match", writes the validated fields back onto dev (so downstream
// GMLC registration uses MSAG-validated address data). No-op when disabled.
type MSAGClient struct {
	cfg      *misc.ExternalEndpointSetting
	transport Transport
}

// NewMSAGClient builds an MSAG client.
func NewMSAGClient(cfg *misc.ExternalEndpointSetting, t Transport) *MSAGClient {
	if t == nil {
		t = NotImplementedTransport{}
	}
	return &MSAGClient{cfg: cfg, transport: t}
}

// Name returns the system identifier.
func (c *MSAGClient) Name() string { return "msag" }

// Enabled requires url + username + password.
func (c *MSAGClient) Enabled() bool {
	return c.cfg != nil && strOrEmpty(c.cfg.URL) != "" &&
		strOrEmpty(c.cfg.Username) != "" && strOrEmpty(c.cfg.Password) != ""
}

// Add performs the MSAG lookup and, on a match, updates dev's address fields.
func (c *MSAGClient) Add(ctx context.Context, dev *DeviceContext) error {
	if !c.Enabled() {
		return nil
	}
	q := url.Values{}
	q.Set("house", dev.HouseNumber)
	q.Set("street", dev.StreetName)
	q.Set("city", dev.City)
	q.Set("state", dev.State)
	q.Set("zip", dev.PostalCode)
	q.Set("recordID", newUUID())
	u := fmt.Sprintf("%s?%s", strings.TrimRight(strOrEmpty(c.cfg.URL), "/"), q.Encode())

	resp, err := c.transport.RoundTrip(ctx, &TransportRequest{
		Method:  "GET",
		URL:     u,
		Headers: map[string]string{"Authorization": basicAuth(strOrEmpty(c.cfg.Username), strOrEmpty(c.cfg.Password))},
	})
	if err != nil {
		return err
	}
	var mx msagXML
	if err := xml.Unmarshal(resp.Body, &mx); err != nil {
		return fmt.Errorf("parse msag response: %w", err)
	}
	if mx.Status.Code == "match" {
		dev.StreetName = mx.MSAG.StreetName
		dev.State = mx.MSAG.StateProvince
		dev.StreetSuffix = mx.MSAG.StreetSuffix
		dev.City = mx.MSAG.MSAGCommunity
		dev.HouseNumber = mx.MSAG.MatchedHouseNum
	}
	return nil
}

// Delete is a no-op for MSAG (address normalization is not reversible).
func (c *MSAGClient) Delete(_ context.Context, _ *DeviceContext) error { return nil }

// ---------------------------------------------------------------------------
// BMC — old (SOAP XML, creds in body) + new (JSON, Basic auth)
// ---------------------------------------------------------------------------

// bmcAddFemtoXML models the Alcatel m2m:M2MRequest body for the old BMC API.
type bmcAddFemtoXML struct {
	XMLName      xml.Name `xml:"m2m:M2MRequest"`
	XmlnsM2M     string   `xml:"xmlns:m2m,attr"`
	XmlnsC       string   `xml:"xmlns:c,attr"`
	NetworkCfg   bmcNetCfg `xml:"c:networkConfigUpdate"`
}
type bmcNetCfg struct {
	CellID             int     `xml:"cellId"`
	NeID               int     `xml:"neId"`
	Latitude           float64 `xml:"latitude"`
	Longitude          float64 `xml:"longitude"`
	Radius             int     `xml:"radius"`
	Width              int     `xml:"width"`
	Direction          int     `xml:"direction"`
	Street             string  `xml:"street"`
	City               string  `xml:"city"`
	State              string  `xml:"state"`
	MobileCountryCode  string  `xml:"mobileCountryCode"`
	MobileNetworkCode  string  `xml:"mobileNetworkCode"`
	TaiMCC             string  `xml:"taiMobileCountryCode"`
	TaiMNC             string  `xml:"taiMobileNetworkCode"`
	TaiTAC             string  `xml:"taiTac"`
	GnbID              int     `xml:"gnbId"`
	Username           string  `xml:"username"`
	Password           string  `xml:"password"`
}

// bmcJsonBody models the new BMC JSON API request.
type bmcJsonBody struct {
	CellID           int            `json:"cellId"`
	NeType           string         `json:"neType"`
	MobileCountryCode string        `json:"mobileCountryCode"`
	MobileNetworkCode string        `json:"mobileNetworkCode"`
	TaiMCC           string         `json:"taiMobileCountryCode"`
	TaiMNC           string         `json:"taiMobileNetworkCode"`
	TaiTAC           string         `json:"taiTac"`
	GNBId            int            `json:"gNBId"`
	GlobalSettings   string         `json:"globalSettings"`
	Address          bmcAddress     `json:"address"`
	AntennaDesc      string         `json:"antennaDesc"`
	NGRANCellName    string         `json:"ngranCellName"`
	Location         bmcLocation    `json:"location"`
}
type bmcAddress struct {
	City   string `json:"city"`
	State  string `json:"state"`
	Street string `json:"street"`
}
type bmcLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// BMCClient implements both the old (XML) and new (JSON) BMC APIs behind a
// single Registrar. newAPI selects the variant.
type BMCClient struct {
	name      string
	cfg       *misc.ExternalEndpointSetting
	newAPI    bool
	transport Transport
}

// NewBMCClientOld builds the legacy XML BMC client.
func NewBMCClientOld(cfg *misc.ExternalEndpointSetting, t Transport) *BMCClient {
	return newBMC("bmc", cfg, false, t)
}

// NewBMCClientNew builds the JSON BMC client.
func NewBMCClientNew(cfg *misc.ExternalEndpointSetting, t Transport) *BMCClient {
	return newBMC("newbmc", cfg, true, t)
}

func newBMC(name string, cfg *misc.ExternalEndpointSetting, newAPI bool, t Transport) *BMCClient {
	if t == nil {
		t = NotImplementedTransport{}
	}
	return &BMCClient{name: name, cfg: cfg, newAPI: newAPI, transport: t}
}

// Name returns the system identifier.
func (c *BMCClient) Name() string { return c.name }

// Enabled: old requires url+user+pass; new requires url (+user optional).
func (c *BMCClient) Enabled() bool {
	if c.cfg == nil || strOrEmpty(c.cfg.URL) == "" {
		return false
	}
	if c.newAPI {
		return true
	}
	return strOrEmpty(c.cfg.Username) != "" && strOrEmpty(c.cfg.Password) != ""
}

// Add registers the cell with BMC.
func (c *BMCClient) Add(ctx context.Context, dev *DeviceContext) error {
	if !c.Enabled() {
		return nil
	}
	base := strings.TrimRight(strOrEmpty(c.cfg.URL), "/")
	if c.newAPI {
		body, _ := json.Marshal(bmcJsonBody{
			CellID:            dev.CellID,
			NeType:            "NGRAN",
			MobileCountryCode: dev.MCC,
			MobileNetworkCode: dev.MNC,
			TaiMCC:            dev.MCC,
			TaiMNC:            dev.MNC,
			TaiTAC:            fmt.Sprintf("%d", dev.TAC),
			GNBId:             dev.GnbID,
			GlobalSettings:    "NONE",
			Address:           bmcAddress{City: dev.City, State: dev.State, Street: dev.StreetName},
			AntennaDesc:       dev.SerialNumber,
			NGRANCellName:     fmt.Sprintf("%s_%d", dev.SerialNumber, dev.CellID),
			Location:          bmcLocation{Latitude: dev.Latitude, Longitude: dev.Longitude},
		})
		resp, err := c.transport.RoundTrip(ctx, &TransportRequest{
			Method:  "PUT",
			URL:     base + "/cells",
			Headers: map[string]string{"Content-Type": "application/json", "Authorization": basicAuth(strOrEmpty(c.cfg.Username), strOrEmpty(c.cfg.Password))},
			Body:    body,
		})
		if err != nil {
			return err
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("bmc(new) add cell failed: status %d", resp.StatusCode)
		}
		return nil
	}
	// Old API: XML with creds embedded in the body.
	x := bmcAddFemtoXML{
		XmlnsM2M: "http://m2m.messaging.alcatel_lucent.com/xmlinterfaces/m2m/v1",
		XmlnsC:   "http://oamp.bmc.messaging.alcatel_lucent.com/xmlinterfaces/common/v1",
		NetworkCfg: bmcNetCfg{
			CellID: dev.CellID, NeID: dev.GnbID, Latitude: dev.Latitude, Longitude: dev.Longitude,
			Radius: 15, Width: 360, Direction: 0, Street: dev.StreetName, City: dev.City, State: dev.State,
			MobileCountryCode: dev.MCC, MobileNetworkCode: dev.MNC, TaiMCC: dev.MCC, TaiMNC: dev.MNC,
			TaiTAC: fmt.Sprintf("%d", dev.TAC), GnbID: dev.GnbID,
			Username: strOrEmpty(c.cfg.Username), Password: strOrEmpty(c.cfg.Password),
		},
	}
	body, err := xml.MarshalIndent(x, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal bmc xml: %w", err)
	}
	body = append([]byte(xml.Header), body...)
	resp, err := c.transport.RoundTrip(ctx, &TransportRequest{
		Method:  "POST",
		URL:     base,
		Headers: map[string]string{"Content-Type": "application/xml"},
		Body:    body,
	})
	if err != nil {
		return err
	}
	// <overallStatus>SUCCESS</overallStatus>
	if !strings.Contains(string(resp.Body), "SUCCESS") {
		return fmt.Errorf("bmc(old) add cell not successful: %s", string(resp.Body))
	}
	return nil
}

// Delete rolls the cell back at BMC.
func (c *BMCClient) Delete(ctx context.Context, dev *DeviceContext) error {
	if !c.Enabled() {
		return nil
	}
	base := strings.TrimRight(strOrEmpty(c.cfg.URL), "/")
	if c.newAPI {
		_, err := c.transport.RoundTrip(ctx, &TransportRequest{
			Method: "DELETE",
			URL:    fmt.Sprintf("%s/cells?netype=NGRAN&name=%d&neId=%s-%s-%d", base, dev.GnbID, dev.MCC, dev.MNC, dev.GnbID),
			Headers: map[string]string{"Authorization": basicAuth(strOrEmpty(c.cfg.Username), strOrEmpty(c.cfg.Password))},
		})
		return err
	}
	x := bmcAddFemtoXML{NetworkCfg: bmcNetCfg{CellID: 1, NeID: dev.GnbID, MobileCountryCode: dev.MCC, MobileNetworkCode: dev.MNC, GnbID: dev.GnbID}}
	body, _ := xml.MarshalIndent(x, "", "  ")
	body = append([]byte(xml.Header), body...)
	_, err := c.transport.RoundTrip(ctx, &TransportRequest{Method: "POST", URL: base, Headers: map[string]string{"Content-Type": "application/xml"}, Body: body})
	return err
}

// ---------------------------------------------------------------------------
// LMF — 1..4 instances, X-Auth-Token session, JSON cell data
// ---------------------------------------------------------------------------

// lmfCellData models the LMF add-cell JSON body.
type lmfCellData struct {
	Mcc            string  `json:"mcc"`
	Mnc            string  `json:"mnc"`
	CellID         int     `json:"cellId"`
	GnodebID       int     `json:"gnodebId"`
	GnodebIDLength int     `json:"gnodebIdLength"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	Altitude       float64 `json:"altitude"`
	CellOpeningAngle int   `json:"cellOpeningAngle"`
	CellBearing    int     `json:"cellBearing"`
	CellRadius     int     `json:"cellRadius"`
	NrPci          int     `json:"nrPci"`
	Timestamp      int64   `json:"timestamp"`
	Tac            int     `json:"tac"`
	Power          int     `json:"power"`
	ArfcnDl        int     `json:"arfcnDl"`
	ArfcnUl        int     `json:"arfcnUl"`
}

// LMFClient registers a cell with one LMF instance. Up to four are created
// (LMF, LMF2, LMF3, LMF4), each with its own config + transport.
type LMFClient struct {
	name          string
	cfg           *misc.ExternalEndpointSetting
	transport     Transport
	token         string // cached X-Auth-Token session token
	lastTokenTime time.Time
}

// NewLMFClient builds an LMF client for a named instance (e.g. "lmf", "lmf-2").
func NewLMFClient(name string, cfg *misc.ExternalEndpointSetting, t Transport) *LMFClient {
	if t == nil {
		t = NotImplementedTransport{}
	}
	return &LMFClient{name: name, cfg: cfg, transport: t}
}

// Name returns the system identifier.
func (c *LMFClient) Name() string { return c.name }

// Enabled requires url + username + password.
func (c *LMFClient) Enabled() bool {
	return c.cfg != nil && strOrEmpty(c.cfg.URL) != "" &&
		strOrEmpty(c.cfg.Username) != "" && strOrEmpty(c.cfg.Password) != ""
}

// refreshToken obtains a session X-Auth-Token from the LMF /tokens endpoint.
// Mirrors Java LMFHelper.freshToken: the token is acquired with an
// LMFLoginDTO{auth:{method:"password",password:{user_id,password}}} body and
// read from the X-Auth-Token RESPONSE header. Java refreshes only on a 4-minute
// TTL (not on 401/409), so we re-fetch when the cached token is empty or older
// than 4 minutes.
func (c *LMFClient) refreshToken(ctx context.Context) error {
	if c.token != "" && time.Since(c.lastTokenTime) < 4*time.Minute {
		return nil
	}
	body, _ := json.Marshal(map[string]interface{}{
		"auth": map[string]interface{}{
			"method": "password",
			"password": map[string]string{
				"user_id":  strOrEmpty(c.cfg.Username),
				"password": strOrEmpty(c.cfg.Password),
			},
		},
	})
	resp, err := c.transport.RoundTrip(ctx, &TransportRequest{
		Method:  "POST",
		URL:     strings.TrimRight(strOrEmpty(c.cfg.URL), "/") + "/tokens",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    body,
	})
	if err != nil {
		return err
	}
	if resp.Headers == nil {
		return fmt.Errorf("lmf %s: no X-Auth-Token in response", c.name)
	}
	tok, ok := resp.Headers["X-Auth-Token"]
	if !ok || tok == "" {
		return fmt.Errorf("lmf %s: no X-Auth-Token in response", c.name)
	}
	c.token = tok
	c.lastTokenTime = time.Now()
	return nil
}

// Add registers the cell with the LMF instance.
func (c *LMFClient) Add(ctx context.Context, dev *DeviceContext) error {
	if !c.Enabled() {
		return nil
	}
	if err := c.refreshToken(ctx); err != nil {
		return err
	}
	body, _ := json.Marshal(lmfCellData{
		Mcc: dev.MCC, Mnc: dev.MNC, CellID: dev.CellID, GnodebID: dev.GnbID, GnodebIDLength: 24,
		Latitude: dev.Latitude, Longitude: dev.Longitude, Altitude: dev.Altitude,
		CellOpeningAngle: 1200, CellBearing: 0, CellRadius: 15, NrPci: dev.NrPci,
		Timestamp: 0, Tac: dev.TAC, Power: 24, ArfcnDl: dev.ArfcnDl, ArfcnUl: dev.ArfcnUl,
	})
	resp, err := c.transport.RoundTrip(ctx, &TransportRequest{
		Method: "POST",
		URL:    strings.TrimRight(strOrEmpty(c.cfg.URL), "/") + "/cells/nr/cid_cells",
		Headers: map[string]string{"Content-Type": "application/json", "X-Auth-Token": c.token},
		Body:   body,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode == 409 {
		// Conflict: delete then retry once.
		if err := c.Delete(ctx, dev); err != nil {
			return err
		}
		resp2, err := c.transport.RoundTrip(ctx, &TransportRequest{
			Method: "POST",
			URL:    strings.TrimRight(strOrEmpty(c.cfg.URL), "/") + "/cells/nr/cid_cells",
			Headers: map[string]string{"Content-Type": "application/json", "X-Auth-Token": c.token},
			Body:   body,
		})
		if err != nil {
			return err
		}
		resp = resp2
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("lmf %s add cell failed: status %d", c.name, resp.StatusCode)
	}
	return nil
}

// Delete rolls the cell back at the LMF instance.
func (c *LMFClient) Delete(ctx context.Context, dev *DeviceContext) error {
	if !c.Enabled() {
		return nil
	}
	if err := c.refreshToken(ctx); err != nil {
		return err
	}
	_, err := c.transport.RoundTrip(ctx, &TransportRequest{
		Method: "DELETE",
		URL:    fmt.Sprintf("%s/cells/nr/cid_cells/%s-%s-%d", strings.TrimRight(strOrEmpty(c.cfg.URL), "/"), dev.MCC, dev.MNC, dev.CellID),
		Headers: map[string]string{"X-Auth-Token": c.token},
	})
	return err
}

// ---------------------------------------------------------------------------
// GMLC — SOAP/SPML addRequest
// ---------------------------------------------------------------------------

// gmlcAddRequest models the SPML addRequest envelope for a GMLC cell.
type gmlcAddRequest struct {
	XMLName     xml.Name      `xml:"spml:addRequest"`
	XmlnsSpml   string        `xml:"xmlns:spml,attr"`
	XmlnsGmlc   string        `xml:"xmlns:gmlc,attr"`
	Version     string        `xml:"version"`
	GMLCCell    gmlcCell      `xml:"gmlc:GMLCCell"`
}
type gmlcCell struct {
	Identifier       string           `xml:"identifier"`
	PsapID           string           `xml:"psapId"`
	SiteID           string           `xml:"siteId"`
	SectorID         string           `xml:"sectorId"`
	CellType         string           `xml:"cellType"`
	CellTech         string           `xml:"cellTech"`
	CellCounty       string           `xml:"cellCounty"`
	CellMarket       string           `xml:"cellMarket"`
	CellLocationDesc gmlcLocationDesc `xml:"cellLocationDesc"`
	CellCoordinates  gmlcCoordinates  `xml:"cellCoordinates"`
}
type gmlcLocationDesc struct {
	AddrMSAGValidated string `xml:"addrMSAGValidated"`
	CellID            int    `xml:"cellId"`
	CompanyID         string `xml:"companyId"`
	CountyID          string `xml:"countyId"`
	CustomerName      string `xml:"customerName"`
	HouseNumber       string `xml:"houseNumber"`
	Location          string `xml:"location"`
	ServiceCommunity  string `xml:"serviceCommunity"`
	Direction         string `xml:"direction"`
	State             string `xml:"state"`
	StreetName        string `xml:"streetName"`
	StreetNameSuffix  string `xml:"streetNameSuffix"`
	PostalCode        string `xml:"postalCode"`
}
type gmlcCoordinates struct {
	Latitude    float64 `xml:"latitude"`
	Longitude   float64 `xml:"longitude"`
	Uncertainty float64 `xml:"uncertainty"`
}

// GMLCClient registers a cell with the GMLC via SOAP/SPML.
type GMLCClient struct {
	cfg      *misc.ExternalEndpointSetting
	transport Transport
}

// NewGMLCClient builds a GMLC client.
func NewGMLCClient(cfg *misc.ExternalEndpointSetting, t Transport) *GMLCClient {
	if t == nil {
		t = NotImplementedTransport{}
	}
	return &GMLCClient{cfg: cfg, transport: t}
}

// Name returns the system identifier.
func (c *GMLCClient) Name() string { return "gmlc" }

// Enabled requires url + username + password.
func (c *GMLCClient) Enabled() bool {
	return c.cfg != nil && strOrEmpty(c.cfg.URL) != "" &&
		strOrEmpty(c.cfg.Username) != "" && strOrEmpty(c.cfg.Password) != ""
}

// Add registers the cell with GMLC.
func (c *GMLCClient) Add(ctx context.Context, dev *DeviceContext) error {
	if !c.Enabled() {
		return nil
	}
	reqBody := gmlcAddRequest{
		XmlnsSpml: "urn:siemens:names:prov:gw:SPML:2:0",
		XmlnsGmlc: "urn:siemens:names:prov:gw:SPML:2:0:gmlc",
		Version:   "1.0",
		GMLCCell: gmlcCell{
			Identifier: fmt.Sprintf("%s_%s_%d", dev.MCC, dev.MNC, dev.GnbID*4096+1),
			PsapID:     dev.PsapID,
			SiteID:     dev.SerialNumber,
			SectorID:   fmt.Sprintf("%d", dev.CellID),
			CellType:   "femto",
			CellTech:   "5G",
			CellCounty: dev.CountyID,
			CellMarket: dev.Market,
			CellLocationDesc: gmlcLocationDesc{
				AddrMSAGValidated: "true",
				CellID:            dev.CellID,
				CompanyID:         dev.CompanyID,
				CountyID:          dev.CountyID,
				CustomerName:      dev.CustomerName,
				HouseNumber:       dev.HouseNumber,
				Location:          fmt.Sprintf("%s %s", dev.StreetName, dev.City),
				ServiceCommunity:  dev.City,
				Direction:         "0",
				State:             dev.State,
				StreetName:        dev.StreetName,
				StreetNameSuffix:  dev.StreetSuffix,
				PostalCode:        dev.PostalCode,
			},
			CellCoordinates: gmlcCoordinates{Latitude: dev.Latitude, Longitude: dev.Longitude, Uncertainty: dev.Uncertainty},
		},
	}
	body, err := xml.MarshalIndent(reqBody, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal gmlc xml: %w", err)
	}
	body = append([]byte(xml.Header), body...)
	resp, err := c.transport.RoundTrip(ctx, &TransportRequest{
		Method:  "POST",
		URL:     strings.TrimRight(strOrEmpty(c.cfg.URL), "/"),
		Headers: map[string]string{"Content-Type": "application/xml", "SOAPAction": "\"\""},
		Body:    body,
	})
	if err != nil {
		return err
	}
	if !strings.Contains(string(resp.Body), "success") {
		// error code 3504 / "already exists" → delete then retry once.
		if strings.Contains(string(resp.Body), "3504") || strings.Contains(string(resp.Body), "already exists") {
			if err := c.Delete(ctx, dev); err != nil {
				return err
			}
			resp2, err := c.transport.RoundTrip(ctx, &TransportRequest{
				Method:  "POST",
				URL:     strings.TrimRight(strOrEmpty(c.cfg.URL), "/"),
				Headers: map[string]string{"Content-Type": "application/xml", "SOAPAction": "\"\""},
				Body:    body,
			})
			if err != nil {
				return err
			}
			resp = resp2
		}
	}
	if !strings.Contains(string(resp.Body), "success") {
		return fmt.Errorf("gmlc add cell not successful: %s", string(resp.Body))
	}
	return nil
}

// Delete rolls the cell back at GMLC (spml:deleteRequest).
func (c *GMLCClient) Delete(ctx context.Context, dev *DeviceContext) error {
	if !c.Enabled() {
		return nil
	}
	delBody := fmt.Sprintf(`<spml:deleteRequest xmlns:spml="urn:siemens:names:prov:gw:SPML:2:0"><identifier>%s_%s_%d</identifier></spml:deleteRequest>`,
		dev.MCC, dev.MNC, dev.GnbID*4096+1)
	_, err := c.transport.RoundTrip(ctx, &TransportRequest{
		Method:  "POST",
		URL:     strings.TrimRight(strOrEmpty(c.cfg.URL), "/"),
		Headers: map[string]string{"Content-Type": "application/xml", "SOAPAction": "\"\""},
		Body:    []byte(delBody),
	})
	return err
}

// ---------------------------------------------------------------------------
