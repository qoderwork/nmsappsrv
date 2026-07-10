package health

import (
	"fmt"

	"gorm.io/gorm"
)

// Repository handles database operations for health checks
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetMysqlGlobalStatus executes SHOW GLOBAL STATUS and returns metrics as a map
func (r *Repository) GetMysqlGlobalStatus() (map[string]string, error) {
	rows, err := r.db.Raw("SHOW GLOBAL STATUS").Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to get mysql status: %w", err)
	}
	defer rows.Close()

	metrics := make(map[string]string)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			continue
		}
		metrics[name] = value
	}
	return metrics, nil
}
