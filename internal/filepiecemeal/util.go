package filepiecemeal

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strings"
)

// Sha256Hex returns the hex SHA-256 digest of the file at path.
func Sha256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// safeFileName rejects path separators and "..".
func safeFileName(name string) bool {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return false
	}
	return true
}
