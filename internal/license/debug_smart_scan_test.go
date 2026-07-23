package license

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"sync"
	"testing"
)

func TestDebug_SmartQuickScan(t *testing.T) {
	raw, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Skip(err)
	}
	t.Logf("File len=%d, salt=%s", len(raw), hex.EncodeToString(raw[0:8]))

	markers := [][]byte{
		[]byte("<java"),
		[]byte("<?xml"),
		[]byte("GenericCertificate"),
		[]byte("LicenseContent"),
		[]byte("<object"),
		[]byte("<void"),
		[]byte("HashMap"),
		[]byte{0xAC, 0xED, 0x00, 0x05},
	}
	containsAnyMarker := func(b []byte) []string {
		var hits []string
		for _, m := range markers {
			if bytes.Contains(b, m) {
				hits = append(hits, string(m))
			}
		}
		return hits
	}
	printableRatio := func(b []byte) float64 {
		n := min(256, len(b))
		p := 0
		for _, c := range b[:n] {
			if (c >= 0x20 && c <= 0x7E) || c == 0x0A || c == 0x0D || c == 0x09 {
				p++
			}
		}
		return float64(p) / float64(n)
	}

	pwStr := "public_password4321"
	type enc struct {
		name string
		b    []byte
	}
	N := len(pwStr)
	utf16be := make([]byte, 0, N*2)
	for _, c := range pwStr {
		utf16be = append(utf16be, byte(c>>8), byte(c&0xff))
	}
	utf16le := make([]byte, 0, N*2)
	for _, c := range pwStr {
		utf16le = append(utf16le, byte(c&0xff), byte(c>>8))
	}
	pwEncs := []enc{
		{"UTF-8", []byte(pwStr)},
		{"UTF-16BE", utf16be},
		{"UTF-16LE", utf16le},
	}

	layouts := []struct {
		name string
		salt []byte
		ct   []byte
	}{
		{"salt@head[0:8]", raw[0:8], raw[8:]},
	}

	iters := []int{1, 20, 50, 100, 200, 256, 500, 1000, 1024, 1234, 2000, 2048, 3000, 4096, 5000, 65536}
	algs := []pbeCipher{pbeCipherDES, pbeCipher3DES}
	kdfs := map[string]func(pw, salt []byte, iter, dkLen int) []byte{
		"PKCS5v1(SunJCE)": pkcs5v1MD5,
		"PKCS5v2(PS each)": kdfIterAppendsPS2,
	}

	hits := 0
	for _, pe := range pwEncs {
		for _, lt := range layouts {
			if len(lt.ct)%8 != 0 {
				continue
			}
			for _, alg := range algs {
				keyLen := 8
				if alg == pbeCipher3DES {
					keyLen = 24
				}
				dkLen := keyLen + 8
				for kdfName, kdf := range kdfs {
					for _, iter := range iters {
						dk := kdf(pe.b, lt.salt, iter, dkLen)
						key := make([]byte, keyLen)
						copy(key, dk[:keyLen])
						iv_dk := make([]byte, 8)
						copy(iv_dk, dk[keyLen:keyLen+8])

						ivs := map[string][]byte{
							"iv=dk_tail":          iv_dk,
							"iv=salt":             lt.salt,
							"iv=zeros":            make([]byte, 8),
						}
						if alg == pbeCipherDES {
							ivs["iv=dk_head(key=dk8_15)"] = dk[0:8]
						}
						for ivName, iv := range ivs {
							modes := map[string]func(cipher.Block, []byte, []byte) []byte{
								"CBC+PKCS5": doCBC,
								"CBC_raw":   doCBCraw,
								"ECB+PKCS5": doECB,
								"ECB_raw":   doECBraw,
								"CFB64":     doCFB,
								"OFB":       doOFB,
								"CTR":       doCTR,
							}
							for modeName, modeFn := range modes {
								var blk cipher.Block
								var e error
								if alg == pbeCipher3DES {
									blk, e = des.NewTripleDESCipher(key)
								} else {
									blk, e = des.NewCipher(key)
								}
								if e != nil {
									continue
								}
								plain := modeFn(blk, iv, lt.ct)
								if len(plain) < 16 {
									continue
								}
								mh := containsAnyMarker(plain)
								pr := printableRatio(plain)
								algN := "DES"
								if alg == pbeCipher3DES {
									algN = "3DES"
								}
								if len(mh) > 0 {
									t.Logf("★ HIT markers=%v | pw=%s lt=%s kdf=%s iter=%d alg=%s iv=%s mode=%s | ratio=%.0f%% head=%s",
										mh, pe.name, lt.name, kdfName, iter, algN, ivName, modeName,
										pr*100, hex.EncodeToString(plain[:min(16, len(plain))]))
									t.Logf("   preview: %s", previewSmart(plain, 300))
									hits++
								} else if pr > 0.82 {
									t.Logf("  (high printable %.0f%%) pw=%s iter=%d alg=%s kdf=%s iv=%s mode=%s head=%s",
										pr*100, pe.name, iter, algN, kdfName, ivName, modeName,
										previewSmart(plain, 80))
								}
							}
						}
					}
				}
			}
		}
	}
	t.Logf("Done. Total marker hits = %d", hits)
}

type md5Hasher struct{ h hash.Hash }

var md5Pool = sync.Pool{New: func() interface{} { return &md5Hasher{md5.New()} }}

func kdfIterAppendsPS2(pw, salt []byte, iterations, dkLen int) []byte {
	uLen := 16
	nBlock := (dkLen + uLen - 1) / uLen
	dk := make([]byte, 0, nBlock*uLen)
	h := md5Pool.Get().(*md5Hasher)
	defer md5Pool.Put(h)
	h.h.Reset()
	h.h.Write(pw)
	h.h.Write(salt)
	prev := h.h.Sum(nil)
	for l := 0; l < nBlock; l++ {
		u := prev
		for i := 1; i < iterations; i++ {
			h.h.Reset()
			h.h.Write(u)
			h.h.Write(pw)
			h.h.Write(salt)
			u = h.h.Sum(nil)
		}
		dk = append(dk, u...)
		if l+1 < nBlock {
			h.h.Reset()
			h.h.Write(pw)
			h.h.Write(salt)
			h.h.Write(u)
			prev = h.h.Sum(nil)
		}
	}
	return dk[:dkLen]
}

func doCBC(blk cipher.Block, iv, ct []byte) []byte {
	if len(ct)%blk.BlockSize() != 0 {
		return nil
	}
	out := make([]byte, len(ct))
	cipher.NewCBCDecrypter(blk, iv).CryptBlocks(out, ct)
	if len(out) == 0 {
		return out
	}
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
			return out[:len(out)-pad]
		}
	}
	return out
}

func doCBCraw(blk cipher.Block, iv, ct []byte) []byte {
	if len(ct)%blk.BlockSize() != 0 {
		return nil
	}
	out := make([]byte, len(ct))
	cipher.NewCBCDecrypter(blk, iv).CryptBlocks(out, ct)
	return out
}

func doECB(blk cipher.Block, _, ct []byte) []byte {
	if len(ct)%blk.BlockSize() != 0 {
		return nil
	}
	out := make([]byte, len(ct))
	bs := blk.BlockSize()
	for i := 0; i < len(ct); i += bs {
		blk.Decrypt(out[i:i+bs], ct[i:i+bs])
	}
	if len(out) == 0 {
		return out
	}
	pad := int(out[len(out)-1])
	if pad > 0 && pad <= bs && pad <= len(out) {
		ok := true
		for i := 0; i < pad; i++ {
			if out[len(out)-1-i] != byte(pad) {
				ok = false
				break
			}
		}
		if ok {
			return out[:len(out)-pad]
		}
	}
	return out
}

func doECBraw(blk cipher.Block, _, ct []byte) []byte {
	if len(ct)%blk.BlockSize() != 0 {
		return nil
	}
	out := make([]byte, len(ct))
	bs := blk.BlockSize()
	for i := 0; i < len(ct); i += bs {
		blk.Decrypt(out[i:i+bs], ct[i:i+bs])
	}
	return out
}

func doCFB(blk cipher.Block, iv, ct []byte) []byte {
	out := make([]byte, len(ct))
	cipher.NewCFBDecrypter(blk, iv).XORKeyStream(out, ct)
	return out
}

func doOFB(blk cipher.Block, iv, ct []byte) []byte {
	out := make([]byte, len(ct))
	cipher.NewOFB(blk, iv).XORKeyStream(out, ct)
	return out
}

func doCTR(blk cipher.Block, iv, ct []byte) []byte {
	out := make([]byte, len(ct))
	cipher.NewCTR(blk, iv).XORKeyStream(out, ct)
	return out
}

func previewSmart(b []byte, n int) string {
	if len(b) > n {
		b = b[:n]
	}
	return fmt.Sprintf("%q", string(b))
}
