package external

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nmsappsrv/internal/misc"
)

// ---------------------------------------------------------------------------
// fakeTransport records every RoundTrip and returns scripted responses.
// ---------------------------------------------------------------------------

type callRecord struct {
	method  string
	url     string
	headers map[string]string
	body    string
}

type fakeTransport struct {
	calls   []callRecord
	respFn  func(req *TransportRequest, idx int) (*TransportResponse, error)
}

func (f *fakeTransport) RoundTrip(_ context.Context, req *TransportRequest) (*TransportResponse, error) {
	f.calls = append(f.calls, callRecord{req.Method, req.URL, req.Headers, string(req.Body)})
	if f.respFn != nil {
		return f.respFn(req, len(f.calls)-1)
	}
	return &TransportResponse{StatusCode: 200, Body: []byte("success")}, nil
}

func (f *fakeTransport) callsTo(urlSubstr string) []callRecord {
	var out []callRecord
	for _, c := range f.calls {
		if strings.Contains(c.url, urlSubstr) {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeTransport) bodyContains(substr string) bool {
	for _, c := range f.calls {
		if strings.Contains(c.body, substr) {
			return true
		}
	}
	return false
}

func ptr(s string) *string { return &s }

func basicAuthValue(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

// ---------------------------------------------------------------------------
// FromZTPSetting
// ---------------------------------------------------------------------------

func TestFromZTPSetting(t *testing.T) {
	t.Run("nested preferred over flat", func(t *testing.T) {
		s := &misc.ZTPSetting{
			SpectrumSpatialURL: ptr("http://flat-spectrum"),
			SpectrumSpatial:    &misc.SpectrumSpatialSetting{URL: ptr("http://nested-spectrum")},
			MSAGUrl:            ptr("http://flat-msag"),
			MSAG:               &misc.ExternalEndpointSetting{URL: ptr("http://nested-msag")},
		}
		cfg := FromZTPSetting(s)
		require.NotNil(t, cfg.Spectrum)
		assert.Equal(t, "http://nested-spectrum", strOrEmpty(cfg.Spectrum.URL))
		require.NotNil(t, cfg.MSAG)
		assert.Equal(t, "http://nested-msag", strOrEmpty(cfg.MSAG.URL))
	})

	t.Run("flat fallback when nested nil", func(t *testing.T) {
		s := &misc.ZTPSetting{
			SpectrumSpatialURL: ptr("http://flat-spectrum"),
			MSAGUrl:            ptr("http://flat-msag"),
			BMCUrl:             ptr("http://flat-bmc"),
			GMLCUrl:            ptr("http://flat-gmlc"),
		}
		cfg := FromZTPSetting(s)
		require.NotNil(t, cfg.Spectrum)
		assert.Equal(t, "http://flat-spectrum", strOrEmpty(cfg.Spectrum.URL))
		require.NotNil(t, cfg.MSAG)
		assert.Equal(t, "http://flat-msag", strOrEmpty(cfg.MSAG.URL))
		require.NotNil(t, cfg.BMC)
		assert.Equal(t, "http://flat-bmc", strOrEmpty(cfg.BMC.URL))
		require.NotNil(t, cfg.GMLC)
		assert.Equal(t, "http://flat-gmlc", strOrEmpty(cfg.GMLC.URL))
	})

	t.Run("LMF count from nested", func(t *testing.T) {
		s := &misc.ZTPSetting{
			LMF:  &misc.ExternalEndpointSetting{URL: ptr("u1")},
			LMF2: &misc.ExternalEndpointSetting{URL: ptr("u2")},
			LMF3: &misc.ExternalEndpointSetting{URL: ptr("u3")},
			LMF4: &misc.ExternalEndpointSetting{URL: ptr("u4")},
		}
		cfg := FromZTPSetting(s)
		require.Len(t, cfg.LMF, 4)
	})

	t.Run("LMF flat fallback", func(t *testing.T) {
		s := &misc.ZTPSetting{LMFUrls: []string{"u1", "u2"}}
		cfg := FromZTPSetting(s)
		require.Len(t, cfg.LMF, 2)
	})

	t.Run("radius threshold default and override", func(t *testing.T) {
		assert.Equal(t, 20.0, FromZTPSetting(nil).RadiusThreshold)
		r := 35.0
		cfg := FromZTPSetting(&misc.ZTPSetting{RadiusThreshold: &r})
		assert.Equal(t, 35.0, cfg.RadiusThreshold)
	})
}

// ---------------------------------------------------------------------------
// parseSpectrum
// ---------------------------------------------------------------------------

func TestParseSpectrum(t *testing.T) {
	t.Run("Output variant", func(t *testing.T) {
		body := []byte(`{"Output":[{"psapId":"P1","latitude":1.5,"longitude":2.5}]}`)
		loc, err := parseSpectrum(body)
		require.NoError(t, err)
		require.NotNil(t, loc)
		assert.Equal(t, "P1", loc.PsapID)
		assert.Equal(t, 1.5, loc.Latitude)
	})

	t.Run("output_port variant", func(t *testing.T) {
		body := []byte(`{"output_port":[{"psapId":"P2","latitude":3.0,"longitude":4.0}]}`)
		loc, err := parseSpectrum(body)
		require.NoError(t, err)
		require.NotNil(t, loc)
		assert.Equal(t, "P2", loc.PsapID)
	})

	t.Run("empty", func(t *testing.T) {
		_, err := parseSpectrum([]byte(`{"Output":[]}`))
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// MSAG
// ---------------------------------------------------------------------------

func TestMSAGAddMatch(t *testing.T) {
	ft := &fakeTransport{}
	c := NewMSAGClient(&misc.ExternalEndpointSetting{URL: ptr("http://msag"), Username: ptr("u"), Password: ptr("p")}, ft)
	dev := &DeviceContext{HouseNumber: "5", StreetName: "Elm", City: "Town", State: "NJ", PostalCode: "07001"}

	resp := `<response><status><code>match</code></status><msag><streetName>Main</streetName><stateProvince>NY</stateProvince><streetSuffix>St</streetSuffix><msagCommunity>CityX</msagCommunity><matchedHouseNum>10</matchedHouseNum></msag></response>`
	ft.respFn = func(_ *TransportRequest, _ int) (*TransportResponse, error) {
		return &TransportResponse{StatusCode: 200, Body: []byte(resp)}, nil
	}

	err := c.Add(context.Background(), dev)
	require.NoError(t, err)
	assert.Equal(t, "Main", dev.StreetName)
	assert.Equal(t, "NY", dev.State)
	assert.Equal(t, "St", dev.StreetSuffix)
	assert.Equal(t, "CityX", dev.City)
	assert.Equal(t, "10", dev.HouseNumber)

	require.Len(t, ft.calls, 1)
	assert.Equal(t, "GET", ft.calls[0].method)
	assert.Equal(t, basicAuthValue("u", "p"), ft.calls[0].headers["Authorization"])
}

func TestMSAGAddNoMatch(t *testing.T) {
	ft := &fakeTransport{}
	c := NewMSAGClient(&misc.ExternalEndpointSetting{URL: ptr("http://msag"), Username: ptr("u"), Password: ptr("p")}, ft)
	dev := &DeviceContext{StreetName: "Elm", City: "Town"}
	ft.respFn = func(_ *TransportRequest, _ int) (*TransportResponse, error) {
		return &TransportResponse{StatusCode: 200, Body: []byte(`<response><status><code>nomatch</code></status></response>`)}, nil
	}
	err := c.Add(context.Background(), dev)
	require.NoError(t, err)
	assert.Equal(t, "Elm", dev.StreetName, "no-match must leave address unchanged")
}

// ---------------------------------------------------------------------------
// BMC (new JSON API)
// ---------------------------------------------------------------------------

func TestBMCNewAddSuccess(t *testing.T) {
	ft := &fakeTransport{}
	c := NewBMCClientNew(&misc.ExternalEndpointSetting{URL: ptr("http://newbmc"), Username: ptr("u"), Password: ptr("p")}, ft)
	dev := &DeviceContext{CellID: 1, MCC: "310", MNC: "260", TAC: 123, GnbID: 7, City: "C", State: "S", StreetName: "St", SerialNumber: "SN", Latitude: 1, Longitude: 2}

	ft.respFn = func(_ *TransportRequest, _ int) (*TransportResponse, error) {
		return &TransportResponse{StatusCode: 200}, nil
	}
	err := c.Add(context.Background(), dev)
	require.NoError(t, err)
	require.Len(t, ft.calls, 1)
	assert.Equal(t, "PUT", ft.calls[0].method)
	assert.True(t, strings.HasSuffix(ft.calls[0].url, "/cells"))
	assert.Equal(t, "application/json", ft.calls[0].headers["Content-Type"])
	assert.Equal(t, basicAuthValue("u", "p"), ft.calls[0].headers["Authorization"])
}

func TestBMCNewAddNon200(t *testing.T) {
	ft := &fakeTransport{}
	c := NewBMCClientNew(&misc.ExternalEndpointSetting{URL: ptr("http://newbmc"), Username: ptr("u"), Password: ptr("p")}, ft)
	ft.respFn = func(_ *TransportRequest, _ int) (*TransportResponse, error) {
		return &TransportResponse{StatusCode: 500}, nil
	}
	err := c.Add(context.Background(), devStub())
	require.Error(t, err)
}

func TestBMCNewDisabled(t *testing.T) {
	ft := &fakeTransport{}
	c := NewBMCClientNew(nil, ft)
	err := c.Add(context.Background(), devStub())
	require.NoError(t, err)
	assert.Empty(t, ft.calls, "disabled registrar must not call transport")
}

// ---------------------------------------------------------------------------
// GMLC (SOAP/SPML)
// ---------------------------------------------------------------------------

func TestGMLCAddSuccess(t *testing.T) {
	ft := &fakeTransport{}
	c := NewGMLCClient(&misc.ExternalEndpointSetting{URL: ptr("http://gmlc"), Username: ptr("u"), Password: ptr("p")}, ft)
	ft.respFn = func(_ *TransportRequest, _ int) (*TransportResponse, error) {
		return &TransportResponse{StatusCode: 200, Body: []byte("<x>success</x>")}, nil
	}
	err := c.Add(context.Background(), devStub())
	require.NoError(t, err)
	require.Len(t, ft.calls, 1)
	assert.Contains(t, ft.calls[0].body, "spml:addRequest")
	assert.Equal(t, "\"\"", ft.calls[0].headers["SOAPAction"])
}

func TestGMLCAdd3504Retry(t *testing.T) {
	ft := &fakeTransport{}
	c := NewGMLCClient(&misc.ExternalEndpointSetting{URL: ptr("http://gmlc"), Username: ptr("u"), Password: ptr("p")}, ft)
	addCount := 0
	ft.respFn = func(req *TransportRequest, _ int) (*TransportResponse, error) {
		if strings.Contains(string(req.Body), "deleteRequest") {
			return &TransportResponse{StatusCode: 200, Body: []byte("success")}, nil
		}
		addCount++
		if addCount == 1 {
			return &TransportResponse{StatusCode: 200, Body: []byte("error code 3504 already exists")}, nil
		}
		return &TransportResponse{StatusCode: 200, Body: []byte("success")}, nil
	}
	err := c.Add(context.Background(), devStub())
	require.NoError(t, err)
	// add, delete, add-again = 3 round trips.
	assert.Len(t, ft.calls, 3)
	assert.True(t, ft.bodyContains("deleteRequest"), "must issue a delete on 3504")
}

// ---------------------------------------------------------------------------
// LMF (token + cell data)
// ---------------------------------------------------------------------------

func TestLMFAddSuccess(t *testing.T) {
	ft := &fakeTransport{}
	c := NewLMFClient("lmf", &misc.ExternalEndpointSetting{URL: ptr("http://lmf"), Username: ptr("u"), Password: ptr("p")}, ft)
	ft.respFn = func(req *TransportRequest, _ int) (*TransportResponse, error) {
		if strings.HasSuffix(req.URL, "/tokens") {
			return &TransportResponse{StatusCode: 200, Headers: map[string]string{"X-Auth-Token": "TOK"}}, nil
		}
		return &TransportResponse{StatusCode: 200}, nil
	}
	err := c.Add(context.Background(), devStub())
	require.NoError(t, err)
	// token + cell add
	require.Len(t, ft.calls, 2)
	assert.Equal(t, "TOK", ft.calls[1].headers["X-Auth-Token"], "cell add must carry the session token")
}

func TestLMFAddNoToken(t *testing.T) {
	ft := &fakeTransport{}
	c := NewLMFClient("lmf", &misc.ExternalEndpointSetting{URL: ptr("http://lmf"), Username: ptr("u"), Password: ptr("p")}, ft)
	ft.respFn = func(req *TransportRequest, _ int) (*TransportResponse, error) {
		if strings.HasSuffix(req.URL, "/tokens") {
			return &TransportResponse{StatusCode: 200}, nil // no X-Auth-Token
		}
		return &TransportResponse{StatusCode: 200}, nil
	}
	err := c.Add(context.Background(), devStub())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Auth-Token")
}

// ---------------------------------------------------------------------------
// RunRegistration (orchestration) + Rollback gating
// ---------------------------------------------------------------------------

func fullConfig() *ExternalConfig {
	return &ExternalConfig{
		MSAG:    &misc.ExternalEndpointSetting{URL: ptr("http://msag"), Username: ptr("u"), Password: ptr("p")},
		BMC:     &misc.ExternalEndpointSetting{URL: ptr("http://bmc"), Username: ptr("u"), Password: ptr("p")},
		NewBMC:  &misc.ExternalEndpointSetting{URL: ptr("http://newbmc"), Username: ptr("u"), Password: ptr("p")},
		LMF:     []*misc.ExternalEndpointSetting{{URL: ptr("http://lmf"), Username: ptr("u"), Password: ptr("p")}},
		GMLC:    &misc.ExternalEndpointSetting{URL: ptr("http://gmlc"), Username: ptr("u"), Password: ptr("p")},
	}
}

// msagNomatchXML is a minimal but valid MSAG response (no address match) so
// the client's xml.Unmarshal succeeds without mutating the device.
const msagNomatchXML = `<response><status><code>nomatch</code></status></response>`

// successTransport returns the response each client expects for a happy path:
// a session token for /tokens, "SUCCESS" for the old BMC, MSAG nomatch XML,
// "success" for GMLC, and 200 otherwise (new BMC PUT, LMF cell add, deletes).
func successTransport() *fakeTransport {
	return &fakeTransport{respFn: func(req *TransportRequest, _ int) (*TransportResponse, error) {
		u, b := req.URL, string(req.Body)
		if strings.HasSuffix(u, "/tokens") {
			return &TransportResponse{StatusCode: 200, Headers: map[string]string{"X-Auth-Token": "TOK"}}, nil
		}
		if strings.Contains(u, "/cells/nr/cid_cells") {
			return &TransportResponse{StatusCode: 200}, nil
		}
		if req.Method == "DELETE" {
			return &TransportResponse{StatusCode: 200}, nil
		}
		if strings.Contains(u, "http://bmc") {
			return &TransportResponse{StatusCode: 200, Body: []byte("SUCCESS")}, nil
		}
		if strings.Contains(u, "http://newbmc") {
			return &TransportResponse{StatusCode: 200}, nil
		}
		if strings.Contains(u, "http://msag") {
			return &TransportResponse{StatusCode: 200, Body: []byte(msagNomatchXML)}, nil
		}
		if strings.Contains(u, "http://gmlc") {
			return &TransportResponse{StatusCode: 200, Body: []byte("success")}, nil
		}
		if strings.Contains(b, "addRequest") {
			return &TransportResponse{StatusCode: 200, Body: []byte("success")}, nil
		}
		return &TransportResponse{StatusCode: 200, Body: []byte("success")}, nil
	}}
}

func TestRunRegistrationSuccess(t *testing.T) {
	ft := successTransport()
	reg := NewRegistry(fullConfig(), ft)
	cancel := &CancelDTO{}
	step, err := RunRegistration(context.Background(), reg, devStub(), cancel)
	require.NoError(t, err)
	assert.Empty(t, step)
	assert.True(t, cancel.BmcAdded)
	assert.True(t, cancel.LmfAdded)
	assert.True(t, cancel.GmlcAdded)
	// MSAG + BMC(old) + NewBMC + LMF(token+cell) + GMLC = 6 pushes.
	assert.Len(t, ft.calls, 6)
}

func TestRunRegistrationFailureRollback(t *testing.T) {
	ft := &fakeTransport{respFn: func(req *TransportRequest, _ int) (*TransportResponse, error) {
		u, b := req.URL, string(req.Body)
		if strings.HasSuffix(u, "/tokens") {
			return &TransportResponse{StatusCode: 200, Headers: map[string]string{"X-Auth-Token": "TOK"}}, nil
		}
		if strings.Contains(u, "/cells/nr/cid_cells") {
			return &TransportResponse{StatusCode: 200}, nil
		}
		if req.Method == "DELETE" {
			return &TransportResponse{StatusCode: 200}, nil
		}
		if strings.Contains(u, "http://bmc") {
			return &TransportResponse{StatusCode: 200, Body: []byte("SUCCESS")}, nil
		}
		if strings.Contains(u, "http://newbmc") {
			return &TransportResponse{StatusCode: 200}, nil
		}
		if strings.Contains(u, "http://msag") {
			return &TransportResponse{StatusCode: 200, Body: []byte(msagNomatchXML)}, nil
		}
		if strings.Contains(u, "http://gmlc") {
			if strings.Contains(b, "addRequest") {
				return &TransportResponse{StatusCode: 200, Body: []byte("gmlc rejected the cell")}, nil
			}
			return &TransportResponse{StatusCode: 200, Body: []byte("success")}, nil
		}
		return &TransportResponse{StatusCode: 200, Body: []byte("success")}, nil
	}}

	reg := NewRegistry(fullConfig(), ft)
	cancel := &CancelDTO{}
	step, err := RunRegistration(context.Background(), reg, devStub(), cancel)
	require.Error(t, err)
	assert.Equal(t, "GMLC", step)
	assert.True(t, cancel.BmcAdded)
	assert.True(t, cancel.LmfAdded)
	assert.False(t, cancel.GmlcAdded, "GMLC never succeeded → must not be flagged for rollback")

	// Rollback only the systems that were pushed (BMC + NewBMC + LMF).
	NewRollback(reg).DeleteInfoFromE911Components(context.Background(), cancel, devStub())

	assert.NotEmpty(t, ft.callsTo("http://bmc"), "old BMC must be rolled back")
	assert.NotEmpty(t, ft.callsTo("http://newbmc"), "new BMC must be rolled back")
	assert.NotEmpty(t, ft.callsTo("http://lmf"), "LMF must be rolled back")
	// GMLC add failed before flagging, so no GMLC delete should have happened.
	gmlcDeletes := 0
	for _, c := range ft.callsTo("http://gmlc") {
		if strings.Contains(c.body, "deleteRequest") {
			gmlcDeletes++
		}
	}
	assert.Equal(t, 0, gmlcDeletes, "GMLC must NOT be rolled back when its add never succeeded")
}

func TestRollbackGating(t *testing.T) {
	ft := successTransport()
	reg := NewRegistry(fullConfig(), ft)
	// Only BMC was pushed.
	cancel := &CancelDTO{BmcAdded: true}
	NewRollback(reg).DeleteInfoFromE911Components(context.Background(), cancel, devStub())

	assert.NotEmpty(t, ft.callsTo("http://bmc"))
	assert.NotEmpty(t, ft.callsTo("http://newbmc"))
	assert.Empty(t, ft.callsTo("http://lmf"), "LMF not flagged → no LMF rollback")
	assert.Empty(t, ft.callsTo("http://gmlc"), "GMLC not flagged → no GMLC rollback")
}

// devStub returns a minimal DeviceContext for client Add calls.
func devStub() *DeviceContext {
	return &DeviceContext{
		ElementID: 1, SerialNumber: "SN", Market: "mkt", Mode: "GPS",
		Latitude: 40.0, Longitude: -74.0, MCC: "310", MNC: "260",
		CellID: 1, TAC: 123, GnbID: 7, PsapID: "P1",
	}
}
