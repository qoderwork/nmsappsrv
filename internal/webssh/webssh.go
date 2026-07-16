package webssh

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"nmsappsrv/pkg/logger"
)

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

// DeviceSSHInfo holds everything needed to dial an SSH session to a device.
// Credentials are provided by the client via the WebSocket "connect" message,
// aligned with Java WebSSHServiceImpl semantics.
type DeviceSSHInfo struct {
	Host     string // SSH host (management IP)
	Port     int    // SSH port (default 22)
	Username string
	Password string
}

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// Service defines the business-logic contract for WebSSH.
type Service interface {
	// DialSSH establishes an SSH connection using client-provided credentials
	// and returns a *ssh.Client + *ssh.Session ready for interactive use.
	DialSSH(info *DeviceSSHInfo) (*ssh.Client, *ssh.Session, error)
}

type service struct{}

// NewService creates a new WebSSH service.
func NewService() Service {
	return &service{}
}

func (s *service) DialSSH(info *DeviceSSHInfo) (*ssh.Client, *ssh.Session, error) {
	authMethods := []ssh.AuthMethod{}
	if info.Password != "" {
		authMethods = append(authMethods, ssh.Password(info.Password))
	}
	if len(authMethods) == 0 {
		return nil, nil, fmt.Errorf("no SSH auth method available (password empty)")
	}

	sshConfig := &ssh.ClientConfig{
		User:            info.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	port := info.Port
	if port <= 0 {
		port = 22
	}

	addr := net.JoinHostPort(info.Host, strconv.Itoa(port))
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("ssh new session: %w", err)
	}

	return client, session, nil
}

// ---------------------------------------------------------------------------
// SSHSession -- wraps an SSH session for interactive terminal I/O
// ---------------------------------------------------------------------------

// SSHSession wraps ssh.Session with a pty and provides thread-safe Write.
type SSHSession struct {
	Client  *ssh.Client
	Session *ssh.Session
	Stdin   io.WriteCloser
	Stdout  io.Reader
	Stderr  io.Reader

	mu     sync.Mutex
	closed bool
}

// NewSSHSession requests a PTY, wires up stdin/stdout/stderr pipes, and
// starts an interactive shell.
func NewSSHSession(client *ssh.Client, session *ssh.Session, cols, rows int) (*SSHSession, error) {
	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	if err := session.RequestPty("xterm", rows, cols, modes); err != nil {
		return nil, fmt.Errorf("request pty: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := session.Shell(); err != nil {
		return nil, fmt.Errorf("start shell: %w", err)
	}

	return &SSHSession{
		Client:  client,
		Session: session,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
	}, nil
}

// Write sends data to the SSH session stdin (thread-safe).
func (s *SSHSession) Write(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("session closed")
	}
	_, err := s.Stdin.Write(data)
	return err
}

// Resize changes the PTY window size.
func (s *SSHSession) Resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("session closed")
	}
	return s.Session.WindowChange(rows, cols)
}

// Close shuts down the SSH session and client connection.
func (s *SSHSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true

	if err := s.Stdin.Close(); err != nil {
		logger.Debugf("webssh: close stdin: %v", err)
	}
	if err := s.Session.Close(); err != nil {
		// "exit status 1" is normal when shell exits
		logger.Debugf("webssh: close session: %v", err)
	}
	if err := s.Client.Close(); err != nil {
		logger.Debugf("webssh: close client: %v", err)
	}
}

// Wait blocks until the SSH session exits.
func (s *SSHSession) Wait() error {
	return s.Session.Wait()
}
