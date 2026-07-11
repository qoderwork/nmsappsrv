package systemsettings

import (
	"errors"

	"gorm.io/gorm"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/baserepo"
)

// systemConfigModel maps to the system_config table (shared with misc package).
type systemConfigModel struct {
	Id    int     `gorm:"primaryKey;autoIncrement"`
	Key   *string `gorm:"column:config_key;type:varchar(255);uniqueIndex"`
	Value *string `gorm:"column:config_value;type:longtext"`
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

// GetSystemConfig reads a system_config entry by config_key.
func (r *SystemSettingsRepository) GetSystemConfig(key string) (string, error) {
	var cfg systemConfigModel
	if err := r.db.Where("config_key = ?", key).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", apperror.Wrap(err, "GET_SYSTEM_CONFIG_FAILED", 500, "failed to get system config")
	}
	if cfg.Value == nil {
		return "", nil
	}
	return *cfg.Value, nil
}

// SaveSystemConfig upserts a system_config entry by config_key.
func (r *SystemSettingsRepository) SaveSystemConfig(key, value string) error {
	var cfg systemConfigModel
	err := r.db.Where("config_key = ?", key).First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cfg = systemConfigModel{
				Key:   &key,
				Value: &value,
			}
			return r.db.Create(&cfg).Error
		}
		return apperror.Wrap(err, "QUERY_SYSTEM_CONFIG_FAILED", 500, "failed to query system config")
	}
	cfg.Value = &value
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
