package license

import (
	"crypto/ed25519"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/qoderwork/go-infra/licensing"
	"github.com/qoderwork/go-infra/licensing/machine"

	"nmsappsrv/internal/config"
	"nmsappsrv/pkg/logger"
)

// ---------------------------------------------------------------------------
// License abstraction layer (Java TrueLicense migration path)
//
// These two interfaces decouple the Enforcer from a specific signing format
// so we can swap the backend from the current go-infra/licensing → a
// TrueLicense-compatible verifier without touching callers (main.go,
// LicenseMiddleware, license handler, ...).
// ---------------------------------------------------------------------------

// LicenseView exposes the minimum set of fields the enforcer and callers
// need. Backed today by *licensing.License; backed tomorrow by
// *TrueLicenseContent (same wire bytes as Java).
type LicenseView interface {
	// GetSubject mirrors licensing.License.Subject (== customer / license id).
	GetSubject() string
	// GetExpiry mirrors licensing.License.Expiry (zero == no expiry).
	GetExpiry() time.Time
	// Raw returns the underlying concrete value (used by handler today for
	// serialization). Callers must type-assert — guarded behind a single
	// point so the transition plan stays grep-able.
	Raw() interface{}
}

// LicenseVerifier parses and cryptographically validates a raw license
// envelope. Implementations:
//   - goInfraVerifier (today, local dev): go-infra/licensing with ed25519
//   - trueLicenseVerifier (follow-up, production match Java): TrueLicense
//     XML + PKCS7/CMS signature with X509 certificate chain
type LicenseVerifier interface {
	Verify(raw []byte) (LicenseView, error)
}

// goInfraLicenseView wraps the existing *licensing.License in LicenseView.
type goInfraLicenseView struct{ inner *licensing.License }

func (v goInfraLicenseView) GetSubject() string       { return v.inner.Subject }
func (v goInfraLicenseView) GetExpiry() time.Time     { return v.inner.Expiry }
func (v goInfraLicenseView) Raw() interface{}         { return v.inner }

// goInfraVerifier adapts *licensing.Verifier to LicenseVerifier. It exposes
// the same builder-style methods so NewEnforcer stays unchanged when the
// backend is swapped.
type goInfraVerifier struct{ inner *licensing.Verifier }

func (v *goInfraVerifier) Verify(raw []byte) (LicenseView, error) {
	lic, err := v.inner.Verify(raw)
	if err != nil {
		return nil, err
	}
	return goInfraLicenseView{inner: lic}, nil
}

func (v *goInfraVerifier) WithFingerprint(fp func() (string, error)) *goInfraVerifier {
	v.inner = v.inner.WithFingerprint(fp)
	return v
}

func (v *goInfraVerifier) WithMinClock(clock int64) *goInfraVerifier {
	v.inner = v.inner.WithMinClock(clock)
	return v
}

//go:embed keys/public.pem
var publicKeyFS embed.FS

// activeLicenseFile is the on-disk name of the verified envelope inside InstallDir.
const activeLicenseFile = "active.lic"

// Enforcer verifies and caches the active signed license.
//
// The concrete verifier (currently goInfraVerifier, planned
// trueLicenseVerifier) is swapped via the LicenseVerifier interface. Every
// caller-facing method on Enforcer uses the LicenseView abstraction — no
// outside code imports licensing.License directly, so the Java TrueLicense
// drop-in is a single-file swap.
//
// On a successful verify it (1) persists the envelope to disk so it survives
// restarts, and (2) advances the persisted anti-rollback clock. LoadPersisted
// re-verifies the on-disk envelope at startup and is the source of truth after
// a restart. Activation state lives ONLY in memory + file (not DB) to prevent
// tampering via direct SQL updates.
type Enforcer struct {
	cfg      config.LicenseConfig
	verifier LicenseVerifier
	minClock int64

	mu     sync.RWMutex
	active LicenseView
	status string // active | expired | invalid | missing
	detail string
}

// NewEnforcer builds an Enforcer. It picks a verifier backend and applies
// host machine binding + anti-clock-rollback (min-clock file). Verifier
// selection rules (first match wins):
//
//  1. TrueLicense backend: when cfg.TrueLicensePublicKeysStorePath != ""
//     loads the JCEKS store, extracts the X509 public certificate and uses
//     TrueLicense wire-format parsing + VmParam/LicenseCheckModel checks.
//  2. go-infra backend (dev/test default): loads ed25519 PEM public key
//     (embedded default, overridable via cfg.PublicKeyPath).
func NewEnforcer(cfg config.LicenseConfig) (*Enforcer, error) {
	minClock := readMinClock(cfg.MaxClockFile)

	// --- backend 1: TrueLicense (Java parity, production) ----------------
	if cfg.TrueLicensePublicKeysStorePath != "" {
		tv, err := newTrueLicenseVerifier(cfg)
		if err != nil {
			return nil, err
		}
		logger.Infof("license: TrueLicense backend selected (keystore=%s, subject=%q)",
			cfg.TrueLicensePublicKeysStorePath, cfg.TrueLicenseSubject)
		return &Enforcer{
			cfg:      cfg,
			verifier: tv,
			minClock: minClock,
			status:   "missing",
			detail:   "enforcer initialized (TrueLicense backend), no license loaded yet",
		}, nil
	}

	// --- backend 2: go-infra (ed25519, dev/test fallback) ----------------
	pub, err := loadPublicKey(cfg.PublicKeyPath)
	if err != nil {
		return nil, err
	}
	inner := licensing.NewVerifier(pub, licensing.CurrentVersion).
		WithFingerprint(fpFunc(cfg.MachineFingerprintOverride)).
		WithMinClock(minClock)
	v := &goInfraVerifier{inner: inner}
	logger.Infof("license: go-infra backend selected (ed25519, dev/test fallback)")
	return &Enforcer{
		cfg:      cfg,
		verifier: v,
		minClock: minClock,
		status:   "missing",
		detail:   "enforcer initialized (go-infra backend), no license loaded yet",
	}, nil
}

// Required reports whether license gating is enabled.
func (e *Enforcer) Required() bool { return e.cfg.Required }

// IsValid reports whether a currently-active, non-expired license is enforced.
func (e *Enforcer) IsValid() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.status != "active" || e.active == nil {
		return false
	}
	exp := e.active.GetExpiry()
	if !exp.IsZero() && time.Now().After(exp) {
		return false
	}
	return true
}

// GetActive returns the cached license (as interface{}, to preserve external
// callers that type-assert to *licensing.License today) and its enforcement
// status/detail.
//
// NOTE: the first return value is intentionally typed `interface{}` so
// existing callers in handler.go/ can continue to use *licensing.License
// without a sweeping refactor. Once the TrueLicense verifier lands, change
// this signature to return LicenseView and update the two type-assert sites.
func (e *Enforcer) GetActive() (interface{}, string, string) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.active == nil {
		return nil, e.status, e.detail
	}
	return e.active.Raw(), e.status, e.detail
}

// LoadPersisted re-verifies the on-disk envelope at startup. It is NOT fatal:
// a missing or invalid license simply leaves gating disabled until one is
// uploaded, which the middleware will then enforce.
func (e *Enforcer) LoadPersisted() error {
	path := filepath.Join(e.cfg.InstallDir, activeLicenseFile)
	data, err := os.ReadFile(path)
	if err != nil {
		e.setStatus("missing", "no active license on disk")
		logger.Warnf("license: %s — service will gate until a license is uploaded", e.detail)
		return nil
	}
	lic, err := e.verifier.Verify(data)
	if err != nil {
		e.setStatus("invalid", err.Error())
		logger.Warnf("license: persisted license invalid: %v", err)
		return nil
	}
	e.mu.Lock()
	e.applyVerifiedLocked(lic, "loaded from disk")
	e.mu.Unlock()
	logger.Infof("license: active license loaded (subject=%s, expiry=%s)", lic.GetSubject(), expiryStr(lic))
	return nil
}

// VerifyAndStore verifies an uploaded envelope, persists it to disk and DB,
// advances the anti-rollback clock, and activates it. Returns the verified
// license as its underlying concrete value (handler.go type-asserts to
// *licensing.License today — will switch to TrueLicenseContent at swap).
func (e *Enforcer) VerifyAndStore(raw []byte, originalName string) (interface{}, error) {
	lic, err := e.verifier.Verify(raw)
	if err != nil {
		return nil, fmt.Errorf("license verification failed: %w", err)
	}

	if err := os.MkdirAll(e.cfg.InstallDir, 0o700); err != nil {
		return nil, fmt.Errorf("license: create install dir: %w", err)
	}
	diskPath := filepath.Join(e.cfg.InstallDir, activeLicenseFile)
	if err := os.WriteFile(diskPath, raw, 0o600); err != nil {
		return nil, fmt.Errorf("license: persist envelope: %w", err)
	}

	e.mu.Lock()
	// Advance anti-rollback clock: never let "max seen time" go backwards.
	now := time.Now().Unix()
	if now > e.minClock {
		e.minClock = now
		writeMinClock(e.cfg.MaxClockFile, e.minClock)
		if gi, ok := e.verifier.(*goInfraVerifier); ok {
			// go-infra backend supports WithMinClock rebuild; TrueLicense
			// backend will handle the anti-rollback clock internally.
			gi.WithMinClock(e.minClock)
		}
	}
	e.applyVerifiedLocked(lic, "uploaded")
	e.mu.Unlock()

	logger.Infof("license: activated (subject=%s, expiry=%s)", lic.GetSubject(), expiryStr(lic))
	return lic.Raw(), nil
}

// applyVerifiedLocked updates the in-memory cache and status. Caller must hold e.mu.
// Activation state is intentionally NOT persisted to DB — the .lic file on disk
// is the single source of truth (signature-protected against tampering).
func (e *Enforcer) applyVerifiedLocked(lic LicenseView, detail string) {
	e.active = lic
	e.status = "active"
	e.detail = detail
}

func (e *Enforcer) setStatus(status, detail string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.status = status
	e.detail = detail
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func loadPublicKey(path string) (ed25519.PublicKey, error) {
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			return licensing.DecodePublicKeyPEM(data)
		} else {
			logger.Warnf("license: public key override %q unreadable (%v), falling back to embedded", path, err)
		}
	}
	data, err := publicKeyFS.ReadFile("keys/public.pem")
	if err != nil {
		return nil, fmt.Errorf("license: read embedded public key: %w", err)
	}
	return licensing.DecodePublicKeyPEM(data)
}

// fpFunc returns the fingerprint source. When an override is configured (dev/
// test only) it is returned directly; otherwise the real host system-uuid is
// used via machine.Fingerprint.
func fpFunc(override string) func() (string, error) {
	if override != "" {
		return func() (string, error) { return override, nil }
	}
	return machine.Fingerprint
}

func readMinClock(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var v int64
	if _, err := fmt.Sscanf(string(data), "%d", &v); err != nil {
		return 0
	}
	return v
}

func writeMinClock(path string, unix int64) {
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, []byte(fmt.Sprintf("%d", unix)), 0o600)
}

func ptrTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func expiryStr(lic LicenseView) string {
	if lic.GetExpiry().IsZero() {
		return "never"
	}
	return lic.GetExpiry().Format(time.RFC3339)
}
