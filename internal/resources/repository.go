package resources

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// Repository provides database operations for resources
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetSystemConfig reads a system_config entry by config_key
func (r *Repository) GetSystemConfig(key string) (string, error) {
	var cfg systemConfigModel
	if err := r.db.Where("id = ?", key).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	if cfg.Config == nil {
		return "", nil
	}
	return *cfg.Config, nil
}

// SaveSystemConfig upserts a system_config entry
func (r *Repository) SaveSystemConfig(key, value string) error {
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
		return err
	}
	cfg.Config = &value
	return r.db.Save(&cfg).Error
}

// GetTableStatus returns MySQL table sizes via information_schema
func (r *Repository) GetTableStatus() ([]TableStatusVO, error) {
	var results []TableStatusVO

	rows, err := r.db.Raw(`
		SELECT
			table_name,
			table_rows,
			ROUND((data_length + index_length) / 1024 / 1024 / 1024, 2) AS size_gb
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		ORDER BY (data_length + index_length) DESC
	`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var t TableStatusVO
		if err := rows.Scan(&t.TableName, &t.TableRows, &t.SizeGB); err != nil {
			continue
		}
		// Rename tables containing "cpe" to "ne" (match Java behaviour)
		if strings.Contains(t.TableName, "cpe") {
			t.TableName = strings.ReplaceAll(t.TableName, "cpe", "ne")
		}
		results = append(results, t)
	}

	return results, nil
}
