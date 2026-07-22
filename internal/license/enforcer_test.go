package license

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/qoderwork/go-infra/licensing"

	"nmsappsrv/internal/config"
)

// signTestLicense builds and signs a license with the embedded dev private key,
// binding it to the given fingerprint. Returns the envelope bytes to upload.
func signTestLicense(t *testing.T, fp string, expiry time.Time) []byte {
	t.Helper()
	privPEM, err := os.ReadFile("keys/private.pem")
	if err != nil {
		t.Fatalf("read dev private key: %v", err)
	}
	priv, err := licensing.DecodePrivateKeyPEM(privPEM)
	if err != nil {
		t.Fatalf("decode private key: %v", err)
	}
	lic := &licensing.License{
		Version:   licensing.CurrentVersion,
		ID:        "test-lic-001",
		Product:   "nmsappsrv",
		Subject:   "Acme Corp",
		Issuer:    "QoderWork",
		Features:  []string{"core", "monitor"},
		Capacity:  map[string]int64{"enb": 100, "user": 50},
		IssuedAt:  time.Now().UTC(),
		Expiry:    expiry,
		Machine:   &licensing.MachineBinding{Fingerprint: fp},
	}
	env, err := licensing.NewSigner(priv, licensing.CurrentVersion).Sign(lic)
	if err != nil {
		t.Fatalf("sign license: %v", err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return raw
}

func newTestEnforcer(t *testing.T, cfg config.LicenseConfig) *Enforcer {
	t.Helper()
	enf, err := NewEnforcer(cfg)
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}
	return enf
}

func TestEnforcerVerifyStoreAndLoad(t *testing.T) {
	fp := "test-fingerprint-abc123"
	dir := t.TempDir()
	cfg := config.LicenseConfig{
		Required:                   true,
		InstallDir:                 dir,
		MaxClockFile:               filepath.Join(dir, ".maxclock"),
		MachineFingerprintOverride: fp,
	}
	enf := newTestEnforcer(t, cfg)

	raw := signTestLicense(t, fp, time.Now().Add(24*time.Hour))
	if _, err := enf.VerifyAndStore(raw, "license.lic"); err != nil {
		t.Fatalf("VerifyAndStore: %v", err)
	}
	if !enf.IsValid() {
		t.Fatal("expected enforcer to be valid after store")
	}
	lic, status, _ := enf.GetActive()
	if lic == nil || status != "active" {
		t.Fatalf("unexpected active state: status=%s", status)
	}
	if lic.Subject != "Acme Corp" {
		t.Fatalf("subject = %q", lic.Subject)
	}

	// Simulate a restart: a fresh enforcer on the same InstallDir should
	// re-verify from disk and become valid again.
	enf2 := newTestEnforcer(t, cfg)
	if err := enf2.LoadPersisted(); err != nil {
		t.Fatalf("LoadPersisted: %v", err)
	}
	if !enf2.IsValid() {
		t.Fatal("expected enforcer to be valid after LoadPersisted")
	}
}

func TestEnforcerRejectsExpired(t *testing.T) {
	fp := "test-fingerprint-exp"
	dir := t.TempDir()
	cfg := config.LicenseConfig{
		Required:                   true,
		InstallDir:                 dir,
		MaxClockFile:               filepath.Join(dir, ".maxclock"),
		MachineFingerprintOverride: fp,
	}
	enf := newTestEnforcer(t, cfg)
	raw := signTestLicense(t, fp, time.Now().Add(-time.Hour)) // already expired
	if _, err := enf.VerifyAndStore(raw, "expired.lic"); err == nil {
		t.Fatal("expected expired license to be rejected")
	}
	if enf.IsValid() {
		t.Fatal("enforcer must not be valid after rejecting an expired license")
	}
}

func TestEnforcerRejectsMachineMismatch(t *testing.T) {
	dir := t.TempDir()
	cfg := config.LicenseConfig{
		Required:                   true,
		InstallDir:                 dir,
		MaxClockFile:               filepath.Join(dir, ".maxclock"),
		MachineFingerprintOverride: "fp-expected",
	}
	enf := newTestEnforcer(t, cfg)
	// Sign for a different fingerprint than the one this host verifies with.
	raw := signTestLicense(t, "fp-other", time.Now().Add(time.Hour))
	if _, err := enf.VerifyAndStore(raw, "mismatch.lic"); err == nil {
		t.Fatal("expected machine-mismatch license to be rejected")
	}
}

func TestEnforcerDisabledWhenNotRequired(t *testing.T) {
	fp := "test-fingerprint-disabled"
	dir := t.TempDir()
	cfg := config.LicenseConfig{
		Required:                   false, // runtime disable
		InstallDir:                 dir,
		MaxClockFile:               filepath.Join(dir, ".maxclock"),
		MachineFingerprintOverride: fp,
	}
	enf := newTestEnforcer(t, cfg)
	if enf.Required() {
		t.Fatal("Required() should report false when configured disabled")
	}
	// Even with no license uploaded, a disabled enforcer must not block.
	if !enf.IsValid() {
		// IsValid is about having a valid license; with Required=false the
		// middleware lets everything through regardless. IsValid may be false,
		// which is fine — Required() is what the middleware checks first.
		t.Log("IsValid false with no license (expected); gating is via Required()")
	}
}
