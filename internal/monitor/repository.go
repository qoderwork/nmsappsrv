package monitor

import (
	"time"

	"gorm.io/gorm"
)

// Repository provides data access for monitor-related models.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new monitor repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ---------- MonitorTask ----------

func (r *Repository) FindMonitorTasks(licenseId int) ([]MonitorTask, error) {
	var items []MonitorTask
	err := r.db.Where("license_id = ?", licenseId).Find(&items).Error
	return items, err
}

func (r *Repository) FindMonitorTaskByID(id int) (*MonitorTask, error) {
	var item MonitorTask
	err := r.db.Where("id = ?", id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateMonitorTask(t *MonitorTask) error {
	return r.db.Create(t).Error
}

func (r *Repository) UpdateMonitorTask(t *MonitorTask) error {
	return r.db.Save(t).Error
}

func (r *Repository) DeleteMonitorTask(id int) error {
	return r.db.Where("id = ?", id).Delete(&MonitorTask{}).Error
}

// ---------- MonitorData ----------

func (r *Repository) FindMonitorData(elementId int64, parameterId string, startTime, endTime time.Time) ([]MonitorData, error) {
	var items []MonitorData
	err := r.db.Where("element_id = ? AND parameter_id = ? AND sample_time BETWEEN ? AND ?",
		elementId, parameterId, startTime, endTime).Find(&items).Error
	return items, err
}

// ---------- MonitorElements ----------

func (r *Repository) FindMonitorElements(taskId int) ([]MonitorElements, error) {
	var items []MonitorElements
	err := r.db.Where("task_id = ?", taskId).Find(&items).Error
	return items, err
}

func (r *Repository) SaveMonitorElements(taskId int, elementIds []int64) error {
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

func (r *Repository) FindMonitorParameters(taskId int) ([]MonitorParameters, error) {
	var items []MonitorParameters
	err := r.db.Where("task_id = ?", taskId).Find(&items).Error
	return items, err
}

func (r *Repository) SaveMonitorParameters(taskId int, parameterIds []string) error {
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
