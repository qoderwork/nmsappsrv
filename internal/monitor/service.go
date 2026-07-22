package monitor

import (
	"time"

	"gorm.io/gorm"

	"nmsappsrv/pkg/apperror"
)

// MonitorStatPoint is a single monitor_data sample in a statistics series.
type MonitorStatPoint struct {
	ElementId  int64     `json:"element_id"`
	SampleTime time.Time `json:"sample_time"`
	Value      float64   `json:"value"`
}

// MonitorStatistics is the aggregated time-series for a parameter over a window.
type MonitorStatistics struct {
	ParameterId string            `json:"parameter_id"`
	Points      []MonitorStatPoint `json:"points"`
	Avg         float64           `json:"avg"`
	Min         float64           `json:"min"`
	Max         float64           `json:"max"`
	Count       int               `json:"count"`
}

// Service defines the monitor business-logic contract.
type Service interface {
	ListMonitorTasks(tenantId int) ([]MonitorTask, error)
	GetMonitorTask(id int) (*MonitorTask, error)
	CreateMonitorTask(t *MonitorTask) error
	UpdateMonitorTask(t *MonitorTask) error
	DeleteMonitorTask(id int) error
	GetMonitorData(elementId int64, parameterId string, startTime, endTime string) ([]MonitorData, error)
	GetMonitorStatistics(parameterId string, elementIds []int64, startTime, endTime string) (*MonitorStatistics, error)
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

func (s *service) ListMonitorTasks(tenantId int) ([]MonitorTask, error) {
	return s.repo.FindMonitorTasks(tenantId)
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

// GetMonitorStatistics returns the aggregated time-series for a parameter across the
// given elements and time window (mirrors Java getMonitorStatistics).
func (s *service) GetMonitorStatistics(parameterId string, elementIds []int64, startTime, endTime string) (*MonitorStatistics, error) {
	st, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return nil, apperror.ErrInvalidInput.WithMessage("invalid start_time format, expected RFC3339")
	}
	et, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return nil, apperror.ErrInvalidInput.WithMessage("invalid end_time format, expected RFC3339")
	}
	rows, err := s.repo.FindMonitorDataSeries(parameterId, elementIds, st, et)
	if err != nil {
		return nil, err
	}
	stat := &MonitorStatistics{ParameterId: parameterId, Points: make([]MonitorStatPoint, 0, len(rows))}
	if len(rows) == 0 {
		return stat, nil
	}
	var sum float64
	stat.Min = *rows[0].Value
	stat.Max = *rows[0].Value
	for _, r := range rows {
		v := *r.Value
		stat.Points = append(stat.Points, MonitorStatPoint{
			ElementId:  *r.ElementId,
			SampleTime: *r.SampleTime,
			Value:      v,
		})
		sum += v
		if v < stat.Min {
			stat.Min = v
		}
		if v > stat.Max {
			stat.Max = v
		}
	}
	stat.Count = len(rows)
	stat.Avg = sum / float64(stat.Count)
	return stat, nil
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
