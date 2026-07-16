package external

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nmsappsrv/internal/misc"
)

// genTestTLS builds a CA + server cert + client cert (all self-signed by the
// same CA) and returns the server TLS config (RequireAnyClientCert) plus a
// client tls.Certificate. Used by the mTLS tests.
func genTestTLS(t *testing.T) (*tls.Config, tls.Certificate) {
	t.Helper()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ztp-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	mkCert := func(cn string, isClient bool) tls.Certificate {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(time.Now().UnixNano()),
			Subject:      pkix.Name{CommonName: cn},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(24 * time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		}
		if isClient {
			tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
		require.NoError(t, err)
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
		c, err := tls.X509KeyPair(certPEM, keyPEM)
		require.NoError(t, err)
		return c
	}

	serverCert := mkCert("localhost", false)
	clientCert := mkCert("ztp-client", true)

	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAnyClientCert,
	}
	return serverTLS, clientCert
}

// TestHTTPTransportRoundTripNoCert exercises the real HTTP path (no client
// cert) against a self-signed httptest TLS server, verifying request shape and
// response normalization.
func TestHTTPTransportRoundTripNoCert(t *testing.T) {
	var gotMethod, gotBody string
	var gotHeaders http.Header
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotHeaders = r.Header.Clone()
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		w.Header().Set("X-Echo", "pong")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))
	srv.StartTLS()
	defer srv.Close()

	tr, err := NewHTTPTransport("", "")
	require.NoError(t, err)

	resp, err := tr.RoundTrip(context.Background(), &TransportRequest{
		Method:  "POST",
		URL:     srv.URL + "/cells",
		Headers: map[string]string{"Content-Type": "application/json", "Authorization": "Basic abc"},
		Body:    []byte(`{"a":1}`),
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "ok", string(resp.Body))
	assert.Equal(t, "pong", resp.Headers["X-Echo"])
	assert.Equal(t, "POST", gotMethod)
	assert.Equal(t, `{"a":1}`, gotBody)
	assert.Equal(t, "Basic abc", gotHeaders.Get("Authorization"))
}

// TestHTTPTransportRoundTripMTLS verifies a client certificate is presented to
// a server requiring one, and the call succeeds.
func TestHTTPTransportRoundTripMTLS(t *testing.T) {
	var peerCertPresent int32
	srvTLS, clientCert := genTestTLS(t)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.TLS.PeerCertificates) > 0 {
			atomic.StoreInt32(&peerCertPresent, 1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = srvTLS
	srv.StartTLS()
	defer srv.Close()

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:       &tls.Config{Certificates: []tls.Certificate{clientCert}, InsecureSkipVerify: true},
			ResponseHeaderTimeout: 5 * time.Second,
		},
	}
	tr := &HTTPTransport{client: client} // in-package: inject pre-built client

	resp, err := tr.RoundTrip(context.Background(), &TransportRequest{Method: "GET", URL: srv.URL})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&peerCertPresent), "server should have seen a client cert")
}

// TestHTTPTransportMTLSServerRequiresCert proves the cert path actually
// matters: a transport with no client cert fails against a RequireAnyClientCert
// server.
func TestHTTPTransportMTLSServerRequiresCert(t *testing.T) {
	srvTLS, _ := genTestTLS(t)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = srvTLS
	srv.StartTLS()
	defer srv.Close()

	tr, err := NewHTTPTransport("", "") // no client cert
	require.NoError(t, err)
	_, err = tr.RoundTrip(context.Background(), &TransportRequest{Method: "GET", URL: srv.URL})
	assert.Error(t, err, "server requires a client cert; no-cert transport must fail")
}

// TestNewTransportsNoCerts confirms transports build (non-mTLS) when no cert
// files are present — matching Java's fallback.
func TestNewTransportsNoCerts(t *testing.T) {
	tr, err := NewTransports(DefaultTransportConfig())
	require.NoError(t, err)
	require.NotNil(t, tr.Shared)
	require.Len(t, tr.LMF, 4)
}

// TestRegistryPerSystemTransports verifies MSAG uses the shared (GMLC-cert)
// transport while LMF uses its own instance transport.
func TestRegistryPerSystemTransports(t *testing.T) {
	sharedFT := &fakeTransport{respFn: func(req *TransportRequest, _ int) (*TransportResponse, error) {
		// MSAG parses the body as XML; return a valid (non-match) response.
		return &TransportResponse{StatusCode: 200, Body: []byte(`<?xml version="1.0"?><response><status><code>nomatch</code></status></response>`)}, nil
	}}
	lmfFT := &fakeTransport{respFn: func(req *TransportRequest, _ int) (*TransportResponse, error) {
		if strings.Contains(req.URL, "/tokens") {
			return &TransportResponse{StatusCode: 200, Headers: map[string]string{"X-Auth-Token": "TOK"}}, nil
		}
		return &TransportResponse{StatusCode: 200}, nil
	}}
	tr := &Transports{Shared: sharedFT, LMF: []Transport{lmfFT, lmfFT, lmfFT, lmfFT}}

	cfg := &ExternalConfig{
		MSAG: &misc.ExternalEndpointSetting{URL: strPtr("https://msag")},
		LMF:  []*misc.ExternalEndpointSetting{{URL: strPtr("https://lmf")}},
	}
	cfg.MSAG.Username = strPtr("u")
	cfg.MSAG.Password = strPtr("p")
	cfg.LMF[0].Username = strPtr("u")
	cfg.LMF[0].Password = strPtr("p")

	reg := NewRegistryWithTransports(cfg, tr)

	require.NoError(t, reg.MSAG.Add(context.Background(), &DeviceContext{HouseNumber: "1", StreetName: "Main", City: "C", State: "S", PostalCode: "00000"}))
	require.NoError(t, reg.LMF[0].Add(context.Background(), &DeviceContext{CellID: 1, MCC: "310", MNC: "260", GnbID: 1, TAC: 1}))

	require.Len(t, sharedFT.calls, 1, "MSAG must use the shared transport")
	require.Len(t, lmfFT.calls, 2, "LMF: /tokens (token) + /cells/nr/cid_cells (cell)")
	assert.Contains(t, sharedFT.calls[0].url, "https://msag")
	assert.Contains(t, lmfFT.calls[0].url, "/tokens")
	assert.Contains(t, lmfFT.calls[1].url, "/cells/nr/cid_cells")
}
