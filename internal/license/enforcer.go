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

//go:embed keys/public.pem
var publicKeyFS embed.FS

// activeLicenseFile is the on-disk name of the verified envelope inside InstallDir.
const activeLicenseFile = "active.lic"

// Enforcer verifies and caches the active signed license (go-infra/licensing).
//
// On a successful verify it (1) persists the envelope to disk so it survives
// restarts, and (2) advances the persisted anti-rollback clock. LoadPersisted
// re-verifies the on-disk envelope at startup and is the source of truth after
// a restart. Activation state lives ONLY in memory + file (not DB) to prevent
// tampering via direct SQL updates.
type Enforcer struct {
	cfg      config.LicenseConfig
	verifier *licensing.Verifier
	minClock int64

	mu     sync.RWMutex
	active *licensing.License
	status string // active | expired | invalid | missing
	detail string
}

// NewEnforcer builds an Enforcer. It loads the public key (file override or
// embedded default) and constructs the verifier with host machine binding and
// anti-clock-rollback (WithMinClock) using the persisted max-clock file.
func NewEnforcer(cfg config.LicenseConfig) (*Enforcer, error) {
	pub, err := loadPublicKey(cfg.PublicKeyPath)
	if err != nil {
		return nil, err
	}
	minClock := readMinClock(cfg.MaxClockFile)
	v := licensing.NewVerifier(pub, licensing.CurrentVersion).
		WithFingerprint(fpFunc(cfg.MachineFingerprintOverride)).
		WithMinClock(minClock)
	return &Enforcer{
		cfg:      cfg,
		verifier: v,
		minClock: minClock,
		status:   "missing",
		detail:   "enforcer initialized, no license loaded yet",
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
	if !e.active.Expiry.IsZero() && time.Now().After(e.active.Expiry) {
		return false
	}
	return true
}

// GetActive returns the cached license and its enforcement status/detail.
func (e *Enforcer) GetActive() (*licensing.License, string, string) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.active, e.status, e.detail
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
	logger.Infof("license: active license loaded (subject=%s, expiry=%s)", lic.Subject, expiryStr(lic))
	return nil
}

// VerifyAndStore verifies an uploaded envelope, persists it to disk and DB,
// advances the anti-rollback clock, and activates it. Returns the verified
// license on success.
func (e *Enforcer) VerifyAndStore(raw []byte, originalName string) (*licensing.License, error) {
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
		e.verifier = e.verifier.WithMinClock(e.minClock)
	}
	e.applyVerifiedLocked(lic, "uploaded")
	e.mu.Unlock()

	logger.Infof("license: activated (subject=%s, expiry=%s)", lic.Subject, expiryStr(lic))
	return lic, nil
}

// applyVerifiedLocked updates the in-memory cache and status. Caller must hold e.mu.
// Activation state is intentionally NOT persisted to DB — the .lic file on disk
// is the single source of truth (signature-protected against tampering).
func (e *Enforcer) applyVerifiedLocked(lic *licensing.License, detail string) {
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

func expiryStr(lic *licensing.License) string {
	if lic.Expiry.IsZero() {
		return "never"
	}
	return lic.Expiry.Format(time.RFC3339)
}
