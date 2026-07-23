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

func TestDebug_PasswordEncoding(t *testing.T) {
	raw, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Skip(err)
	}
	salt := raw[0:8]
	ct := raw[8:]

	pwStr := "public_password4321"
	// Test various password encodings used by different PBE impls.
	// Order: UTF-8, UTF-16BE, UTF-16BE+2null (PKCS12), UTF-16LE, ASCII low-byte (== UTF-8)
	type encT struct {
		name string
		b    []byte
	}
	makeEnc := func() []encT {
		N := len(pwStr)
		utf8 := []byte(pwStr) // 19 bytes
		utf16be := make([]byte, 0, N*2)
		for _, c := range pwStr {
			utf16be = append(utf16be, byte(c>>8), byte(c&0xff))
		}
		utf16bePkcs12 := append(append([]byte(nil), utf16be...), 0x00, 0x00)
		utf16le := make([]byte, 0, N*2)
		for _, c := range pwStr {
			utf16le = append(utf16le, byte(c&0xff), byte(c>>8))
		}
		iso8859 := make([]byte, N)
		for i, c := range pwStr {
			iso8859[i] = byte(c & 0xff)
		}
		return []encT{
			{"UTF-8 (ASCII)", utf8},
			{"UTF-16BE (SunJCE PBEKey)", utf16be},
			{"UTF-16BE + PKCS12 NUL", utf16bePkcs12},
			{"UTF-16LE", utf16le},
			{"ISO-8859-1 (low byte)", iso8859},
		}
	}
	encs := makeEnc()
	iters := []int{2048, 1024, 4096, 20, 2000, 1000, 512}
	algs := []pbeCipher{pbeCipherDES, pbeCipher3DES}

	// Expected markers
	checks := func(name, pwe string, iter int, alg pbeCipher, plain []byte) {
		if len(plain) < 16 {
			return
		}
		java := len(plain) >= 4 && plain[0] == 0xAC && plain[1] == 0xED && plain[2] == 0x00 && plain[3] == 0x05
		xml := len(plain) >= 5 && bytes.HasPrefix(plain, []byte("<?xml"))
		der := plain[0] == 0x30 && len(plain) >= 4
		// Count printable
		printable := 0
		for _, b := range plain[:min(128, len(plain))] {
			if (b >= 0x20 && b <= 0x7E) || b == 0x0A || b == 0x0D || b == 0x09 {
				printable++
			}
		}
		ratio := float64(printable) / float64(min(128, len(plain)))
		flag := ""
		if java {
			flag = "★★★ JAVA SER STREAM! ★★★"
		} else if xml {
			flag = "★★★ XML! ★★★"
		} else if der && (plain[1]&0x80) != 0 {
			// likely DER SEQUENCE
			flag = "★ DER?"
		} else if ratio > 0.9 {
			flag = "(printable)"
		}
		if flag != "" || ratio > 0.7 {
			algn := "DES"
			if alg == pbeCipher3DES {
				algn = "3DES"
			}
			t.Logf("%s %s pw=%s iter=%d %s ratio=%.0f%%  head=%s  preview=%s",
				flag, name, pwe, iter, algn,
				ratio*100,
				hex.EncodeToString(plain[:min(24, len(plain))]),
				previewPlain_pe(plain, 200))
		}
	}

	for _, enc := range encs {
		for _, iter := range iters {
			for _, alg := range algs {
				dk := sunJcePbeDerive(enc.b, salt, iter, alg)
				keyLen := 8
				if alg == pbeCipher3DES {
					keyLen = 24
				}
				key := make([]byte, keyLen)
				copy(key, dk[:keyLen])
				iv := make([]byte, 8)
				copy(iv, dk[keyLen:keyLen+8])

				// CBC + PKCS5 unpad
				var blk cipher.Block
				if alg == pbeCipher3DES {
					blk, err = des.NewTripleDESCipher(key)
				} else {
					blk, err = des.NewCipher(key)
				}
				if err != nil {
					continue
				}
				if len(ct)%blk.BlockSize() != 0 {
					continue
				}
				out := make([]byte, len(ct))
				cipher.NewCBCDecrypter(blk, iv).CryptBlocks(out, ct)
				// try with PKCS5 unpad
				u, ok := unpadPe(out, blk.BlockSize())
				if ok {
					checks("CBC+PKCS5unpad", enc.name, iter, alg, u)
				} else {
					checks("CBC(no unpad)", enc.name, iter, alg, out)
				}
				// Also try without unpadding but just raw
			}
		}
	}
}

// sunJcePbeDerive follows SunJCE PBE key derivation exactly as documented:
//
//	DK = T_1 || T_2 || ...
//	T_1       = MD5^iter (P || S)
//	T_i (i>1) = MD5^iter (P || S || T_{i-1})
//
// where MD5^iter means:
//
//	cur = initial_input
//	for i = 1..iter-1: cur = MD5(cur)   // only hashes previous output!
func sunJcePbeDerive(pw, salt []byte, iter int, alg pbeCipher) []byte {
	var out []byte
	var prev []byte
	blocks := 1
	if alg == pbeCipher3DES {
		blocks = 2
	}
	for l := 0; l < blocks; l++ {
		h := md5.New()
		if l == 0 {
			h.Write(pw)
			h.Write(salt)
		} else {
			h.Write(pw)
			h.Write(salt)
			h.Write(prev)
		}
		u := h.Sum(nil)
		for i := 1; i < iter; i++ {
			h2 := md5.New()
			h2.Write(u)
			u = h2.Sum(nil)
		}
		prev = u
		out = append(out, u...)
	}
	return out
}

func unpadPe(src []byte, bs int) ([]byte, bool) {
	if len(src) == 0 {
		return src, false
	}
	pad := int(src[len(src)-1])
	if pad <= 0 || pad > bs || pad > len(src) {
		return src, false
	}
	for i := 0; i < pad; i++ {
		if src[len(src)-1-i] != byte(pad) {
			return src, false
		}
	}
	return src[:len(src)-pad], true
}

func previewPlain_pe(plain []byte, n int) string {
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
	return fmt.Sprintf("%q", string(s))
}
