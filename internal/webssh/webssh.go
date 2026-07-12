package webssh

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"nmsappsrv/pkg/logger"
)

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

// DeviceSSHInfo holds everything needed to dial an SSH session to a device.
type DeviceSSHInfo struct {
	ElementID  int64
	DeviceName string
	Host       string // management IP
	Port       int
	Username   string
	Password   string
}

// WebSSHConfig mirrors the JSON stored in system_config under key "webssh_config".
type WebSSHConfig struct {
	Username *string `json:"username"`
	Password *string `json:"password"` // plain text (server-side only)
	Port     *int    `json:"port"`
}

// ---------------------------------------------------------------------------
// Repository
// ---------------------------------------------------------------------------

// Repository defines the data-access contract for WebSSH.
type Repository interface {
	FindDeviceSSHInfo(elementID int64) (*DeviceSSHInfo, error)
	GetWebSSHConfig() (*WebSSHConfig, error)
}

type repository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed Repository.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// FindDeviceSSHInfo looks up the device management IP and basic info from
// cpe_element. It does NOT fill Username/Password -- those come from
// GetWebSSHConfig or are passed by the client.
func (r *repository) FindDeviceSSHInfo(elementID int64) (*DeviceSSHInfo, error) {
	var row struct {
		NeNeid     int64   `gorm:"column:ne_neid"`
		DeviceName *string `gorm:"column:device_name"`
		DeviceIp   *string `gorm:"column:device_ip"`
		Ip         *string `gorm:"column:ip"`
	}
	err := r.db.Table("cpe_element").
		Select("ne_neid, device_name, device_ip, ip").
		Where("ne_neid = ? AND deleted = 0", elementID).
		Scan(&row).Error
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	host := ""
	if row.DeviceIp != nil && *row.DeviceIp != "" {
		host = *row.DeviceIp
	} else if row.Ip != nil && *row.Ip != "" {
		host = *row.Ip
	}
	if host == "" {
		return nil, fmt.Errorf("device %d has no management IP", elementID)
	}

	name := ""
	if row.DeviceName != nil {
		name = *row.DeviceName
	}

	return &DeviceSSHInfo{
		ElementID:  row.NeNeid,
		DeviceName: name,
		Host:       host,
		Port:       22, // default; overridden by WebSSHConfig
	}, nil
}

// GetWebSSHConfig reads the "webssh_config" key from system_config.
func (r *repository) GetWebSSHConfig() (*WebSSHConfig, error) {
	var row struct {
		Value *string `gorm:"column:config_value"`
	}
	err := r.db.Table("system_config").
		Select("config_value").
		Where("config_key = ?", "webssh_config").
		Scan(&row).Error
	if err != nil {
		// Not found -- return empty config, caller uses defaults.
		return &WebSSHConfig{}, nil
	}
	if row.Value == nil || *row.Value == "" {
		return &WebSSHConfig{}, nil
	}
	var cfg WebSSHConfig
	if err := json.Unmarshal([]byte(*row.Value), &cfg); err != nil {
		return &WebSSHConfig{}, nil
	}
	return &cfg, nil
}

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// Service defines the business-logic contract for WebSSH.
type Service interface {
	// ResolveDeviceInfo returns the fully-resolved SSH connection info for a
	// device, merging DB data with system-wide WebSSH config.
	ResolveDeviceInfo(elementID int64) (*DeviceSSHInfo, error)

	// DialSSH establishes an SSH connection and returns a *ssh.Session ready
	// for interactive use.
	DialSSH(info *DeviceSSHInfo) (*ssh.Client, *ssh.Session, error)
}

type service struct {
	repo Repository
}

// NewService creates a new WebSSH service.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

func (s *service) ResolveDeviceInfo(elementID int64) (*DeviceSSHInfo, error) {
	info, err := s.repo.FindDeviceSSHInfo(elementID)
	if err != nil {
		return nil, err
	}

	cfg, err := s.repo.GetWebSSHConfig()
	if err != nil {
		return nil, fmt.Errorf("load webssh config: %w", err)
	}

	if cfg.Username != nil && *cfg.Username != "" {
		info.Username = *cfg.Username
	} else {
		info.Username = "root" // fallback default
	}
	if cfg.Password != nil && *cfg.Password != "" {
		info.Password = *cfg.Password
	}
	if cfg.Port != nil && *cfg.Port > 0 {
		info.Port = *cfg.Port
	}

	return info, nil
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

	addr := net.JoinHostPort(info.Host, strconv.Itoa(info.Port))
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
