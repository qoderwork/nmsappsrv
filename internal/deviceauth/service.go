package deviceauth

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"nmsappsrv/internal/misc"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

const (
	configDBKey     = "device_auth_config"
	redisKeyPrefix  = "device_authentication_"
	redisCacheTTL   = 24 * time.Hour
)

// Service defines the business-logic contract for device auth configuration.
type Service interface {
	GetConfig(licenseId string) (*DeviceAuthConfig, error)
	SaveConfig(cfg *DeviceAuthConfig) error
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// GetConfig loads the device auth config for a tenant.
// Checks Redis cache first, falls back to DB.
func (s *service) GetConfig(licenseId string) (*DeviceAuthConfig, error) {
	// Try Redis cache
	redisKey := redisKeyPrefix + licenseId
	ctx := context.Background()
	cached, err := redis.Get(ctx, redisKey)
	if err == nil && cached != "" {
		var cfg DeviceAuthConfig
		if json.Unmarshal([]byte(cached), &cfg) == nil {
			return &cfg, nil
		}
	}

	// Load from DB
	sc, err := s.repo.GetConfig(configDBKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if sc.Config == nil || *sc.Config == "" {
		return nil, nil
	}

	var cfg DeviceAuthConfig
	if err := json.Unmarshal([]byte(*sc.Config), &cfg); err != nil {
		return nil, err
	}

	// Cache to Redis
	if jsonBytes, err := json.Marshal(cfg); err == nil {
		redis.Set(ctx, redisKey, string(jsonBytes), redisCacheTTL)
	}

	return &cfg, nil
}

// SaveConfig saves the device auth config to DB and updates Redis for all tenants.
func (s *service) SaveConfig(cfg *DeviceAuthConfig) error {
	jsonBytes, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	jsonStr := string(jsonBytes)

	sc := misc.SystemConfig{Id: configDBKey, Config: &jsonStr}
	if err := s.repo.SaveConfig(&sc); err != nil {
		return err
	}

	// Invalidate Redis cache
	ctx := context.Background()
	redis.Del(ctx, redisKeyPrefix+"*")

	logger.Info("device auth config saved")
	return nil
}
