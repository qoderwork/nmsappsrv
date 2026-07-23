package license

import (
	"bytes"
	"os"
	"testing"
)

func TestDebug_ParseCMSAndFindXML(t *testing.T) {
	raw, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Skip(err)
	}

	candidates := []struct {
		name       string
		cipher     pbeCipher
		iterations int
		password   string
	}{
		{"3DES/empty/4096", pbeCipher3DES, 4096, ""},
		{"3DES/public_password4321/20", pbeCipher3DES, 20, "public_password4321"},
		{"3DES/public_password4321/1024", pbeCipher3DES, 1024, "public_password4321"},
		{"3DES/public_password4321/2048", pbeCipher3DES, 2048, "public_password4321"},
		{"DES/empty/20", pbeCipherDES, 20, ""},
		{"DES/public_password4321/20", pbeCipherDES, 20, "public_password4321"},
		{"DES/public_password4321/1024", pbeCipherDES, 1024, "public_password4321"},
		{"DES/public_password4321/2048", pbeCipherDES, 2048, "public_password4321"},
	}

	for _, c := range candidates {
		t.Logf("\n=== Trying: %s ===", c.name)
		cfg := privacyGuardCfg{
			Cipher:     c.cipher,
			Iterations: c.iterations,
			Password:   c.password,
		}
		der, err := privacyGuardDecode(raw, cfg)
		if err != nil {
			t.Logf("  PBE decode failed: %v", err)
			continue
		}
		t.Logf("  PBE decoded: %d bytes", len(der))
		
		if len(der) >= 2 {
			t.Logf("  First byte (tag): 0x%02x", der[0])
			length, skip, err := readDERLength(der[1:])
			if err != nil {
				t.Logf("  Top-level length parse failed: %v", err)
			} else {
				t.Logf("  Top-level length: %d bytes (skip=%d)", length, skip)
				t.Logf("  Total expected: %d vs actual: %d", 1+skip+length, len(der))
			}
		}

		xmlMarkers := [][]byte{[]byte("<?xml"), []byte("<java"), []byte("LicenseContent")}
		foundAny := false
		for _, m := range xmlMarkers {
			if idx := bytes.Index(der, m); idx >= 0 {
				foundAny = true
				t.Logf("  ★ Found %q at offset %d!", m, idx)
				snippet := der[idx:min(idx+200, len(der))]
				t.Logf("  XML snippet: %q", string(snippet))
				
				content, err := parseXMLEncoder(snippet)
				if err != nil {
					t.Logf("  XMLEncoder parse err: %v", err)
				} else {
					t.Logf("  XMLEncoder OK → Subject=%q, ConsumerType=%q, NotBefore=%v, NotAfter=%v",
						content.Subject, content.ConsumerType,
						content.NotBefore.Format("2006-01-02 15:04:05"),
						content.NotAfter.Format("2006-01-02 15:04:05"))
				}
			}
		}
		if !foundAny {
			t.Logf("  No XML markers found. Printable ratio: %.0f%%", printableRatio(der))
		}
	}
}

func printableRatio(b []byte) float64 {
	n := min(64, len(b))
	p := 0
	for _, c := range b[:n] {
		if (c >= 0x20 && c <= 0x7E) || c == 0x0A || c == 0x0D || c == 0x09 {
			p++
		}
	}
	return 100 * float64(p) / float64(n)
}