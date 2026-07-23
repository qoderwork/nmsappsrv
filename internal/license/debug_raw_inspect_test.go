package license

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestDebug_RawInspection dumps the raw license.lic bytes plus checks for
// DER/CMS markers WITHOUT applying PBE first. This tells us whether the
// file is (a) plain DER/CMS, (b) DER/CMS wrapped in fixed header magic,
// or (c) fully encrypted by PrivacyGuard.
func TestDebug_RawInspection(t *testing.T) {
	raw, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Skip(err)
	}
	t.Logf("Total size: %d bytes", len(raw))
	t.Logf("First 64 hex:")
	t.Logf("%s", hex.Dump(raw[:min(64, len(raw))]))
	if len(raw) > 64 {
		t.Logf("Next 64 hex:")
		t.Logf("%s", hex.Dump(raw[64:min(128, len(raw))]))
	}
	if len(raw) > 128 {
		mid := len(raw) / 2
		t.Logf("Middle 32 hex:")
		t.Logf("%s", hex.Dump(raw[mid:min(mid+32, len(raw))]))
	}
	tail := raw[max(0, len(raw)-32):]
	t.Logf("Last 32 hex:")
	t.Logf("%s", hex.Dump(tail))

	// Is raw itself DER SEQUENCE?
	if len(raw) >= 2 {
		if raw[0] == 0x30 {
			length, skip, err := readDERLength(raw[1:])
			if err == nil {
				actual := 1 + skip + length
				t.Logf("YES — raw itself looks like DER SEQUENCE: length=%dB (skip len bytes=%d), total=%dB vs raw=%dB",
					length, skip, actual, len(raw))
				if actual == len(raw) {
					t.Logf("★ RAW = single DER SEQUENCE (exact fit), NO PBE wrapper!")
				}
			}
		}
	}

	// Check for "<?xml" or "<java" anywhere in raw
	for _, m := range [][]byte{[]byte("<?xml"), []byte("<java"), []byte("LicenseContent"), []byte("trueLicense")} {
		if idx := bytes.Index(raw, m); idx >= 0 {
			t.Logf("★ Found %q AT OFFSET %d in RAW (no PBE!)", m, idx)
		}
	}

	// Check for common magic bytes / Java serialization markers at offset 0
	magics := []struct {
		magic  []byte
		name   string
	}{
		{[]byte{0xAC, 0xED}, "Java ObjectOutputStream SERIALIZATION_MAGIC (0xACED)"},
		{[]byte("PK"), "ZIP/JAR"},
		{[]byte{0x1F, 0x8B}, "gzip"},
		{[]byte{0x30, 0x82}, "DER SEQUENCE long-form 2-byte length (typical CMS SignedData)"},
		{[]byte{0x30, 0x81}, "DER SEQUENCE long-form 1-byte length"},
		{[]byte("-----BEGIN"), "PEM armour"},
		{[]byte("<?xml"), "raw XML"},
	}
	for _, m := range magics {
		if bytes.HasPrefix(raw, m.magic) {
			t.Logf("★ STARTS WITH %s", m.name)
		}
	}

	// Printable count
	printable := 0
	for _, b := range raw {
		if (b >= 0x20 && b <= 0x7E) || b == 0x0A || b == 0x0D || b == 0x09 {
			printable++
		}
	}
	t.Logf("Printable byte ratio in raw: %.0f%%", 100*float64(printable)/float64(len(raw)))

	// Base64 check — is the entire file printable Base64 (newlines allowed)?
	if isMaybeBase64(raw) {
		t.Logf("★ Possibly Base64 encoded (no wrapping PBE, try b64 decode first)")
	}

	// Print head 256 as printable (with non-printable shown as ".") — this
	// often reveals text fragments even inside binary wrappers.
	buf := make([]byte, 0, 512)
	show := raw[:min(256, len(raw))]
	for i := 0; i < len(show); i += 16 {
		end := i + 16
		if end > len(show) {
			end = len(show)
		}
		row := show[i:end]
		hexPart := hex.EncodeToString(row)
		ascii := make([]byte, 16)
		for j, b := range row {
			if b >= 0x20 && b <= 0x7E {
				ascii[j] = b
			} else {
				ascii[j] = '.'
			}
		}
		buf = append(buf, fmt.Sprintf("  %04x: %-32s  %s\n", i, hexPart, ascii)...)
	}
	t.Logf("Hex+ASCII dump first %dB:\n%s", len(show), buf)

	// Check if the license actually starts with a fixed 4-byte magic followed
	// by salt (so dataOffset = 12)
	if len(raw) >= 12 {
		for _, off := range []int{4, 8, 12} {
			salt := raw[off : off+8]
			rest := raw[off+8:]
			t.Logf("Hypothesis salt@%d: %s (rest=%dB, blockAlign=%v)",
				off, hex.EncodeToString(salt), len(rest), len(rest)%8 == 0)
		}
	}
}

func isMaybeBase64(b []byte) bool {
	okChars := 0
	totChars := 0
	for _, c := range b {
		if c == '\n' || c == '\r' || c == '\t' || c == ' ' {
			continue
		}
		totChars++
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=' {
			okChars++
		}
	}
	if totChars < 10 {
		return false
	}
	return float64(okChars)/float64(totChars) > 0.98
}

// Silence imported `strings` if unused (we use it later)
var _ = strings.HasPrefix
