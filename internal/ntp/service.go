package ntp

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"gorm.io/gorm"

	"nmsappsrv/pkg/logger"
)

// Service defines the NTP business-logic contract.
type Service interface {
	GetConfig() (*NTPConfig, error)
	UpdateConfig(req *NTPConfigRequest) error
	GetStatus() (string, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
	mu   sync.Mutex
}

// NewService creates a new NTP service.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// GetConfig reads NTP config from system_config.
func (s *service) GetConfig() (*NTPConfig, error) {
	return s.loadConfig()
}

// UpdateConfig persists NTP config and optionally writes the local ntp.conf file.
func (s *service) UpdateConfig(req *NTPConfigRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := &NTPConfig{
		NTPServer: req.NTPServer,
		Enable:    req.Enable,
	}

	if err := s.saveConfig(cfg); err != nil {
		return err
	}

	// Write local ntp.conf (best-effort)
	if cfg.Enable {
		content := fmt.Sprintf("ON,%s", cfg.NTPServer)
		if err := os.WriteFile("/home/conf/conf/ntp/ntp.conf", []byte(content), 0644); err != nil {
			logger.Warnf("ntp: write ntp.conf: %v (non-fatal)", err)
		}
	} else {
		if err := os.Remove("/home/conf/conf/ntp/ntp.conf"); err != nil && !os.IsNotExist(err) {
			logger.Warnf("ntp: remove ntp.conf: %v (non-fatal)", err)
		}
	}
	return nil
}

// GetStatus reads NTP sync status from the local status file.
func (s *service) GetStatus() (string, error) {
	data, err := os.ReadFile("/home/ntp_status")
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read ntp status: %w", err)
	}
	return string(data), nil
}

// ---------- repository helpers ----------

func (s *service) loadConfig() (*NTPConfig, error) {
	key := "ntp"
	sc, err := s.repo.FindConfigByKey(key)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &NTPConfig{}, nil
		}
		return nil, err
	}
	if sc.Config == nil || *sc.Config == "" {
		return &NTPConfig{}, nil
	}
	var cfg NTPConfig
	if err := json.Unmarshal([]byte(*sc.Config), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *service) saveConfig(cfg *NTPConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	val := string(data)
	key := "ntp"

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
