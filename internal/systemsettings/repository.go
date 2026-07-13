package systemsettings

import (
	"errors"

	"gorm.io/gorm"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/baserepo"
)

// systemConfigModel maps to the system_config table.
// The real table schema (id varchar PK + config longtext) is owned/migrated by
// site.SystemConfig and shared by platform/resources/deviceauth. Any other
// column naming (config_key/config_value) does not match the live table.
type systemConfigModel struct {
	Id     string  `gorm:"primaryKey;column:id;type:varchar(32)"`
	Config *string `gorm:"column:config;type:longtext"`
}

func (systemConfigModel) TableName() string { return "system_config" }

// SystemSettingsRepository provides database operations for system settings.
type SystemSettingsRepository struct {
	*baserepo.BaseRepository[SysParameter, int] // embedded generic CRUD for SysParameter
	db *gorm.DB
}

// NewSystemSettingsRepository creates a new SystemSettingsRepository.
func NewSystemSettingsRepository(db *gorm.DB) *SystemSettingsRepository {
	return &SystemSettingsRepository{
		BaseRepository: baserepo.New[SysParameter, int](db, "id"),
		db:             db,
	}
}

// GetSystemConfig reads a system_config entry by id (config key).
func (r *SystemSettingsRepository) GetSystemConfig(key string) (string, error) {
	var cfg systemConfigModel
	if err := r.db.Where("id = ?", key).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", apperror.Wrap(err, "GET_SYSTEM_CONFIG_FAILED", 500, "failed to get system config")
	}
	if cfg.Config == nil {
		return "", nil
	}
	return *cfg.Config, nil
}

// SaveSystemConfig upserts a system_config entry by id (config key).
func (r *SystemSettingsRepository) SaveSystemConfig(key, value string) error {
	var cfg systemConfigModel
	err := r.db.Where("id = ?", key).First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cfg = systemConfigModel{
				Id:     key,
				Config: &value,
			}
			return r.db.Create(&cfg).Error
		}
		return apperror.Wrap(err, "QUERY_SYSTEM_CONFIG_FAILED", 500, "failed to query system config")
	}
	cfg.Config = &value
	return r.db.Save(&cfg).Error
}

// GetSysParameter reads a sys_parameter entry by config_key.
func (r *SystemSettingsRepository) GetSysParameter(key string) (string, error) {
	var param SysParameter
	if err := r.db.Where("config_key = ?", key).First(&param).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", apperror.Wrap(err, "GET_SYS_PARAMETER_FAILED", 500, "failed to get sys parameter")
	}
	if param.Value == nil {
		return "", nil
	}
	return *param.Value, nil
}

// SaveSysParameter upserts a sys_parameter entry by config_key.
func (r *SystemSettingsRepository) SaveSysParameter(key, value string) error {
	var param SysParameter
	err := r.db.Where("config_key = ?", key).First(&param).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			param = SysParameter{
				Key:   &key,
				Value: &value,
			}
			return r.Create(&param)
		}
		return apperror.Wrap(err, "QUERY_SYS_PARAMETER_FAILED", 500, "failed to query sys parameter")
	}
	param.Value = &value
	return r.Save(&param)
}
