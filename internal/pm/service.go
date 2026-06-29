package pm

import (
	"time"

	"gorm.io/gorm"
)

// Service contains PM business logic.
type Service struct {
	repo *Repository
}

// NewService creates a new PM service.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ---------- PerformanceKpi ----------

func (s *Service) ListKPIs(tenancyId int) ([]PerformanceKpi, error) {
	return s.repo.FindKPIs(tenancyId)
}

func (s *Service) GetKPI(id string) (*PerformanceKpi, error) {
	return s.repo.FindKPIByID(id)
}

func (s *Service) CreateKPI(k *PerformanceKpi) error {
	return s.repo.CreateKPI(k)
}

func (s *Service) UpdateKPI(k *PerformanceKpi) error {
	return s.repo.UpdateKPI(k)
}

func (s *Service) DeleteKPI(id string) error {
	return s.repo.DeleteKPI(id)
}

// ---------- PerformanceKpiSet ----------

func (s *Service) ListKPISets(tenancyId int) ([]PerformanceKpiSet, error) {
	return s.repo.FindKPISets(tenancyId)
}

func (s *Service) CreateKPISet(set *PerformanceKpiSet) error {
	return s.repo.CreateKPISet(set)
}

// ---------- PerformanceKpiTemplate ----------

func (s *Service) ListKPITemplates(tenancyId int) ([]PerformanceKpiTemplate, error) {
	return s.repo.FindKPITemplates(tenancyId)
}

func (s *Service) CreateKPITemplate(t *PerformanceKpiTemplate) error {
	return s.repo.CreateKPITemplate(t)
}

func (s *Service) UpdateKPITemplate(t *PerformanceKpiTemplate) error {
	return s.repo.UpdateKPITemplate(t)
}

func (s *Service) DeleteKPITemplate(id int) error {
	return s.repo.DeleteKPITemplate(id)
}

// ---------- PMFileLog ----------

func (s *Service) ListPMFileLogs(tenancyId int, page, pageSize int) ([]PMFileLog, int64, error) {
	offset := (page - 1) * pageSize
	return s.repo.FindPMFileLogs(tenancyId, offset, pageSize)
}

// ---------- KpiAlarmTemplate ----------

func (s *Service) ListKPIAlarmTemplates(tenancyId int) ([]KpiAlarmTemplate, error) {
	return s.repo.FindKPIAlarmTemplates(tenancyId)
}

func (s *Service) CreateKPIAlarmTemplate(t *KpiAlarmTemplate) error {
	return s.repo.CreateKPIAlarmTemplate(t)
}

func (s *Service) UpdateKPIAlarmTemplate(t *KpiAlarmTemplate) error {
	return s.repo.UpdateKPIAlarmTemplate(t)
}

func (s *Service) DeleteKPIAlarmTemplate(id int) error {
	return s.repo.DeleteKPIAlarmTemplate(id)
}

// ---------- Dashboard ----------

func (s *Service) GetDashboardData(tenancyId int, startTime, endTime string) ([]DashboardPmStatisticData, error) {
	st, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return nil, err
	}
	et, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return nil, err
	}
	return s.repo.FindDashboardData(tenancyId, st, et)
}

// ---------- PDCPTraffic ----------

func (s *Service) GetPDCPTraffic(tenancyId int, startTime, endTime string) ([]PDCPTraffic, error) {
	st, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return nil, err
	}
	et, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return nil, err
	}
	return s.repo.FindPDCPTraffic(tenancyId, st, et)
}
