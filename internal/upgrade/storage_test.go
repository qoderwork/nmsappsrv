package upgrade

import (
	"os"
	"path/filepath"
	"testing"

	"nmsappsrv/internal/config"
)

func TestSaveUpgradeFile(t *testing.T) {
	dir := t.TempDir()
	config.Cfg = &config.Config{Upgrade: config.UpgradeConfig{UploadDir: dir}}
	defer func() { config.Cfg = nil }()

	path, err := saveUpgradeFile(7, "firmware.bin", []byte("hello"))
	if err != nil {
		t.Fatalf("saveUpgradeFile failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("content mismatch: %q", string(data))
	}
	if filepath.Dir(path) != filepath.Join(dir, "7") {
		t.Fatalf("unexpected tenant dir: %s", filepath.Dir(path))
	}
}

func TestDeviceDownloadURL(t *testing.T) {
	config.Cfg = &config.Config{TR069: config.TR069Config{FileServerIp: "http://10.0.0.1:8080"}}
	defer func() { config.Cfg = nil }()

	got := deviceDownloadURL(42)
	want := "http://10.0.0.1:8080/acs-file-server/upgrade/downloadFile/42"
	if got != want {
		t.Fatalf("deviceDownloadURL = %q, want %q", got, want)
	}
}
