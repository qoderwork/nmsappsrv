package license

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"encoding/hex"
	"os"
	"testing"
)

func TestDebug_PBE_CipherModes(t *testing.T) {
	raw, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Skip(err)
	}
	t.Logf("file len=%d, head=%s", len(raw), hex.EncodeToString(raw[:min(48, len(raw))]))

	passwords := []string{"public_password4321", "", "public_password1234", "nmsappsrv"}
	iters := []int{20, 50, 100, 1024, 2048, 4096}
	// Key markers for "successful" decryption (first bytes):
	// - 0xACED 0005 = Java ObjectOutputStream
	// - 0x30 = DER SEQUENCE (CMS SignedData)
	// - 0x3c3f 786d 6c3e = "<?xml>"
	expectHead := [][]byte{
		{0xAC, 0xED, 0x00, 0x05}, // Java serialization
		{0x30},                   // DER SEQUENCE
		[]byte("<?xml"),
		[]byte("PKCS"),
	}

	hits := 0
	for _, pw := range passwords {
		for _, iter := range iters {
			salt := raw[0:8]
			ct := raw[8:]
			if len(ct)%8 != 0 {
				continue
			}
			pwBytes := []byte(pw)
			// --- PKCS5 key derivation + all modes ---
			for _, alg := range []pbeCipher{pbeCipherDES, pbeCipher3DES} {
				// PKCS5 dk
				dk := pkcs5v1MD5(pwBytes, salt, iter, 32)
				// --- Try all cipher modes ---
				results := runAllCipherModes(alg, dk, salt, ct)
				for name, plain := range results {
					if len(plain) < 4 {
						continue
					}
					for _, expect := range expectHead {
						if bytes.HasPrefix(plain, expect) ||
							(len(plain) >= 8 && bytes.Contains(plain[:min(64, len(plain))], expect)) {
							t.Logf("★ HIT[%d]: pw=%q iter=%d alg=%d %s → head=%s, preview=%s",
								hits, pw, iter, alg, name,
								hex.EncodeToString(plain[:min(24, len(plain))]),
								previewPlain(plain, 160))
							hits++
							if hits > 20 {
								return
							}
						}
					}
				}
			}

			// Also try: data is NOT raw[8:], maybe the entire file is ciphertext and
			// salt is embedded inside PBE params differently (not first 8 bytes)
			// Alternative interpretation: the SALT is encoded as a trailing 8 bytes.
			if len(raw) > 16 {
				salt2 := raw[len(raw)-8:]
				ct2 := raw[:len(raw)-8]
				if len(ct2)%8 != 0 {
					continue
				}
				for _, alg := range []pbeCipher{pbeCipherDES, pbeCipher3DES} {
					dk := pkcs5v1MD5(pwBytes, salt2, iter, 32)
					results := runAllCipherModes(alg, dk, salt2, ct2)
					for name, plain := range results {
						if len(plain) < 4 {
							continue
						}
						for _, expect := range expectHead {
							if bytes.HasPrefix(plain, expect) ||
								(len(plain) >= 8 && bytes.Contains(plain[:min(64, len(plain))], expect)) {
								t.Logf("★ HIT[%d] (salt=trail): pw=%q iter=%d alg=%d %s → head=%s preview=%s",
									hits, pw, iter, alg, name,
									hex.EncodeToString(plain[:min(24, len(plain))]),
									previewPlain(plain, 160))
								hits++
								if hits > 20 {
									return
								}
							}
						}
					}
				}
			}
		}
	}

	// Fallback: for the "most likely" param combo (DES/public_password4321/2048)
	// print the head of every cipher mode output (just for debugging)
	t.Logf("=== Debug dump: DES / public_password4321 / 2048 ===")
	pw := "public_password4321"
	iter := 2048
	salt := raw[0:8]
	ct := raw[8:]
	dk := pkcs5v1MD5([]byte(pw), salt, iter, 32)
	results := runAllCipherModes(pbeCipherDES, dk, salt, ct)
	for name, plain := range results {
		if len(plain) >= 16 {
			printable := 0
			for _, b := range plain[:min(64, len(plain))] {
				if (b >= 0x20 && b <= 0x7E) || b == 0x0A || b == 0x0D || b == 0x09 {
					printable++
				}
			}
			ratio := float64(printable) / float64(min(64, len(plain)))
			t.Logf("  %-28s head=%s  printable(64)=%.0f%%  first_line=%s",
				name,
				hex.EncodeToString(plain[:16]),
				ratio*100,
				previewPlain(plain, 80))
		}
	}
	t.Logf("Total hits = %d", hits)
}

func runAllCipherModes(alg pbeCipher, dk []byte, salt []byte, ct []byte) map[string][]byte {
	out := map[string][]byte{}
	makeBlock := func(key []byte) cipher.Block {
		var blk cipher.Block
		var err error
		if alg == pbeCipher3DES {
			blk, err = des.NewTripleDESCipher(key)
		} else {
			blk, err = des.NewCipher(key)
		}
		if err != nil {
			return nil
		}
		return blk
	}
	unpad := func(src []byte, bs int) ([]byte, bool) {
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

	key8 := dk[:8]
	key24 := make([]byte, 24)
	copy(key24, dk[:24])
	iv_dk8_16 := dk[8:16]  // pkcs5 dk bytes[8:16]
	iv_dk24_32 := dk[24:32] // pkcs5 dk bytes[24:32]
	iv_salt := make([]byte, 8)
	copy(iv_salt, salt)
	zeros := make([]byte, 8)

	bs := 8
	if len(ct)%bs != 0 {
		return out
	}
	maybeUnpad := func(src []byte) []byte {
		u, ok := unpad(src, bs)
		if ok {
			return u
		}
		return src
	}

	runCBC := func(name string, key, iv []byte) {
		blk := makeBlock(key)
		if blk == nil {
			return
		}
		buf := make([]byte, len(ct))
		cipher.NewCBCDecrypter(blk, iv).CryptBlocks(buf, ct)
		out[name] = maybeUnpad(buf)
	}
	runECB := func(name string, key []byte) {
		blk := makeBlock(key)
		if blk == nil {
			return
		}
		buf := make([]byte, len(ct))
		for i := 0; i < len(ct); i += bs {
			blk.Decrypt(buf[i:i+bs], ct[i:i+bs])
		}
		out[name] = maybeUnpad(buf)
	}
	runCFB := func(name string, key, iv []byte, cfbBits int) {
		blk := makeBlock(key)
		if blk == nil {
			return
		}
		buf := make([]byte, len(ct))
		if cfbBits == 64 {
			cipher.NewCFBDecrypter(blk, iv).XORKeyStream(buf, ct)
		} else {
			// 8-bit CFB: emulate
			_ = cfbBits
			// Simplified: just use 64-bit CFB for this search; 8-bit is rare without explicit mode
			cipher.NewCFBDecrypter(blk, iv).XORKeyStream(buf, ct)
		}
		out[name] = maybeUnpad(buf)
	}
	runOFB := func(name string, key, iv []byte) {
		blk := makeBlock(key)
		if blk == nil {
			return
		}
		buf := make([]byte, len(ct))
		cipher.NewOFB(blk, iv).XORKeyStream(buf, ct)
		out[name] = maybeUnpad(buf)
	}
	runCTR := func(name string, key, iv []byte) {
		blk := makeBlock(key)
		if blk == nil {
			return
		}
		buf := make([]byte, len(ct))
		cipher.NewCTR(blk, iv).XORKeyStream(buf, ct)
		out[name] = maybeUnpad(buf)
	}

	if alg == pbeCipherDES {
		runCBC("DES/CBC/pkcs5_iv", key8, iv_dk8_16)
		runCBC("DES/CBC/salt_iv", key8, iv_salt)
		runCBC("DES/CBC/zero_iv", key8, zeros)
		runECB("DES/ECB", key8)
		runCFB("DES/CFB64/pkcs5_iv", key8, iv_dk8_16, 64)
		runCFB("DES/CFB64/salt_iv", key8, iv_salt, 64)
		runOFB("DES/OFB/pkcs5_iv", key8, iv_dk8_16)
		runOFB("DES/OFB/salt_iv", key8, iv_salt)
		runCTR("DES/CTR/pkcs5_iv", key8, iv_dk8_16)
	} else {
		runCBC("3DES/CBC/pkcs5_iv", key24, iv_dk8_16)
		runCBC("3DES/CBC/salt_iv", key24, iv_salt)
		runCBC("3DES/CBC/zero_iv", key24, zeros)
		runCBC("3DES/CBC/dk24_32iv", key24, iv_dk24_32)
		runECB("3DES/ECB", key24)
		runCFB("3DES/CFB64/pkcs5_iv", key24, iv_dk8_16, 64)
		runOFB("3DES/OFB/pkcs5_iv", key24, iv_dk8_16)
	}
	return out
}
