package security

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

// PasswordHasher reproduces the Java SHA256Util class EXACTLY, including the
// deliberate 500ms anti-rainbow-table busy-wait inside every hash call.
//
// Java reference (SHA256Util.java):
//
//	encrypt_SHA256(str):
//	    sha256(str).toLowerHex()
//	    while (elapsed < 500ms) sleep(10ms)
//
//	encrypt_SHA256_Salt(str, salt):
//	    if salt empty: return encrypt_SHA256(str)
//	    return encrypt_SHA256( encrypt_SHA256(str + salt) + salt )
//
// Admin convention (from MyAuthenticationProvider.java line 329):
//
//	if (!"admin".equals(user.getUsername())) { salt = saltHolder.get(id) }
//
// i.e. the built-in admin user always hashes WITHOUT salt (pure single-pass
// SHA-256 of the plaintext password). Non-admin users use the two-level
// salted construction.
//
// These conventions are part of the data contract with the existing MySQL
// rows stored by the Java server; any deviation breaks every existing
// password on the shared database. DO NOT "optimize" or refactor this code.

const minHashElapsed = 500 * time.Millisecond
const sleepGranularity = 10 * time.Millisecond

// sha256Hex returns the lowercase hex-encoded SHA-256 digest of s, mirroring
// Java MessageDigest + Hex.encodeHex.
func sha256Hex(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// throttle ensures the hash call takes at least minHashElapsed wall-clock
// time, mirroring the Java while(true) + Thread.sleep(10) busy-wait loop.
// The explicit anti-fast-guessing throttle is intentional Java behaviour.
func throttle(start time.Time) {
	elapsed := time.Since(start)
	if elapsed >= minHashElapsed {
		return
	}
	remaining := minHashElapsed - elapsed
	// Sleep in 10ms chunks to match Java's Thread.sleep(10) granularity.
	for remaining > 0 {
		chunk := sleepGranularity
		if remaining < chunk {
			chunk = remaining
		}
		time.Sleep(chunk)
		remaining -= chunk
	}
}

// EncryptSHA256 is the port of SHA256Util.encrypt_SHA256(String).
// Pure single-pass SHA-256 with the mandatory 500ms throttle.
func EncryptSHA256(plain string) (string, error) {
	start := time.Now()
	if plain == "" {
		return "", errors.New("empty password")
	}
	digest := sha256Hex(plain)
	throttle(start)
	return digest, nil
}

// EncryptSHA256Salt is the port of SHA256Util.encrypt_SHA256_Salt(String, String).
//
//	salt == "":         EncryptSHA256(plain)                      (admin case)
//	salt != "":         sha256( sha256(plain+salt) + salt )       (user case)
//
// Both code paths include the mandatory 500ms throttle, and both produce
// the exact same hex strings as the Java server on identical inputs.
func EncryptSHA256Salt(plain, salt string) (string, error) {
	start := time.Now()
	if plain == "" {
		return "", errors.New("empty password")
	}
	if salt == "" {
		digest := sha256Hex(plain)
		throttle(start)
		return digest, nil
	}
	inner := sha256Hex(plain + salt)
	digest := sha256Hex(inner + salt)
	throttle(start)
	return digest, nil
}

// VerifyPassword compares a plaintext password against a stored digest.
// * username drives the admin/no-salt convention.
// * userId drives lookup of the per-user salt (via SaltHolder).
// * storedDigest is the value of sys_user.password in MySQL.
func VerifyPassword(plainPassword, storedDigest, username string, userId int, salt SaltHolder) (bool, error) {
	if storedDigest == "" || plainPassword == "" {
		return false, nil
	}
	var saltValue string
	if !IsAdminUser(username) {
		v, err := salt.GetSalt(userId)
		if err != nil {
			return false, err
		}
		saltValue = v
	}
	candidate, err := EncryptSHA256Salt(plainPassword, saltValue)
	if err != nil {
		return false, err
	}
	return constantTimeEq(candidate, storedDigest), nil
}

// HashPassword returns the digest to store in sys_user.password for a
// (username, userId, plainPassword) tuple. Matches what the Java server
// would persist on create-user / change-password / reset-password calls.
func HashPassword(plainPassword, username string, userId int, salt SaltHolder) (string, error) {
	var saltValue string
	if !IsAdminUser(username) {
		v, err := salt.GetSalt(userId)
		if err != nil {
			return "", err
		}
		saltValue = v
	}
	return EncryptSHA256Salt(plainPassword, saltValue)
}

// IsAdminUser mirrors the Java convention for admin-vs-user divergence
// (see MyAuthenticationProvider.java line 296, 314, 329, 380: all use
// `"admin".equals(...)` or `.equalsIgnoreCase` with a hard-coded string).
func IsAdminUser(username string) bool {
	if len(username) != 5 {
		return false
	}
	// Case-insensitive match per MyAuthenticationProvider line 296.
	return (username[0]|32) == 'a' &&
		(username[1]|32) == 'd' &&
		(username[2]|32) == 'm' &&
		(username[3]|32) == 'i' &&
		(username[4]|32) == 'n'
}

// constantTimeEq compares two hex strings without timing side-channels.
// Java's String.equals is not constant-time, but the Go verifier should be.
func constantTimeEq(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
