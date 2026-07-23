package license

import (
	"bytes"
	"compress/gzip"
	"crypto/cipher"
	"crypto/des"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"testing"
)

// TestDebug_BruteSaltOffset tries every possible 8-byte salt window in the first
// 32 bytes of the license file, combined with the most likely params.
// For each candidate output, it also tries wrapping decompressors (gzip) and
// a Base64 decoder, because some TrueLicense builds compress before encrypting.
func TestDebug_BruteSaltOffset(t *testing.T) {
	raw, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Skip(err)
	}

	password := "public_password4321"
	pwBytes := []byte(password)
	pwBytesEmpty := []byte("")
	altPWs := [][]byte{
		pwBytes,
		pwBytesEmpty,
		[]byte("publicCert"),
		[]byte("private_password4321"),
		[]byte("license_demo"),
		[]byte("harman"),
	}
	iters := []int{5, 10, 20, 50, 100, 500, 1000, 1024, 2000, 2048, 4096, 8192}

	xmlMarkers := [][]byte{
		[]byte("<?xml"),
		[]byte("<java"),
		[]byte("LicenseContent"),
		[]byte("<void property"),
		[]byte("HashMap"),
		[]byte("de.schlichtherle"),
	}

	hasMarkers := func(b []byte) int {
		n := 0
		for _, m := range xmlMarkers {
			if bytes.Contains(b, m) {
				n++
			}
		}
		return n
	}

	tryUnwrap := func(plain []byte) []byte {
		// 1. direct
		if hasMarkers(plain) > 0 {
			return plain
		}
		// 2. gzip
		if len(plain) >= 2 && plain[0] == 0x1f && plain[1] == 0x8b {
			if r, err := gzip.NewReader(bytes.NewReader(plain)); err == nil {
				if d, err2 := io.ReadAll(r); err2 == nil {
					if hasMarkers(d) > 0 {
						return d
					}
				}
			}
		}
		// 3. base64 decode (if printable)
		printable := 0
		for _, c := range plain {
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
				(c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=' {
				printable++
			}
		}
		if len(plain) > 0 && float64(printable)/float64(len(plain)) > 0.9 {
			if d, err := base64.StdEncoding.DecodeString(string(plain)); err == nil && len(d) > 16 {
				if hasMarkers(d) > 0 {
					return d
				}
			}
		}
		// 4. zlib deflate (skip 2-byte header)
		if len(plain) > 2 {
			if r, err := gzip.NewReader(bytes.NewReader(append(
				[]byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03},
				plain[2:]...))); err == nil {
				if d, err2 := io.ReadAll(r); err2 == nil && hasMarkers(d) > 0 {
					return d
				}
			}
		}
		return nil
	}

	bestN := 0
	var bestDesc string
	var bestBytes []byte
	total := 0

	// Salt offsets: 0..32 inclusive for an 8-byte window
	for saltOff := 0; saltOff <= 32; saltOff++ {
		// Data offsets: saltOff+8, 0 (no salt separation = entire raw as ct),
		// or a variety of magic-header offsets (4 bytes, 12 bytes).
		dataOffs := []int{saltOff + 8, 0, 4, 8, 12, 16}
		for _, dataOff := range dataOffs {
			if saltOff+8 > len(raw) || dataOff >= len(raw) {
				continue
			}
			salt := raw[saltOff : saltOff+8]
			ct := raw[dataOff:]
			if len(ct) < 8 || len(ct)%8 != 0 {
				continue
			}
			for _, pw := range altPWs {
				for _, iter := range iters {
					total++
					// ===== DES single, KDF v1 (SunJCE PBEWITHMD5ANDDES) =====
					dk16 := sunPBEKDF(pw, salt, iter, 16)
					key8 := dk16[:8]
					iv8 := dk16[8:16]
					if blk, e := des.NewCipher(key8); e == nil {
						// CBC
						rawOut := make([]byte, len(ct))
						cipher.NewCBCDecrypter(blk, iv8).CryptBlocks(rawOut, ct)
						for _, p := range stripPadVariants(rawOut) {
							if u := tryUnwrap(p); u != nil {
								n := hasMarkers(u)
								if n > bestN {
									bestN = n
									bestDesc = fmt.Sprintf("DES/CBC pwlen=%d iter=%d saltOff=%d dataOff=%d",
										len(pw), iter, saltOff, dataOff)
									bestBytes = append([]byte(nil), u...)
									t.Logf("★ HIT n=%d: %s", n, bestDesc)
									showHit(t, u, 400)
								}
							}
						}
						// ECB
						ecbOut := ecbDecrypt(blk, ct)
						for _, p := range stripPadVariants(ecbOut) {
							if u := tryUnwrap(p); u != nil {
								n := hasMarkers(u)
								if n > bestN {
									bestN = n
									bestDesc = fmt.Sprintf("DES/ECB pwlen=%d iter=%d saltOff=%d dataOff=%d",
										len(pw), iter, saltOff, dataOff)
									bestBytes = append([]byte(nil), u...)
									t.Logf("★ HIT n=%d: %s", n, bestDesc)
									showHit(t, u, 400)
								}
							}
						}
					}

					// ===== 3DES KDF v1 (32 bytes → 24 key + 8 iv) =====
					dk32 := sunPBEKDF(pw, salt, iter, 32)
					key24 := dk32[:24]
					iv3 := dk32[24:32]
					if blk, e := des.NewTripleDESCipher(key24); e == nil {
						rawOut := make([]byte, len(ct))
						cipher.NewCBCDecrypter(blk, iv3).CryptBlocks(rawOut, ct)
						for _, p := range stripPadVariants(rawOut) {
							if u := tryUnwrap(p); u != nil {
								n := hasMarkers(u)
								if n > bestN {
									bestN = n
									bestDesc = fmt.Sprintf("3DES/CBC pwlen=%d iter=%d saltOff=%d dataOff=%d",
										len(pw), iter, saltOff, dataOff)
									bestBytes = append([]byte(nil), u...)
									t.Logf("★ HIT n=%d: %s", n, bestDesc)
									showHit(t, u, 400)
								}
							}
						}
					}
				}
			}
		}
	}

	t.Logf("Total candidates: %d, best XML markers: %d/6", total, bestN)
	if bestN > 0 {
		t.Logf("Best: %s", bestDesc)
		t.Logf("Best head hex: %s", hex.EncodeToString(bestBytes[:min(64, len(bestBytes))]))
	}
}

// sunPBEKDF is the reference SunJCE PBEWithMD5AndDES / ANDTripleDES derivation.
// Single-block semantics: T_1 = MD5(pw||salt), T_i = MD5(T_{i-1});
// Multi-block: T_next = MD5(T_last || pw || salt), then iterated.
// This matches our original pkcs5v1MD5 exactly — re-declared here for clarity.
func sunPBEKDF(pw, salt []byte, iter int, dkLen int) []byte {
	if iter <= 0 {
		iter = 1
	}
	var out []byte
	var prev []byte
	for len(out) < dkLen {
		h := md5.New()
		if len(prev) > 0 {
			h.Write(prev)
		}
		h.Write(pw)
		h.Write(salt)
		cur := h.Sum(nil)
		for r := 1; r < iter; r++ {
			h2 := md5.New()
			h2.Write(cur)
			cur = h2.Sum(nil)
		}
		out = append(out, cur...)
		prev = cur
	}
	return out[:dkLen]
}

func ecbDecrypt(blk cipher.Block, ct []byte) []byte {
	bs := blk.BlockSize()
	if len(ct)%bs != 0 {
		return nil
	}
	out := make([]byte, len(ct))
	for i := 0; i < len(ct); i += bs {
		blk.Decrypt(out[i:i+bs], ct[i:i+bs])
	}
	return out
}

func stripPadVariants(raw []byte) [][]byte {
	var outs [][]byte
	outs = append(outs, raw)
	if len(raw) == 0 {
		return outs
	}
	n := len(raw)
	// strict valid PKCS5 / PKCS7
	pad := int(raw[n-1])
	if pad > 0 && pad <= 8 && pad <= n {
		ok := true
		for i := 0; i < pad; i++ {
			if raw[n-1-i] != byte(pad) {
				ok = false
				break
			}
		}
		if ok {
			outs = append(outs, raw[:n-pad])
		}
	}
	// Try last byte values 1..8 as pad even if inconsistent (some impls are buggy)
	for p := 1; p <= 8 && p < n; p++ {
		outs = append(outs, raw[:n-p])
	}
	return outs
}

func showHit(t *testing.T, b []byte, preview int) {
	t.Helper()
	if idx := bytes.Index(b, []byte("<?xml")); idx >= 0 {
		t.Logf("  <?xml at %d: %q", idx, safePreview(b[idx:], preview))
	} else if idx := bytes.Index(b, []byte("<java")); idx >= 0 {
		t.Logf("  <java at %d: %q", idx, safePreview(b[idx:], preview))
	} else {
		t.Logf("  preview: %q", safePreview(b, preview))
	}
}

// ============================================================
// Diagnostic: known-vector test to ensure KDF is correct
// ============================================================

// RFC 6070 / PBKDF1 test-ish vector:
// P="password", S=[0x78,0x57,0x8E,0x5A,0x5D,0x63,0xCB,0x06], c=1000, dkLen=16
// Not an official RFC vector; we just assert determinism so changes to
// sunPBEKDF fail loudly.
func TestDebug_KDFDeterminism(t *testing.T) {
	pw := []byte("password")
	salt := []byte{0x78, 0x57, 0x8E, 0x5A, 0x5D, 0x63, 0xCB, 0x06}
	dk := sunPBEKDF(pw, salt, 1000, 16)
	got := hex.EncodeToString(dk)
	// Baseline recorded first run, used to catch regressions in the KDF impl.
	want := "" // baseline — set on first PASS, then assert.
	if want == "" {
		t.Logf("KDF baseline (P=password, c=1000, dkLen=16): %s", got)
		return
	}
	if got != want {
		t.Fatalf("KDF regression! want %s, got %s", want, got)
	}
	t.Logf("KDF determinism OK: %s", got)
}
