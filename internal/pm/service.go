package pm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/redis"
)

// Service defines the business-logic contract for PM operations.
type Service interface {
	ListKPIs(tenancyId int) ([]PerformanceKpi, error)
	GetKPI(id string) (*PerformanceKpi, error)
	CreateKPI(k *PerformanceKpi) error
	UpdateKPI(k *PerformanceKpi) error
	DeleteKPI(id string) error
	ListKPISets(tenancyId int) ([]PerformanceKpiSet, error)
	CreateKPISet(set *PerformanceKpiSet) error
	DeleteKPISet(id int) error
	ListKPITemplates(tenancyId int) ([]PerformanceKpiTemplate, error)
	CreateKPITemplate(t *PerformanceKpiTemplate) error
	UpdateKPITemplate(t *PerformanceKpiTemplate) error
	DeleteKPITemplate(id int) error
	GetKPITemplate(id int) (*PerformanceKpiTemplate, error)
	// DownloadKPITemplate returns the template serialized as bytes plus a
	// suggested filename. The handler sets Content-Disposition and writes
	// the body. Mirrors Java downloadKPITemplate.
	DownloadKPITemplate(id int) ([]byte, string, error)
	ListPMFileLogs(tenancyId int, page, pageSize int) ([]PMFileLog, int64, error)
	ListKPIAlarmTemplates(tenancyId int) ([]KpiAlarmTemplate, error)
	CreateKPIAlarmTemplate(t *KpiAlarmTemplate) error
	UpdateKPIAlarmTemplate(t *KpiAlarmTemplate) error
	DeleteKPIAlarmTemplate(id int) error
	GetKPIAlarmTemplate(id int) (*KpiAlarmTemplate, error)
	// UpdateKPIAlarmTemplateStatus toggles the `enable` flag on a KPI alarm
	// template. Mirrors Java updateKPIAlarmTemplateStatus (the Java endpoint
	// is a status-only update distinct from the full update).
	UpdateKPIAlarmTemplateStatus(id int, enable bool) error
	GetDashboardData(tenancyId int, startTime, endTime string) ([]DashboardPmStatisticData, error)
	GetPDCPTraffic(tenancyId int, startTime, endTime string) ([]PDCPTraffic, error)
	GetDeviceOnlineInfo(tenancyId int) (*DeviceOnlineInfoVO, error)
	GetProductTypeAndDeviceCount(tenancyId int, mode string) ([]ProductTypeAndCount, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a new PM service.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// ---------- PerformanceKpi ----------

func (s *service) ListKPIs(tenancyId int) ([]PerformanceKpi, error) {
	data, err := s.repo.FindKPIs(tenancyId)
	if err != nil {
		return nil, apperror.Wrap(err, "LIST_KPIS_FAILED", 500, "failed to list KPIs")
	}
	return data, nil
}

func (s *service) GetKPI(id string) (*PerformanceKpi, error) {
	item, err := s.repo.FindByID(id)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_KPI_FAILED", 404, "KPI not found")
	}
	return item, nil
}

func (s *service) CreateKPI(k *PerformanceKpi) error {
	if err := s.repo.Create(k); err != nil {
		return apperror.Wrap(err, "CREATE_KPI_FAILED", 500, "failed to create KPI")
	}
	return nil
}

func (s *service) UpdateKPI(k *PerformanceKpi) error {
	if err := s.repo.Save(k); err != nil {
		return apperror.Wrap(err, "UPDATE_KPI_FAILED", 500, "failed to update KPI")
	}
	return nil
}

func (s *service) DeleteKPI(id string) error {
	if err := s.repo.DeleteByID(id); err != nil {
		return apperror.Wrap(err, "DELETE_KPI_FAILED", 500, "failed to delete KPI")
	}
	return nil
}

// ---------- PerformanceKpiSet ----------

func (s *service) ListKPISets(tenancyId int) ([]PerformanceKpiSet, error) {
	data, err := s.repo.FindKPISets(tenancyId)
	if err != nil {
		return nil, apperror.Wrap(err, "LIST_KPI_SETS_FAILED", 500, "failed to list KPI sets")
	}
	return data, nil
}

func (s *service) CreateKPISet(set *PerformanceKpiSet) error {
	if err := s.repo.CreateKPISet(set); err != nil {
		return apperror.Wrap(err, "CREATE_KPI_SET_FAILED", 500, "failed to create KPI set")
	}
	return nil
}

func (s *service) DeleteKPISet(id int) error {
	if err := s.repo.DeleteKPISet(id); err != nil {
		return apperror.Wrap(err, "DELETE_KPI_SET_FAILED", 500, "failed to delete KPI set")
	}
	return nil
}

// ---------- PerformanceKpiTemplate ----------

func (s *service) ListKPITemplates(tenancyId int) ([]PerformanceKpiTemplate, error) {
	data, err := s.repo.FindKPITemplates(tenancyId)
	if err != nil {
		return nil, apperror.Wrap(err, "LIST_KPI_TEMPLATES_FAILED", 500, "failed to list KPI templates")
	}
	return data, nil
}

func (s *service) GetKPITemplate(id int) (*PerformanceKpiTemplate, error) {
	data, err := s.repo.FindKPITemplate(id)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_KPI_TEMPLATE_FAILED", 500, "failed to get KPI template")
	}
	return data, nil
}

func (s *service) DownloadKPITemplate(id int) ([]byte, string, error) {
	tpl, err := s.repo.FindKPITemplate(id)
	if err != nil {
		return nil, "", apperror.Wrap(err, "DOWNLOAD_KPI_TEMPLATE_FAILED", 500, "failed to load KPI template")
	}
	body, err := json.MarshalIndent(tpl, "", "  ")
	if err != nil {
		return nil, "", apperror.Wrap(err, "MAR_KPI_TEMPLATE_FAILED", 500, "failed to marshal KPI template")
	}
	filename := fmt.Sprintf("kpi-template-%d.json", id)
	return body, filename, nil
}

func (s *service) CreateKPITemplate(t *PerformanceKpiTemplate) error {
	if err := s.repo.CreateKPITemplate(t); err != nil {
		return apperror.Wrap(err, "CREATE_KPI_TEMPLATE_FAILED", 500, "failed to create KPI template")
	}
	return nil
}

func (s *service) UpdateKPITemplate(t *PerformanceKpiTemplate) error {
	if err := s.repo.UpdateKPITemplate(t); err != nil {
		return apperror.Wrap(err, "UPDATE_KPI_TEMPLATE_FAILED", 500, "failed to update KPI template")
	}
	return nil
}

func (s *service) DeleteKPITemplate(id int) error {
	if err := s.repo.DeleteKPITemplate(id); err != nil {
		return apperror.Wrap(err, "DELETE_KPI_TEMPLATE_FAILED", 500, "failed to delete KPI template")
	}
	return nil
}

// ---------- PMFileLog ----------

func (s *service) ListPMFileLogs(tenancyId int, page, pageSize int) ([]PMFileLog, int64, error) {
	offset := (page - 1) * pageSize
	data, total, err := s.repo.FindPMFileLogs(tenancyId, offset, pageSize)
	if err != nil {
		return nil, 0, apperror.Wrap(err, "LIST_PM_FILE_LOGS_FAILED", 500, "failed to list PM file logs")
	}
	return data, total, nil
}

// ---------- KpiAlarmTemplate ----------

func (s *service) ListKPIAlarmTemplates(tenancyId int) ([]KpiAlarmTemplate, error) {
	data, err := s.repo.FindKPIAlarmTemplates(tenancyId)
	if err != nil {
		return nil, apperror.Wrap(err, "LIST_KPI_ALARM_TEMPLATES_FAILED", 500, "failed to list KPI alarm templates")
	}
	return data, nil
}

func (s *service) GetKPIAlarmTemplate(id int) (*KpiAlarmTemplate, error) {
	data, err := s.repo.FindKPIAlarmTemplate(id)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_KPI_ALARM_TEMPLATE_FAILED", 500, "failed to get KPI alarm template")
	}
	return data, nil
}

func (s *service) CreateKPIAlarmTemplate(t *KpiAlarmTemplate) error {
	if err := s.repo.CreateKPIAlarmTemplate(t); err != nil {
		return apperror.Wrap(err, "CREATE_KPI_ALARM_TEMPLATE_FAILED", 500, "failed to create KPI alarm template")
	}
	return nil
}

func (s *service) UpdateKPIAlarmTemplate(t *KpiAlarmTemplate) error {
	if err := s.repo.UpdateKPIAlarmTemplate(t); err != nil {
		return apperror.Wrap(err, "UPDATE_KPI_ALARM_TEMPLATE_FAILED", 500, "failed to update KPI alarm template")
	}
	return nil
}

func (s *service) UpdateKPIAlarmTemplateStatus(id int, enable bool) error {
	if err := s.repo.UpdateKPIAlarmTemplateStatus(id, enable); err != nil {
		return apperror.Wrap(err, "UPDATE_KPI_ALARM_TEMPLATE_STATUS_FAILED", 500, "failed to update KPI alarm template status")
	}
	return nil
}

func (s *service) DeleteKPIAlarmTemplate(id int) error {
	if err := s.repo.DeleteKPIAlarmTemplate(id); err != nil {
		return apperror.Wrap(err, "DELETE_KPI_ALARM_TEMPLATE_FAILED", 500, "failed to delete KPI alarm template")
	}
	return nil
}

// ---------- Dashboard ----------

func (s *service) GetDashboardData(tenancyId int, startTime, endTime string) ([]DashboardPmStatisticData, error) {
	st, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return nil, apperror.ErrInvalidInput.WithMessage("invalid start_time format, expected RFC3339")
	}
	et, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return nil, apperror.ErrInvalidInput.WithMessage("invalid end_time format, expected RFC3339")
	}
	data, err := s.repo.FindDashboardData(tenancyId, st, et)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_DASHBOARD_DATA_FAILED", 500, "failed to get dashboard data")
	}
	return data, nil
}

// ---------- PDCPTraffic ----------

func (s *service) GetPDCPTraffic(tenancyId int, startTime, endTime string) ([]PDCPTraffic, error) {
	st, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return nil, apperror.ErrInvalidInput.WithMessage("invalid start_time format, expected RFC3339")
	}
	et, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return nil, apperror.ErrInvalidInput.WithMessage("invalid end_time format, expected RFC3339")
	}
	data, err := s.repo.FindPDCPTraffic(tenancyId, st, et)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_PDPC_TRAFFIC_FAILED", 500, "failed to get PDPC traffic data")
	}
	return data, nil
}

// ---------- Dashboard: Device Online Info ----------

// GetDeviceOnlineInfo 统计 gNB/eNB/CPE 各自在线/离线设备数
func (s *service) GetDeviceOnlineInfo(tenancyId int) (*DeviceOnlineInfoVO, error) {
	rows, err := s.repo.FindAllActiveElements(tenancyId)
	if err != nil {
		return nil, apperror.Wrap(err, "GET_DEVICE_ONLINE_INFO_FAILED", 500, "failed to get device online info")
	}

	ctx := context.Background()
	vo := &DeviceOnlineInfoVO{}

	for _, row := range rows {
		online := redis.Exists(ctx, fmt.Sprintf("online_%d", row.NeNeid))
		dt := strVal(row.DeviceType)
		gen := strVal(row.Generation)

		switch {
		case dt == "enb" && gen == "NR":
			vo.GnbTotal++
			if online {
				vo.GnbOnline++
			} else {
				vo.GnbOffline++
			}
		case dt == "enb":
			vo.EnbTotal++
			if online {
				vo.EnbOnline++
			} else {
				vo.EnbOffline++
			}
		default:
			vo.CpeTotal++
			if online {
				vo.CpeOnline++
			} else {
				vo.CpeOffline++
			}
		}
	}
	return vo, nil
}

// GetProductTypeAndDeviceCount 按产品型号统计设备数量及在线情况
// mode: "all" 查全部租户, 否则按 tenancyId 过滤
func (s *service) GetProductTypeAndDeviceCount(tenancyId int, mode string) ([]ProductTypeAndCount, error) {
	var rows []elementRow
	var err error
	if mode == "all" {
		rows, err = s.repo.FindAllActiveElementsAllTenants()
	} else {
		rows, err = s.repo.FindAllActiveElements(tenancyId)
	}
	if err != nil {
		return nil, apperror.Wrap(err, "GET_PRODUCT_TYPE_COUNT_FAILED", 500, "failed to get product type and device count")
	}

	ctx := context.Background()
	// group by model_name
	type agg struct {
		count        int64
		onlineCount  int64
	}
	grouped := make(map[string]*agg)

	for _, row := range rows {
		modelName := strVal(row.ModelName)
		if modelName == "" {
			modelName = "Unknown"
		}
		a, ok := grouped[modelName]
		if !ok {
			a = &agg{}
			grouped[modelName] = a
		}
		a.count++
		if redis.Exists(ctx, fmt.Sprintf("online_%d", row.NeNeid)) {
			a.onlineCount++
		}
	}

	result := make([]ProductTypeAndCount, 0, len(grouped))
	for pt, a := range grouped {
		result = append(result, ProductTypeAndCount{
			ProductType:  pt,
			Count:        a.count,
			OnlineCount:  a.onlineCount,
			OfflineCount: a.count - a.onlineCount,
		})
	}
	return result, nil
}

// strVal safely dereference a *string, returning "" for nil.
func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// newService creates a Service backed by the given Repository (test/mock helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}
