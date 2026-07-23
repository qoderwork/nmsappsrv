package license

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestDebug_ScanPBE_ForXML runs the most promising PBE decrypts and scans for
// any XML / printable patterns inside the plaintext to confirm correctness.
// It also dumps BER structure diagnostics for SEQUENCE bodies.
func TestDebug_ScanPBE_ForXML(t *testing.T) {
	raw, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Skipf("license.lic missing: %v", err)
	}

	// Strongest candidates. Also try:
	//   - no PBE at all (if PrivacyGuard absent in Java gen)
	//   - salt offset=0 / data offset=8 and offset=4/12 variants
	type cand struct {
		n       string
		ciph    pbeCipher
		iter    int
		pw      string
		saltOff int
		dataOff int
	}
	cands := []cand{
		{"3DES pw= iter=4096 off=0/8", pbeCipher3DES, 4096, "", 0, 8},
		{"3DES pw= iter=2048 off=0/8", pbeCipher3DES, 2048, "", 0, 8},
		{"3DES pw= iter=1024 off=0/8", pbeCipher3DES, 1024, "", 0, 8},
		{"3DES pw= iter=4096 off=4/12", pbeCipher3DES, 4096, "", 4, 12},
		{"3DES pw=nmsappsrv iter=4096 off=0/8", pbeCipher3DES, 4096, "nmsappsrv", 0, 8},
		{"DES  pw= iter=20 off=0/8", pbeCipherDES, 20, "", 0, 8},
		{"DES  pw= iter=1024 off=0/8", pbeCipherDES, 1024, "", 0, 8},
		{"DES  pw= iter=2048 off=0/8", pbeCipherDES, 2048, "", 0, 8},
		{"DES  pw=nmsappsrv iter=20 off=0/8", pbeCipherDES, 20, "nmsappsrv", 0, 8},
		{"DES  pw=nmsappsrv iter=2048 off=0/8", pbeCipherDES, 2048, "nmsappsrv", 0, 8},
		{"DES  pw= iter=4096 off=0/8", pbeCipherDES, 4096, "", 0, 8},
	}

	xmlMarkers := [][]byte{
		[]byte("<?xml"),
		[]byte("<java"),
		[]byte("void property"),
		[]byte("LicenseContent"),
		[]byte("trueLicense"),
		[]byte("<string>"),
		[]byte("<object "),
	}

	for _, c := range cands {
		cfg := privacyGuardCfg{
			Cipher:     c.ciph,
			Iterations: c.iter,
			Password:   c.pw,
		}
		out, err := privacyGuardDecode(raw, cfg)
		if err != nil {
			t.Logf("[%-38s] PBE decode err: %v", c.n, err)
			continue
		}
		if len(out) < 32 {
			t.Logf("[%s] output too short (%d bytes)", c.n, len(out))
			continue
		}
		// Head+tail hex
		head := out[:min(32, len(out))]
		tail := out[max(0, len(out)-32):]
		markerHits := []string{}
		for _, m := range xmlMarkers {
			if idx := bytes.Index(out, m); idx >= 0 {
				markerHits = append(markerHits, fmt.Sprintf("%s@%d", m, idx))
			}
		}
		// Printable ratio (utf-8 looking)
		printable := 0
		for _, b := range out {
			if (b >= 0x20 && b <= 0x7E) || b == 0x0A || b == 0x0D || b == 0x09 {
				printable++
			}
		}
		ratio := float64(printable) / float64(len(out))

		t.Logf("[%s] → %d bytes | ASN.1?%c | markers=%v | printable=%.0f%%",
			c.n, len(out), isASNSeq(out), markerHits, ratio*100)
		t.Logf("  head: %s", hex.EncodeToString(head))
		t.Logf("  tail: %s", hex.EncodeToString(tail))
		if markerHits != nil {
			// Show snippet around first marker
			idx := bytes.Index(out, xmlMarkers[0])
			if idx < 0 {
				for _, m := range xmlMarkers[1:] {
					if x := bytes.Index(out, m); x >= 0 {
						idx = x
						break
					}
				}
			}
			if idx >= 0 {
				snippetStart := max(0, idx-8)
				snippetEnd := min(len(out), idx+200)
				snippet := string(out[snippetStart:snippetEnd])
				snippet = strings.ReplaceAll(snippet, "\r", "\\r")
				snippet = strings.ReplaceAll(snippet, "\n", "\\n\n\t")
				t.Logf("  XML snippet: %s", snippet)
			}
		}
		// BER diagnostic: check for 0x30 + 0x80 (indefinite length)
		if isASNSeq(out) == 'Y' {
			diag := berDiag(out)
			if diag != "" {
				t.Logf("  BER: %s", diag)
			}
		}
	}
}

func isASNSeq(b []byte) byte {
	if len(b) < 2 || b[0] != 0x30 {
		return ' '
	}
	if b[1] == 0x80 {
		return 'B' // BER indefinite
	}
	if b[1]&0x80 == 0 {
		// short form
		if 2+int(b[1]) <= len(b) {
			return 'Y'
		}
		return 'P'
	}
	nb := int(b[1] & 0x7F)
	if nb > 4 || 2+nb > len(b) {
		return 'x'
	}
	total := 0
	for i := 0; i < nb; i++ {
		total = (total << 8) | int(b[2+i])
	}
	if 2+nb+total <= len(b) && total > 0 {
		return 'Y'
	}
	return 'L'
}

// berDiag reports interesting BER structural features (indefinite length,
// EOC markers, and a quick field walk accounting for 0x80 length).
func berDiag(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	var out []string
	if b[1] == 0x80 {
		out = append(out, "outer=BER-indef")
		if eoc := bytes.Index(b[2:], []byte{0, 0}); eoc >= 0 {
			out = append(out, fmt.Sprintf("EOC@%d", eoc+2))
		}
	}
	// Check a few fields for BER-indef subfields
	off := 2
	for i := 0; i < 8 && off+2 <= len(b); i++ {
		tag := b[off]
		off++
		if off >= len(b) {
			break
		}
		l := b[off]
		if l == 0x80 {
			out = append(out, fmt.Sprintf("field[%d] tag=0x%02x BER-indef", i, tag))
			// find EOC within
			if eoc := bytes.Index(b[off+1:], []byte{0, 0}); eoc >= 0 {
				off += 1 + eoc + 2
				continue
			}
			break
		}
		length, skip, err := readDERLength(b[off:])
		if err != nil {
			break
		}
		off += skip
		if off+length > len(b) {
			break
		}
		out = append(out, fmt.Sprintf("f[%d]=0x%02x/%dB", i, tag, length))
		off += length
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return strings.Join(out, " ")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
