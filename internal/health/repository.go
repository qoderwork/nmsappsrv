package health

import (
	"nmsappsrv/pkg/apperror"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for health checks
type Repository interface {
	GetMysqlGlobalStatus() (map[string]string, error)
}

// repository is the concrete GORM-backed implementation of Repository.
type repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// GetMysqlGlobalStatus executes SHOW GLOBAL STATUS and returns metrics as a map
func (r *repository) GetMysqlGlobalStatus() (map[string]string, error) {
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
