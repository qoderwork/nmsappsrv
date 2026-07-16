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
	FindMmlCommandByCommand(command string) (*MmlCommand, error)
	FindMmlExecuteResults(elementId int64, offset, limit int) ([]MmlExecuteResult, int64, error)
	FindMmlExecuteResultsByEventLogIds(eventLogIds []int64) ([]MmlExecuteResult, error)
	FindDeviceNameSerialByElementIds(elementIds []int64) ([]DeviceNameSerial, error)
	CreateMmlSet(set *MmlSet) error
	CreateMmlCommand(cmd *MmlCommand) error
	CreateMmlCommandParam(p *MmlCommandParam) error
	FindMmlSetsByVersionAndLicense(version string, licenseId int) ([]MmlSet, error)
	FindTopMmlSets(version string, licenseId int) ([]MmlSet, error)
	FindChildMmlSets(parentId int) ([]MmlSet, error)
	FindMmlSetByParentIdAndName(parentId *int, name string, licenseId int) ([]MmlSet, error)
	FindMmlCommandsBySetIds(ids []int) ([]MmlCommand, error)
	FindMmlCommandParamsByCommandIds(ids []int) ([]MmlCommandParam, error)
	FindMmlVersions(licenseId int) ([]string, error)
	DeleteMmlSetsByIds(ids []int) error
	DeleteMmlCommandsByIds(ids []int) error
	DeleteMmlCommandParamsByIds(ids []int) error
	FindBatchProcessFiles(licenseId int) ([]BatchProcessFile, error)
	FindBatchProcessFileByID(id int) (*BatchProcessFile, error)
	CreateBatchProcessFile(file *BatchProcessFile) error
	UpdateBatchProcessFile(file *BatchProcessFile) error
	DeleteBatchProcessFile(id int) error
	FindBatchProcessLogs(batchFileId int) ([]BatchProcessLog, error)
	CreateBatchProcessLog(log *BatchProcessLog) error
	FindBatchProcessExecuteResults(batchFileId int) ([]MmlExecuteResult, error)
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

// FindMmlCommandByCommand resolves a command definition by its command name, used
// to derive the MML command category (type) for the Device.mml.<type>.CMD downlink path.
func (r *repository) FindMmlCommandByCommand(command string) (*MmlCommand, error) {
	var cmd MmlCommand
	if err := r.db.Where("command = ?", command).First(&cmd).Error; err != nil {
		return nil, err
	}
	return &cmd, nil
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

// DeviceNameSerial is a query-row type holding the device identity columns for a
// given element id, used to enrich MML result views (对齐 Java neElementService.getById).
type DeviceNameSerial struct {
	NeNeid       int64  `gorm:"column:ne_neid"`
	DeviceName   string `gorm:"column:device_name"`
	SerialNumber string `gorm:"column:serial_number"`
}

// FindMmlExecuteResultsByEventLogIds returns all MML execution results whose
// event_log_id is in the given list (对齐 Java MmlExecuteResultRepo.findAllByEventLogIdIn).
func (r *repository) FindMmlExecuteResultsByEventLogIds(eventLogIds []int64) ([]MmlExecuteResult, error) {
	if len(eventLogIds) == 0 {
		return []MmlExecuteResult{}, nil
	}
	var results []MmlExecuteResult
	if err := r.db.Where("event_log_id IN ?", eventLogIds).Find(&results).Error; err != nil {
		logger.Errorf("FindMmlExecuteResultsByEventLogIds error: %v", err)
		return nil, err
	}
	return results, nil
}

// FindDeviceNameSerialByElementIds returns device_name + serial_number for the
// given element ids from cpe_element (ne_neid is the element id).
func (r *repository) FindDeviceNameSerialByElementIds(elementIds []int64) ([]DeviceNameSerial, error) {
	if len(elementIds) == 0 {
		return []DeviceNameSerial{}, nil
	}
	var rows []DeviceNameSerial
	if err := r.db.Table("cpe_element").
		Select("ne_neid, device_name, serial_number").
		Where("ne_neid IN ?", elementIds).
		Find(&rows).Error; err != nil {
		logger.Errorf("FindDeviceNameSerialByElementIds error: %v", err)
		return nil, err
	}
	return rows, nil
}
