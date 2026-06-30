package soap

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	xmlDSigNS  = "http://www.w3.org/2000/09/xmldsig#"
	excC14NURL = "http://www.w3.org/2001/10/xml-exc-c14n#"
	rsaSHA256  = "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"
	sha256Alg  = "http://www.w3.org/2001/04/xmlenc#sha256"
)

// SignSOAPMessage signs a SOAP XML message using RSA-SHA256 with Exclusive C14N.
// The ds:Signature element is inserted into the SOAP Header.
func SignSOAPMessage(soapXML string, privKey *rsa.PrivateKey, cert *x509.Certificate) (string, error) {
	bodyRaw, err := extractBodyRaw(soapXML)
	if err != nil {
		return "", fmt.Errorf("extract body: %w", err)
	}
	bodyDigest := sha256Sum(bodyRaw)
	signedInfoXML := buildSignedInfoXml(bodyDigest)
	siC14N, err := canonicalizeStandaloneXml(signedInfoXML)
	if err != nil {
		return "", fmt.Errorf("c14n SignedInfo: %w", err)
	}
	sigBytes, err := signRSASHA256(privKey, siC14N)
	if err != nil {
		return "", fmt.Errorf("rsa sign: %w", err)
	}
	sigVal := base64.StdEncoding.EncodeToString(sigBytes)
	certB64 := base64.StdEncoding.EncodeToString(cert.Raw)
	sigXML := buildSignatureXml(signedInfoXML, sigVal, certB64)
	result, err := insertSignatureIntoHeader(soapXML, sigXML)
	if err != nil {
		return "", fmt.Errorf("insert signature: %w", err)
	}
	return result, nil
}

// VerifySOAPSignature verifies the XML Digital Signature in a SOAP message.
func VerifySOAPSignature(soapXML string, cert *x509.Certificate) (bool, error) {
	sigXml, err := extractSignatureXml(soapXML)
	if err != nil {
		return false, err
	}
	siXML, sigValB64, digestValB64, err := parseSignatureParts(sigXml)
	if err != nil {
		return false, err
	}
	bodyRaw, err := extractBodyRaw(soapXML)
	if err != nil {
		return false, fmt.Errorf("extract body: %w", err)
	}
	computedDigest := sha256Sum(bodyRaw)
	if computedDigest != digestValB64 {
		return false, fmt.Errorf("digest mismatch: computed %s != signed %s", computedDigest, digestValB64)
	}
	siC14N, err := canonicalizeStandaloneXml(siXML)
	if err != nil {
		return false, fmt.Errorf("c14n SignedInfo: %w", err)
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sigValB64)
	if err != nil {
		return false, fmt.Errorf("decode signature value: %w", err)
	}
	rsaPub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return false, fmt.Errorf("certificate public key is not RSA")
	}
	return verifyRSASHA256(rsaPub, siC14N, sigBytes)
}

// --------------- Key / Certificate parsing ---------------

func ParseRSAPrivateKeyFromPEM(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	switch block.Type {
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rk, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS#8 key is not RSA")
		}
		return rk, nil
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unsupported PEM type: %s", block.Type)
	}
}

func ParseCertificateFromPEM(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found for certificate")
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("PEM type is %s, expected CERTIFICATE", block.Type)
	}
	return x509.ParseCertificate(block.Bytes)
}

func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseRSAPrivateKeyFromPEM(data)
}

func LoadCertificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseCertificateFromPEM(data)
}

// --------------- Crypto helpers ---------------

func sha256Sum(data []byte) string {
	h := sha256.Sum256(data)
	return base64.StdEncoding.EncodeToString(h[:])
}

func signRSASHA256(key *rsa.PrivateKey, data []byte) ([]byte, error) {
	h := sha256.Sum256(data)
	return rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
}

func verifyRSASHA256(pub *rsa.PublicKey, data, sig []byte) (bool, error) {
	h := sha256.Sum256(data)
	err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, h[:], sig)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// --------------- XML extraction helpers ---------------

func extractBodyRaw(xmlStr string) ([]byte, error) {
	startTag := "<soap:Body"
	startAlt := "<SOAP-ENV:Body"
	idx := strings.Index(xmlStr, startTag)
	if idx < 0 {
		idx = strings.Index(xmlStr, startAlt)
	}
	if idx < 0 {
		idx = strings.Index(xmlStr, "<Body")
	}
	if idx < 0 {
		return nil, fmt.Errorf("soap:Body not found")
	}
	dec := xml.NewDecoder(strings.NewReader(xmlStr[idx:]))
	depth := 0
	startOff := -1
	endOff := -1
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				startOff = idx + int(dec.InputOffset()) - lastStartTag(xmlStr[idx:], int(dec.InputOffset()))
			}
			depth++
			_ = t
		case xml.EndElement:
			depth--
			if depth == 0 {
				endOff = idx + int(dec.InputOffset())
				break
			}
		}
		if endOff >= 0 {
			break
		}
	}
	if startOff < 0 || endOff < 0 {
		return nil, fmt.Errorf("could not determine Body boundaries")
	}
	return []byte(xmlStr[startOff:endOff]), nil
}

func lastStartTag(s string, off int) int {
	sub := s[:off]
	for i := len(sub) - 1; i >= 0; i-- {
		if sub[i] == 60 {
			return len(sub) - i
		}
	}
	return off
}

func extractSignatureXml(soapXML string) (string, error) {
	markers := []string{
		"<ds:Signature",
		"<Signature xmlns",
		"<Signature ",
	}
	start := -1
	for _, m := range markers {
		start = strings.Index(soapXML, m)
		if start >= 0 {
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("no ds:Signature found in SOAP message")
	}
	endMarkers := []string{"</ds:Signature>", "</Signature>"}
	end := -1
	for _, em := range endMarkers {
		idx := strings.Index(soapXML[start:], em)
		if idx >= 0 {
			candidate := start + idx + len(em)
			if end < 0 || candidate > end {
				end = candidate
			}
		}
	}
	if end < 0 {
		return "", fmt.Errorf("no closing Signature tag found")
	}
	return soapXML[start:end], nil
}

type parsedSig struct {
	SignedInfoXML string
	SignatureValue string
	DigestValue    string
}

func parseSignatureParts(sigXML string) (siXML, sigVal, digestVal string, err error) {
	siStart := strings.Index(sigXML, "<ds:SignedInfo")
	if siStart < 0 {
		siStart = strings.Index(sigXML, "<SignedInfo")
	}
	if siStart < 0 {
		err = fmt.Errorf("SignedInfo not found")
		return
	}
	siEnd := strings.Index(sigXML[siStart:], "</ds:SignedInfo>")
	closeTag := "</ds:SignedInfo>"
	if siEnd < 0 {
		siEnd = strings.Index(sigXML[siStart:], "</SignedInfo>")
		closeTag = "</SignedInfo>"
	}
	if siEnd < 0 {
		err = fmt.Errorf("SignedInfo close tag not found")
		return
	}
	siXML = sigXML[siStart : siStart+siEnd+len(closeTag)]
	sigVal, err = extractDigestValue(sigXML, "SignatureValue")
	if err != nil {
		return
	}
	digestVal, err = extractDigestValue(sigXML, "DigestValue")
	return
}

func extractDigestValue(xmlStr, elemName string) (string, error) {
	patterns := []string{
		"<ds:" + elemName + ">",
		"<" + elemName + ">",
	}
	for _, pat := range patterns {
		start := strings.Index(xmlStr, pat)
		if start < 0 {
			continue
		}
		contentStart := start + len(pat)
		endPat := strings.Replace(pat, "<", "</", 1)
		end := strings.Index(xmlStr[contentStart:], endPat)
		if end < 0 {
			continue
		}
		return strings.TrimSpace(xmlStr[contentStart : contentStart+end]), nil
	}
	return "", fmt.Errorf("%s not found", elemName)
}

// --------------- Signature construction ---------------

func insertSignatureIntoHeader(soapXML, sigXML string) (string, error) {
	hdrEnd := strings.Index(soapXML, "</soap:Header>")
	if hdrEnd < 0 {
		hdrEnd = strings.Index(soapXML, "</SOAP-ENV:Header>")
	}
	if hdrEnd < 0 {
		return "", fmt.Errorf("SOAP Header close tag not found")
	}
	var buf strings.Builder
	buf.WriteString(soapXML[:hdrEnd])
	buf.WriteString("\n")
	buf.WriteString(sigXML)
	buf.WriteString("\n")
	buf.WriteString(soapXML[hdrEnd:])
	return buf.String(), nil
}

func buildSignedInfoXml(bodyDigest string) string {
	var b strings.Builder
	b.WriteString("<ds:SignedInfo>")
	b.WriteString(`<ds:CanonicalizationMethod Algorithm="`)
	b.WriteString(excC14NURL)
	b.WriteString(`"/>`)
	b.WriteString(`<ds:SignatureMethod Algorithm="`)
	b.WriteString(rsaSHA256)
	b.WriteString(`"/>`)
	b.WriteString("<ds:Reference URI=")
	b.WriteString(`"#soap-body"`)
	b.WriteString(">")
	b.WriteString(`<ds:Transforms><ds:Transform Algorithm="`)
	b.WriteString(excC14NURL)
	b.WriteString(`"/></ds:Transforms>`)
	b.WriteString(`<ds:DigestMethod Algorithm="`)
	b.WriteString(sha256Alg)
	b.WriteString(`"/>`)
	b.WriteString("<ds:DigestValue>")
	b.WriteString(bodyDigest)
	b.WriteString("</ds:DigestValue>")
	b.WriteString("</ds:Reference>")
	b.WriteString("</ds:SignedInfo>")
	return b.String()
}

func buildSignatureXml(siXML, sigVal, certB64 string) string {
	var b strings.Builder
	b.WriteString(`<ds:Signature xmlns:ds="`)
	b.WriteString(xmlDSigNS)
	b.WriteString(`">`)
	b.WriteString(siXML)
	b.WriteString("<ds:SignatureValue>")
	b.WriteString(sigVal)
	b.WriteString("</ds:SignatureValue>")
	b.WriteString("<ds:KeyInfo>")
	b.WriteString("<ds:X509Data><ds:X509Certificate>")
	b.WriteString(certB64)
	b.WriteString("</ds:X509Certificate></ds:X509Data>")
	b.WriteString("</ds:KeyInfo>")
	b.WriteString("</ds:Signature>")
	return b.String()
}

// ============================================================
//  Exclusive XML Canonicalization (C14N)
// ============================================================

type treeNode struct {
	Prefix   string
	Local    string
	NS       map[string]string
	Attrs    []treeAttr
	Children []interface{}
	Text     string
	IsText   bool
}

type treeAttr struct {
	Prefix string
	Local  string
	Value  string
}

func canonicalizeStandaloneXml(xmlStr string) ([]byte, error) {
	root, err := parseTree(xmlStr)
	if err != nil {
		return nil, err
	}
	ancestorNS := make(map[string]string)
	var buf bytes.Buffer
	writeElementC14n(&buf, root, ancestorNS)
	return buf.Bytes(), nil
}

func parseTree(xmlStr string) (*treeNode, error) {
	dec := xml.NewDecoder(strings.NewReader(xmlStr))
	return buildTree(dec)
}

func buildTree(dec *xml.Decoder) (*treeNode, error) {
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("token error: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			node := &treeNode{
				NS: make(map[string]string),
			}
			node.Prefix, node.Local = extractPrefixFromTag(t.Name)
			for _, a := range t.Attr {
				ap, al := resolveAttrNS(a.Name)
				if ap == "xmlns" {
					node.NS[al] = a.Value
				} else if ap == "" && al == "xmlns" {
					node.NS[""] = a.Value
				} else {
					node.Attrs = append(node.Attrs, treeAttr{Prefix: ap, Local: al, Value: a.Value})
				}
			}
			for {
				peek, perr := dec.Token()
				if perr != nil {
					break
				}
				switch pt := peek.(type) {
				case xml.StartElement:
					childNode := &treeNode{NS: make(map[string]string)}
					childNode.Prefix, childNode.Local = extractPrefixFromTag(pt.Name)
					for _, a := range pt.Attr {
						ap2, al2 := resolveAttrNS(a.Name)
						if ap2 == "xmlns" {
							childNode.NS[al2] = a.Value
						} else if ap2 == "" && al2 == "xmlns" {
							childNode.NS[""] = a.Value
						} else {
							childNode.Attrs = append(childNode.Attrs, treeAttr{Prefix: ap2, Local: al2, Value: a.Value})
						}
					}
					collectChildren(dec, childNode)
					node.Children = append(node.Children, childNode)
				case xml.CharData:
					txt := string([]byte(pt))
					if strings.TrimSpace(txt) != "" {
						node.Children = append(node.Children, &treeNode{IsText: true, Text: txt})
					}
				case xml.EndElement:
					return node, nil
				}
			}
		case xml.CharData:
			continue
		}
	}
}

func collectChildren(dec *xml.Decoder, node *treeNode) {
	for {
		tok, err := dec.Token()
		if err != nil {
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			child := &treeNode{NS: make(map[string]string)}
			child.Prefix, child.Local = extractPrefixFromTag(t.Name)
			for _, a := range t.Attr {
				ap, al := resolveAttrNS(a.Name)
				if ap == "xmlns" {
					child.NS[al] = a.Value
				} else if ap == "" && al == "xmlns" {
					child.NS[""] = a.Value
				} else {
					child.Attrs = append(child.Attrs, treeAttr{Prefix: ap, Local: al, Value: a.Value})
				}
			}
			collectChildren(dec, child)
			node.Children = append(node.Children, child)
		case xml.CharData:
			txt := string([]byte(t))
			if strings.TrimSpace(txt) != "" {
				node.Children = append(node.Children, &treeNode{IsText: true, Text: txt})
			}
		case xml.EndElement:
			return
		}
	}
}

func resolveAttrNS(name xml.Name) (prefix, local string) {
	if name.Space == "" {
		if name.Local == "xmlns" {
			return "", "xmlns"
		}
		return "", name.Local
	}
	if strings.HasPrefix(name.Local, "xmlns:") {
		return "xmlns", name.Local[6:]
	}
	return name.Space, name.Local
}

func extractPrefixFromTag(name xml.Name) (prefix, local string) {
	if name.Space == "" {
		return "", name.Local
	}
	return name.Space, name.Local
}

func collectVisiblyUtilized(node *treeNode) map[string]bool {
	vu := make(map[string]bool)
	if node.Prefix != "" {
		vu[node.Prefix] = true
	}
	for _, a := range node.Attrs {
		if a.Prefix != "" {
			vu[a.Prefix] = true
		}
		extractQNamePrefixes(a.Value, vu)
	}
	collectVURecursive(node, vu)
	return vu
}

func collectVURecursive(node *treeNode, vu map[string]bool) {
	for _, c := range node.Children {
		child, ok := c.(*treeNode)
		if !ok || child.IsText {
			continue
		}
		if child.Prefix != "" {
			vu[child.Prefix] = true
		}
		for _, a := range child.Attrs {
			if a.Prefix != "" {
				vu[a.Prefix] = true
			}
			extractQNamePrefixes(a.Value, vu)
		}
		collectVURecursive(child, vu)
	}
}

func extractQNamePrefixes(value string, prefixes map[string]bool) {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return !isNCNameChar(r) && r != 58
	})
	for _, f := range fields {
		ci := strings.Index(f, ":")
		if ci > 0 {
			prefixes[f[:ci]] = true
		}
	}
}

func isNCNameChar(r rune) bool {
	if (r >= 65 && r <= 90) || (r >= 97 && r <= 122) || (r >= 48 && r <= 57) {
		return true
	}
	return r == 46 || r == 45 || r == 95
}

type c14nAttr struct {
	NSPrefix  string
	NSLocal   string
	NSURI     string
	AttrPrefix string
	AttrLocal  string
	AttrValue  string
	IsNS       bool
}

func writeElementC14n(buf *bytes.Buffer, node *treeNode, ancestorNS map[string]string) {
	vu := make(map[string]bool)
	if node.Prefix != "" {
		vu[node.Prefix] = true
	}
	for _, a := range node.Attrs {
		if a.Prefix != "" {
			vu[a.Prefix] = true
		}
		extractQNamePrefixes(a.Value, vu)
	}
	collectVURecursive(node, vu)

	allNS := make(map[string]string)
	for k, v := range ancestorNS {
		allNS[k] = v
	}
	for k, v := range node.NS {
		allNS[k] = v
	}

	var nsDecls []c14nAttr
	for pfx, uri := range node.NS {
		if !vu[pfx] {
			continue
		}
		if ancURI, ok := ancestorNS[pfx]; ok && ancURI == uri {
			continue
		}
		nsDecls = append(nsDecls, c14nAttr{IsNS: true, NSPrefix: pfx, NSURI: uri})
	}
	sort.Slice(nsDecls, func(i, j int) bool {
		return nsDecls[i].NSPrefix < nsDecls[j].NSPrefix
	})

	var attrs []c14nAttr
	for _, a := range node.Attrs {
		uri := ""
		if a.Prefix != "" {
			uri = allNS[a.Prefix]
		}
		attrs = append(attrs, c14nAttr{
			AttrPrefix: a.Prefix,
			AttrLocal:  a.Local,
			AttrValue:  a.Value,
			NSURI:      uri,
		})
	}
	sort.Slice(attrs, func(i, j int) bool {
		if attrs[i].NSURI != attrs[j].NSURI {
			return attrs[i].NSURI < attrs[j].NSURI
		}
		return attrs[i].AttrLocal < attrs[j].AttrLocal
	})

	buf.WriteString("<")
	if node.Prefix != "" {
		buf.WriteString(node.Prefix)
		buf.WriteString(":")
	}
	buf.WriteString(node.Local)

	for _, ns := range nsDecls {
		if ns.NSPrefix == "" {
			buf.WriteString(` xmlns="`)
		} else {
			buf.WriteString(" xmlns:")
			buf.WriteString(ns.NSPrefix)
			buf.WriteString("=")
			buf.WriteString(`"`)
		}
		writeEscapedAttr(buf, ns.NSURI)
		buf.WriteString(`"`)
	}
	for _, a := range attrs {
		buf.WriteString(" ")
		if a.AttrPrefix != "" {
			buf.WriteString(a.AttrPrefix)
			buf.WriteString(":")
		}
		buf.WriteString(a.AttrLocal)
		buf.WriteString("=")
		buf.WriteString(`"`)
		writeEscapedAttr(buf, a.AttrValue)
		buf.WriteString(`"`)
	}
	buf.WriteString(">")

	for _, c := range node.Children {
		if tn, ok := c.(*treeNode); ok {
			if tn.IsText {
				writeEscapedText(buf, tn.Text)
			} else {
				writeElementC14n(buf, tn, allNS)
			}
		}
	}

	buf.WriteString("</")
	if node.Prefix != "" {
		buf.WriteString(node.Prefix)
		buf.WriteString(":")
	}
	buf.WriteString(node.Local)
	buf.WriteString(">")
}

func writeEscapedText(buf *bytes.Buffer, s string) {
	for _, r := range s {
		switch r {
		case 38:
			buf.WriteString("&amp;")
		case 60:
			buf.WriteString("&lt;")
		case 62:
			buf.WriteString("&gt;")
		case 13:
			buf.WriteString("&#xD;")
		default:
			buf.WriteRune(r)
		}
	}
}

func writeEscapedAttr(buf *bytes.Buffer, s string) {
	for _, r := range s {
		switch r {
		case 38:
			buf.WriteString("&amp;")
		case 60:
			buf.WriteString("&lt;")
		case 34:
			buf.WriteString("&quot;")
		case 9:
			buf.WriteString("&#x9;")
		case 10:
			buf.WriteString("&#xA;")
		case 13:
			buf.WriteString("&#xD;")
		default:
			buf.WriteRune(r)
		}
	}
}
