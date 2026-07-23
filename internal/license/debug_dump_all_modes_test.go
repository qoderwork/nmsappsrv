package license

import (
	"crypto/cipher"
	"crypto/des"
	"encoding/hex"
	"os"
	"testing"
)

func TestDebug_DumpAllModes(t *testing.T) {
	raw, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Skip(err)
	}
	salt := raw[0:8]
	ct := raw[8:]
	t.Logf("File len=%d, salt=%s, ct len=%d", len(raw), hex.EncodeToString(salt), len(ct))

	pw := "public_password4321"
	for _, iter := range []int{2048, 1024, 4096, 20} {
		t.Logf("=== DES / pw=%q / iter=%d ===", pw, iter)
		dk := pkcs5v1MD5([]byte(pw), salt, iter, 32)
		results := runAllCipherModes_debug(pbeCipherDES, dk, salt, ct)
		for name, p := range results {
			if len(p) < 16 {
				continue
			}
			printable := 0
			for _, b := range p[:64] {
				if (b >= 0x20 && b <= 0x7E) || b == 0x0A || b == 0x0D || b == 0x09 {
					printable++
				}
			}
			ratio := float64(printable) / 64
			isDER := p[0] == 0x30 && len(p) >= 4
			isJava := len(p) >= 4 && p[0] == 0xAC && p[1] == 0xED && p[2] == 0x00 && p[3] == 0x05
			isXML := len(p) >= 5 && string(p[0:5]) == "<?xml"
			flag := ""
			if isJava {
				flag = "★JAVA★"
			} else if isXML {
				flag = "★XML★"
			} else if isDER {
				flag = "★DER?"
			} else if ratio > 0.8 {
				flag = "(high-printable)"
			}
			t.Logf("  %-30s head=%s  p64=%.0f%% %s   first_line=%q",
				name,
				hex.EncodeToString(p[:16]),
				ratio*100,
				flag,
				previewPlain_debug(p, 80))
		}
	}
}

func runAllCipherModes_debug(alg pbeCipher, dk []byte, salt []byte, ct []byte) map[string][]byte {
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
	iv_dk8_16 := dk[8:16]
	iv_salt := make([]byte, 8)
	copy(iv_salt, salt)
	zeros := make([]byte, 8)
	ones := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

	bs := 8
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
	runCFB := func(name string, key, iv []byte) {
		blk := makeBlock(key)
		if blk == nil {
			return
		}
		buf := make([]byte, len(ct))
		cipher.NewCFBDecrypter(blk, iv).XORKeyStream(buf, ct)
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
		runCBC("CBC/dk_iv", key8, iv_dk8_16)
		runCBC("CBC/salt_iv", key8, iv_salt)
		runCBC("CBC/zero_iv", key8, zeros)
		runCBC("CBC/ones_iv", key8, ones)
		runECB("ECB", key8)
		runCFB("CFB64/dk_iv", key8, iv_dk8_16)
		runCFB("CFB64/salt_iv", key8, iv_salt)
		runCFB("CFB64/zero_iv", key8, zeros)
		runOFB("OFB/dk_iv", key8, iv_dk8_16)
		runOFB("OFB/salt_iv", key8, iv_salt)
		runCTR("CTR/dk_iv", key8, iv_dk8_16)
	}
	return out
}

func previewPlain_debug(plain []byte, n int) string {
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
