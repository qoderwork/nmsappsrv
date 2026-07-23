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

func TestDebug_PBE_VariantScavenger(t *testing.T) {
	raw, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Skip(err)
	}

	passwords := []string{
		"public_password4321",
		"",
		"public_password1234",
		"private_password4321",
		"nmsappsrv",
	}
	iters := []int{20, 50, 100, 256, 500, 1000, 1024, 2000, 2048, 4096, 8192, 65536}

	xml := [][]byte{
		[]byte("<?xml"),
		[]byte("<java"),
		[]byte("LicenseContent"),
		[]byte("HashMap"),
		[]byte("<void"),
	}
	isHit := func(b []byte) int {
		c := 0
		for _, m := range xml {
			if bytes.Contains(b, m) {
				c++
			}
		}
		return c
	}

	hits := 0
	bestHit := -1
	bestDesc := ""

	for pwIdx, pw := range passwords {
		pwBytes := []byte(pw)
		pwUTF16LE := make([]byte, 0, len(pwBytes)*2)
		for _, b := range pwBytes {
			pwUTF16LE = append(pwUTF16LE, b, 0)
		}
		pwVariants := [][]byte{pwBytes, pwUTF16LE}
		if pw == "" {
			pwVariants = [][]byte{pwBytes}
		}
		for pvi, pwv := range pwVariants {
			for _, iter := range iters {
				for saltDataOff := 0; saltDataOff < 2; saltDataOff++ {
					var salt, ct []byte
					switch saltDataOff {
					case 0:
						if len(raw) < 16 {
							continue
						}
						salt = raw[0:8]
						ct = raw[8:]
					case 1:
						if len(raw) < 24 {
							continue
						}
						salt = raw[8:16]
						ct = raw[16:]
					}
					if len(ct)%8 != 0 {
						continue
					}

					for _, alg := range []pbeCipher{pbeCipherDES, pbeCipher3DES} {
						plain, err := pbeDecryptVariant(ct, pwv, salt, iter, alg, variantPKCS5)
						if err == nil && len(plain) >= 32 {
							if sc := isHit(plain); sc > bestHit {
								bestHit = sc
								bestDesc = desc("PKCS5", pwIdx, pvi, pw, iter, alg, saltDataOff)
								if sc > 0 {
									t.Logf("★ HIT sc=%d %s head=%s preview=%s",
										sc, bestDesc,
										hex.EncodeToString(plain[:min(16, len(plain))]),
										previewPlain(plain, 200))
									hits++
								}
							}
						}
						plain2, err := pbeDecryptVariant(ct, pwv, salt, iter, alg, variantPKCS5_IVDirectSalt)
						if err == nil && len(plain2) >= 32 {
							if sc := isHit(plain2); sc > bestHit {
								bestHit = sc
								bestDesc = desc("IV=Salt", pwIdx, pvi, pw, iter, alg, saltDataOff)
								if sc > 0 {
									t.Logf("★ HIT sc=%d %s head=%s preview=%s",
										sc, bestDesc,
										hex.EncodeToString(plain2[:min(16, len(plain2))]),
										previewPlain(plain2, 200))
									hits++
								}
							}
						}
						plain3, err := pbeDecryptVariant(ct, pwv, salt, iter, alg, variantPKCS12)
						if err == nil && len(plain3) >= 32 {
							if sc := isHit(plain3); sc > bestHit {
								bestHit = sc
								bestDesc = desc("PKCS12", pwIdx, pvi, pw, iter, alg, saltDataOff)
								if sc > 0 {
									t.Logf("★ HIT sc=%d %s head=%s preview=%s",
										sc, bestDesc,
										hex.EncodeToString(plain3[:min(16, len(plain3))]),
										previewPlain(plain3, 200))
									hits++
								}
							}
						}
						plain4, err := pbeDecryptVariant(ct, pwv, salt, iter, alg, variantKeySpecIVSalt)
						if err == nil && len(plain4) >= 32 {
							if sc := isHit(plain4); sc > bestHit {
								bestHit = sc
								bestDesc = desc("KeySpec+IVSalt", pwIdx, pvi, pw, iter, alg, saltDataOff)
								if sc > 0 {
									t.Logf("★ HIT sc=%d %s head=%s preview=%s",
										sc, bestDesc,
										hex.EncodeToString(plain4[:min(16, len(plain4))]),
										previewPlain(plain4, 200))
									hits++
								}
							}
						}
					}
				}
			}
		}
	}

	t.Logf("Total XML hits = %d. Best XML score = %d via %s", hits, bestHit, bestDesc)
	if bestHit <= 0 {
		t.Logf("Searching for DER/printable near-misses...")
		pw := "public_password4321"
		pwBytes := []byte(pw)
		salt := raw[0:8]
		ct := raw[8:]
		for _, iter := range []int{2048, 1024, 4096, 2000, 20, 1} {
			for vi, variant := range []int{variantPKCS5, variantPKCS5_IVDirectSalt, variantPKCS12, variantKeySpecIVSalt} {
				for _, alg := range []pbeCipher{pbeCipherDES, pbeCipher3DES} {
					plain, err := pbeDecryptVariant(ct, pwBytes, salt, iter, alg, variant)
					if err != nil || len(plain) < 32 {
						continue
					}
					printable := 0
					for _, b := range plain {
						if (b >= 0x20 && b <= 0x7E) || b == 0x0A || b == 0x0D || b == 0x09 {
							printable++
						}
					}
					ratio := float64(printable) / float64(len(plain))
					if plain[0] == 0x30 || ratio > 0.55 {
						t.Logf("  NEAR: pw=%q iter=%d alg=%d variant=%d ratio=%.0f%% start=%x",
							pw, iter, alg, vi, ratio*100, plain[:min(12, len(plain))])
					}
				}
			}
		}
	}
}

const (
	variantPKCS5              = 0
	variantPKCS5_IVDirectSalt = 1
	variantPKCS12             = 2
	variantKeySpecIVSalt      = 3
)

func pbeDecryptVariant(ct, pw, salt []byte, iter int, alg pbeCipher, variant int) ([]byte, error) {
	var (
		key []byte
		iv  []byte
		blk cipher.Block
		err error
	)
	dkLen := 16
	if alg == pbeCipher3DES {
		dkLen = 32
	}

	switch variant {
	case variantPKCS5:
		dk := pkcs5v1MD5(pw, salt, iter, dkLen)
		if alg == pbeCipher3DES {
			key = make([]byte, 24)
			copy(key, dk[:24])
			iv = make([]byte, 8)
			copy(iv, dk[24:32])
		} else {
			key = make([]byte, 8)
			copy(key, dk[:8])
			iv = make([]byte, 8)
			copy(iv, dk[8:16])
		}
	case variantPKCS5_IVDirectSalt:
		if alg == pbeCipher3DES {
			key = pkcs5v1MD5(pw, salt, iter, 24)
		} else {
			key = pkcs5v1MD5(pw, salt, iter, 8)
		}
		iv = make([]byte, 8)
		copy(iv, salt)
	case variantKeySpecIVSalt:
		kb := pkcs5v1MD5(pw, salt, iter, dkLen)
		if alg == pbeCipher3DES {
			key = make([]byte, 24)
			copy(key, kb[:24])
		} else {
			key = make([]byte, 8)
			copy(key, kb[:8])
		}
		iv = make([]byte, 8)
		copy(iv, salt)
	case variantPKCS12:
		keyLen := 8
		if alg == pbeCipher3DES {
			keyLen = 24
		}
		key = pkcs12Derive(pw, salt, iter, 1, keyLen)
		iv = pkcs12Derive(pw, salt, iter, 2, 8)
	}

	if alg == pbeCipher3DES {
		blk, err = des.NewTripleDESCipher(key)
	} else {
		blk, err = des.NewCipher(key)
	}
	if err != nil {
		return nil, err
	}
	if len(ct)%blk.BlockSize() != 0 {
		return nil, nil
	}
	mode := cipher.NewCBCDecrypter(blk, iv)
	out := make([]byte, len(ct))
	mode.CryptBlocks(out, ct)
	if len(out) == 0 {
		return out, nil
	}
	pad := int(out[len(out)-1])
	if pad > 0 && pad <= blk.BlockSize() && pad <= len(out) {
		valid := true
		for i := 0; i < pad; i++ {
			if out[len(out)-1-i] != byte(pad) {
				valid = false
				break
			}
		}
		if valid {
			return out[:len(out)-pad], nil
		}
	}
	return out, nil
}

func pkcs12Derive(pw []byte, salt []byte, iter int, id byte, n int) []byte {
	u := md5.Size
	v := 64
	bePw := make([]byte, 0, len(pw)*2+2)
	for _, b := range pw {
		bePw = append(bePw, 0, b)
	}
	bePw = append(bePw, 0, 0)
	D := make([]byte, v)
	for i := range D {
		D[i] = id
	}
	S := repeatToLen(salt, v*((len(salt)+v-1)/v))
	P := repeatToLen(bePw, v*((len(bePw)+v-1)/v))
	I := append(append([]byte(nil), S...), P...)
	c := (n + u - 1) / u
	var A []byte
	for i := 0; i < c; i++ {
		h := md5.New()
		h.Write(D)
		h.Write(I)
		cur := h.Sum(nil)
		for r := 1; r < iter; r++ {
			h2 := md5.New()
			h2.Write(cur)
			cur = h2.Sum(nil)
		}
		A = append(A, cur...)
		B := repeatToLen(cur, v)
		nI := len(I) / v
		newI := make([]byte, len(I))
		for j := 0; j < nI; j++ {
			block := I[j*v : (j+1)*v]
			var carry uint16 = 1
			for k := v - 1; k >= 0; k-- {
				carry += uint16(block[k]) + uint16(B[k])
				newI[j*v+k] = byte(carry & 0xff)
				carry >>= 8
			}
		}
		I = newI
	}
	return A[:n]
}

func repeatToLen(in []byte, n int) []byte {
	if len(in) == 0 || n == 0 {
		return make([]byte, n)
	}
	out := make([]byte, n)
	for i := 0; i < n; i += len(in) {
		copy(out[i:], in[:min(len(in), n-i)])
	}
	return out
}

func desc(kind string, pwIdx, pvi int, pw string, iter int, alg pbeCipher, saltDataOff int) string {
	return fmt.Sprintf("%s pwIdx=%d/%d pw=%q iter=%d alg=%d saltLayout=%d",
		kind, pwIdx, pvi, pw, iter, alg, saltDataOff)
}

func previewPlain(plain []byte, n int) string {
	if len(plain) > n {
		plain = plain[:n]
	}
	s := make([]byte, len(plain))
	for i, b := range plain {
		if (b >= 0x20 && b <= 0x7E) || b == 0x0A || b == 0x0D || b == 0x09 {
			s[i] = b
		} else {
			s[i] = '.'
		}
	}
	return string(s)
}
