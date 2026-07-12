package mail

import (
	"gorm.io/gorm"
)

// Repository defines the data-access contract for mail.
type Repository interface {
	FindConfigByKey(key string) (*SystemConfig, error)
	CreateConfig(sc *SystemConfig) error
	SaveConfig(sc *SystemConfig) error
	FindUserEmailByUsername(username string) (string, error)
}

// repository is the concrete GORM-backed implementation of Repository.
type repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// FindConfigByKey loads system_config by id
func (r *repository) FindConfigByKey(key string) (*SystemConfig, error) {
	var sc SystemConfig
	if err := r.db.Where("id = ?", key).First(&sc).Error; err != nil {
		return nil, err
	}
	return &sc, nil
}

// CreateConfig creates a new system_config record
func (r *repository) CreateConfig(sc *SystemConfig) error {
	return r.db.Create(sc).Error
}

// SaveConfig updates an existing system_config record
func (r *repository) SaveConfig(sc *SystemConfig) error {
	return r.db.Save(sc).Error
}

// FindUserEmailByUsername looks up a user's email by username
func (r *repository) FindUserEmailByUsername(username string) (string, error) {
	var email string
	if err := r.db.Table("user").Where("username = ?", username).Pluck("email", &email).Error; err != nil {
		return "", err
	}
	return email, nil
}
