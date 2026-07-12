package monitor

import (
	"time"

	"gorm.io/gorm"

	"nmsappsrv/pkg/apperror"
)

// Service defines the monitor business-logic contract.
type Service interface {
	ListMonitorTasks(licenseId int) ([]MonitorTask, error)
	GetMonitorTask(id int) (*MonitorTask, error)
	CreateMonitorTask(t *MonitorTask) error
	UpdateMonitorTask(t *MonitorTask) error
	DeleteMonitorTask(id int) error
	GetMonitorData(elementId int64, parameterId string, startTime, endTime string) ([]MonitorData, error)
	GetMonitorElements(taskId int) ([]MonitorElements, error)
	SaveMonitorElements(taskId int, elementIds []int64) error
	GetMonitorParameters(taskId int) ([]MonitorParameters, error)
	SaveMonitorParameters(taskId int, parameterIds []string) error
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a new monitor service.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// ---------- MonitorTask ----------

func (s *service) ListMonitorTasks(licenseId int) ([]MonitorTask, error) {
	return s.repo.FindMonitorTasks(licenseId)
}

func (s *service) GetMonitorTask(id int) (*MonitorTask, error) {
	return s.repo.FindByID(id)
}

func (s *service) CreateMonitorTask(t *MonitorTask) error {
	return s.repo.Create(t)
}

func (s *service) UpdateMonitorTask(t *MonitorTask) error {
	return s.repo.Save(t)
}

func (s *service) DeleteMonitorTask(id int) error {
	return s.repo.DeleteByID(id)
}

// ---------- MonitorData ----------

func (s *service) GetMonitorData(elementId int64, parameterId string, startTime, endTime string) ([]MonitorData, error) {
	st, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return nil, apperror.ErrInvalidInput.WithMessage("invalid start_time format, expected RFC3339")
	}
	et, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return nil, apperror.ErrInvalidInput.WithMessage("invalid end_time format, expected RFC3339")
	}
	return s.repo.FindMonitorData(elementId, parameterId, st, et)
}

// ---------- MonitorElements ----------

func (s *service) GetMonitorElements(taskId int) ([]MonitorElements, error) {
	return s.repo.FindMonitorElements(taskId)
}

func (s *service) SaveMonitorElements(taskId int, elementIds []int64) error {
	return s.repo.SaveMonitorElements(taskId, elementIds)
}

// ---------- MonitorParameters ----------

func (s *service) GetMonitorParameters(taskId int) ([]MonitorParameters, error) {
	return s.repo.FindMonitorParameters(taskId)
}

func (s *service) SaveMonitorParameters(taskId int, parameterIds []string) error {
	return s.repo.SaveMonitorParameters(taskId, parameterIds)
}
