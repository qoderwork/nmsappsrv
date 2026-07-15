// Package external implements the client-side integration with the external
// systems the Java ZTP subsystem registers each femto cell against during
// zero-touch provisioning: Spectrum Spatial (reverse-geocode → PSAP id),
// MSAG (address normalization), BMC (old XML + new JSON), LMF (1–4 instances,
// X-Auth-Token), GMLC (SOAP/SPML), and the E911 rollback orchestrator.
//
// Phase 2a scope: every client builds the correct request shape (SOAP
// envelope / JSON body / query string), derives its enable-state from the
// persisted ZTPSetting, and runs through a Transport abstraction. The real
// wire transport (mTLS client certs, POP token signing, LMF session tokens,
// KML parsing) is intentionally left as a TODO — the default Transport
// returns ErrNotImplemented so the orchestrator's behaviour (gating, state
// machine, allocation, rollback) is fully exercisable without dialling out.
// Phase 2b swaps in a real Transport implementation.
package external

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/google/uuid"

	"nmsappsrv/internal/misc"
)

// basicAuth returns an HTTP Basic auth header value for user/pass.
func basicAuth(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

// newUUID returns a random UUID string (used for MSAG recordID etc.).
func newUUID() string {
	return uuid.New().String()
}

// strPtr returns a pointer to s.
func strPtr(s string) *string { return &s }

// ErrNotImplemented is returned by the default (Phase 2a) Transport. It signals
// that the request was built correctly but no real network call is wired yet.
var ErrNotImplemented = errors.New("ztp/external: wire transport not implemented (Phase 2b)")

// ---------------------------------------------------------------------------
// Per-system config (mirrors Java ZTPSettingDTO nested settings)
//
// These types live in the misc package (misc.PTPSetting,
// misc.SpectrumSpatialSetting, misc.ExternalEndpointSetting,
// misc.TPlatformSetting) so the persisted ZTPSetting and the external clients
// share a single definition. This file re-uses them directly.
// ---------------------------------------------------------------------------

// ExternalConfig is the normalized view of ZTPSetting consumed by the
// external clients. It is produced by FromZTPSetting.
type ExternalConfig struct {
	GnbIDStart      *int
	GnbIDEnd        *int
	TacStart        *int
	TacEnd          *int
	RadiusThreshold float64 // metres; defaults to 20
	GoogleAPIKey    string

	PTP       *misc.PTPSetting
	Spectrum  *misc.SpectrumSpatialSetting
	MSAG      *misc.ExternalEndpointSetting
	BMC       *misc.ExternalEndpointSetting
	NewBMC    *misc.ExternalEndpointSetting
	LMF       []*misc.ExternalEndpointSetting // up to 4 (LMF, LMF2, LMF3, LMF4)
	GMLC      *misc.ExternalEndpointSetting
	TPlatform *misc.TPlatformSetting
}

// FromZTPSetting normalizes a misc.ZTPSetting into ExternalConfig. Nested
// per-system settings are preferred; when nil, the flat top-level URL fields
// (consumed by the SPV worker) are used as a fallback so legacy configs keep
// working.
func FromZTPSetting(s *misc.ZTPSetting) *ExternalConfig {
	if s == nil {
		return &ExternalConfig{RadiusThreshold: 20}
	}
	cfg := &ExternalConfig{
		GnbIDStart: s.GnbIdStart,
		GnbIDEnd:   s.GnbIdEnd,
		TacStart:   s.TacStart,
		TacEnd:     s.TacEnd,
		PTP:        s.PTP,
		Spectrum:   s.SpectrumSpatial,
		MSAG:       s.MSAG,
		BMC:        s.BMC,
		NewBMC:     s.NewBMC,
		GMLC:       s.GMLC,
		TPlatform:  s.TPlatform,
	}
	if s.GoogleAPIKey != nil {
		cfg.GoogleAPIKey = *s.GoogleAPIKey
	}
	if s.RadiusThreshold != nil {
		cfg.RadiusThreshold = *s.RadiusThreshold
	} else {
		cfg.RadiusThreshold = 20
	}

	// Flat fallbacks for the shared-shape endpoints.
	if cfg.Spectrum == nil && s.SpectrumSpatialURL != nil {
		u := *s.SpectrumSpatialURL
		cfg.Spectrum = &misc.SpectrumSpatialSetting{URL: &u}
	}
	if cfg.MSAG == nil && s.MSAGUrl != nil {
		u := *s.MSAGUrl
		cfg.MSAG = &misc.ExternalEndpointSetting{URL: &u}
	}
	if cfg.BMC == nil && s.BMCUrl != nil {
		u := *s.BMCUrl
		cfg.BMC = &misc.ExternalEndpointSetting{URL: &u}
	}
	if cfg.GMLC == nil && s.GMLCUrl != nil {
		u := *s.GMLCUrl
		cfg.GMLC = &misc.ExternalEndpointSetting{URL: &u}
	}
	if cfg.TPlatform == nil && s.TPlatformUrl != nil {
		u := *s.TPlatformUrl
		cfg.TPlatform = &misc.TPlatformSetting{URL: &u}
	}

	// LMF: prefer the four nested instances, else fall back to the flat list.
	var lmfs []*misc.ExternalEndpointSetting
	for _, l := range []*misc.ExternalEndpointSetting{s.LMF, s.LMF2, s.LMF3, s.LMF4} {
		if l != nil {
			lmfs = append(lmfs, l)
		}
	}
	if len(lmfs) == 0 && len(s.LMFUrls) > 0 {
		for _, u := range s.LMFUrls {
			uu := u
			lmfs = append(lmfs, &misc.ExternalEndpointSetting{URL: &uu})
		}
	}
	cfg.LMF = lmfs

	return cfg
}

// ---------------------------------------------------------------------------
// Transport abstraction (Phase 2a: request built, wire is TODO)
// ---------------------------------------------------------------------------

// TransportRequest is a normalized outbound HTTP request.
type TransportRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
}

// TransportResponse is a normalized inbound HTTP response.
type TransportResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// Transport performs an outbound call. The default NotImplementedTransport
// returns ErrNotImplemented; Phase 2b provides a real (mTLS/PoP) impl.
type Transport interface {
	RoundTrip(ctx context.Context, req *TransportRequest) (*TransportResponse, error)
}

// NotImplementedTransport is the Phase 2a default: it confirms the request
// was built but performs no network I/O.
type NotImplementedTransport struct{}

// RoundTrip always returns ErrNotImplemented.
func (NotImplementedTransport) RoundTrip(_ context.Context, _ *TransportRequest) (*TransportResponse, error) {
	return nil, ErrNotImplemented
}

// FuncTransport adapts a closure to the Transport interface (used in tests).
type FuncTransport func(ctx context.Context, req *TransportRequest) (*TransportResponse, error)

// RoundTrip calls the underlying function.
func (f FuncTransport) RoundTrip(ctx context.Context, req *TransportRequest) (*TransportResponse, error) {
	return f(ctx, req)
}

// ---------------------------------------------------------------------------
// Device context + registrar interface
// ---------------------------------------------------------------------------

// DeviceContext carries the resolved per-device values the registrars need to
// build their request payloads. It is populated by the orchestrator from the
// device row, the allocated gnbId/TAC, and the Spectrum reverse-geocode.
type DeviceContext struct {
	ElementID    int64
	SerialNumber string
	Market       string
	Mode         string // GPS / WIFI / MANUAL (from device wifi_or_gps_info)
	Latitude     float64
	Longitude    float64
	Altitude     float64
	MCC          string
	MNC          string
	CellID       int
	TAC          int
	GnbID        int
	NrPci        int
	ArfcnDl      int
	ArfcnUl      int
	PsapID       string
	// Civic address (from TBG / MSAG normalization).
	HouseNumber string
	StreetName  string
	StreetSuffix string
	City        string
	State       string
	PostalCode  string
	CustomerName string
	CompanyID   string
	CountyID    string
	// Uncertainty (metres) for the GMLC civic location.
	Uncertainty float64
}

// Registrar is implemented by every external system the orchestrator pushes a
// cell to. Add registers the cell; Delete rolls it back. A disabled registrar
// is a no-op (returns nil) and is treated as "skipped" by the orchestrator,
// matching Java's behaviour when a system's setting is not configured.
type Registrar interface {
	// Name is the system identifier (e.g. "msag", "bmc", "lmf-2", "gmlc").
	Name() string
	// Enabled reports whether the system is configured (URL + credentials).
	Enabled() bool
	// Add registers the cell. Returns an error only on a real failure (a
	// disabled registrar returns nil without calling the transport).
	Add(ctx context.Context, dev *DeviceContext) error
	// Delete rolls the cell back. No-op when disabled.
	Delete(ctx context.Context, dev *DeviceContext) error
}

// helper: deref string pointer.
func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
