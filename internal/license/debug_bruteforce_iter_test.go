package license

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"crypto/md5"
	"os"
	"testing"
)

// TestDebug_BruteforceIterations brute forces:
// - KDF variants (pkcs5 v1 md5: 2 variants)
// - iter: 1..10000
// - alg: DES / 3DES
// - pw: UTF-8 ("public_password4321") vs UTF-16BE vs UTF-16LE
// - salt layout: first 8 bytes vs last 8 bytes
// Looking for: output[0:4] = 0xACED0005 (Java ObjectStream magic)
func TestDebug_BruteforceIterations(t *testing.T) {
	raw, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Skip(err)
	}
	pwStr := "public_password4321"
	N := len(pwStr)

	// Password encodings
	type pwEnc struct {
		name string
		b    []byte
	}
	pwEncs := []pwEnc{
		{"UTF-8", []byte(pwStr)},
	}
	// UTF-16BE
	be := make([]byte, 0, N*2)
	for _, c := range pwStr {
		be = append(be, byte(c>>8), byte(c&0xff))
	}
	pwEncs = append(pwEncs, pwEnc{"UTF-16BE", be})
	// UTF-16LE
	le := make([]byte, 0, N*2)
	for _, c := range pwStr {
		le = append(le, byte(c&0xff), byte(c>>8))
	}
	pwEncs = append(pwEncs, pwEnc{"UTF-16LE", le})

	type saltLayout struct {
		name string
		salt []byte
		ct   []byte
	}
	layouts := []saltLayout{}
	if len(raw) >= 16 {
		layouts = append(layouts, saltLayout{"salt@head[0:8]", raw[0:8], raw[8:]})
		layouts = append(layouts, saltLayout{"salt@head[8:16]", raw[8:16], raw[16:]})
	}
	if len(raw) >= 16 {
		layouts = append(layouts, saltLayout{"salt@tail[-8:]", raw[len(raw)-8:], raw[:len(raw)-8]})
	}

	type kdfFn func(pw, salt []byte, iter, dkLen int) []byte
	kdfs := map[string]kdfFn{
		// PKCS5 v1: iter1 = MD5(pw||salt), iterN (N>1) = MD5(prev) for 1st block, then next block init = MD5(pw||salt||prev)
		"PKCS5v1": func(pw, salt []byte, iter, dkLen int) []byte { return pkcs5v1MD5(pw, salt, iter, dkLen) },
		// Full PKCS5 strict: every iter step hashes prev+pw+salt (not just prev)
		"PKCS5v2-full": kdfIterAppendsPS,
	}

	algs := []pbeCipher{pbeCipherDES, pbeCipher3DES}
	expect := []byte{0xAC, 0xED, 0x00, 0x05}
	xmlExpect := []byte("<?xml")

	hits := 0

	for _, pe := range pwEncs {
		for _, layout := range layouts {
			if len(layout.ct)%8 != 0 {
				continue
			}
			for algIdx, alg := range algs {
				keyLen := 8
				if alg == pbeCipher3DES {
					keyLen = 24
				}
				dkLen := keyLen + 8 // +8 for IV
				for kdfName, kdf := range kdfs {
					// Iterate 1..10000
					for iter := 1; iter <= 10000; iter++ {
						dk := kdf(pe.b, layout.salt, iter, dkLen)
						key := make([]byte, keyLen)
						copy(key, dk[:keyLen])
						iv := make([]byte, 8)
						copy(iv, dk[keyLen:keyLen+8])
						var (
							blk cipher.Block
							e   error
						)
						if alg == pbeCipher3DES {
							blk, e = des.NewTripleDESCipher(key)
						} else {
							blk, e = des.NewCipher(key)
						}
						if e != nil {
							continue
						}
						if len(layout.ct)%blk.BlockSize() != 0 {
							continue
						}
						out := make([]byte, len(layout.ct))
						cipher.NewCBCDecrypter(blk, iv).CryptBlocks(out, layout.ct)
						// Try unpad
						if len(out) >= 4 {
							if bytes.HasPrefix(out, expect) ||
								(len(out) >= 5 && bytes.HasPrefix(out, xmlExpect)) {
								// Check padding validity for actual unpad version
								var u []byte
								pad := int(out[len(out)-1])
								if pad > 0 && pad <= blk.BlockSize() && pad <= len(out) {
									ok := true
									for i := 0; i < pad; i++ {
										if out[len(out)-1-i] != byte(pad) {
											ok = false
											break
										}
									}
									if ok {
										u = out[:len(out)-pad]
									}
								}
								algName := "DES"
								if algIdx == 1 {
									algName = "3DES"
								}
								head := out[:min(32, len(out))]
								t.Logf("★★★ HIT: pw=%s layout=%s kdf=%s iter=%d alg=%s  unpadded=%v  head=%q",
									pe.name, layout.name, kdfName, iter, algName, u != nil, head)
								hits++
								if hits >= 5 {
									return
								}
							}
						}
					}
				}
			}
		}
	}

	if hits == 0 {
		t.Logf("No brute force hits for ACED0005 or <?xml> in 1..10000 iterations")
		t.Logf("Trying 10001..65536 for UTF-8/PKCS5v1/DES/salt@head only (quick pass)...")
		pe := pwEncs[0]
		layout := layouts[0]
		kdf := kdfs["PKCS5v1"]
		dkLen := 16
		_ = pbeCipherDES
		for iter := 10001; iter <= 65536; iter++ {
			dk := kdf(pe.b, layout.salt, iter, dkLen)
			key := dk[:8]
			iv := dk[8:16]
			blk, e := des.NewCipher(key)
			if e != nil {
				continue
			}
			if len(layout.ct)%8 != 0 {
				continue
			}
			out := make([]byte, len(layout.ct))
			cipher.NewCBCDecrypter(blk, iv).CryptBlocks(out, layout.ct)
			if len(out) >= 4 && (bytes.HasPrefix(out, expect) ||
				(len(out) >= 5 && bytes.HasPrefix(out, xmlExpect))) {
				t.Logf("★★★ HIT(extended): iter=%d alg=DES  head=%q", iter, out[:min(32, len(out))])
				hits++
				break
			}
		}
		if hits == 0 {
			t.Logf("No extended search hits either.")
		}
	}
}

// kdfIterAppendsPS: each iteration step re-appends pw+salt (standard PKCS5 full chain)
func kdfIterAppendsPS(pw, salt []byte, iterations, dkLen int) []byte {
	uLen := md5.Size
	nBlock := (dkLen + uLen - 1) / uLen
	dk := make([]byte, 0, nBlock*uLen)

	h := md5.New()
	h.Write(pw)
	h.Write(salt)
	prev := h.Sum(nil)

	for l := 0; l < nBlock; l++ {
		u := prev
		// iterations - 1 more hashes: each step = MD5(prev + pw + salt)
		for i := 1; i < iterations; i++ {
			h.Reset()
			h.Write(u)
			h.Write(pw)
			h.Write(salt)
			u = h.Sum(nil)
		}
		dk = append(dk, u...)
		if l+1 < nBlock {
			h.Reset()
			h.Write(pw)
			h.Write(salt)
			h.Write(u)
			prev = h.Sum(nil)
		}
	}
	return dk[:dkLen]
}
