package mail

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/smtp"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// Service contains mail business logic.
type Service interface {
	GetConfig() (*MailConfig, error)
	UpdateConfig(req *UpdateMailConfigRequest) error
	SendTestMail() error
	SendEmailCode(username, grantType string) error
	CheckEmailCode(username, code string) (bool, error)
	IsEmailAuthEnabled() (bool, error)
	SendMail(to []string, subject, body string) error
	GetSuperUserEmail() (string, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo   Repository
	aesKey []byte // 32-byte AES-256 key
}

// NewService creates a new mail service.
// aesKeyHex is a hex-encoded key used for AES-GCM encryption of sensitive fields.
func NewService(db *gorm.DB, aesKeyHex string) Service {
	key := make([]byte, 32)
	if aesKeyHex != "" {
		decoded, err := hex.DecodeString(aesKeyHex)
		if err == nil && len(decoded) >= 16 {
			copy(key, decoded)
		}
	}
	return &service{repo: NewRepository(db), aesKey: key}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// ---------- public methods ----------

// GetConfig reads and decrypts the mail configuration. Password is masked.
func (s *service) GetConfig() (*MailConfig, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, err
	}
	cfg.Username = s.decrypt(cfg.Username)
	cfg.Password = maskPassword(s.decrypt(cfg.Password))
	cfg.SuperUserEmail = s.decrypt(cfg.SuperUserEmail)
	return cfg, nil
}

// UpdateConfig encrypts sensitive fields and persists the mail configuration.
func (s *service) UpdateConfig(req *UpdateMailConfigRequest) error {
	if err := s.validateRequest(req); err != nil {
		return err
	}

	cfg := &MailConfig{
		Host:               req.Host,
		Port:               req.Port,
		Username:           s.encrypt(req.Username),
		MailAuthentication: req.MailAuthentication,
		SuperUserEmail:     s.encrypt(req.SuperUserEmail),
	}

	// Only update password if a new one is provided
	if req.Password != "" {
		cfg.Password = s.encrypt(req.Password)
	} else {
		existing, _ := s.loadConfig()
		if existing != nil {
			cfg.Password = existing.Password
		}
	}

	return s.saveConfig(cfg)
}

// SendTestMail sends a test email to all configured super user addresses.
func (s *service) SendTestMail() error {
	cfg, err := s.loadConfig()
	if err != nil {
		return err
	}
	username := s.decrypt(cfg.Username)
	password := s.decrypt(cfg.Password)
	emails := s.decrypt(cfg.SuperUserEmail)

	recipients := strings.Split(emails, ";")
	for _, r := range recipients {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if err := s.sendMail(cfg, username, password, []string{r}, "NMS Test Email", "This is a test email from NMS mail configuration."); err != nil {
			return fmt.Errorf("send to %s: %w", r, err)
		}
	}
	return nil
}

// SendEmailCode generates a 6-digit verification code, stores it in Redis,
// and emails it to the user.
func (s *service) SendEmailCode(username, grantType string) error {
	ctx := context.Background()

	// Rate limit: 1 code per minute
	rateKey := fmt.Sprintf("in_get_email_code_%s", username)
	if redis.Exists(ctx, rateKey) {
		return fmt.Errorf("please wait before requesting another code")
	}

	// Determine recipient email
	var email string
	if grantType == "radius" {
		email = username
	} else {
		// Look up user email from DB
		userEmail, _ := s.repo.FindUserEmailByUsername(username)
		email = userEmail
	}
	if email == "" || !isValidEmail(email) {
		return fmt.Errorf("invalid email address for user %s", username)
	}

	// Generate 6-digit code
	code, _ := rand.Int(rand.Reader, big.NewInt(999999))
	codeStr := fmt.Sprintf("%06d", code.Int64())

	// Store in Redis with 10-minute TTL
	codeKey := fmt.Sprintf("email_code_%s", username)
	redis.Set(ctx, codeKey, codeStr, 10*time.Minute)
	redis.Set(ctx, rateKey, "1", 1*time.Minute)

	// Send email
	cfg, err := s.loadConfig()
	if err != nil {
		return err
	}
	u := s.decrypt(cfg.Username)
	p := s.decrypt(cfg.Password)

	body := fmt.Sprintf(`<h2>Email Verification Code</h2><p>Your verification code is:</p><h1 style="color:#1a73e8;font-size:32px">%s</h1><p>This code will expire in 10 minutes.</p>`, codeStr)
	return s.sendMail(cfg, u, p, []string{email}, "Email Verification Code", body)
}

// CheckEmailCode verifies the submitted code against Redis.
func (s *service) CheckEmailCode(username, code string) (bool, error) {
	ctx := context.Background()
	codeKey := fmt.Sprintf("email_code_%s", username)
	stored, err := redis.Get(ctx, codeKey)
	if err != nil {
		return false, fmt.Errorf("verification code expired or not found")
	}
	if stored != code {
		return false, nil
	}
	redis.Del(ctx, codeKey)
	return true, nil
}

// IsEmailAuthEnabled checks whether email authentication is enabled.
func (s *service) IsEmailAuthEnabled() (bool, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return false, err
	}
	return cfg.MailAuthentication, nil
}

// SendMail sends an email to the given recipients using the stored mail config.
// This is the public entry point used by other modules (e.g. alarm notifier).
func (s *service) SendMail(to []string, subject, body string) error {
	cfg, err := s.loadConfig()
	if err != nil {
		return fmt.Errorf("load mail config: %w", err)
	}
	username := s.decrypt(cfg.Username)
	password := s.decrypt(cfg.Password)
	return s.sendMail(cfg, username, password, to, subject, body)
}

// GetSuperUserEmail returns the decrypted super-user email addresses
// (semicolon-separated) from the mail configuration.
func (s *service) GetSuperUserEmail() (string, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return "", err
	}
	return s.decrypt(cfg.SuperUserEmail), nil
}

// ---------- repository helpers ----------

func (s *service) loadConfig() (*MailConfig, error) {
	key := "mail"
	sc, err := s.repo.FindConfigByKey(key)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &MailConfig{}, nil
		}
		return nil, err
	}
	if sc.Config == nil || *sc.Config == "" {
		return &MailConfig{}, nil
	}
	var cfg MailConfig
	if err := json.Unmarshal([]byte(*sc.Config), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *service) saveConfig(cfg *MailConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	val := string(data)
	key := "mail"

	sc, err := s.repo.FindConfigByKey(key)
	if err == gorm.ErrRecordNotFound {
		return s.repo.CreateConfig(&SystemConfig{Id: key, Config: &val})
	}
	if err != nil {
		return err
	}
	sc.Config = &val
	return s.repo.SaveConfig(sc)
}

// ---------- AES-GCM encryption ----------

func (s *service) encrypt(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	block, err := aes.NewCipher(s.aesKey)
	if err != nil {
		logger.Errorf("mail encrypt: %v", err)
		return plaintext
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		logger.Errorf("mail encrypt gcm: %v", err)
		return plaintext
	}
	nonce := make([]byte, aead.NonceSize())
	io.ReadFull(rand.Reader, nonce)
	ciphertext := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext)
}

func (s *service) decrypt(ciphertext string) string {
	if ciphertext == "" {
		return ""
	}
	data, err := hex.DecodeString(ciphertext)
	if err != nil {
		return ciphertext // not encrypted
	}
	block, err := aes.NewCipher(s.aesKey)
	if err != nil {
		return ciphertext
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return ciphertext
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return ciphertext
	}
	nonce, ct := data[:nonceSize], data[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return ciphertext // decryption failed, return as-is
	}
	return string(plaintext)
}

// ---------- validation ----------

func (s *service) validateRequest(req *UpdateMailConfigRequest) error {
	if req.Port < 1 || req.Port > 65535 {
		return fmt.Errorf("invalid port number")
	}
	if !isValidEmail(req.Username) {
		return fmt.Errorf("invalid username email format")
	}
	if !isValidHost(req.Host) {
		return fmt.Errorf("invalid host")
	}
	if req.SuperUserEmail != "" {
		for _, e := range strings.Split(req.SuperUserEmail, ";") {
			e = strings.TrimSpace(e)
			if e != "" && !isValidEmail(e) {
				return fmt.Errorf("invalid super user email: %s", e)
			}
		}
	}
	return nil
}

// ---------- SMTP ----------

func (s *service) sendMail(cfg *MailConfig, username, password string, to []string, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		username, strings.Join(to, ","), subject, body)

	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, cfg.Host)
	}

	return smtp.SendMail(addr, auth, username, to, []byte(msg))
}

// ---------- helpers ----------

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func isValidEmail(email string) bool {
	if strings.Contains(email, ":") {
		return false
	}
	return emailRegex.MatchString(email)
}

func isValidHost(host string) bool {
	ipRegex := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	hostRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*$`)
	return ipRegex.MatchString(host) || hostRegex.MatchString(host)
}

func maskPassword(password string) string {
	if len(password) <= 4 {
		return "****"
	}
	return password[:2] + strings.Repeat("*", len(password)-4) + password[len(password)-2:]
}
