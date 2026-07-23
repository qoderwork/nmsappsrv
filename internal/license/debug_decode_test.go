package license

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/square/certigo/jceks"
)

const (
	debugStorePath    = "./keys/publicCerts.keystore"
	debugLicensePath  = "d:/1-sw-work/das/wirelessnetwork/nmsappsrv/license.lic"
	debugStorePass    = "public_password4321"
	debugPublicAlias  = "publiccert"
	debugPBEIterations = 2048
)

func TestDebug_LoadKeystore(t *testing.T) {
	data, err := os.ReadFile(debugStorePath)
	if err != nil {
		t.Fatalf("read keystore: %v", err)
	}
	t.Logf("Keystore file size: %d bytes", len(data))
	ks, err := jceks.LoadFromReader(bytes.NewReader(data), []byte(debugStorePass))
	if err != nil {
		t.Fatalf("parse jceks: %v", err)
	}
	// List all aliases
	aliases := ks.ListCerts()
	t.Logf("JCEKS cert aliases (%d):", len(aliases))
	for _, a := range aliases {
		t.Logf("  - %q", a)
	}
	pkAliases := ks.ListPrivateKeys()
	t.Logf("JCEKS private key aliases (%d):", len(pkAliases))
	for _, a := range pkAliases {
		t.Logf("  - %q", a)
	}

	cert, err := ks.GetCert(debugPublicAlias)
	if err != nil {
		t.Fatalf("get alias %q: %v", debugPublicAlias, err)
	}
	if cert == nil {
		t.Fatalf("cert is nil for alias %q", debugPublicAlias)
	}
	t.Logf("Loaded cert: algo=%s", cert.PublicKeyAlgorithm)
	if len(cert.Subject.Country) > 0 || cert.Subject.CommonName != "" || cert.Subject.Organization != nil {
		t.Logf("  Subject CN=%q O=%v C=%v",
			cert.Subject.CommonName,
			cert.Subject.Organization,
			cert.Subject.Country)
	}
	if cert.SerialNumber != nil {
		t.Logf("  Serial=%s", cert.SerialNumber.String())
	} else {
		t.Log("  SerialNumber is nil")
	}
	pub := cert.PublicKey
	if pub != nil {
		t.Logf("Public key type: %T", pub)
	} else {
		t.Log("Public key is nil (embedded cert may not have it)")
	}
}

func TestDebug_LicenseBytes(t *testing.T) {
	data, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Fatalf("read license: %v", err)
	}
	t.Logf("License size: %d bytes", len(data))
	if len(data) < 128 {
		t.Fatalf("license too small: %d bytes", len(data))
	}

	t.Logf("First 64 bytes hex:")
	t.Log(hex.Dump(data[:64]))

	t.Logf("Bytes 64-128 hex:")
	t.Log(hex.Dump(data[64:128]))

	magic := data[:8]
	t.Logf("First 8 bytes (potential salt/header): %s", hex.EncodeToString(magic))

	// Check for common patterns
	// Java ObjectStream signature: AC ED 00 05
	if len(data) >= 4 && data[0] == 0xAC && data[1] == 0xED && data[2] == 0x00 && data[3] == 0x05 {
		t.Log("WARNING: Looks like Java ObjectStream (serialized), not encrypted!")
	}

	// ASN.1 SEQUENCE tag = 0x30 - this means no PBE wrapper (raw CMS)
	if data[0] == 0x30 {
		t.Log("INFO: First byte is 0x30 (ASN.1 SEQUENCE) — likely raw CMS, no PrivacyGuard PBE wrapper")
	}
}

// derivePBEKeyAndIV derives DES-EDE3 key (24 bytes) and IV (8 bytes) from
// password + salt + iterations using the PKCS#5 V1.5 MD5-based PBE algorithm,
// matching SunJCE "PBEWithMD5AndTripleDES" / "PBEWithMD5AndDES3CBC".
// This is the de-facto standard used by TrueLicense PrivacyGuard.
// PKCS5 v1.5 key derivation — matches SunJCE PBEWithMD5AndDES:
//   D_0 = MD5(password || salt)
//   D_i = MD5(D_{i-1})     for i=1..iterations-1
// Then dk = D_final (16 bytes) → key(8) + iv(8)
func debugPKCS5v1MD5(password, salt []byte, iterations int, dkLen int) []byte {
	// For PBEWithMD5AndDES: dkLen=16 (for single DES)
	// For PBEWithMD5AndDESede: we need 32 bytes, so need DK-T2 too:
	//   T_1 = above (16 bytes)
	//   T_2 = MD5(password || salt || T_1) iterated iterations-1 more times
	if iterations <= 0 {
		iterations = 1
	}
	var out []byte
	var prevHash []byte
	for len(out) < dkLen {
		h := md5.New()
		if len(prevHash) > 0 {
			h.Write(prevHash)
		}
		h.Write(password)
		h.Write(salt)
		cur := h.Sum(nil)
		// iter-1 additional rounds (cur already round 1)
		for r := 1; r < iterations; r++ {
			h2 := md5.New()
			h2.Write(cur)
			cur = h2.Sum(nil)
		}
		out = append(out, cur...)
		prevHash = cur
	}
	return out[:dkLen]
}

func derivePBEKeyAndIVSingle(password, salt []byte, iterations int) (key []byte, iv []byte) {
	dk := debugPKCS5v1MD5(password, salt, iterations, 16)
	key = make([]byte, 8)
	copy(key, dk[:8])
	iv = make([]byte, 8)
	copy(iv, dk[8:16])
	return key, iv
}

func derivePBEKeyAndIV(password, salt []byte, iterations int) (key []byte, iv []byte) {
	dk := debugPKCS5v1MD5(password, salt, iterations, 32)
	key = make([]byte, 24)
	copy(key, dk[:24])
	iv = make([]byte, 8)
	copy(iv, dk[24:32])
	return key, iv
}

func pbeMD5TripleDESDecrypt(ciphertext, password, salt []byte, iterations int) ([]byte, error) {
	key, iv := derivePBEKeyAndIV(password, salt, iterations)

	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, fmt.Errorf("NewTripleDESCipher: %w", err)
	}
	if len(ciphertext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("ciphertext len %d not multiple of block size %d",
			len(ciphertext), block.BlockSize())
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// PKCS#5 / PKCS#7 unpadding
	if len(plaintext) == 0 {
		return plaintext, nil
	}
	pad := int(plaintext[len(plaintext)-1])
	if pad <= 0 || pad > block.BlockSize() {
		// Invalid pad — return raw to let caller inspect
		return plaintext, nil
	}
	for i := 0; i < pad; i++ {
		if plaintext[len(plaintext)-1-i] != byte(pad) {
			return plaintext, nil
		}
	}
	return plaintext[:len(plaintext)-pad], nil
}

func TestDebug_PBEDecrypt(t *testing.T) {
	data, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Fatalf("read license: %v", err)
	}
	if len(data) < 16 {
		t.Fatalf("too small: %d", len(data))
	}

	salt := data[:8]
	ciphertext := data[8:]
	t.Logf("License: total=%d bytes, salt=%s, ciphertext=%d bytes",
		len(data), hex.EncodeToString(salt), len(ciphertext))

	checkOutcome := func(name string, plaintext []byte) bool {
		if len(plaintext) < 4 {
			return false
		}
		gotASN1 := plaintext[0] == 0x30
		gotXML := bytes.Contains(plaintext[:min(256, len(plaintext))], []byte("<?xml")) ||
			bytes.Contains(plaintext[:min(256, len(plaintext))], []byte("<java"))
		gotObj := len(plaintext) >= 4 &&
			plaintext[0] == 0xAC && plaintext[1] == 0xED
		readable := 0
		for _, b := range plaintext[:min(64, len(plaintext))] {
			if (b >= 0x20 && b < 0x7F) || b == 0x0A || b == 0x0D || b == 0x09 {
				readable++
			}
		}
		mostlyReadable := float64(readable)/float64(min(64, len(plaintext))) > 0.7
		if gotASN1 || gotXML || gotObj || mostlyReadable {
			t.Logf("  ★★★ GOT LIKELY HIT with %s!", name)
			if gotASN1 {
				t.Logf("    → ASN.1 SEQUENCE detected (0x30) = CMS GenericCertificate")
			}
			if gotXML {
				if idx := bytes.Index(plaintext, []byte("<?xml")); idx >= 0 {
					t.Logf("    → XML snippet: %q", string(plaintext[idx:min(idx+300, len(plaintext))]))
				}
				if idx := bytes.Index(plaintext, []byte("<java")); idx >= 0 {
					t.Logf("    → <java snippet: %q", string(plaintext[idx:min(idx+300, len(plaintext))]))
				}
			}
			if gotObj {
				t.Logf("    → Java ObjectStream detected")
			}
			if mostlyReadable {
				t.Logf("    → Mostly readable: %q", string(plaintext[:min(200, len(plaintext))]))
			}
			t.Logf("    Full first 64 hex:")
			t.Log(hex.Dump(plaintext[:min(64, len(plaintext))]))
			return true
		}
		return false
	}

	// Try many combinations
	passwords := []string{
		"public_password4321",
		"public_password1234",
		"nmsappsrv",
		"private_password4321",
		"private_password1234",
		"license_demo",
		"",
	}
	iterationsList := []int{1, 2, 20, 128, 512, 1000, 1024, 2048, 4096, 5000, 8192, 65536}
	algos := []struct {
		name string
		fn   func(ct, pw, sl []byte, iter int) ([]byte, error)
	}{
		{"3DES-CBC", pbeMD5TripleDESDecrypt},
		{"DES-CBC", pbeMD5SingleDESDecrypt},
	}

	totalAttempts := 0
	hits := 0
	for _, algo := range algos {
		for _, iter := range iterationsList {
			for _, pw := range passwords {
				// Variant 1: salt = first 8 bytes, ciphertext = rest
				pt1, _ := algo.fn(ciphertext, []byte(pw), salt, iter)
				totalAttempts++
				if checkOutcome(fmt.Sprintf("%s iter=%d pw=%q (salt head 8)", algo.name, iter, pw), pt1) {
					hits++
				}
				// Variant 2: salt = bytes[4:12] (4 byte magic first)
				if len(data) > 20 {
					salt2 := data[4:12]
					ct2 := data[12:]
					pt2, _ := algo.fn(ct2, []byte(pw), salt2, iter)
					totalAttempts++
					if checkOutcome(fmt.Sprintf("%s iter=%d pw=%q (salt bytes 4:12)", algo.name, iter, pw), pt2) {
						hits++
					}
				}
				// Variant 3: no salt — treat entire payload as plain
				// (i.e. PrivacyGuard is not active)
			}
		}
	}

	// Check if the raw license (no PBE at all) already contains markers
	t.Logf("Checking raw license (no PBE) for markers...")
	checkOutcome("raw-license", data)

	// Check raw as CMS (ASN.1 DER) structure — first byte 0x57 != 0x30 so unlikely
	t.Logf("Total attempts: %d, likely hits: %d", totalAttempts, hits)
	if hits == 0 {
		t.Log("No PBE hit. Next step: check if TrueLicense used a 'keyPass' different from storePass,")
		t.Log("  or if PrivacyGuard format uses additional header length fields.")
		t.Log("  Also check: ciphertext length = 696 bytes, divisible by 8? (DES/3DES block) →",
			696%8 == 0, "696 mod 8 =", 696%8)
	}
}

// pbeMD5SingleDESDecrypt is the single-DES variant of above
// (SunJCE: PBEWithMD5AndDES)
func pbeMD5SingleDESDecrypt(ciphertext, password, salt []byte, iterations int) ([]byte, error) {
	// PBEWithMD5AndDES needs 16 bytes derived: 8 key + 8 iv
	key, iv := derivePBEKeyAndIVSingle(password, salt, iterations)

	block, err := des.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("NewCipher DES: %w", err)
	}
	if len(ciphertext)%block.BlockSize() != 0 {
		return ciphertext, nil
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)
	if len(plaintext) == 0 {
		return plaintext, nil
	}
	pad := int(plaintext[len(plaintext)-1])
	if pad <= 0 || pad > block.BlockSize() {
		return plaintext, nil
	}
	for i := 0; i < pad; i++ {
		if plaintext[len(plaintext)-1-i] != byte(pad) {
			return plaintext, nil
		}
	}
	return plaintext[:len(plaintext)-pad], nil
}

func TestDebug_PBE_Variants(t *testing.T) {
	data, err := os.ReadFile(debugLicensePath)
	if err != nil {
		t.Fatalf("read license: %v", err)
	}

	checkASN1Valid := func(b []byte) (bool, int) {
		if len(b) < 2 {
			return false, 0
		}
		if b[0] != 0x30 {
			return false, 0
		}
		l := int(b[1])
		if l&0x80 == 0 {
			return 2+l <= len(b) && l > 0, 2 + l
		}
		nb := l & 0x7f
		if nb == 0 || nb > 2 || 2+nb > len(b) {
			return false, 0
		}
		total := 0
		for i := 0; i < nb; i++ {
			total = (total << 8) | int(b[2+i])
		}
		return 2+nb+total <= len(b) && total > 0, 2 + nb + total
	}

	checkPadding := func(b []byte) bool {
		if len(b) == 0 {
			return false
		}
		pad := int(b[len(b)-1])
		if pad <= 0 || pad > 8 {
			return false
		}
		for i := 0; i < pad; i++ {
			if b[len(b)-1-i] != byte(pad) {
				return false
			}
			_ = i
		}
		return true
	}

	variants := []struct {
		name string
		key  func(pw, sl []byte, iter int) (k, iv []byte)
		blk  func(k []byte) (cipher.Block, error)
	}{
		{"DES-CBC", derivePBEKeyAndIVSingle, func(k []byte) (cipher.Block, error) { return des.NewCipher(k) }},
		{"3DES-CBC", derivePBEKeyAndIV, func(k []byte) (cipher.Block, error) { return des.NewTripleDESCipher(k) }},
	}

	pwEncodings := []struct {
		name string
		fn   func(string) []byte
	}{
		{"UTF8", func(s string) []byte { return []byte(s) }},
		{"UTF16BE", func(s string) []byte {
			out := make([]byte, 0, len(s)*2)
			for _, r := range s {
				out = append(out, byte(r>>8), byte(r))
			}
			return out
		}},
		{"UTF16LE", func(s string) []byte {
			out := make([]byte, 0, len(s)*2)
			for _, r := range s {
				out = append(out, byte(r), byte(r>>8))
			}
			return out
		}},
	}

	passwords := []string{
		"public_password4321",
		"private_password4321",
		"public_password1234",
		"private_password1234",
		"nmsappsrv",
		"license_demo",
		"",
	}
	iters := []int{1, 20, 128, 256, 512, 1000, 1024, 2048, 4096, 8192, 65536}
	starts := []struct {
		name string
		salt []byte
		ct   []byte
	}{
		{"off0", data[:8], data[8:]},
		{"off4", data[4:12], data[12:]},
		{"off8", data[8:16], data[16:]},
		{"off16", data[16:24], data[24:]},
	}

	gotHit := false
	for _, start := range starts {
		if len(start.ct) < 16 || len(start.ct)%8 != 0 {
			continue
		}
		for _, v := range variants {
			for _, pwEnc := range pwEncodings {
				for _, pw := range passwords {
					pwBytes := pwEnc.fn(pw)
					for _, iter := range iters {
						k, iv := v.key(pwBytes, start.salt, iter)
						blk, err := v.blk(k)
						if err != nil {
							continue
						}
						mode := cipher.NewCBCDecrypter(blk, iv)
						raw := make([]byte, len(start.ct))
						mode.CryptBlocks(raw, start.ct)
						ok, _ := checkASN1Valid(raw)
						padOK := checkPadding(raw)
						if ok && padOK {
							gotHit = true
							t.Logf("★★★ PERFECT HIT: %s/%s pw=%q iter=%d start=%s → ASN1+PKCS5 both OK",
								v.name, pwEnc.name, pw, iter, start.name)
							t.Logf("  First 64 hex: %s", hex.EncodeToString(raw[:min(64, len(raw))]))
							t.Logf("  Last 16 hex:  %s", hex.EncodeToString(raw[len(raw)-16:]))
						} else if ok {
							t.Logf("  ASN1 ok (no pad): %s/%s pw=%q iter=%d start=%s",
								v.name, pwEnc.name, pw, iter, start.name)
						}
					}
				}
			}
		}
	}

	if !gotHit {
		t.Log("No combined ASN1+PKCS5 hit. Trying SunJCE alternative KDF (counter-based).")
	}
}

// parseGenericCertificate does a best-effort DER parse of the TrueLicense
// GenericCertificate structure to extract the encoded XML content.
//
// The GenericCertificate as serialized by de.schlichtherle is a DER-encoded:
//   SEQUENCE {
//     info        SEQUENCE -- e.g. algorithm + content-type + signedAttrs
//     signature   BIT STRING
//     encoded     OCTET STRING -- the XMLEncoder bytes (UTF-8 String)
//   }
// Because the internal layout can vary between TrueLicense v1 vs v2, we
// walk top-level fields looking for a large OCTET STRING / SEQUENCE containing
// printable characters (the XML).
func parseGenericCertificate(der []byte) ([]byte, error) {
	// Permissive: scan for OCTET STRING (0x04) or SEQUENCE (0x30) containing
	// printable ASCII with "<?xml" inside.
	for off := 0; off < len(der)-4; {
		tag := der[off]
		off++
		if off >= len(der) {
			break
		}
		// Read length
		length := int(der[off])
		off++
		if length&0x80 != 0 {
			numBytes := length & 0x7F
			if numBytes == 0 || numBytes > 4 || off+numBytes > len(der) {
				return nil, fmt.Errorf("bad long-form length at %d", off-1)
			}
			length = 0
			for i := 0; i < numBytes; i++ {
				length = (length << 8) | int(der[off])
				off++
			}
		}
		if length < 0 || off+length > len(der) {
			return nil, fmt.Errorf("length out of range: tag=0x%02x off=%d len=%d tot=%d", tag, off, length, len(der))
		}
		content := der[off : off+length]
		off += length

		// OCTET STRING (0x04) or UTF8String (0x0C) with sizeable content
		if (tag == 0x04 || tag == 0x0C) && length > 64 {
			if bytes.Contains(content[:min(length, 256)], []byte("<?xml")) ||
				bytes.Contains(content[:min(length, 256)], []byte("<java")) {
				return content, nil
			}
			// Maybe content is nested, recurse one level
			if inner, err := parseGenericCertificate(content); err == nil {
				return inner, nil
			}
		}
		// SEQUENCE (0x30) / SET (0x31) — recurse
		if tag == 0x30 || tag == 0x31 {
			if inner, err := parseGenericCertificate(content); err == nil {
				return inner, nil
			}
		}
	}
	return nil, fmt.Errorf("no encoded xml field found in DER")
}

// parseXMLEncoder parses the Java XMLEncoder output into TrueLicenseContent.
//
// XMLEncoder format (simplified):
//   <?xml version="1.0" encoding="UTF-8"?>
//   <java version="1.8.0_xxx" class="java.beans.XMLDecoder">
//     <object class="de.schlichtherle.license.LicenseContent">
//       <void property="subject"><string>demo</string></void>
//       <void property="notBefore">
//         <object class="java.util.Date"><long>1735689600000</long></object>
//       </void>
//       <void property="notAfter">
//         <object class="java.util.Date"><long>1767225600000</long></object>
//       </void>
//       <void property="issued">
//         <object class="java.util.Date"><long>1735603200000</long></object>
//       </void>
//       <void property="consumerType"><string>USER</string></void>
//       <void property="consumerAmount"><int>1</int></void>
//       <void property="info"><string>desc</string></void>
//       <void property="extra">
//         <object class="java.util.HashMap">
//           <void method="put">
//             <string>deviceNumber</string>
//             <string>100</string>
//           </void>
//           ...
//         </object>
//       </void>
//       ... also holder / issuer (X500Principal Map) ...
//     </object>
//   </java>
//
// The fields match TrueLicenseContent exactly — case sensitive "property"
// names are the same as our Go JSON tags.
func debugParseXMLEncoder(xmlBytes []byte) (*TrueLicenseContent, error) {
	// Use encoding/xml with best-effort parsing. Since we only care about
	// specific known properties, walk the document for <void property="X">
	// tags and extract child values.
	type xmlNode struct {
		XMLName  string `xml:"-"`
		CharData string `xml:",chardata"`
		Property string `xml:"property,attr"`
		Method   string `xml:"method,attr"`
		Class    string `xml:"class,attr"`
		Children []byte `xml:",innerxml"`
	}

	// Simplify: use regex + string scanning for robustness, because XMLEncoder
	// sometimes has unbalanced tags in edge cases and encoding/xml chokes.
	// We'll use a simple stateful scanner tuned for TrueLicense's output.
	s := string(xmlBytes)
	c := &TrueLicenseContent{
		Extra: map[string]string{},
		Holder: map[string]string{},
		Issuer: map[string]string{},
	}

	extractTextBetween := func(doc, startTag, endTag string) (string, bool) {
		i := strings.Index(doc, startTag)
		if i < 0 {
			return "", false
		}
		i += len(startTag)
		j := strings.Index(doc[i:], endTag)
		if j < 0 {
			return "", false
		}
		return doc[i : i+j], true
	}

	// Helper: find first "<void property=\"NAME\">" block and return inner xml
	extractVoidProperty := func(doc, name string) (string, bool) {
		needle := fmt.Sprintf(`<void property="%s">`, name)
		i := strings.Index(doc, needle)
		if i < 0 {
			return "", false
		}
		i += len(needle)
		// Find matching </void> — count nesting
		depth := 1
		cur := i
		for depth > 0 && cur < len(doc) {
			open := strings.Index(doc[cur:], "<void")
			close := strings.Index(doc[cur:], "</void>")
			if close < 0 {
				return "", false
			}
			if open >= 0 && open < close {
				depth++
				cur += open + 5
			} else {
				depth--
				cur += close + 7
				if depth == 0 {
					return doc[i : cur-7], true
				}
			}
		}
		return "", false
	}

	// Simple string inside void: <void property="X"><string>VAL</string></void>
	getStringProp := func(name string) (string, bool) {
		block, ok := extractVoidProperty(s, name)
		if !ok {
			return "", false
		}
		if v, ok := extractTextBetween(block, "<string>", "</string>"); ok {
			return v, true
		}
		return "", false
	}

	// java.util.Date: <object class="java.util.Date"><long>MILLIS</long></object>
	getDateProp := func(name string) (time.Time, bool) {
		block, ok := extractVoidProperty(s, name)
		if !ok {
			return time.Time{}, false
		}
		msStr, ok := extractTextBetween(block, "<long>", "</long>")
		if !ok {
			return time.Time{}, false
		}
		var ms int64
		if _, err := fmt.Sscanf(msStr, "%d", &ms); err != nil {
			return time.Time{}, false
		}
		return time.UnixMilli(ms), true
	}

	// int: <int>NUM</int>
	getIntProp := func(name string) (int32, bool) {
		block, ok := extractVoidProperty(s, name)
		if !ok {
			return 0, false
		}
		sVal, ok := extractTextBetween(block, "<int>", "</int>")
		if !ok {
			// try <long> fallback
			sVal, ok = extractTextBetween(block, "<long>", "</long>")
			if !ok {
				return 0, false
			}
		}
		var n int64
		if _, err := fmt.Sscanf(sVal, "%d", &n); err != nil {
			return 0, false
		}
		return int32(n), true
	}

	if v, ok := getStringProp("subject"); ok {
		c.Subject = v
	}
	if v, ok := getDateProp("notBefore"); ok {
		c.NotBefore = v
	}
	if v, ok := getDateProp("notAfter"); ok {
		c.NotAfter = v
	}
	if v, ok := getDateProp("issued"); ok {
		c.Issued = v
	}
	if v, ok := getStringProp("consumerType"); ok {
		c.ConsumerType = v
	}
	if v, ok := getIntProp("consumerAmount"); ok {
		c.ConsumerAmount = v
	}
	if v, ok := getStringProp("info"); ok {
		c.Info = v
	}

	// Extra: HashMap put pairs
	extraBlock, ok := extractVoidProperty(s, "extra")
	if ok {
		// find all <void method="put"><string>KEY</string><string>VAL</string></void>
		cur := 0
		for {
			needle := `<void method="put">`
			i := strings.Index(extraBlock[cur:], needle)
			if i < 0 {
				break
			}
			start := cur + i + len(needle)
			j := strings.Index(extraBlock[start:], "</void>")
			if j < 0 {
				break
			}
			inner := extraBlock[start : start+j]
			k, kOk := extractTextBetween(inner, "<string>", "</string>")
			// second string: find next occurrence
			remain := inner
			if kOk {
				idx := strings.Index(remain, "</string>")
				if idx >= 0 {
					remain = remain[idx+len("</string>"):]
				}
			}
			v, vOk := extractTextBetween(remain, "<string>", "</string>")
			if kOk && vOk {
				c.Extra[k] = v
			} else if kOk {
				// Try <int>/<long> value
				if vi, ok2 := extractTextBetween(remain, "<int>", "</int>"); ok2 {
					c.Extra[k] = vi
				} else if vl, ok2 := extractTextBetween(remain, "<long>", "</long>"); ok2 {
					c.Extra[k] = vl
				} else {
					c.Extra[k] = ""
				}
			}
			cur = start + j + 7
		}
	}

	// Holder / Issuer: X500Principal maps with getName() entries
	for _, prop := range []string{"holder", "issuer"} {
		block, ok := extractVoidProperty(s, prop)
		if !ok {
			continue
		}
		m := c.Holder
		if prop == "issuer" {
			m = c.Issuer
		}
		// Map entries may be in HashMap with put, or serialized as list of
		// <string>CN=foo</string><string>O=bar</string> ... etc.
		// Try both shapes:
		cur := 0
		for {
			needle := `<void method="put">`
			i := strings.Index(block[cur:], needle)
			if i < 0 {
				break
			}
			start := cur + i + len(needle)
			j := strings.Index(block[start:], "</void>")
			if j < 0 {
				break
			}
			inner := block[start : start+j]
			k, kOk := extractTextBetween(inner, "<string>", "</string>")
			remain := inner
			if kOk {
				idx := strings.Index(remain, "</string>")
				if idx >= 0 {
					remain = remain[idx+len("</string>"):]
				}
			}
			v, vOk := extractTextBetween(remain, "<string>", "</string>")
			if kOk && vOk {
				m[k] = v
			}
			cur = start + j + 7
		}
	}

	return c, nil
}
