package monitor

import (
	"time"

	"gorm.io/gorm"

	"nmsappsrv/pkg/baserepo"
)

// Repository defines the data-access contract for monitor-related models.
// It embeds BaseRepository[MonitorTask, int] for standard CRUD on MonitorTask,
// and retains module-specific methods for custom queries and other entity types.
type Repository interface {
	Create(entity *MonitorTask) error
	Save(entity *MonitorTask) error
	FindByID(id int) (*MonitorTask, error)
	DeleteByID(id int) error
	DeleteByIDs(ids []int) error
	SoftDelete(id int) error
	UpdateFields(id int, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]MonitorTask, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[MonitorTask], error)
	FindMonitorTasks(licenseId int) ([]MonitorTask, error)
	FindMonitorData(elementId int64, parameterId string, startTime, endTime time.Time) ([]MonitorData, error)
	FindMonitorElements(taskId int) ([]MonitorElements, error)
	SaveMonitorElements(taskId int, elementIds []int64) error
	FindMonitorParameters(taskId int) ([]MonitorParameters, error)
	SaveMonitorParameters(taskId int, parameterIds []string) error
}

// repository is the concrete GORM-backed implementation of Repository.
// It embeds BaseRepository[MonitorTask, int] for standard CRUD on MonitorTask,
// and retains module-specific methods for custom queries and other entity types.
type repository struct {
	*baserepo.BaseRepository[MonitorTask, int]
	db *gorm.DB
}

// NewRepository creates a new monitor repository.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[MonitorTask, int](db, "id"),
		db:             db,
	}
}

// ---------- MonitorTask ----------

func (r *repository) FindMonitorTasks(licenseId int) ([]MonitorTask, error) {
	var items []MonitorTask
	err := r.db.Where("license_id = ?", licenseId).Find(&items).Error
	return items, err
}

// ---------- MonitorData ----------

func (r *repository) FindMonitorData(elementId int64, parameterId string, startTime, endTime time.Time) ([]MonitorData, error) {
	var items []MonitorData
	err := r.db.Where("element_id = ? AND parameter_id = ? AND sample_time BETWEEN ? AND ?",
		elementId, parameterId, startTime, endTime).Find(&items).Error
	return items, err
}

// ---------- MonitorElements ----------

func (r *repository) FindMonitorElements(taskId int) ([]MonitorElements, error) {
	var items []MonitorElements
	err := r.db.Where("task_id = ?", taskId).Find(&items).Error
	return items, err
}

func (r *repository) SaveMonitorElements(taskId int, elementIds []int64) error {
	// Delete existing elements for this task, then batch insert.
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ?", taskId).Delete(&MonitorElements{}).Error; err != nil {
			return err
		}
		for _, eid := range elementIds {
			elem := MonitorElements{
				ElementId: &eid,
				TaskId:    &taskId,
			}
			if err := tx.Create(&elem).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ---------- MonitorParameters ----------

func (r *repository) FindMonitorParameters(taskId int) ([]MonitorParameters, error) {
	var items []MonitorParameters
	err := r.db.Where("task_id = ?", taskId).Find(&items).Error
	return items, err
}

func (r *repository) SaveMonitorParameters(taskId int, parameterIds []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ?", taskId).Delete(&MonitorParameters{}).Error; err != nil {
			return err
		}
		for _, pid := range parameterIds {
			param := MonitorParameters{
				ParameterId: &pid,
				TaskId:      &taskId,
			}
			if err := tx.Create(&param).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
