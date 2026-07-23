package license

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

type rep struct {
	name  string
	score int
	head  string
	ratio float64
}

// TestDebug_ComprehensivePBESearch runs a broad PBE parameter sweep over both
// likely salt-offsets, ciphers, passwords, and iteration counts. For every
// decrypted output, it runs nested wrapper detectors (gzip/zlib/base64) plus
// the XML marker scan.
func TestDebug_ComprehensivePBESearch(t *testing.T) {
	raw, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Skip(err)
	}

	saltCandidates := []struct {
		name    string
		saltOff int
		dataOff int
	}{
		{"salt[0:8] data[8:]", 0, 8},
		{"salt[8:16] data[16:]", 8, 16},
	}

	passwords := []string{
		"",
		"nmsappsrv",
		"public_password1234",
		"public_password4321",
		"private_password1234",
		"private_password4321",
		"waveoss",
		"harman",
		"TrueLicense",
		"changeme",
		"password",
	}
	iters := []int{1, 5, 10, 20, 50, 100, 256, 512, 1000, 1024, 2000, 2048, 4096, 8192, 10000, 65536}
	ciphers := []pbeCipher{pbeCipherDES, pbeCipher3DES}

	xmlMarkers := [][]byte{
		[]byte("<?xml"),
		[]byte("<java"),
		[]byte("</java>"),
		[]byte("<void property"),
		[]byte("<object class=\"java.util.HashMap\""),
		[]byte("<string>"),
		[]byte("de.schlichtherle"),
		[]byte("LicenseContent"),
	}

	bestScore := 0
	var bestName string
	var bestBytes []byte
	var bestScore2 int // wrapper-detected score

	for _, sc := range saltCandidates {
		_ = raw[sc.saltOff : sc.saltOff+8]
		rawCT := raw[sc.dataOff:]
		if len(rawCT)%8 != 0 {
			continue
		}
		for _, ci := range ciphers {
			for _, pw := range passwords {
				for _, iter := range iters {
					cfg := privacyGuardCfg{
						Cipher:     ci,
						Iterations: iter,
						Password:   pw,
					}
					out, err := privacyGuardDecodeCustom(raw, cfg)
					if err != nil {
						continue
					}
					if len(out) < 16 {
						continue
					}
					// Score XML markers directly
					directScore := 0
					for _, m := range xmlMarkers {
						if bytes.Contains(out, m) {
							directScore++
						}
					}
					if directScore > bestScore {
						bestScore = directScore
						bestName = fmt.Sprintf("%s/%s/pwlen=%d/iter=%d",
							sc.name, cipherName(ci), len(pw), iter)
						bestBytes = out
					}

					// Wrapped: try gzip/zlib
					wrapped, wrapper, ws := tryWrappers(out, xmlMarkers)
					if ws > bestScore2 || (ws > 0 && ws == bestScore2 && wrapper != "") {
						bestScore2 = ws
						cn := fmt.Sprintf("%s/%s/pwlen=%d/iter=%d wrapper=%s",
							sc.name, cipherName(ci), len(pw), iter, wrapper)
						if wrapped != nil && ws > bestScore {
							bestScore = ws
							bestName = cn
							bestBytes = wrapped
						} else if ws > 0 {
							t.Logf("WRAPPER HIT (sc=%d): %s — first 128: %s",
								ws, cn, preview(wrapped, 128))
						}
					}
				}
			}
		}
	}

	t.Logf("BEST direct XML score: %d / %d — %s", bestScore, len(xmlMarkers), bestName)
	if bestBytes != nil {
		t.Logf("  BEST head hex: %s", hex.EncodeToString(bestBytes[:min(48, len(bestBytes))]))
		t.Logf("  BEST head ascii: %q", preview(bestBytes, 300))
		// Printable ratio
		printable := 0
		for _, b := range bestBytes {
			if (b >= 0x20 && b <= 0x7E) || b == 0x0A || b == 0x0D || b == 0x09 {
				printable++
			}
		}
		t.Logf("  Printable ratio: %.0f%%", 100*float64(printable)/float64(len(bestBytes)))
	}
	if bestScore2 > 0 {
		t.Logf("BEST wrapper XML score: %d / %d", bestScore2, len(xmlMarkers))
	}

	if bestScore == 0 && bestScore2 == 0 {
		t.Logf("No XML found in any candidate.")
		// Show top 5 ASN.1-looking outputs as a fallback report.
		var top []rep
		for _, sc := range saltCandidates {
			for _, ci := range ciphers {
				for _, pw := range passwords {
					for _, iter := range []int{20, 2048, 4096, 2000, 1024} {
						cfg := privacyGuardCfg{
							Cipher:     ci,
							Iterations: iter,
							Password:   pw,
						}
						out, err := privacyGuardDecodeCustom(raw, cfg)
						if err != nil || len(out) < 16 {
							continue
						}
						score := 0
						if out[0] == 0x30 {
							score++
							if len(out) > 2 && (out[1]&0x80 == 0 || out[1] == 0x81 || out[1] == 0x82) {
								score++
							}
						}
						printable := 0
						for _, b := range out {
							if (b >= 0x20 && b <= 0x7E) || b == 0x0A || b == 0x0D || b == 0x09 {
								printable++
							}
						}
						ratio := float64(printable) / float64(len(out))
						if ratio > 0.7 {
							score += 3
						} else if ratio > 0.5 {
							score += 1
						}
						if score > 0 {
							top = append(top, rep{
								fmt.Sprintf("%s/%s/pwlen=%d/iter=%d", sc.name, cipherName(ci), len(pw), iter),
								score, hex.EncodeToString(out[:min(16, len(out))]), ratio,
							})
						}
					}
				}
			}
		}
		// Sort top by score desc, ratio desc, show first 10.
		sortReps(top)
		lim := 10
		if len(top) < lim {
			lim = len(top)
		}
		for i := 0; i < lim; i++ {
			t.Logf("  fallback[%d] sc=%d ratio=%.0f%% head=%s %s",
				i, top[i].score, top[i].ratio*100, top[i].head, top[i].name)
		}
	}
}

func privacyGuardDecodeCustom(raw []byte, cfg privacyGuardCfg) ([]byte, error) {
	if len(raw) < 8 {
		return nil, fmt.Errorf("file too small")
	}
	return pbeDecrypt(raw, []byte(cfg.Password), trueLicenseSalt, cfg.Iterations, cfg.Cipher)
}

func cipherName(c pbeCipher) string {
	switch c {
	case pbeCipher3DES:
		return "3DES"
	default:
		return "DES"
	}
}

func preview(b []byte, maxB int) string {
	if len(b) > maxB {
		b = b[:maxB]
	}
	s := make([]byte, 0, len(b))
	for _, c := range b {
		if (c >= 0x20 && c <= 0x7E) || c == 0x0A || c == 0x0D || c == 0x09 {
			s = append(s, c)
		} else {
			s = append(s, '.')
		}
	}
	return string(s)
}

// tryWrappers detects and decodes gzip / zlib / base64 and returns XML score.
func tryWrappers(out []byte, markers [][]byte) ([]byte, string, int) {
	// gzip magic 1F 8B 08
	if len(out) > 3 && out[0] == 0x1F && out[1] == 0x8B && out[2] == 0x08 {
		if gr, err := gzip.NewReader(bytes.NewReader(out)); err == nil {
			if dec, err2 := io.ReadAll(gr); err2 == nil {
				return dec, "gzip", markerScore(dec, markers)
			}
		}
	}
	// zlib 78 01/78 9C/78 DA etc.
	if len(out) > 2 && out[0] == 0x78 && (out[1] == 0x01 || out[1] == 0x9C || out[1] == 0xDA || out[1] == 0x5E) {
		if zr, err := zlib.NewReader(bytes.NewReader(out)); err == nil {
			if dec, err2 := io.ReadAll(zr); err2 == nil {
				return dec, "zlib", markerScore(dec, markers)
			}
		}
	}
	// base64: if 98% printable + in alphabet
	printable := 0
	for _, c := range out {
		if c == '\n' || c == '\r' {
			continue
		}
		printable++
	}
	okB64 := 0
	for _, c := range out {
		if c == '\n' || c == '\r' || c == '\t' || c == ' ' {
			continue
		}
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=' {
			okB64++
		}
	}
	if printable > 10 && float64(okB64)/float64(printable) > 0.97 {
		if dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out))); err == nil && len(dec) > 10 {
			if sc := markerScore(dec, markers); sc > 0 {
				return dec, "base64", sc
			}
			// Maybe gzip/zlib INSIDE base64 — recurse once
			if nxt, _, sc2 := tryWrappers(dec, markers); sc2 > 0 {
				return nxt, "base64→inner", sc2
			}
			return dec, "base64", 0
		}
	}
	return nil, "", 0
}

func markerScore(b []byte, markers [][]byte) int {
	sc := 0
	for _, m := range markers {
		if bytes.Contains(b, m) {
			sc++
		}
	}
	return sc
}

func sortReps(rs []rep) {
	// insertion sort is fine for ~100 elements
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 &&
			(rs[j].score > rs[j-1].score ||
				(rs[j].score == rs[j-1].score && rs[j].ratio > rs[j-1].ratio)); j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}
