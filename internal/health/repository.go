package health

import (
	"nmsappsrv/pkg/apperror"

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
		return nil, apperror.Wrap(err, "GET_MYSQL_STATUS_FAILED", 500, "failed to get mysql status")
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
