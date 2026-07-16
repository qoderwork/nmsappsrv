package external

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"golang.org/x/crypto/pkcs12"
)

// HTTPTransport is the real Phase 2b Transport. It performs outbound HTTP(S)
// calls with optional mTLS client-certificate authentication, mirroring
// Java's WebClientConfig: a PKCS12 (.pfx) client certificate is loaded from
// disk and presented to the server, while server-certificate verification is
// disabled (Java's createInsecureTrustManagers trusts any chain). All calls
// carry 5s connect/read/write/response timeouts, matching the Java WebClient.
type HTTPTransport struct {
	client *http.Client
}

// NewHTTPTransport builds an HTTPTransport. certPath is a PKCS12 (.pfx) client
// certificate. When certPath is empty (or the file is absent) the transport
// still performs real TLS but without presenting a client certificate —
// matching Java, where the nbiClient falls back to an insecure trust manager
// with no key manager when /ssl/NBI/<SYS>/client.pfx does not exist.
func NewHTTPTransport(certPath, certPassword string) (*HTTPTransport, error) {
	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, // Java disables server cert verification
	}
	if certPath != "" {
		if _, err := os.Stat(certPath); err == nil {
			cert, err := loadClientCert(certPath, certPassword)
			if err != nil {
				return nil, fmt.Errorf("load client cert %s: %w", certPath, err)
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat client cert %s: %w", certPath, err)
		}
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:       tlsCfg,
			ResponseHeaderTimeout: 5 * time.Second,
			MaxIdleConns:          2,
			MaxIdleConnsPerHost:   2,
			IdleConnTimeout:       20 * time.Second,
		},
	}
	return &HTTPTransport{client: client}, nil
}

// loadClientCert loads a PKCS12 (.pfx) client certificate and returns a
// tls.Certificate. Mirrors Java KeyStore.getInstance("PKCS12") + kmf.init.
// (golang.org/x/crypto/pkcs12 v0.54.0 exposes Decode only, so the leaf cert is
// returned without the intermediate CA chain; servers trust the leaf directly.)
func loadClientCert(path, password string) (tls.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return tls.Certificate{}, err
	}
	key, cert, err := pkcs12.Decode(data, password)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("decode pkcs12: %w", err)
	}
	return tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  key,
		Leaf:        cert,
	}, nil
}

// RoundTrip performs the HTTP call and normalizes the response.
func (h *HTTPTransport) RoundTrip(ctx context.Context, req *TransportRequest) (*TransportResponse, error) {
	var bodyReader io.Reader
	if len(req.Body) > 0 {
		bodyReader = bytes.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build http request: %w", err)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := h.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http %s %s: %w", req.Method, req.URL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	headers := make(map[string]string, len(resp.Header))
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}
	return &TransportResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
	}, nil
}
