package license

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/square/certigo/jceks"
)

// TestProductionChain runs the production 3-step decoder against the real
// license.lic with the strongest PBE candidates from debug analysis. This is
// the integration test that drives parameter alignment with Java.
func TestProductionChain_RealLicense(t *testing.T) {
	raw, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Skipf("real license.lic not found: %v", err)
	}
	if len(raw) < 16 {
		t.Skip("license.lic too small")
	}

	// Locate keystore next to test binary or at nmsappsrv/keystores/public.keystore
	ksPath := ""
	for _, p := range []string{
		debugLicensePath,
		filepath.Join(filepath.Dir(debugLicensePath), "..", "keystores", "public.keystore"),
		filepath.Join(filepath.Dir(debugLicensePath), "keystores", "public.keystore"),
	} {
		if _, err := os.Stat(p); err == nil {
			ksPath = p
			break
		}
	}
	_ = ksPath

	// Load keystore if available — use newTrueLicenseVerifier's path.
	var cert *x509.Certificate
	for _, p := range []string{
		filepath.Join(filepath.Dir(debugLicensePath), "keystores", "public.keystore"),
		filepath.Join(filepath.Dir(debugLicensePath), "..", "keystores", "public.keystore"),
		filepath.Join(".", "keystores", "public.keystore"),
	} {
		data, err2 := os.ReadFile(p)
		if err2 != nil {
			continue
		}
		for _, alias := range []string{"publiccert", "publicCert", "PublicCert", "nmsappsrv-cert"} {
			for _, pw := range []string{"nmsappsrv", "", "public_password1234", "public_password4321"} {
				ks, err3 := jceks.LoadFromReader(bytes.NewReader(data), []byte(pw))
				if err3 != nil {
					continue
				}
				c, err4 := ks.GetCert(alias)
				if err4 == nil && c != nil {
					cert = c
					t.Logf("loaded cert alias=%q pw=%q from %s", alias, pw, p)
					goto FOUND
				}
			}
		}
	}
FOUND:
	if cert != nil {
		t.Logf("public cert: CN=%q, PubAlgo=%v, SigAlgo=%v",
			cert.Subject.CommonName, cert.PublicKeyAlgorithm, cert.SignatureAlgorithm)
	}

	// Try each candidate PBE combination and log progress:
	candidates := []struct {
		name string
		cfg  privacyGuardCfg
	}{
		{"DES/public_password4321/2005 (TrueLicense 1.32 confirmed)", privacyGuardCfg{
			Cipher: pbeCipherDES, Iterations: 2005, Password: "public_password4321", VerifySig: cert != nil,
		}},
	}

	bestXML := 0
	bestIdx := -1
	for i, c := range candidates {
		t.Logf("=== candidate %d: %s ===", i, c.name)
		der, err := privacyGuardDecode(raw, c.cfg)
		if err != nil {
			t.Logf("  PBE decode err: %v", err)
			continue
		}
		if len(der) < 2 {
			t.Logf("  der too short: %d bytes", len(der))
			continue
		}
		t.Logf("  PBE+GZIP OK → %d bytes (GenericCertificate XML)", len(der))

		// Extract HTML-escaped LicenseContent XML from GenericCertificate
		licenseXML, err := parseGenericCertificateXML(der)
		if err != nil {
			t.Logf("  GC XML parse err: %v", err)
			continue
		}
		t.Logf("  LicenseContent XML extracted: %d bytes", len(licenseXML))

		// Head preview:
		preview := string(licenseXML[:min(64, len(licenseXML))])
		t.Logf("  XML head preview: %q", preview)

		// Look for XML markers
		xmlScore := 0
		for _, marker := range []string{"<?xml", "<java", "<void property=", "<string>", "<object class="} {
			if bytes.Contains(licenseXML, []byte(marker)) {
				xmlScore++
			}
		}
		t.Logf("  XML marker score: %d/5", xmlScore)
		if xmlScore > bestXML {
			bestXML = xmlScore
			bestIdx = i
		}

		// Try to parse as XMLEncoder → content
		content, err := parseXMLEncoder(licenseXML)
		if err != nil {
			t.Logf("  XMLEncoder parse err: %v", err)
			continue
		}
		t.Logf("  XMLEncoder OK → Subject=%q, ConsumerType=%q, NotBefore=%v, NotAfter=%v, Issued=%v",
			content.Subject, content.ConsumerType,
			content.NotBefore.Format("2006-01-02"),
			content.NotAfter.Format("2006-01-02"),
			content.Issued.Format("2006-01-02"))
		t.Logf("    Extra keys: %d (%v)", len(content.Extra), sampleKeys(content.Extra))
		t.Logf("    Holder keys: %d (%v)", len(content.Holder), sampleKeys(content.Holder))
		t.Logf("    Issuer keys: %d (%v)", len(content.Issuer), sampleKeys(content.Issuer))
		for k, v := range content.Extra {
			if len(v) < 120 {
				t.Logf("      Extra[%q] = %q", k, v)
			} else {
				t.Logf("      Extra[%q] = %q... (%d bytes)", k, v[:100], len(v))
			}
		}
	}
	t.Logf("=== Summary: best XML marker score = %d/5 (cand %d %s) ===",
		bestXML, bestIdx, func() string {
			if bestIdx >= 0 {
				return candidates[bestIdx].name
			}
			return "<none>"
		}())
}

func sampleKeys(m map[string]string) []string {
	out := []string{}
	i := 0
	for k := range m {
		out = append(out, k)
		i++
		if i >= 8 {
			break
		}
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Silence unused import in some environments.
var _ = fmt.Sprintf
