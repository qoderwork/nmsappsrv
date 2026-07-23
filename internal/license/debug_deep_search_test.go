package license

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
)

// ============================================================
// KDF variants — exhaust different SunJCE PBE key derivation
// ============================================================

// kdf_v1: pkcs5v1MD5 — first block prevHash=empty, then password+salt, iterated
// (matches our pkcs5v1MD5 function)
func kdf_v1(pw, salt []byte, iter int, dkLen int) []byte {
	if iter <= 0 {
		iter = 1
	}
	var out []byte
	var prevHash []byte
	for len(out) < dkLen {
		h := md5.New()
		if len(prevHash) > 0 {
			h.Write(prevHash)
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
		prevHash = cur
	}
	return out[:dkLen]
}

// kdf_v2: each iteration re-appends password+salt (PBKDF1-style chaining)
// T_1 = MD5(pw||salt), T_i = MD5(T_{i-1}||pw||salt)
func kdf_v2(pw, salt []byte, iter int, dkLen int) []byte {
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
			h2.Write(pw)
			h2.Write(salt)
			cur = h2.Sum(nil)
		}
		out = append(out, cur...)
		prev = cur
	}
	return out[:dkLen]
}

// kdf_v3: T_1=MD5(salt||pw), T_i=MD5(T_{i-1}) — reversed pw+salt order
func kdf_v3(pw, salt []byte, iter int, dkLen int) []byte {
	if iter <= 0 {
		iter = 1
	}
	var out []byte
	var prevHash []byte
	for len(out) < dkLen {
		h := md5.New()
		if len(prevHash) > 0 {
			h.Write(prevHash)
		}
		h.Write(salt)
		h.Write(pw)
		cur := h.Sum(nil)
		for r := 1; r < iter; r++ {
			h2 := md5.New()
			h2.Write(cur)
			cur = h2.Sum(nil)
		}
		out = append(out, cur...)
		prevHash = cur
	}
	return out[:dkLen]
}

// kdf_v4: T=MD5(pw||salt) iterated iter times but NO multi-block extension
// (take 16-byte hash, re-hash in-place, then pad/truncate to dkLen)
func kdf_v4(pw, salt []byte, iter int, dkLen int) []byte {
	if iter <= 0 {
		iter = 1
	}
	h := md5.New()
	h.Write(pw)
	h.Write(salt)
	cur := h.Sum(nil)
	for r := 1; r < iter; r++ {
		h2 := md5.New()
		h2.Write(cur)
		cur = h2.Sum(nil)
	}
	// Extend by repeating hash if dkLen > 16
	out := make([]byte, 0, dkLen)
	for len(out) < dkLen {
		out = append(out, cur...)
		h3 := md5.New()
		h3.Write(cur)
		cur = h3.Sum(nil)
	}
	return out[:dkLen]
}

// ============================================================
// Password encodings
// ============================================================

type pwEnc struct {
	name string
	b    []byte
}

func encodePasswords(pwStr string) []pwEnc {
	// For ASCII pwStr: UTF-8 == low-byte-only. Still try all for completeness.
	utf8 := []byte(pwStr)
	N := len(pwStr)
	utf16be := make([]byte, 0, N*2)
	for _, c := range pwStr {
		utf16be = append(utf16be, byte(c>>8), byte(c&0xff))
	}
	utf16le := make([]byte, 0, N*2)
	for _, c := range pwStr {
		utf16le = append(utf16le, byte(c&0xff), byte(c>>8))
	}
	// Null-terminated variants (Java PBEKeySpec does NOT null-terminate, but some impls do)
	utf8z := append([]byte(nil), utf8...)
	utf8z = append(utf8z, 0)
	return []pwEnc{
		{"UTF-8", utf8},
		{"UTF-16BE", utf16be},
		{"UTF-16LE", utf16le},
		{"UTF-8+NUL", utf8z},
	}
}

// ============================================================
// Test: deep exhaustive search with full GC+XML parse validate
// ============================================================

func TestDebug_DeepPBESearch(t *testing.T) {
	raw, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Skip(err)
	}

	// Test both salt layouts
	type layout struct {
		name    string
		salt    []byte
		ct      []byte
	}
	layouts := []layout{
		{"salt[0:8] data[8:]", raw[0:8], raw[8:]},
	}
	if len(raw) >= 16 {
		ct := raw[16:]
		if len(ct)%8 == 0 {
			layouts = append(layouts, layout{"salt[8:16] data[16:]", raw[8:16], ct})
		}
	}

	passwords := []string{
		"",
		"public_password4321",
		"public_password1234",
		"private_password4321",
		"private_password1234",
		"nmsappsrv",
		"license_demo",
		"publicCert",
		"publiccert",
		"harman",
		"waveoss",
		"changeme",
		"default",
	}

	iters := []int{1, 2, 5, 10, 20, 50, 100, 256, 512, 1000, 1024, 1234,
		1500, 2000, 2048, 2500, 3000, 4096, 5000, 8192, 10000, 16384, 20000, 65536}

	kdfs := []struct {
		name string
		fn   func(pw, salt []byte, iter, dkLen int) []byte
	}{
		{"v1(prev;pw+salt)", kdf_v1},
		{"v2(pw+salt each iter)", kdf_v2},
		{"v3(salt+pw order)", kdf_v3},
		{"v4(single-block extend)", kdf_v4},
	}

	algos := []struct {
		name  string
		keySz int // total derived for key+iv
		mkBlk func(key []byte) (cipher.Block, error)
	}{
		{"DES", 16, func(k []byte) (cipher.Block, error) { return des.NewCipher(k) }},
		{"3DES", 32, func(k []byte) (cipher.Block, error) {
			key24 := make([]byte, 24)
			copy(key24, k[:24])
			return des.NewTripleDESCipher(key24)
		}},
	}

	xmlMarkers := [][]byte{
		[]byte("<?xml"),
		[]byte("<java"),
		[]byte("LicenseContent"),
		[]byte("<void property"),
		[]byte("HashMap"),
		[]byte("subject"),
	}

	bestMarkerCount := 0
	var bestResult []byte
	var bestDesc string
	totalTests := 0

	for _, lo := range layouts {
		if len(lo.ct)%8 != 0 {
			continue
		}
		for _, alg := range algos {
			for _, pwStr := range passwords {
				pwEncs := encodePasswords(pwStr)
				for _, pe := range pwEncs {
					for _, kdf := range kdfs {
						for _, iter := range iters {
							totalTests++

							// Derive key + IV
							dk := kdf.fn(pe.b, lo.salt, iter, alg.keySz)
							keyLen := alg.keySz - 8
							key := make([]byte, keyLen)
							copy(key, dk[:keyLen])
							iv := make([]byte, 8)
							copy(iv, dk[keyLen:alg.keySz])

							blk, err := alg.mkBlk(key)
							if err != nil {
								continue
							}

							// CBC decrypt (raw, no unpadding — apply later)
							plainRaw := make([]byte, len(lo.ct))
							cipher.NewCBCDecrypter(blk, iv).CryptBlocks(plainRaw, lo.ct)

							// Try with unpadding too
							var candidates [][]byte
							candidates = append(candidates, plainRaw)
							if len(plainRaw) > 0 {
								pad := int(plainRaw[len(plainRaw)-1])
								if pad > 0 && pad <= 8 && pad <= len(plainRaw) {
									ok := true
									for i := 0; i < pad; i++ {
										if plainRaw[len(plainRaw)-1-i] != byte(pad) {
											ok = false
											break
										}
									}
									if ok {
										candidates = append(candidates, plainRaw[:len(plainRaw)-pad])
									}
								}
							}

							for _, plain := range candidates {
								if len(plain) < 16 {
									continue
								}
								// 1) Direct XML marker count
								mc := 0
								for _, m := range xmlMarkers {
									if bytes.Contains(plain, m) {
										mc++
									}
								}
								// 2) Try parseGenericCertificateDER → encoded field
								if mc == 0 {
									if gc, errGc := parseGenericCertificateDER(plain); errGc == nil && len(gc.Encoded) > 0 {
										for _, m := range xmlMarkers {
											if bytes.Contains(gc.Encoded, m) {
												mc++
											}
										}
									}
								}
								if mc > bestMarkerCount {
									bestMarkerCount = mc
									bestResult = append([]byte(nil), plain...)
									bestDesc = fmt.Sprintf("%s | %s | %s | pw=%q(%s) | kdf=%s | iter=%d",
										lo.name, alg.name, cipherMode(plain),
										pwStr, pe.name, kdf.name, iter)
									t.Logf("★ NEW BEST mc=%d: %s", mc, bestDesc)
									if mc >= 2 {
										// Show content preview
										if idx := bytes.Index(plain, []byte("<?xml")); idx >= 0 {
											t.Logf("  XML snippet: %q", safePreview(plain[idx:], 400))
										} else if idx := bytes.Index(plain, []byte("<java")); idx >= 0 {
											t.Logf("  <java snippet: %q", safePreview(plain[idx:], 400))
										} else {
											t.Logf("  head hex: %s", hex.EncodeToString(plain[:min(48, len(plain))]))
											// Try GC parse
											if gc, egc := parseGenericCertificateDER(plain); egc == nil && len(gc.Encoded) > 0 {
												t.Logf("  GC Encoded head: %q", safePreview(gc.Encoded, 400))
											}
										}
									}
								}
								if mc >= 4 {
									// Very strong hit — stop early
									t.Logf("EARLY STOP — strong marker hit")
									goto endSearch
								}
							}
						}
					}
				}
			}
		}
	}
endSearch:
	t.Logf("Total tests: %d, best marker count=%d/6", totalTests, bestMarkerCount)
	if bestMarkerCount > 0 {
		t.Logf("Best params: %s", bestDesc)
		if bestResult != nil {
			// Print first 80 bytes hex and ASCII
			t.Logf("Best head hex=%s", hex.EncodeToString(bestResult[:min(80, len(bestResult))]))
		}
	}
}

func cipherMode(b []byte) string {
	if len(b) >= 2 && b[0] == 0x30 {
		return "looks-DER"
	}
	return "unknown"
}

func safePreview(b []byte, n int) string {
	if len(b) > n {
		b = b[:n]
	}
	// Replace non-printable with ?
	out := make([]byte, len(b))
	for i, c := range b {
		if (c >= 0x20 && c <= 0x7E) || c == 0x0A || c == 0x0D || c == 0x09 {
			out[i] = c
		} else {
			out[i] = '?'
		}
	}
	return string(out)
}

// ========== Additional sanity: verify parseGenericCertificateDER works on known structure ==========

func TestDebug_ParseDummyGC(t *testing.T) {
	// Dummy GC-like DER to ensure parser is working
	xml := []byte(`<?xml version="1.0"?><java><object class="de.schlichtherle.license.LicenseContent"><void property="subject"><string>demo</string></void></object></java>`)
	// OCTET STRING tag + length
	body := append([]byte{0x04, byte(len(xml))}, xml...)
	// Wrap in SEQUENCE (top-level SEQ + tbs SEQ + algo SEQ + sig BITSTRING + encoded body)
	// Simplified: just a top-level SEQ containing our encoded OCTET STRING
	total := len(body)
	var hdr []byte
	if total < 128 {
		hdr = []byte{0x30, byte(total)}
	} else if total < 256 {
		hdr = []byte{0x30, 0x81, byte(total)}
	} else {
		hdr = []byte{0x30, 0x82, byte(total >> 8), byte(total & 0xff)}
	}
	der := append(hdr, body...)
	gc, err := parseGenericCertificateDER(der)
	if err != nil {
		t.Fatalf("dummy GC parse failed: %v", err)
	}
	if !bytes.Contains(gc.Encoded, []byte("de.schlichtherle.license.LicenseContent")) {
		t.Fatalf("encoded field missing expected content, got %q", string(gc.Encoded[:min(100, len(gc.Encoded))]))
	}
	t.Logf("Dummy GC parser OK — encoded=%dB", len(gc.Encoded))

	// Also verify parseXMLEncoder on known input
	c, err := parseXMLEncoder(xml)
	if err != nil {
		t.Fatalf("xml encoder parse: %v", err)
	}
	if c.Subject != "demo" {
		t.Fatalf("expected subject=demo got %q", c.Subject)
	}
	t.Logf("parseXMLEncoder OK — subject=%q", c.Subject)
}
