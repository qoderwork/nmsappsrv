package license

import (
	"bytes"
	"compress/gzip"
	"crypto/cipher"
	"crypto/des"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"html"
	"os"
	"testing"
)

// TestDebug_JavaExactDecrypt 用从 TrueLicense 1.32 字节码中反编译出的
// 确切 PBE 参数解密 license.lic：
//   - 算法: PBEWithMD5AndDES
//   - salt: 硬编码 {0xCE,0xFB,0xDE,0xAC,0x05,0x02,0x19,0x71}
//   - iteration count: 2005
//   - 密码: CipherParam.getKeyPwd() = storePass = "public_password4321"
//   - 整个文件都是密文（无 salt 前缀）
//   - 解密后需 GZIP 解压
func TestDebug_JavaExactDecrypt(t *testing.T) {
	raw, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Skip(err)
	}
	t.Logf("License file: %d bytes", len(raw))

	// TrueLicense 1.32 硬编码的 salt（从字节码反编译）
	salt := []byte{0xCE, 0xFB, 0xDE, 0xAC, 0x05, 0x02, 0x19, 0x71}
	t.Logf("Hardcoded salt: %s", hex.EncodeToString(salt))

	// iteration count = 2005
	iterations := 2005

	// 密码 = storePass
	password := debugStorePass
	t.Logf("Password: %q, iterations: %d", password, iterations)

	// PKCS5 v1.5 MD5 KDF — 生成 16 字节 (8 key + 8 iv)
	dk := pkcs5v1MD5([]byte(password), salt, iterations, 16)
	key := dk[:8]
	iv := dk[8:16]
	t.Logf("Derived key: %s", hex.EncodeToString(key))
	t.Logf("Derived IV:  %s", hex.EncodeToString(iv))

	// DES CBC 解密
	block, err := des.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher DES: %v", err)
	}
	if len(raw)%block.BlockSize() != 0 {
		t.Fatalf("ciphertext len %d not multiple of block size %d", len(raw), block.BlockSize())
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(raw))
	mode.CryptBlocks(decrypted, raw)
	t.Logf("PBE decrypted: %d bytes", len(decrypted))
	t.Logf("First 32 bytes: %s", hex.EncodeToString(decrypted[:32]))

	// 检查 PKCS5 padding
	pad := int(decrypted[len(decrypted)-1])
	t.Logf("Padding byte: %d", pad)
	if pad > 0 && pad <= 8 {
		valid := true
		for i := 0; i < pad; i++ {
			if decrypted[len(decrypted)-1-i] != byte(pad) {
				valid = false
				break
			}
		}
		if valid {
			t.Logf("PKCS5 padding valid, stripping %d bytes", pad)
			decrypted = decrypted[:len(decrypted)-pad]
		} else {
			t.Logf("PKCS5 padding invalid, keeping raw")
		}
	}

	// 检查 GZIP 魔数 (0x1f 0x8b)
	if len(decrypted) >= 2 && decrypted[0] == 0x1f && decrypted[1] == 0x8b {
		t.Logf("★ GZIP magic detected! Decompressing...")
		gzReader, err := gzip.NewReader(bytes.NewReader(decrypted))
		if err != nil {
			t.Fatalf("gzip reader: %v", err)
		}
		defer gzReader.Close()
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(gzReader); err != nil {
			t.Fatalf("gzip decompress: %v", err)
		}
		uncompressed := buf.Bytes()
		t.Logf("★ GZIP decompressed: %d bytes", len(uncompressed))

		// 打印前 500 字节（应该是 XML）
		preview := uncompressed
		if len(preview) > 500 {
			preview = preview[:500]
		}
		t.Logf("★ Decompressed content preview:\n%s", string(preview))

		// 尝试解析 XML
		if idx := bytes.Index(uncompressed, []byte("<?xml")); idx >= 0 {
			t.Logf("★ Found <?xml at offset %d", idx)
		}
		if idx := bytes.Index(uncompressed, []byte("<java")); idx >= 0 {
			t.Logf("★ Found <java at offset %d", idx)
		}
		if idx := bytes.Index(uncompressed, []byte("LicenseContent")); idx >= 0 {
			t.Logf("★ Found LicenseContent at offset %d", idx)
		}

		// GenericCertificate XML 中 encoded 属性包含 HTML 转义的 LicenseContent XML
		// 需要提取并反转义
		encodedStart := []byte(`<void property="encoded">`)
		if idx := bytes.Index(uncompressed, encodedStart); idx >= 0 {
			// 找到 <string> 标签开始
			strStart := bytes.Index(uncompressed[idx:], []byte("<string>"))
			if strStart >= 0 {
				strStart += idx + len([]byte("<string>"))
				strEnd := bytes.Index(uncompressed[strStart:], []byte("</string>"))
				if strEnd >= 0 {
					escapedXML := string(uncompressed[strStart : strStart+strEnd])
					t.Logf("★ Found encoded (escaped) XML, length=%d", len(escapedXML))

					// HTML 反转义
					licenseXML := htmlUnescape(escapedXML)
					t.Logf("★ Unescaped LicenseContent XML length=%d", len(licenseXML))
					preview2 := licenseXML
					if len(preview2) > 500 {
						preview2 = preview2[:500]
					}
					t.Logf("★ LicenseContent XML preview:\n%s", preview2)

					// 用 parseXMLEncoder 解析反转义后的 XML
					content, err := parseXMLEncoder([]byte(licenseXML))
					if err != nil {
						t.Logf("parseXMLEncoder err: %v", err)
					} else {
						t.Logf("★ LicenseContent parsed successfully!")
						t.Logf("  Subject:       %q", content.Subject)
						t.Logf("  ConsumerType:  %q", content.ConsumerType)
						t.Logf("  ConsumerAmount: %d", content.ConsumerAmount)
						if !content.NotBefore.IsZero() {
							t.Logf("  NotBefore:     %s", content.NotBefore.Format("2006-01-02 15:04:05"))
						}
						if !content.NotAfter.IsZero() {
							t.Logf("  NotAfter:      %s", content.NotAfter.Format("2006-01-02 15:04:05"))
						}
						if !content.Issued.IsZero() {
							t.Logf("  Issued:        %s", content.Issued.Format("2006-01-02 15:04:05"))
						}
						if content.Info != "" {
							t.Logf("  Info:          %q", content.Info)
						}
						t.Logf("  Extra keys:    %d", len(content.Extra))
						for k, v := range content.Extra {
							t.Logf("    %s = %s", k, v)
						}
					}
				}
			}
		}
	} else {
		t.Logf("No GZIP magic (0x1f 0x8b). First 2 bytes: 0x%02x 0x%02x",
			decrypted[0], decrypted[1])

		// 可能不需要 GZIP，直接尝试找 XML
		for _, m := range [][]byte{[]byte("<?xml"), []byte("<java"), []byte("LicenseContent")} {
			if idx := bytes.Index(decrypted, m); idx >= 0 {
				t.Logf("★ Found %q at offset %d", m, idx)
			}
		}

		// 也尝试用 GZIP 解压（可能 padding 去掉后才是 GZIP）
		t.Logf("Trying GZIP on raw decrypted (no unpad)...")
		gzReader2, err2 := gzip.NewReader(bytes.NewReader(decrypted))
		if err2 == nil {
			var buf2 bytes.Buffer
			buf2.ReadFrom(gzReader2)
			gzReader2.Close()
			if buf2.Len() > 0 {
				t.Logf("★ GZIP (raw) decompressed: %d bytes", buf2.Len())
				t.Logf("  preview: %q", string(buf2.Bytes()[:min(500, buf2.Len())]))
			}
		}
	}

	// 也尝试 salt@0 布局（文件前8字节作为salt，其余作为密文）
	t.Logf("\n=== Also trying salt-from-file layout ===")
	saltFromFile := raw[:8]
	ctFromFile := raw[8:]
	t.Logf("salt from file: %s, ct: %d bytes", hex.EncodeToString(saltFromFile), len(ctFromFile))
	if len(ctFromFile)%8 == 0 {
		dk2 := pkcs5v1MD5([]byte(password), saltFromFile, iterations, 16)
		block2, _ := des.NewCipher(dk2[:8])
		mode2 := cipher.NewCBCDecrypter(block2, dk2[8:16])
		dec2 := make([]byte, len(ctFromFile))
		mode2.CryptBlocks(dec2, ctFromFile)
		t.Logf("salt-from-file decrypted first 4: %s", hex.EncodeToString(dec2[:4]))
		if len(dec2) >= 2 && dec2[0] == 0x1f && dec2[1] == 0x8b {
			t.Logf("★ GZIP magic with salt-from-file!")
		}
	}
}

func md5Sum(b []byte) []byte {
	s := md5.Sum(b)
	return s[:]
}

func htmlUnescape(s string) string {
	return html.UnescapeString(s)
}

var _ = fmt.Sprintf
