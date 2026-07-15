package sftp

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// helper: wait for the listener to be ready (caller passes a started server).
func waitReady(t *testing.T, s *Server) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		addr := s.Addr()
		if addr != "" {
			return addr
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("server did not start listening within 2s")
	return ""
}

func dialSFTP(t *testing.T, addr, user, pass string) (*sftp.Client, error) {
	t.Helper()
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         2 * time.Second,
	}
	conn, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, err
	}
	c, err := sftp.NewClient(conn)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return c, nil
}

func TestServer_AuthAndServeFile(t *testing.T) {
	// 1. Prepare a temp directory with a test AOS file.
	dir := t.TempDir()
	aosName := "AOS_GPS_0_SN-TEST-001_20260715.xml"
	body := []byte("<?xml version=\"1.0\"?><autoConfigFile serialNumber=\"SN-TEST-001\"/>")
	if err := os.WriteFile(filepath.Join(dir, aosName), body, 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Start server.
	auth := func(u, p string) bool { return u == "alice" && p == "s3cret" }
	s := NewServer("127.0.0.1:0", "", dir, auth)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()
	addr := waitReady(t, s)

	// 3. Connect with valid credentials and read the file.
	c, err := dialSFTP(t, addr, "alice", "s3cret")
	if err != nil {
		t.Fatalf("dial (good): %v", err)
	}
	defer c.Close()
	f, err := c.Open(aosName)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("body mismatch: got %q want %q", got, body)
	}
}

func TestServer_AuthRejectsBadCreds(t *testing.T) {
	dir := t.TempDir()
	auth := func(u, p string) bool { return u == "alice" && p == "s3cret" }
	s := NewServer("127.0.0.1:0", "", dir, auth)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()
	addr := waitReady(t, s)

	// Wrong password should fail.
	if _, err := dialSFTP(t, addr, "alice", "wrong"); err == nil {
		t.Errorf("expected auth failure for wrong password, got nil")
	}
}

func TestServer_NoAuthFn(t *testing.T) {
	dir := t.TempDir()
	s := NewServer("127.0.0.1:0", "", dir, nil)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()
	addr := waitReady(t, s)

	// No auth callback => every login rejected.
	if _, err := dialSFTP(t, addr, "anyone", "anything"); err == nil {
		t.Errorf("expected auth failure when no callback is set, got nil")
	}
}

func TestServer_HostKeyPersistence(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(t.TempDir(), "hostkey.pem")
	auth := func(u, p string) bool { return u == "u" && p == "p" }

	// First start: generate + persist.
	s1 := NewServer("127.0.0.1:0", keyPath, dir, auth)
	if err := s1.Start(); err != nil {
		t.Fatalf("Start1: %v", err)
	}
	_ = s1.Stop()
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("host key not persisted: %v", err)
	}

	// Second start: must load the same key.
	s2 := NewServer("127.0.0.1:0", keyPath, dir, auth)
	if err := s2.Start(); err != nil {
		t.Fatalf("Start2: %v", err)
	}
	defer func() { _ = s2.Stop() }()
	waitReady(t, s2)
}

func TestServer_StartStopStart(t *testing.T) {
	dir := t.TempDir()
	auth := func(u, p string) bool { return true }
	s := NewServer("127.0.0.1:0", "", dir, auth)
	for i := 0; i < 3; i++ {
		if err := s.Start(); err != nil {
			t.Fatalf("Start #%d: %v", i, err)
		}
		if !s.IsRunning() {
			t.Fatalf("not running after Start #%d", i)
		}
		if err := s.Stop(); err != nil {
			t.Fatalf("Stop #%d: %v", i, err)
		}
		if s.IsRunning() {
			t.Fatalf("still running after Stop #%d", i)
		}
	}
}

// silence "imported and not used" if the test file is ever compiled down to nothing.
var _ = net.IPv4len
