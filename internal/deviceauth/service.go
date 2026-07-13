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
	// configDBKey is the per-tenant system_config id prefix. The tenant
	// license id is appended so each tenant owns an isolated config row,
	// matching Java nms-serv (device_config_<tenancyId>).
	configDBKey = "device_auth_config_"
	// legacyConfigDBKey is the pre-multi-tenancy global key. Kept only as a
	// read-only fallback so existing single-tenant deployments keep working
	// until they save their own per-tenant config.
	legacyConfigDBKey = "device_auth_config"
	redisKeyPrefix    = "device_authentication_"
	redisCacheTTL     = 24 * time.Hour
)

// Service defines the business-logic contract for device auth configuration.
type Service interface {
	GetConfig(licenseId string) (*DeviceAuthConfig, error)
	SaveConfig(cfg *DeviceAuthConfig, licenseId string) error
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
// Checks Redis cache first, falls back to the per-tenant DB row, and finally
// to the legacy global key (read-only) so single-tenant deployments keep
// working during migration.
func (s *service) GetConfig(licenseId string) (*DeviceAuthConfig, error) {
	// Try Redis cache (already per-tenant).
	redisKey := redisKeyPrefix + licenseId
	ctx := context.Background()
	if cached, err := redis.Get(ctx, redisKey); err == nil && cached != "" {
		var cfg DeviceAuthConfig
		if json.Unmarshal([]byte(cached), &cfg) == nil {
			return &cfg, nil
		}
	}

	// Load from DB, per-tenant first.
	sc, err := s.repo.GetConfig(configDBKey + licenseId)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		// Per-tenant row missing: fall back to legacy global config.
		if licenseId == "" {
			return nil, nil
		}
		legacy, lerr := s.repo.GetConfig(legacyConfigDBKey)
		if lerr != nil {
			return nil, nil
		}
		sc = legacy
	}

	if sc.Config == nil || *sc.Config == "" {
		return nil, nil
	}

	var cfg DeviceAuthConfig
	if err := json.Unmarshal([]byte(*sc.Config), &cfg); err != nil {
		return nil, err
	}

	// Cache to Redis (per-tenant).
	if jsonBytes, err := json.Marshal(cfg); err == nil {
		redis.Set(ctx, redisKey, string(jsonBytes), redisCacheTTL)
	}

	return &cfg, nil
}

// SaveConfig persists the device auth config for a tenant to DB and
// invalidates that tenant's Redis cache. Storage is keyed by license id so
// tenants are isolated (mirrors Java nms-serv device_config_<tenancyId>).
func (s *service) SaveConfig(cfg *DeviceAuthConfig, licenseId string) error {
	jsonBytes, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	jsonStr := string(jsonBytes)

	sc := misc.SystemConfig{Id: configDBKey + licenseId, Config: &jsonStr}
	if err := s.repo.SaveConfig(&sc); err != nil {
		return err
	}

	// Invalidate this tenant's Redis cache only.
	ctx := context.Background()
	redis.Del(ctx, redisKeyPrefix+licenseId)

	logger.Infof("device auth config saved for tenant %s", licenseId)
	return nil
}
