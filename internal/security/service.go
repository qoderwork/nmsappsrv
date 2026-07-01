package security

import (
	"encoding/json"
	"fmt"
	"sync"

	"gorm.io/gorm"
)

// SystemConfig mirrors system_config table for key-value config storage.
type SystemConfig struct {
	Id     string  `gorm:"primaryKey;column:id;type:varchar(255)" json:"id"`
	Config *string `gorm:"column:config;type:longtext" json:"config"`
}

func (SystemConfig) TableName() string { return "system_config" }

// SecurityRule is the JSON payload stored in system_config (key="security_rule").
type SecurityRule struct {
	MaxFailedLoginAttempts int `json:"maximumNumberOfFailedLoginAttempts"`
	MinUsernameLen         int `json:"minimumLengthOfUsername"`
	MaxUsernameLen         int `json:"maximumLengthOfUsername"`
	MinPasswordLen         int `json:"minimumLengthOfPassword"`
	MaxPasswordLen         int `json:"maximumLengthOfPassword"`
	MinAdminPasswordLen    int `json:"minimumLengthOfAdminPassword"`
	MaxAdminPasswordLen    int `json:"maximumLengthOfAdminPassword"`
}

// UpdateSecurityRuleRequest is the JSON body for POST /updateSecurityRule.
type UpdateSecurityRuleRequest struct {
	MaxFailedLoginAttempts *int `json:"maximumNumberOfFailedLoginAttempts"`
	MinUsernameLen         *int `json:"minimumLengthOfUsername"`
	MaxUsernameLen         *int `json:"maximumLengthOfUsername"`
	MinPasswordLen         *int `json:"minimumLengthOfPassword"`
	MaxPasswordLen         *int `json:"maximumLengthOfPassword"`
	MinAdminPasswordLen    *int `json:"minimumLengthOfAdminPassword"`
	MaxAdminPasswordLen    *int `json:"maximumLengthOfAdminPassword"`
}

// PasswordStrategyResponse is the response for GET /getPasswordStrategy.
type PasswordStrategyResponse struct {
	MinPasswordLen      int `json:"minimumLengthOfPassword"`
	MaxPasswordLen      int `json:"maximumLengthOfPassword"`
	MinAdminPasswordLen int `json:"minimumLengthOfAdminPassword"`
	MaxAdminPasswordLen int `json:"maximumLengthOfAdminPassword"`
}

// defaults
var defaultRule = SecurityRule{
	MaxFailedLoginAttempts: 5,
	MinUsernameLen:         4,
	MaxUsernameLen:         20,
	MinPasswordLen:         14,
	MaxPasswordLen:         100,
	MinAdminPasswordLen:    30,
	MaxAdminPasswordLen:    120,
}

// Service contains security rule business logic.
type Service struct {
	db *gorm.DB
	mu sync.Mutex
}

// NewService creates a new security rule service.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// GetRule reads the security rule config, filling defaults for missing fields.
func (s *Service) GetRule() (*SecurityRule, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		r := defaultRule
		return &r, nil
	}
	s.fillDefaults(cfg)
	return cfg, nil
}

// UpdateRule validates and persists the security rule config.
func (s *Service) UpdateRule(req *UpdateSecurityRuleRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.loadConfig()
	if err != nil {
		return err
	}
	if current == nil {
		r := defaultRule
		current = &r
	}

	if req.MaxFailedLoginAttempts != nil {
		if *req.MaxFailedLoginAttempts <= 0 {
			return fmt.Errorf("maximumNumberOfFailedLoginAttempts must be > 0")
		}
		current.MaxFailedLoginAttempts = *req.MaxFailedLoginAttempts
	}
	if req.MinUsernameLen != nil {
		current.MinUsernameLen = *req.MinUsernameLen
	}
	if req.MaxUsernameLen != nil {
		current.MaxUsernameLen = *req.MaxUsernameLen
	}
	if req.MinPasswordLen != nil {
		current.MinPasswordLen = *req.MinPasswordLen
	}
	if req.MaxPasswordLen != nil {
		current.MaxPasswordLen = *req.MaxPasswordLen
	}
	if req.MinAdminPasswordLen != nil {
		current.MinAdminPasswordLen = *req.MinAdminPasswordLen
	}
	if req.MaxAdminPasswordLen != nil {
		current.MaxAdminPasswordLen = *req.MaxAdminPasswordLen
	}

	if current.MaxUsernameLen < current.MinUsernameLen {
		return fmt.Errorf("maximum username length must >= minimum")
	}
	if current.MaxPasswordLen < current.MinPasswordLen {
		return fmt.Errorf("maximum password length must >= minimum")
	}

	return s.saveConfig(current)
}

// GetPasswordStrategy returns only the password length strategy.
func (s *Service) GetPasswordStrategy() (*PasswordStrategyResponse, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return &PasswordStrategyResponse{
			MinPasswordLen:      defaultRule.MinPasswordLen,
			MaxPasswordLen:      defaultRule.MaxPasswordLen,
			MinAdminPasswordLen: defaultRule.MinAdminPasswordLen,
			MaxAdminPasswordLen: defaultRule.MaxAdminPasswordLen,
		}, nil
	}
	return &PasswordStrategyResponse{
		MinPasswordLen:      cfg.MinPasswordLen,
		MaxPasswordLen:      cfg.MaxPasswordLen,
		MinAdminPasswordLen: cfg.MinAdminPasswordLen,
		MaxAdminPasswordLen: cfg.MaxAdminPasswordLen,
	}, nil
}

// ---------- repository helpers ----------

func (s *Service) loadConfig() (*SecurityRule, error) {
	var sc SystemConfig
	key := "security_rule"
	if err := s.db.Where("id = ?", key).First(&sc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	if sc.Config == nil || *sc.Config == "" {
		return nil, nil
	}
	var cfg SecurityRule
	if err := json.Unmarshal([]byte(*sc.Config), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *Service) saveConfig(cfg *SecurityRule) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	val := string(data)
	key := "security_rule"

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

func (s *Service) fillDefaults(cfg *SecurityRule) {
	if cfg.MaxFailedLoginAttempts == 0 {
		cfg.MaxFailedLoginAttempts = defaultRule.MaxFailedLoginAttempts
	}
	if cfg.MinUsernameLen == 0 {
		cfg.MinUsernameLen = defaultRule.MinUsernameLen
	}
	if cfg.MaxUsernameLen == 0 {
		cfg.MaxUsernameLen = defaultRule.MaxUsernameLen
	}
	if cfg.MinPasswordLen == 0 {
		cfg.MinPasswordLen = defaultRule.MinPasswordLen
	}
	if cfg.MaxPasswordLen == 0 {
		cfg.MaxPasswordLen = defaultRule.MaxPasswordLen
	}
	if cfg.MinAdminPasswordLen == 0 {
		cfg.MinAdminPasswordLen = defaultRule.MinAdminPasswordLen
	}
	if cfg.MaxAdminPasswordLen == 0 {
		cfg.MaxAdminPasswordLen = defaultRule.MaxAdminPasswordLen
	}
}
