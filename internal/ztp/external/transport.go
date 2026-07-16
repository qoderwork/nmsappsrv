package external

import (
	"fmt"
	"os"
)

// TransportConfig carries the mTLS client-certificate material (global, from
// app config/env), mirroring Java WebClientConfig @Value properties
// (certPassword, lmf1CertPassword..4). Cert paths default to the Java
// deployment layout (/ssl/NBI/<SYS>/client.pfx); override per deployment via
// the corresponding env vars documented on NewTransportConfig.
//
// The T-Platform PoP-token key pair (/cert/t-platform/*.pem) is also carried
// here: the private key signs every PoP JWT; the public key is sent as the
// OAuth2 "cnf" claim when refreshing the access token.
type TransportConfig struct {
	GMLCCertPath     string
	GMLCCertPassword string

	LMF1CertPath     string
	LMF1CertPassword string
	LMF2CertPath     string
	LMF2CertPassword string
	LMF3CertPath     string
	LMF3CertPassword string
	LMF4CertPath     string
	LMF4CertPassword string

	// T-Platform PoP key pair.
	TPlatformPrivateKeyPath string
	TPlatformPublicKeyPath  string
}

// DefaultTransportConfig returns the Java-equivalent default cert paths with
// empty passwords (passwords are normally supplied via env).
func DefaultTransportConfig() *TransportConfig {
	return &TransportConfig{
		GMLCCertPath: "/ssl/NBI/GMLC/client.pfx",
		LMF1CertPath: "/ssl/NBI/LMF1/client.pfx",
		LMF2CertPath: "/ssl/NBI/LMF2/client.pfx",
		LMF3CertPath: "/ssl/NBI/LMF3/client.pfx",
		LMF4CertPath: "/ssl/NBI/LMF4/client.pfx",

		TPlatformPrivateKeyPath: "/cert/t-platform/private-key-pkcs8.pem",
		TPlatformPublicKeyPath:  "/cert/t-platform/public-key.pem",
	}
}

// NewTransportConfig builds a TransportConfig from environment variables,
// falling back to the Java default paths. Supported overrides:
//
//	passwords : CERTPASSWORD, LMF1CERTPASSWORD .. LMF4CERTPASSWORD
//	path      : GMLC_CERT_PATH, LMF1_CERT_PATH .. LMF4_CERT_PATH
func NewTransportConfig() *TransportConfig {
	c := DefaultTransportConfig()
	if v := os.Getenv("CERTPASSWORD"); v != "" {
		c.GMLCCertPassword = v
	}
	lmfPW := []string{"LMF1", "LMF2", "LMF3", "LMF4"}
	for i, p := range lmfPW {
		if v := os.Getenv(p + "CERTPASSWORD"); v != "" {
			setCertPassword(c, i, v)
		}
		if v := os.Getenv(p + "_CERT_PATH"); v != "" {
			setCertPath(c, i, v)
		}
	}
	if v := os.Getenv("GMLC_CERT_PATH"); v != "" {
		c.GMLCCertPath = v
	}
	if v := os.Getenv("TPLATFORM_PRIVATE_KEY_PATH"); v != "" {
		c.TPlatformPrivateKeyPath = v
	}
	if v := os.Getenv("TPLATFORM_PUBLIC_KEY_PATH"); v != "" {
		c.TPlatformPublicKeyPath = v
	}
	return c
}

func setCertPassword(c *TransportConfig, i int, v string) {
	switch i {
	case 0:
		c.LMF1CertPassword = v
	case 1:
		c.LMF2CertPassword = v
	case 2:
		c.LMF3CertPassword = v
	case 3:
		c.LMF4CertPassword = v
	}
}

func setCertPath(c *TransportConfig, i int, v string) {
	switch i {
	case 0:
		c.LMF1CertPath = v
	case 1:
		c.LMF2CertPath = v
	case 2:
		c.LMF3CertPath = v
	case 3:
		c.LMF4CertPath = v
	}
}

// Transports bundles the per-system HTTP transports. The shared (GMLC-cert)
// transport backs MSAG / BMC / NewBMC / Spectrum / GMLC — Java's nbiClient
// reuses the GMLC client cert for all of them. LMF1–4 each get their own
// cert transport. The T-Platform PoP key pair (PEM contents) is carried here
// so the TPlatformClient can sign PoP tokens and read the cnf claim.
type Transports struct {
	Shared Transport
	LMF    []Transport

	TPrivateKeyPEM string
	TPublicKeyPEM  string
}

// NewTransports builds the per-system HTTP transports from a TransportConfig.
// A present cert file that fails to load returns an error; a missing cert file
// silently falls back to a non-mTLS transport (matching Java). The T-Platform
// key files follow the same rule: a missing key file yields an empty PEM (the
// T-Platform client then fails at the wire); a present-but-unreadable key
// file is a hard error.
func NewTransports(tc *TransportConfig) (*Transports, error) {
	if tc == nil {
		tc = DefaultTransportConfig()
	}
	shared, err := NewHTTPTransport(tc.GMLCCertPath, tc.GMLCCertPassword)
	if err != nil {
		return nil, err
	}
	lmf := make([]Transport, 0, 4)
	for _, cp := range []struct{ path, pass string }{
		{tc.LMF1CertPath, tc.LMF1CertPassword},
		{tc.LMF2CertPath, tc.LMF2CertPassword},
		{tc.LMF3CertPath, tc.LMF3CertPassword},
		{tc.LMF4CertPath, tc.LMF4CertPassword},
	} {
		t, err := NewHTTPTransport(cp.path, cp.pass)
		if err != nil {
			return nil, err
		}
		lmf = append(lmf, t)
	}
	privPEM, err := readKeyFile(tc.TPlatformPrivateKeyPath)
	if err != nil {
		return nil, err
	}
	pubPEM, err := readKeyFile(tc.TPlatformPublicKeyPath)
	if err != nil {
		return nil, err
	}
	return &Transports{Shared: shared, LMF: lmf, TPrivateKeyPEM: privPEM, TPublicKeyPEM: pubPEM}, nil
}

// readKeyFile reads a PEM key file into a string. A missing file returns ""
// (caller treats the key as absent); a present-but-unreadable file is a hard
// error, mirroring the cert-loading rule.
func readKeyFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read t-platform key %s: %w", path, err)
	}
	return string(data), nil
}
