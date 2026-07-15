// Package sftp implements the embedded SFTP server that mirrors Java's
// ZTPSftpServer (port 10022) for the ZTP provisioning subsystem. It serves
// the AOS XML root (cfg.FileServer.ZtpDir) to ZTP-capable devices during
// their initial TR-069 session.
//
// The server is opt-in (config ztp.sftp_enabled, default false). The HTTP
// /acs-file-server/ztpFile provider in internal/filebase remains the
// default pull channel; the SFTP listener is only started when a deployment
// needs to match Java's wire protocol (devices that fetch the AOS file via
// SSH/SFTP rather than HTTP).
package sftp

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"nmsappsrv/pkg/logger"
)

// AuthFunc is called for each SFTP login. Returns true if the (username,
// password) pair is valid. The function is injected from main so this
// package stays decoupled from misc (which owns ZTPSetting).
type AuthFunc func(username, password string) bool

// Server is the embedded SFTP server for ZTP AOS file delivery.
type Server struct {
	host    string
	keyPath string
	dir     string
	auth    AuthFunc

	mu       sync.Mutex
	running  bool
	listener net.Listener
}

// NewServer creates a new SFTP server. host is a listen address
// (e.g. ":10022"), keyPath is the persistent SSH host key file (PEM). If
// keyPath is empty or the file does not exist, an Ed25519 key is
// auto-generated and persisted to keyPath on first Start. dir is the
// directory served to authenticated devices.
func NewServer(host, keyPath, dir string, auth AuthFunc) *Server {
	return &Server{host: host, keyPath: keyPath, dir: dir, auth: auth}
}

// Start brings the server up. It is safe to call once; subsequent calls
// are no-ops.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil
	}

	// 1. Load or generate host key.
	signer, err := s.loadOrGenerateHostKey()
	if err != nil {
		return fmt.Errorf("ztp sftp: host key: %w", err)
	}

	// 2. Build SSH server config.
	sshCfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if s.auth == nil {
				return nil, errors.New("auth not configured")
			}
			if s.auth(c.User(), string(pass)) {
				return nil, nil // accept
			}
			return nil, errors.New("invalid credentials")
		},
		ServerVersion: "SSH-2.0-nmsappsrv-ztp-sftp",
	}
	sshCfg.AddHostKey(signer)

	// 3. Listen.
	ln, err := net.Listen("tcp", s.host)
	if err != nil {
		return fmt.Errorf("ztp sftp: listen %s: %w", s.host, err)
	}
	s.listener = ln
	s.running = true

	logger.Infof("ztp sftp: listening on %s, serving %s", s.host, s.dir)

	// 4. Accept loop.
	go s.acceptLoop(sshCfg)
	return nil
}

// Stop shuts the server down. Safe to call when not running.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return nil
	}
	s.running = false
	if s.listener != nil {
		_ = s.listener.Close()
	}
	logger.Info("ztp sftp: stopped")
	return nil
}

// IsRunning reports whether the server is currently listening.
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Addr returns the actual listen address (useful when host was ":0" or empty).
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) acceptLoop(cfg *ssh.ServerConfig) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			running := s.running
			s.mu.Unlock()
			if !running {
				return
			}
			// Transient accept error (e.g. too many FDs); back off briefly.
			time.Sleep(50 * time.Millisecond)
			continue
		}
		go s.handleConn(conn, cfg)
	}
}

func (s *Server) handleConn(conn net.Conn, cfg *ssh.ServerConfig) {
	defer conn.Close()
	sc, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return
	}
	defer sc.Close()
	go ssh.DiscardRequests(reqs)
	for ch := range chans {
		if ch == nil {
			continue
		}
		if ch.ChannelType() != "session" {
			_ = ch.Reject(ssh.Prohibited, "only session channels accepted")
			continue
		}
		ch, requests, err := ch.Accept()
		if err != nil {
			continue
		}
		go s.handleSession(ch, requests)
	}
}

func (s *Server) handleSession(ch ssh.Channel, requests <-chan *ssh.Request) {
	defer ch.Close()
	for req := range requests {
		switch req.Type {
		case "subsystem":
			// Payload is 4 bytes length-prefixed string in SSH wire format;
			// the string "sftp" indicates the SFTP subsystem.
			if len(req.Payload) >= 4 && string(req.Payload[4:]) == "sftp" {
				req.Reply(true, nil)
				if err := s.serveSFTP(ch); err != nil {
					logger.Debugf("ztp sftp: session ended: %v", err)
				}
				return
			}
		}
		if req.WantReply {
			req.Reply(false, nil)
		}
	}
}

func (s *Server) serveSFTP(ch ssh.Channel) error {
	server, err := sftp.NewServer(ch, sftp.WithServerWorkingDirectory(s.dir))
	if err != nil {
		return err
	}
	return server.Serve()
}

func (s *Server) loadOrGenerateHostKey() (ssh.Signer, error) {
	if s.keyPath != "" {
		if data, err := os.ReadFile(s.keyPath); err == nil {
			if signer, err := ssh.ParsePrivateKey(data); err == nil {
				return signer, nil
			}
		}
	}

	// Generate Ed25519 host key.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("ed25519 generate: %w", err)
	}
	pemBlock, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return nil, fmt.Errorf("marshal ed25519: %w", err)
	}
	pemData := pem.EncodeToMemory(pemBlock)
	signer, err := ssh.ParsePrivateKey(pemData)
	if err != nil {
		return nil, fmt.Errorf("parse generated key: %w", err)
	}

	if s.keyPath != "" {
		if err := os.MkdirAll(filepath.Dir(s.keyPath), 0o700); err != nil {
			return nil, fmt.Errorf("mkdir for host key: %w", err)
		}
		if err := os.WriteFile(s.keyPath, pemData, 0o600); err != nil {
			return nil, fmt.Errorf("write host key: %w", err)
		}
		logger.Infof("ztp sftp: generated and persisted ed25519 host key to %s", s.keyPath)
	} else {
		logger.Info("ztp sftp: generated ephemeral ed25519 host key (no sftp_host_key configured)")
	}
	return signer, nil
}

// MarshalED25519PrivateKeyPEM is a helper kept for tests/diagnostics.
// It serializes an ed25519.PrivateKey to PKCS#8 PEM.
func MarshalED25519PrivateKeyPEM(priv ed25519.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}
