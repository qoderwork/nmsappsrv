package filebase

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// safeFileName rejects path separators and ".." so a client-supplied name
// cannot escape its intended directory (Java's FileUtil.isValidFileName guard).
func safeFileName(name string) bool {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return false
	}
	return true
}

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

// saveBody writes the request body (raw or first multipart part) to baseDir/name.
// It mirrors Java's FileServiceImpl.saveFileToDisk + save*File variants which
// accept either an InputStream or a MultipartFile.
func saveBody(reader io.Reader, baseDir, name string) (string, error) {
	if baseDir == "" {
		return "", fmt.Errorf("target directory not configured")
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", err
	}
	full := filepath.Join(baseDir, name)
	// Overwrite like Java (saveFileToDisk deletes the old file first).
	if _, err := os.Stat(full); err == nil {
		_ = os.Remove(full)
	}
	out, err := os.Create(full)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, reader); err != nil {
		_ = os.Remove(full)
		return "", err
	}
	return full, nil
}
