package monitor

import (
	"time"

	"gorm.io/gorm"
)

// Service contains monitor business logic.
type Service struct {
	repo *Repository
}

// NewService creates a new monitor service.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ---------- MonitorTask ----------

func (s *Service) ListMonitorTasks(licenseId int) ([]MonitorTask, error) {
	return s.repo.FindMonitorTasks(licenseId)
}

func (s *Service) GetMonitorTask(id int) (*MonitorTask, error) {
	return s.repo.FindMonitorTaskByID(id)
}

func (s *Service) CreateMonitorTask(t *MonitorTask) error {
	return s.repo.CreateMonitorTask(t)
}

func (s *Service) UpdateMonitorTask(t *MonitorTask) error {
	return s.repo.UpdateMonitorTask(t)
}

func (s *Service) DeleteMonitorTask(id int) error {
	return s.repo.DeleteMonitorTask(id)
}

// ---------- MonitorData ----------

func (s *Service) GetMonitorData(elementId int64, parameterId string, startTime, endTime string) ([]MonitorData, error) {
	st, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return nil, err
	}
	et, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return nil, err
	}
	return s.repo.FindMonitorData(elementId, parameterId, st, et)
}

// ---------- MonitorElements ----------

func (s *Service) GetMonitorElements(taskId int) ([]MonitorElements, error) {
	return s.repo.FindMonitorElements(taskId)
}

func (s *Service) SaveMonitorElements(taskId int, elementIds []int64) error {
	return s.repo.SaveMonitorElements(taskId, elementIds)
}

// ---------- MonitorParameters ----------

func (s *Service) GetMonitorParameters(taskId int) ([]MonitorParameters, error) {
	return s.repo.FindMonitorParameters(taskId)
}

func (s *Service) SaveMonitorParameters(taskId int, parameterIds []string) error {
	return s.repo.SaveMonitorParameters(taskId, parameterIds)
}
