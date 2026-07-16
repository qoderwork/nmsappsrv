package external

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// PoP token (Proof-of-Possession) constants — mirror Java
// com.tmobile.oss.security.taap.poptoken.builder.PopEhtsKey / PopTokenBuilder.
const (
	popEhtsURI        = "uri"
	popEhtsHTTPMethod = "http-method"
	popEhtsBody       = "body"

	// popTokenValidity matches Java POP_TOKEN_VALIDITY_DURATION_IN_MILLIS (2 min).
	popTokenValidity = 2 * time.Minute
)

// popTokenClaims is the JWT claim set Java's PopTokenBuilder emits:
//   ehts = semicolon-joined ehts key names (LinkedHashMap order)
//   edts = SHA-256 of concatenated ehts values (same order), Base64 URL-safe (no pad)
//   jti  = random UUID           (jwt.RegisteredClaims.ID, JSON "jti")
//   v    = "1"
//   iat  = now                   (jwt.RegisteredClaims.IssuedAt, JSON "iat")
//   exp  = now + 2 min           (jwt.RegisteredClaims.ExpiresAt, JSON "exp")
//
// Embedding jwt.RegisteredClaims satisfies the full jwt.Claims interface that
// jwt/v5 requires (audience/issuer/subject/… getters) while keeping the JSON
// field names Java expects.
type popTokenClaims struct {
	jwt.RegisteredClaims
	Ehts string `json:"ehts"`
	Edts string `json:"edts"`
	V    string `json:"v"`
}

// ehtsEntry is one (key, value) pair; order is significant (mirrors Java's
// LinkedHashMap iteration order).
type ehtsEntry struct {
	key   string
	value string
}

// BuildPopToken reproduces Java TPlatformHelper.buildPopTokenWithPrivateKeyPemString
// exactly: a JWT signed with RS256 over a PKCS#8 RSA private key PEM, carrying
// the ehts/edts proof-of-possession claims built from the ordered ehts map
// (Content-Type; Authorization; uri; http-method; body). The PoP token is valid
// for 2 minutes, matching Java.
//
//	privPEM   : contents of the PKCS#8 RSA private key file (e.g. private-key-pkcs8.pem)
//	body      : the request body that will be sent (Java uses the JSON alert request)
//	authToken : the value placed in the ehts "Authorization" entry — Java passes
//	            Base64(clientId + ":" + secret)
//	uri       : the request URI path placed in the ehts "uri" entry
//	            (e.g. /partner-interactions/alert-management/v1/alerts)
func BuildPopToken(privPEM, body, authToken, uri string) (string, error) {
	entries := []ehtsEntry{
		{"Content-Type", "application/json"},
		{"Authorization", authToken},
		{popEhtsURI, uri},
		{popEhtsHTTPMethod, "POST"},
		{popEhtsBody, body},
	}

	var ehtsKeys, edtsSrc strings.Builder
	for i, e := range entries {
		if i > 0 {
			ehtsKeys.WriteString(";")
		}
		ehtsKeys.WriteString(e.key)
		edtsSrc.WriteString(e.value)
	}

	priv, err := parsePKCS8PrivateKey(privPEM)
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := popTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(popTokenValidity)),
		},
		Ehts: ehtsKeys.String(),
		Edts: edtsHash(edtsSrc.String()),
		V:    "1",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return tok.SignedString(priv)
}

// edtsHash returns the SHA-256 of s, Base64 URL-safe encoded with no padding —
// matches Java DigestUtils.sha256 + Base64.encodeBase64URLSafeString.
func edtsHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(sum[:])
}

// parsePKCS8PrivateKey parses an unencrypted PKCS#8 RSA private key PEM string,
// matching Java PopTokenBuilderUtils.keyPemStringToRsaPrivateKey (no password).
func parsePKCS8PrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	pemStr = strings.TrimSpace(pemStr)
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("poptoken: failed to decode private key PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Some exports wrap the key in a PKCS#1 (RSA) structure instead.
		if rsaKey, err2 := x509.ParsePKCS1PrivateKey(block.Bytes); err2 == nil {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("poptoken: parse private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("poptoken: not an RSA private key (%T)", key)
	}
	return rsaKey, nil
}
