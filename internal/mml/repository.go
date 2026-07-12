package mml

import (
	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for MML entities.
// It embeds BaseRepository[MmlExecuteResult, int] for standard CRUD on MmlExecuteResult,
// and retains module-specific methods for other entity types and custom queries.
type Repository interface {
	Create(entity *MmlExecuteResult) error
	Save(entity *MmlExecuteResult) error
	FindByID(id int) (*MmlExecuteResult, error)
	DeleteByID(id int) error
	DeleteByIDs(ids []int) error
	SoftDelete(id int) error
	UpdateFields(id int, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]MmlExecuteResult, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[MmlExecuteResult], error)
	FindMmlSets(licenseId int) ([]MmlSet, error)
	FindMmlCommands(setId int) ([]MmlCommand, error)
	FindMmlCommandParams(commandId int) ([]MmlCommandParam, error)
	FindMmlExecuteResults(elementId int64, offset, limit int) ([]MmlExecuteResult, int64, error)
}

// repository is the concrete GORM-backed implementation of Repository.
// It embeds BaseRepository[MmlExecuteResult, int] for standard CRUD on MmlExecuteResult,
// and retains module-specific methods for other entity types and custom queries.
type repository struct {
	*baserepo.BaseRepository[MmlExecuteResult, int]
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[MmlExecuteResult, int](db, "id"),
		db:             db,
	}
}

// ---------------------------------------------------------------------------
// MmlSet – module-specific queries (different entity type)
// ---------------------------------------------------------------------------

// FindMmlSets returns all MML sets for the given license.
func (r *repository) FindMmlSets(licenseId int) ([]MmlSet, error) {
	var sets []MmlSet
	if err := r.db.Where("license_id = ?", licenseId).Find(&sets).Error; err != nil {
		logger.Errorf("FindMmlSets error: %v", err)
		return nil, err
	}
	return sets, nil
}

// ---------------------------------------------------------------------------
// MmlCommand – module-specific queries (different entity type)
// ---------------------------------------------------------------------------

// FindMmlCommands returns all commands belonging to the given MML set.
func (r *repository) FindMmlCommands(setId int) ([]MmlCommand, error) {
	var cmds []MmlCommand
	if err := r.db.Where("mml_set_id = ?", setId).Find(&cmds).Error; err != nil {
		logger.Errorf("FindMmlCommands error: %v", err)
		return nil, err
	}
	return cmds, nil
}

// ---------------------------------------------------------------------------
// MmlCommandParam – module-specific queries (different entity type)
// ---------------------------------------------------------------------------

// FindMmlCommandParams returns all parameters for the given command.
func (r *repository) FindMmlCommandParams(commandId int) ([]MmlCommandParam, error) {
	var params []MmlCommandParam
	if err := r.db.Where("mml_command_id = ?", commandId).Find(&params).Error; err != nil {
		logger.Errorf("FindMmlCommandParams error: %v", err)
		return nil, err
	}
	return params, nil
}

// ---------------------------------------------------------------------------
// MmlExecuteResult – module-specific queries (uses base CRUD for standard ops)
// ---------------------------------------------------------------------------

// FindMmlExecuteResults returns a paginated list of results for the given element.
func (r *repository) FindMmlExecuteResults(elementId int64, offset, limit int) ([]MmlExecuteResult, int64, error) {
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
