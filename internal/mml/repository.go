package mml

import (
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository handles database operations for MML entities.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// FindMmlSets returns all MML sets for the given license.
func (r *Repository) FindMmlSets(licenseId int) ([]MmlSet, error) {
	var sets []MmlSet
	if err := r.db.Where("license_id = ?", licenseId).Find(&sets).Error; err != nil {
		logger.Errorf("FindMmlSets error: %v", err)
		return nil, err
	}
	return sets, nil
}

// FindMmlCommands returns all commands belonging to the given MML set.
func (r *Repository) FindMmlCommands(setId int) ([]MmlCommand, error) {
	var cmds []MmlCommand
	if err := r.db.Where("mml_set_id = ?", setId).Find(&cmds).Error; err != nil {
		logger.Errorf("FindMmlCommands error: %v", err)
		return nil, err
	}
	return cmds, nil
}

// FindMmlCommandParams returns all parameters for the given command.
func (r *Repository) FindMmlCommandParams(commandId int) ([]MmlCommandParam, error) {
	var params []MmlCommandParam
	if err := r.db.Where("mml_command_id = ?", commandId).Find(&params).Error; err != nil {
		logger.Errorf("FindMmlCommandParams error: %v", err)
		return nil, err
	}
	return params, nil
}

// CreateMmlExecuteResult inserts a new MML execution result record.
func (r *Repository) CreateMmlExecuteResult(result *MmlExecuteResult) error {
	return r.db.Create(result).Error
}

// UpdateMmlExecuteResult saves changes to an existing MML execution result.
func (r *Repository) UpdateMmlExecuteResult(result *MmlExecuteResult) error {
	return r.db.Save(result).Error
}

// FindMmlExecuteResults returns a paginated list of results for the given element.
func (r *Repository) FindMmlExecuteResults(elementId int64, offset, limit int) ([]MmlExecuteResult, int64, error) {
	var results []MmlExecuteResult
	var total int64

	query := r.db.Model(&MmlExecuteResult{}).Where("element_id = ?", elementId)

	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("FindMmlExecuteResults count error: %v", err)
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&results).Error; err != nil {
		logger.Errorf("FindMmlExecuteResults query error: %v", err)
		return nil, 0, err
	}
	return results, total, nil
}

// FindMmlExecuteResultByID returns a single execution result by its primary key.
func (r *Repository) FindMmlExecuteResultByID(id int) (*MmlExecuteResult, error) {
	var result MmlExecuteResult
	if err := r.db.Where("id = ?", id).First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}
