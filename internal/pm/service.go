package pm

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"nmsappsrv/internal/config"
	"nmsappsrv/internal/mq"
	"nmsappsrv/internal/opmsg"
	"nmsappsrv/internal/tr069/soap"
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
	// ListAllKPIs returns every KPI across all tenancies (admin-style lookup).
	// Mirrors Java listAllKPI. Use with care -- no tenancy scoping.
	ListAllKPIs() ([]PerformanceKpi, error)
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
	// ExportPMExcel serialises the dashboard PM data for the license in the
	// given date range as an xlsx workbook. Mirrors Java exportPMExcel
	// (Java uses Apache POI; Go uses excelize). Enforces the Java 7-day cap.
	ExportPMExcel(tenancyId int, startTime, endTime string) ([]byte, string, error)
	// ImportKPIsFromXLSX parses an uploaded xlsx workbook (mirror of Java
	// importKPI) and bulk-inserts the rows. `version` is the KPI-set version
	// (mirrors Java's "version" form field); used to set Type / update time
	// markers but does not gate the import.
	ImportKPIsFromXLSX(data []byte, version string) (int, error)
	// DownloadPMFile serves the newest pm_file_log file for the device in
	// the given time range. Mirrors Java downloadPMFile. The file is read
	// from the configured file_server.pm_dir using the row's file_name
	// (the Go pm_file_log has no file_path column; the Java collector that
	// writes files under the configured root is not ported yet, so this
	// endpoint will 404 until a producer exists).
	DownloadPMFile(elementId int64, startTime, endTime string) ([]byte, string, error)
	// ListKPIMeas returns paginated eNB devices for the license. Mirrors
	// Java listKPIMeas (which queries NeElement where device_type='enb').
	// The Go side has no separate meas_task table -- the meas task is a
	// per-device TR-069 FAP.PerfMgmt.Config.1.* parameter block.
	ListKPIMeas(tenancyId int, searchText string, page, pageSize int) ([]MeasDeviceVo, int64, error)
	// UpdateMeasTaskSwitch sends a SetParameterValues to the device to
	// enable/disable the TR-069 FAP.PerfMgmt.Config.1.* measurement block.
	// Mirrors Java updateMeasTaskSwitch. The SPV is dispatched via the
	// unified operation queue (mq.OperationQueue -> internal/operation).
	UpdateMeasTaskSwitch(elementId int64, enable bool, username string) error
	// AddReplenishTask persists a new replenish task. Mirrors Java
	// addReplenishTask. ElementIds is stored as a comma-separated string.
	AddReplenishTask(t *PMReplenishTask) error
	// ListReplenishTask returns paginated replenish tasks for the license.
	// Mirrors Java listReplenishTask.
	ListReplenishTask(tenancyId int, name string, page, pageSize int) ([]PMReplenishTask, int64, error)
	// ViewReplenishTask returns a single replenish task. Mirrors Java
	// viewReplenishTask.
	ViewReplenishTask(id int) (*PMReplenishTask, error)
	// ListDeviceReplenish returns the cpe_element rows listed in a
	// replenish task's element_ids. Mirrors Java listDeviceReplenish.
	ListDeviceReplenish(taskId int) ([]ReplenishDeviceVo, error)
	// MarkReplenishDeviceDone / IsReplenishDeviceDone are the in-memory
	// per-(task,device) Done flags driven by the replenish worker.
	// Exported so the worker (same package) can call them.
	MarkReplenishDeviceDone(taskId int, elementId int64)
	IsReplenishDeviceDone(taskId int, elementId int64) bool
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
	db   *gorm.DB

	// replenishDone tracks per-(taskId,elementId) Done state for the
	// replenish worker. The Java side persists this in a DB column; the
	// Go side uses an in-memory map as a placeholder -- a process
	// restart resets all states to Done=false. swap to a DB column
	// when the pm_replenish_task_device join table lands.
	replenishDoneMu sync.RWMutex
	replenishDone   map[string]bool
}

// NewService creates a new PM service.
func NewService(db *gorm.DB) Service {
	return &service{
		repo:          NewRepository(db),
		db:            db,
		replenishDone: make(map[string]bool),
	}
}

// replenishDoneKey returns the map key for (taskId, elementId).
func replenishDoneKey(taskId int, elementId int64) string {
	return strconv.FormatInt(int64(taskId), 10) + ":" + strconv.FormatInt(elementId, 10)
}

// MarkReplenishDeviceDone records that the replenish worker has
// finished "replenishing" a device in a task. Process-local.
func (s *service) MarkReplenishDeviceDone(taskId int, elementId int64) {
	s.replenishDoneMu.Lock()
	s.replenishDone[replenishDoneKey(taskId, elementId)] = true
	s.replenishDoneMu.Unlock()
}

// IsReplenishDeviceDone returns the in-memory Done state for one device.
func (s *service) IsReplenishDeviceDone(taskId int, elementId int64) bool {
	s.replenishDoneMu.RLock()
	defer s.replenishDoneMu.RUnlock()
	return s.replenishDone[replenishDoneKey(taskId, elementId)]
}

// ---------- PerformanceKpi ----------

func (s *service) ListKPIs(tenancyId int) ([]PerformanceKpi, error) {
	data, err := s.repo.FindKPIs(tenancyId)
	if err != nil {
		return nil, apperror.Wrap(err, "LIST_KPIS_FAILED", 500, "failed to list KPIs")
	}
	return data, nil
}

func (s *service) ListAllKPIs() ([]PerformanceKpi, error) {
	data, err := s.repo.FindAllKPIs()
	if err != nil {
		return nil, apperror.Wrap(err, "LIST_ALL_KPIS_FAILED", 500, "failed to list all KPIs")
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

// ExportPMExcel serialises the dashboard PM data for the license in the
// given date range as an xlsx workbook. Mirrors Java exportPMExcel
// (Java uses Apache POI; Go uses excelize). Enforces the Java 7-day cap.
func (s *service) ExportPMExcel(tenancyId int, startTime, endTime string) ([]byte, string, error) {
	st, et, err := parseTimeRange(startTime, endTime)
	if err != nil {
		return nil, "", apperror.Wrap(err, "EXPORT_PM_EXCEL_BAD_RANGE", 400, "invalid time range")
	}
	if et.Sub(st) > 7*24*time.Hour {
		return nil, "", apperror.New("EXPORT_PM_EXCEL_RANGE_TOO_LARGE", 400, "time range must be <= 7 days")
	}
	rows, err := s.repo.FindDashboardData(tenancyId, st, et)
	if err != nil {
		return nil, "", apperror.Wrap(err, "EXPORT_PM_EXCEL_FAILED", 500, "failed to load dashboard data")
	}
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())
	header := []string{"id", "time", "pdcp_ul_rate", "pdcp_dl_rate", "cell_available_rate", "tenancy_id"}
	for i, h := range header {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	for rowIdx, r := range rows {
		var timeStr string
		if r.Time != nil {
			timeStr = r.Time.Format(time.RFC3339)
		}
		values := []interface{}{
			r.Id,
			timeStr,
			optFloat(r.PdcpUlRate),
			optFloat(r.PdcpDlRate),
			optFloat(r.CellAvailableRate),
			optInt(r.TenancyId),
		}
		for colIdx, v := range values {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", apperror.Wrap(err, "EXPORT_PM_EXCEL_WRITE_FAILED", 500, "failed to write xlsx")
	}
	filename := fmt.Sprintf("pm-export-%s.xlsx", time.Now().Format("20060102-150405"))
	return buf.Bytes(), filename, nil
}

// ImportKPIsFromXLSX parses an uploaded xlsx workbook (mirror of Java
// importKPI, which uses Apache POI) and bulk-inserts the rows.
// Expected xlsx layout (header row):
//
//	id | kpi_name | kpi_name_translation | kpi | unit | unit_translation |
//	statistic_type | description | description_translation |
//	trigger_point | trigger_point_translation | kpi_set_id | id_formula |
//	type | default_kpi
//
// `version` is the KPI-set version marker (Java's form field); it is
// stored in UpdateUser so the source batch is traceable.
func (s *service) ImportKPIsFromXLSX(data []byte, version string) (int, error) {
	if len(data) == 0 {
		return 0, apperror.New("IMPORT_KPIS_EMPTY_FILE", 400, "empty xlsx file")
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return 0, apperror.Wrap(err, "IMPORT_KPIS_OPEN_FAILED", 400, "failed to open xlsx")
	}
	defer f.Close()
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return 0, apperror.Wrap(err, "IMPORT_KPIS_READ_FAILED", 500, "failed to read sheet")
	}
	if len(rows) < 2 {
		return 0, apperror.New("IMPORT_KPIS_NO_DATA", 400, "xlsx has no data rows")
	}
	// Map header column -> index. Header row is row[0].
	colIdx := make(map[string]int, 16)
	for i, h := range rows[0] {
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}
	now := time.Now()
	out := make([]PerformanceKpi, 0, len(rows)-1)
	for _, r := range rows[1:] {
		if len(r) == 0 {
			continue
		}
		kpi := PerformanceKpi{
			Id:         generateKpiID(),
			UpdateTime: &now,
			UpdateUser: strPtr(version),
		}
		kpi.KpiName = cellString(r, colIdx, "kpi_name")
		kpi.KpiNameTranslation = cellString(r, colIdx, "kpi_name_translation")
		kpi.Kpi = cellString(r, colIdx, "kpi")
		kpi.Unit = cellString(r, colIdx, "unit")
		kpi.UnitTranslation = cellString(r, colIdx, "unit_translation")
		kpi.StatisticType = cellString(r, colIdx, "statistic_type")
		kpi.Description = cellString(r, colIdx, "description")
		kpi.DescriptionTranslation = cellString(r, colIdx, "description_translation")
		kpi.TriggerPoint = cellString(r, colIdx, "trigger_point")
		kpi.TriggerPointTranslation = cellString(r, colIdx, "trigger_point_translation")
		kpi.IdFormula = cellString(r, colIdx, "id_formula")
		if v, ok := cellInt(r, colIdx, "type"); ok {
			kpi.Type = &v
		}
		if v, ok := cellInt(r, colIdx, "kpi_set_id"); ok {
			kpi.KpiSetId = &v
		}
		if v, ok := cellBool(r, colIdx, "default_kpi"); ok {
			kpi.DefaultKpi = &v
		}
		out = append(out, kpi)
	}
	if len(out) == 0 {
		return 0, nil
	}
	if err := s.repo.BulkCreateKPIs(out); err != nil {
		return 0, apperror.Wrap(err, "IMPORT_KPIS_FAILED", 500, "failed to import KPIs")
	}
	return len(out), nil
}

// cellString returns a pointer to the trimmed cell value at the given
// column, or nil if the column is missing / cell is empty.
func cellString(row []string, colIdx map[string]int, col string) *string {
	i, ok := colIdx[col]
	if !ok || i >= len(row) {
		return nil
	}
	v := strings.TrimSpace(row[i])
	if v == "" {
		return nil
	}
	return &v
}

// cellInt parses the cell as an int; ok=false on missing/invalid.
func cellInt(row []string, colIdx map[string]int, col string) (int, bool) {
	s := cellString(row, colIdx, col)
	if s == nil {
		return 0, false
	}
	v, err := strconv.Atoi(*s)
	if err != nil {
		return 0, false
	}
	return v, true
}

// cellBool parses the cell as a bool (true/1/yes/y -> true).
func cellBool(row []string, colIdx map[string]int, col string) (bool, bool) {
	s := cellString(row, colIdx, col)
	if s == nil {
		return false, false
	}
	switch strings.ToLower(*s) {
	case "true", "1", "yes", "y":
		return true, true
	case "false", "0", "no", "n":
		return false, true
	}
	return false, false
}

// generateKpiID returns a short random hex id (16 chars) for new KPI rows.
// Mirrors the Java UUID-style id.
func generateKpiID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func strPtr(s string) *string { return &s }

// DownloadPMFile serves the newest pm_file_log file for the device in
// the given time range. Mirrors Java downloadPMFile. File path is
// constructed from config.FileServer.PmDir + row.FileName (the Go
// pm_file_log has no file_path column).
func (s *service) DownloadPMFile(elementId int64, startTime, endTime string) ([]byte, string, error) {
	st, et, err := parseTimeRange(startTime, endTime)
	if err != nil {
		return nil, "", apperror.Wrap(err, "DOWNLOAD_PM_FILE_BAD_RANGE", 400, "invalid time range")
	}
	rows, err := s.repo.FindPMFileLogsInRange(elementId, st, et)
	if err != nil {
		return nil, "", apperror.Wrap(err, "DOWNLOAD_PM_FILE_FAILED", 500, "failed to load PM file log")
	}
	if len(rows) == 0 {
		return nil, "", apperror.New("DOWNLOAD_PM_FILE_NOT_FOUND", 404, "no PM file in range")
	}
	row := rows[0]
	if row.FileName == nil || *row.FileName == "" {
		return nil, "", apperror.New("DOWNLOAD_PM_FILE_NO_FILENAME", 500, "PM file log has no filename")
	}
	pmDir := ""
	if config.Cfg != nil {
		pmDir = config.Cfg.FileServer.PmDir
	}
	if pmDir == "" {
		return nil, "", apperror.New("DOWNLOAD_PM_FILE_NO_DIR", 500, "file_server.pm_dir not configured")
	}
	path := filepath.Join(pmDir, *row.FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", apperror.Wrap(err, "DOWNLOAD_PM_FILE_READ_FAILED", 500, fmt.Sprintf("read %s: %v", path, err))
	}
	return data, *row.FileName, nil
}

// parseTimeRange parses two RFC3339 strings and returns them as time.Time.
// Empty strings are rejected (the caller can default to "now" if needed).
func parseTimeRange(startTime, endTime string) (time.Time, time.Time, error) {
	if startTime == "" || endTime == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("startTime and endTime are required (RFC3339)")
	}
	st, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("startTime: %w", err)
	}
	et, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("endTime: %w", err)
	}
	return st, et, nil
}

// optFloat formats a *float64 as a string (empty if nil).
func optFloat(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}

// optInt formats a *int as a string (empty if nil).
func optInt(p *int) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(*p)
}

// ListKPIMeas returns paginated eNB devices for the license, optionally
// filtered by a name/serial search text. Mirrors Java listKPIMeas
// (queries NeElement where device_type='enb').
func (s *service) ListKPIMeas(tenancyId int, searchText string, page, pageSize int) ([]MeasDeviceVo, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	items, total, err := s.repo.FindENBDevicesForMeas(tenancyId, searchText, offset, pageSize)
	if err != nil {
		return nil, 0, apperror.Wrap(err, "LIST_KPI_MEAS_FAILED", 500, "failed to list KPI meas devices")
	}
	return items, total, nil
}

// measSwitchParamName returns the full TR-069 path for the FAP PerfMgmt
// Enable flag given a device's root node. Root nodes seen in the wild:
// "Device" (BBU/TR-181) and "InternetGatewayDevice" (legacy).
func measSwitchParamName(rootNode *string) string {
	rn := "Device"
	if rootNode != nil && *rootNode != "" {
		rn = *rootNode
	}
	return rn + ".FAP.PerfMgmt.Config.1.Enable"
}

// UpdateMeasTaskSwitch sends a SetParameterValues to the device to flip
// the FAP.PerfMgmt.Config.1.Enable flag. Mirrors Java updateMeasTaskSwitch.
// Dispatched via mq.OperationQueue -> internal/operation worker ->
// tr069.OperationSender.SendSetParameterValues.
func (s *service) UpdateMeasTaskSwitch(elementId int64, enable bool, username string) error {
	// Look up the device to get its root node and serial number.
	type deviceRow struct {
		SerialNumber *string `gorm:"column:serial_number"`
		RootNode     *string `gorm:"column:root_node"`
	}
	var row deviceRow
	if err := s.db.Table("cpe_element").
		Select("serial_number, root_node").
		Where("ne_neid = ? AND deleted = 0", elementId).
		Scan(&row).Error; err != nil {
		return apperror.Wrap(err, "UPDATE_MEAS_TASK_SWITCH_DEVICE_NOT_FOUND", 404, "device not found")
	}
	if row.SerialNumber == nil || *row.SerialNumber == "" {
		return apperror.New("UPDATE_MEAS_TASK_SWITCH_NO_SN", 400, "device has no serial number")
	}

	paramName := measSwitchParamName(row.RootNode)
	paramValue := "0"
	if enable {
		paramValue = "1"
	}
	params := []soap.ParameterValueStruct{{Name: paramName, Value: paramValue, Type: "xsd:boolean"}}
	paramJSON, _ := json.Marshal(params)

	expiredAt := time.Now().Add(5 * time.Minute).UnixMilli()
	msg := opmsg.Message{
		EventType:      "SetParameterValues",
		NeNeid:         elementId,
		Operation:      "SetParameterValues",
		OperationParam: string(paramJSON),
		OperationUser:  username,
		ProtocolType:   opmsg.ProtocolTR069,
		ExpiredAt:      expiredAt,
	}
	msgBytes, err := msg.Marshal()
	if err != nil {
		return apperror.Wrap(err, "UPDATE_MEAS_TASK_SWITCH_MARSHAL_FAILED", 500, "failed to marshal opmsg")
	}
	ctx := context.Background()
	if err := redis.LPush(ctx, mq.OperationQueue, string(msgBytes)); err != nil {
		return apperror.Wrap(err, "UPDATE_MEAS_TASK_SWITCH_PUSH_FAILED", 500, "failed to enqueue operation")
	}
	return nil
}

// AddReplenishTask persists a new replenish task. Mirrors Java addReplenishTask.
func (s *service) AddReplenishTask(t *PMReplenishTask) error {
	now := time.Now()
	t.OperationTime = &now
	if t.Status == nil {
		waiting := 1
		t.Status = &waiting
	}
	if err := s.repo.CreateReplenishTask(t); err != nil {
		return apperror.Wrap(err, "ADD_REPLENISH_TASK_FAILED", 500, "failed to create replenish task")
	}
	return nil
}

// ListReplenishTask returns paginated replenish tasks for the license.
// Mirrors Java listReplenishTask.
func (s *service) ListReplenishTask(tenancyId int, name string, page, pageSize int) ([]PMReplenishTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	items, total, err := s.repo.FindReplenishTasks(tenancyId, name, offset, pageSize)
	if err != nil {
		return nil, 0, apperror.Wrap(err, "LIST_REPLENISH_TASK_FAILED", 500, "failed to list replenish tasks")
	}
	return items, total, nil
}

// ViewReplenishTask returns a single replenish task. Mirrors Java viewReplenishTask.
func (s *service) ViewReplenishTask(id int) (*PMReplenishTask, error) {
	t, err := s.repo.FindReplenishTask(id)
	if err != nil {
		return nil, apperror.Wrap(err, "VIEW_REPLENISH_TASK_FAILED", 500, "failed to view replenish task")
	}
	return t, nil
}

// ListDeviceReplenish returns the cpe_element rows in a replenish task's
// element_ids. Mirrors Java listDeviceReplenish. The Done field is
// populated from the in-memory replenish worker state (set by
// MarkReplenishDeviceDone).
func (s *service) ListDeviceReplenish(taskId int) ([]ReplenishDeviceVo, error) {
	rows, err := s.repo.FindReplenishTaskDevices(taskId)
	if err != nil {
		return nil, apperror.Wrap(err, "LIST_DEVICE_REPLENISH_FAILED", 500, "failed to list device replenish")
	}
	for i := range rows {
		rows[i].Done = s.IsReplenishDeviceDone(taskId, rows[i].NeNeid)
	}
	return rows, nil
}

// newService creates a Service backed by the given Repository (test/mock helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}
