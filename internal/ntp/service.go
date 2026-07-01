package ntp

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"gorm.io/gorm"

	"nmsappsrv/pkg/logger"
)

// Service contains NTP business logic.
type Service struct {
	db *gorm.DB
	mu sync.Mutex
}

// NewService creates a new NTP service.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// GetConfig reads NTP config from system_config.
func (s *Service) GetConfig() (*NTPConfig, error) {
	return s.loadConfig()
}

// UpdateConfig persists NTP config and optionally writes the local ntp.conf file.
func (s *Service) UpdateConfig(req *NTPConfigRequest) error {
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
func (s *Service) GetStatus() (string, error) {
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

func (s *Service) loadConfig() (*NTPConfig, error) {
	var sc SystemConfig
	key := "ntp"
	if err := s.db.Where("id = ?", key).First(&sc).Error; err != nil {
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

func (s *Service) saveConfig(cfg *NTPConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	val := string(data)
	key := "ntp"

	var sc SystemConfig
	err = s.db.Where("id = ?", key).First(&sc).Error
	if err == gorm.ErrRecordNotFound {
		return s.db.Create(&SystemConfig{Id: key, Config: &val}).Error
	}
	if err != nil {
		return err
	}
	sc.Config = &val
	return s.db.Save(&sc).Error
}
