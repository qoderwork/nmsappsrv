package corenet

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"

	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Service defines the business-logic contract for core network management.
type Service interface {
	ListCoreNetworks(tenantId int) ([]CoreNetwork, error)
	GetCoreNetwork(id int) (*CoreNetwork, error)
	CreateCoreNetwork(cn *CoreNetwork) error
	UpdateCoreNetwork(cn *CoreNetwork) error
	DeleteCoreNetwork(id int) error
	GetCoreNetworkData(coreNetworkId int) (*CoreNetworkData, error)
	SaveCoreNetworkData(data *CoreNetworkData) error
	GetCoreNetworkKpis(coreNetworkId int, startTime, endTime string) ([]CoreNetworkKpi, error)
	GetStatisticData(coreNetworkId int, startTime, endTime string) ([]CoreNetworkStatisticData, error)
	ListOperationLogs(coreNetworkId int, page, pageSize int) ([]CoreNetworkOperationLog, int64, error)

	// Tier 1 corenet KPI batch. Mirrors Java CoreNetworkManagementController +
	// CoreNetworkKPIManagementController.

	// GetCoreNetworkAlarms returns recent license-scoped alarms as a proxy
	// for core-network-scoped alarms (the Go schema has no corenet_alarm
	// join). Mirrors Java getCoreNetworkAlarms.
	GetCoreNetworkAlarms(coreNetworkId int) ([]CoreNetworkAlarmVo, error)
	// ListUEList returns the UE list for a core network. The Go side has
	// no UE table; returns empty list until the schema lands. Mirrors
	// Java listUEList.
	ListUEList(coreNetworkId int) ([]UeListVo, error)
	// ListUENumberStatistic returns aggregated UE counts. No Go schema;
	// returns an empty aggregate. Mirrors Java listUENumberStatistic.
	ListUENumberStatistic(coreNetworkId int) (*UeNumberStatisticVo, error)
	// GetUeInfos returns UE detail records. No Go schema; returns empty
	// list. Mirrors Java getUeInfos.
	GetUeInfos(coreNetworkId int) ([]UeInfo, error)
	// ChangeCoreNetworkSwitch flips the NGCSwitch on the core network's
	// device via TR-069 SetParameterValues (mirrors Java
	// changeCoreNetworkSwitch -> operationDTOGenerateUtil.setParamValue
	// Device.FAP.NGC.NGCSwitch). It also keeps a local marker on the row
	// for traceability (Java relies on device state).
	ChangeCoreNetworkSwitch(coreNetworkId int, enable bool, username string) error
	// GetCoreNetworkUserInfo returns aggregated user counts across the
	// core network's KPI rows. Mirrors Java getCoreNetworkUserInfo.
	GetCoreNetworkUserInfo(coreNetworkId int) (*CoreNetworkUserInfoVo, error)
	// GetCoreNetworkUpfTraffic returns aggregated UPF traffic across the
	// core network's KPI rows. Mirrors Java getCoreNetworkUpfTraffic.
	GetCoreNetworkUpfTraffic(coreNetworkId int) (*CoreNetworkUpfTrafficVo, error)
	// IngestCoreNetworkKpi stores device-reported KPI into core_network_kpi
	// (mirrors Java kpi() / dealUPFKPI). This is the ingest half of the
	// KPI collector that makes core_network_kpi non-empty.
	IngestCoreNetworkKpi(dto IngestCoreNetworkKpiDTO) error

	// Tier 1.5 corenet parameter CRUD (mirrors Java CoreNetworkManagementController
	// getCoreNetworkParameters / setCoreNetworkParameters / queryCoreNetworkParameters /
	// deleteCoreNetworkParameter / addCoreNetworkParameter). These operate on the
	// core-network element's local management API (:33030) plus operation logging.
	GetCoreNetworkParameters(query GetCoreNetworkParametersQuery) (*CoreNetworkParamElementVO, error)
	SetCoreNetworkParameters(dto SetCoreNetworkParametersDTO, user string) (int64, error)
	QueryCoreNetworkParameters(dto QueryCoreNetworkParametersDTO, user string) (int64, error)
	DeleteCoreNetworkParameter(dto DeleteCoreNetworkParameterDTO, user string) (int64, error)
	AddCoreNetworkParameter(dto SetCoreNetworkParametersDTO, user string) (int64, error)

	// GetCoreNetworkElementSystemState returns the per-element system state
	// parsed from core_network_data *Info columns. Mirrors Java
	// getCoreNetworkElementSystemState (pure DB read, no device call).
	GetCoreNetworkElementSystemState(coreNetworkId int) (*GetCoreNetworkElementSystemStateVO, error)

	// PCF UE management (mirrors Java downloadUPFUETemplate / importPCFUE /
	// updatePCFUE / deletePCFUE). These forward to the PCF element's 33030
	// ueManagement API plus operation logging; no Go DB table is involved
	// (UE data lives on the device, like Java).
	DownloadPCFUETemplate() ([]byte, error)
	ImportPCFUE(r io.Reader, coreNetworkId int, user string) error
	UpdatePCFUE(dto PCFUEVO, user string) error
	DeletePCFUE(dto DeletePCFUEDTO, user string) error
	// GetBuiltInCoreNetworkUpfTraffic returns the built-in UPF traffic
	// view (excludes user-supplied KPI rows). Mirrors Java
	// getBuiltInCoreNetworkUpfTraffic.
	GetBuiltInCoreNetworkUpfTraffic(coreNetworkId int) (*CoreNetworkUpfTrafficVo, error)
	// GetBuiltInCoreNetworkUserInfo returns the built-in user-info view.
	// Mirrors Java getBuiltInCoreNetworkUserInfo.
	GetBuiltInCoreNetworkUserInfo(coreNetworkId int) (*CoreNetworkUserInfoVo, error)
	// GetKpiReport returns a time-series report for the given KPI index
	// (0=user, 1=traffic, 2=alarm, ...). Mirrors Java kpiReport/{index}.
	GetKpiReport(coreNetworkId int, index int, startTime, endTime string) ([]KpiReportRow, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo     Repository
	db       *gorm.DB
	opSender *tr069.OperationSender
	msgMgr   *tr069.MessageManager
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	msgMgr := tr069.NewMessageManager()
	opSender := tr069.NewOperationSender(db, msgMgr)
	return &service{repo: NewRepository(db), db: db, opSender: opSender, msgMgr: msgMgr}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// ListCoreNetworks returns all core networks for the given tenancy.
func (s *service) ListCoreNetworks(tenantId int) ([]CoreNetwork, error) {
	return s.repo.FindCoreNetworks(tenantId)
}

// GetCoreNetwork returns a single core network by ID.
func (s *service) GetCoreNetwork(id int) (*CoreNetwork, error) {
	return s.repo.FindByID(id)
}

// CreateCoreNetwork persists a new core network.
func (s *service) CreateCoreNetwork(cn *CoreNetwork) error {
	return s.repo.Create(cn)
}

// UpdateCoreNetwork persists changes to an existing core network.
func (s *service) UpdateCoreNetwork(cn *CoreNetwork) error {
	return s.repo.Save(cn)
}

// DeleteCoreNetwork removes a core network by ID, cascading to its data record.
func (s *service) DeleteCoreNetwork(id int) error {
	if err := s.repo.DeleteByID(id); err != nil {
		return err
	}
	return s.repo.DeleteCoreNetworkData(id)
}

// GetCoreNetworkData returns the data record for a core network.
func (s *service) GetCoreNetworkData(coreNetworkId int) (*CoreNetworkData, error) {
	return s.repo.FindCoreNetworkData(coreNetworkId)
}

// SaveCoreNetworkData upserts a core network data record.
func (s *service) SaveCoreNetworkData(data *CoreNetworkData) error {
	return s.repo.SaveCoreNetworkData(data)
}

// GetCoreNetworkKpis returns KPI records within the given time range.
func (s *service) GetCoreNetworkKpis(coreNetworkId int, startTime, endTime string) ([]CoreNetworkKpi, error) {
	st, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		st, err = time.Parse("2006-01-02 15:04:05", startTime)
		if err != nil {
			return nil, err
		}
	}
	et, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		et, err = time.Parse("2006-01-02 15:04:05", endTime)
		if err != nil {
			return nil, err
		}
	}
	return s.repo.FindCoreNetworkKpis(coreNetworkId, st, et)
}

// GetStatisticData returns statistic data within the given time range.
func (s *service) GetStatisticData(coreNetworkId int, startTime, endTime string) ([]CoreNetworkStatisticData, error) {
	st, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		st, err = time.Parse("2006-01-02 15:04:05", startTime)
		if err != nil {
			return nil, err
		}
	}
	et, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		et, err = time.Parse("2006-01-02 15:04:05", endTime)
		if err != nil {
			return nil, err
		}
	}
	return s.repo.FindCoreNetworkStatisticData(coreNetworkId, st, et)
}

// ListOperationLogs returns a paginated list of operation logs.
func (s *service) ListOperationLogs(coreNetworkId int, page, pageSize int) ([]CoreNetworkOperationLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindOperationLogs(coreNetworkId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// Tier 1 corenet KPI batch
// ---------------------------------------------------------------------------

// GetCoreNetworkAlarms returns recent license-scoped alarms as a proxy
// for core-network-scoped alarms. The Go schema has no corenet_alarm
// join, so we surface the most-recent 50 active alarms. Mirrors Java
// getCoreNetworkAlarms.
func (s *service) GetCoreNetworkAlarms(coreNetworkId int) ([]CoreNetworkAlarmVo, error) {
	type row struct {
		Id              int64     `gorm:"column:id"`
		Severity        *string   `gorm:"column:severity"`
		AlarmIdentifier *string   `gorm:"column:alarm_identifier"`
		ProbableCause   *string   `gorm:"column:probable_cause"`
		EventTime       *time.Time `gorm:"column:event_time"`
		AlarmStatus     *int      `gorm:"column:alarm_status"`
	}
	var rows []row
	q := s.db.Table("alarm").
		Select("id, severity, alarm_identifier, probable_cause, event_time, alarm_status").
		Where("alarm_type = ?", 1). // ACTIVE only
		Order("event_time DESC").
		Limit(50)
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]CoreNetworkAlarmVo, 0, len(rows))
	for _, r := range rows {
		out = append(out, CoreNetworkAlarmVo{
			Id:              r.Id,
			Severity:        r.Severity,
			AlarmIdentifier: r.AlarmIdentifier,
			ProbableCause:   r.ProbableCause,
			EventTime:       r.EventTime,
			AlarmStatus:     r.AlarmStatus,
		})
	}
	return out, nil
}

// ListUEList returns the UE list for a core network. The Go side has
// no UE table; returns an empty list. Mirrors Java listUEList.
func (s *service) ListUEList(coreNetworkId int) ([]UeListVo, error) {
	return []UeListVo{}, nil
}

// ListUENumberStatistic returns aggregated UE counts. The Go side has
// no UE table; returns an empty aggregate. Mirrors Java
// listUENumberStatistic.
func (s *service) ListUENumberStatistic(coreNetworkId int) (*UeNumberStatisticVo, error) {
	return &UeNumberStatisticVo{
		Total:       0,
		ByCategory:  map[string]int64{},
		ByState:     map[string]int64{},
		GeneratedAt: time.Now().Format(time.RFC3339),
	}, nil
}

// GetUeInfos returns UE detail records. The Go side has no UE table;
// returns an empty list. Mirrors Java getUeInfos.
func (s *service) GetUeInfos(coreNetworkId int) ([]UeInfo, error) {
	return []UeInfo{}, nil
}

// ChangeCoreNetworkSwitch flips the NGCSwitch on the core network's device
// via a TR-069 SetParameterValues, mirroring Java changeCoreNetworkSwitch
// (operationDTOGenerateUtil.setParamValue Device.FAP.NGC.NGCSwitch). It also
// keeps a local marker on the row for traceability.
func (s *service) ChangeCoreNetworkSwitch(coreNetworkId int, enable bool, username string) error {
	// Local marker (kept for traceability; Java relies on device state).
	if err := s.repo.UpdateCoreNetworkSwitch(coreNetworkId, enable); err != nil {
		logger.Warnf("ChangeCoreNetworkSwitch: local marker update failed: %v", err)
	}

	// Resolve the linked device and dispatch the SPV to it.
	cn, err := s.repo.FindByID(coreNetworkId)
	if err != nil || cn == nil || cn.ElementId == nil {
		logger.Infof("ChangeCoreNetworkSwitch: core network %d has no linked device; marker only", coreNetworkId)
		return nil
	}
	var serial string
	if err := s.db.Table("cpe_element").
		Select("serial_number").
		Where("ne_neid = ? AND deleted = ?", *cn.ElementId, false).
		Scan(&serial).Error; err != nil || serial == "" {
		logger.Warnf("ChangeCoreNetworkSwitch: no device serial for core network %d", coreNetworkId)
		return nil
	}

	value := "0"
	if enable {
		value = "1"
	}
	params := []soap.ParameterValueStruct{
		{Name: "Device.FAP.NGC.NGCSwitch", Value: value, Type: "xsd:string"},
	}

	operationId := fmt.Sprintf("corenet_switch_%d_%t", coreNetworkId, enable)
	if s.opSender != nil {
		if err := s.opSender.SendSetParameterValues(serial, params, "", operationId); err != nil {
			logger.Errorf("ChangeCoreNetworkSwitch: failed to send SPV to %s: %v", serial, err)
			return err
		}
	}
	return nil
}

// insertCoreNetEventLog writes an event_log row for a core-network device
// operation and returns its id (mirrors parameter.repository.InsertEventLog).
func (s *service) insertCoreNetEventLog(elementId int64, user, headerId string) (int64, error) {
	row := struct {
		Id               int64     `gorm:"primaryKey;autoIncrement"`
		EventType        string    `gorm:"column:event_type"`
		OperationTime    time.Time `gorm:"column:operation_time"`
		User             string    `gorm:"column:user"`
		ElementId        int64     `gorm:"column:element_id"`
		Status           int       `gorm:"column:status"`
		CommandTrackData string    `gorm:"column:command_track_data"`
	}{
		EventType:        "SetParameterValues",
		OperationTime:    time.Now(),
		User:             user,
		ElementId:        elementId,
		Status:           1,
		CommandTrackData: headerId,
	}
	if err := s.db.Table("event_log").Create(&row).Error; err != nil {
		return 0, err
	}
	return row.Id, nil
}

// IngestCoreNetworkKpiDTO is the request body for device-reported KPI
// (mirrors Java CoreNetworkElementKPIDTO).
type IngestCoreNetworkKpiDTO struct {
	Task struct {
		Period struct {
			StartTime string `json:"startTime"`
			EndTime   string `json:"endTime"`
		} `json:"period"`
		Ne struct {
			NeType string `json:"neType"`
			RmUid  string `json:"rmUid"`
			Kpis   []struct {
				KpiId  string `json:"kpiId"`
				KpiId1 string `json:"kpiId1"`
				Value  int    `json:"value"`
			} `json:"kpis"`
		} `json:"ne"`
	} `json:"task"`
}

// IngestCoreNetworkKpi stores device-reported KPI into core_network_kpi.
// Mirrors Java kpi() / dealUPFKPI. This is the ingest half of the KPI
// collector that makes core_network_kpi non-empty (the previous Go side had
// no writer, so traffic/user KPIs were always empty). UDM/IMS KPI feed an
// in-memory user-count cache in Java; Go records their arrival but does not
// yet populate a user-count store.
func (s *service) IngestCoreNetworkKpi(dto IngestCoreNetworkKpiDTO) error {
	ne := dto.Task.Ne
	if ne.RmUid == "" {
		return fmt.Errorf("kpi report missing rmUid")
	}
	startTime := dto.Task.Period.StartTime
	if startTime == "" {
		startTime = time.Now().Format(time.RFC3339)
	}
	st, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		st, err = time.Parse("2006-01-02 15:04:05", startTime)
		if err != nil {
			return err
		}
	}
	bucket := belong5Min(st)

	kpiMap := map[string]int{}
	for _, k := range ne.Kpis {
		id := k.KpiId
		if id == "" {
			id = k.KpiId1
		}
		kpiMap[id] = k.Value
	}

	switch ne.NeType {
	case "UPF":
		if err := s.upsertCoreNetworkKpi(ne.RmUid, bucket,
			float64(kpiMap["UPF.03"]), float64(kpiMap["UPF.04"]),
			float64(kpiMap["UPF.07"]), float64(kpiMap["UPF.08"])); err != nil {
			return err
		}
	case "UDM", "IMS":
		logger.Infof("IngestCoreNetworkKpi: received %s KPI for rmUid=%s (user-count store not yet ported)", ne.NeType, ne.RmUid)
	default:
		logger.Warnf("IngestCoreNetworkKpi: unknown neType %q", ne.NeType)
	}
	return nil
}

// upsertCoreNetworkKpi accumulates a 5-minute KPI bucket into
// core_network_kpi (mirrors Java dealUPFKPI accumulating into its per-(time,
// rmUid) cache entry).
func (s *service) upsertCoreNetworkKpi(rmUid string, bucket time.Time, n6Ul, n6Dl, sgiUl, sgiDl float64) error {
	var existing CoreNetworkKpi
	err := s.db.Table("core_network_kpi").
		Where("rm_uid = ? AND start_time = ?", rmUid, bucket).
		First(&existing).Error
	if err == nil {
		return s.db.Table("core_network_kpi").Where("id = ?", existing.Id).Updates(map[string]interface{}{
			"n6_ul_traffic":  addFloat(existing.N6UlTraffic, n6Ul),
			"n6_dl_traffic":  addFloat(existing.N6DlTraffic, n6Dl),
			"sgi_ul_traffic": addFloat(existing.SgiUlTraffic, sgiUl),
			"sgi_dl_traffic": addFloat(existing.SgiDlTraffic, sgiDl),
		}).Error
	}
	kpi := CoreNetworkKpi{
		StartTime:    &bucket,
		N6UlTraffic:  &n6Ul,
		N6DlTraffic:  &n6Dl,
		SgiUlTraffic: &sgiUl,
		SgiDlTraffic: &sgiDl,
		RmUid:        &rmUid,
	}
	return s.db.Table("core_network_kpi").Create(&kpi).Error
}

// belong5Min floors a time to its 5-minute bucket (mirrors Java
// CoreNetworkKPIManagementServiceImpl.belong5Min).
func belong5Min(t time.Time) time.Time {
	y, mo, d := t.Date()
	h, m, _ := t.Clock()
	m = (m / 5) * 5
	return time.Date(y, mo, d, h, m, 0, 0, t.Location())
}

// addFloat adds add to *base (treating nil as 0) and returns the sum.
func addFloat(base *float64, add float64) float64 {
	if base != nil {
		return *base + add
	}
	return add
}

// logCoreNetOp writes a core_network_operation_log row (mirrors Java
// CoreNetworkOperationLog creation in the parameter endpoints) and returns
// its id. Failures are logged but non-fatal — Java treats the log as
// bookkeeping, not the operation result.
func (s *service) logCoreNetOp(coreNetworkId int, logType, user, info, requestId string) int64 {
	now := time.Now()
	log := &CoreNetworkOperationLog{
		LogType:       &logType,
		User:          &user,
		CoreNetworkId: &coreNetworkId,
		Info:          &info,
		RequestId:     &requestId,
		Result:        intPtr(1),
		OperationTime: &now,
	}
	if err := s.repo.CreateOperationLog(log); err != nil {
		logger.Errorf("logCoreNetOp(%s) failed: %v", logType, err)
		return 0
	}
	return log.Id
}

func intPtr(i int) *int { return &i }

// GetCoreNetworkParameters returns the parameter catalog for an element type,
// enriched with the stored config values from core_network_data. Mirrors
// Java getCoreNetworkParameters.
func (s *service) GetCoreNetworkParameters(query GetCoreNetworkParametersQuery) (*CoreNetworkParamElementVO, error) {
	if query.CoreNetworkId <= 0 || query.ElementType == "" {
		return nil, fmt.Errorf("coreNetworkId and elementType are required")
	}
	vo, err := loadCatalog(query.ElementType)
	if err != nil {
		return nil, err
	}
	var data *CoreNetworkData
	if d, derr := s.repo.FindCoreNetworkData(query.CoreNetworkId); derr == nil {
		data = d
	}
	for i := range vo.Params {
		vo.Params[i].Data = enrichCoreNetParamData(data, query.ElementType, vo.Params[i].Name)
	}
	return vo, nil
}

// SetCoreNetworkParameters pushes config values to the element via CPE proxy
// (HttpRequestProxy SOAP) and logs the operation. Mirrors Java setCoreNetworkParameters.
func (s *service) SetCoreNetworkParameters(dto SetCoreNetworkParametersDTO, user string) (int64, error) {
	if dto.CoreNetworkId == 0 || dto.Name == "" {
		return 0, fmt.Errorf("coreNetworkId and name are required")
	}
	body, _ := json.Marshal(dto.Data)
	requestID := fmt.Sprintf("set:%s", dto.Name)
	info := dto.Name + ":" + string(body)

	sn, err := s.resolveDeviceSN(dto.CoreNetworkId)
	if err != nil {
		return 0, err
	}

	ip := getElementIp(dto.ElementType)
	baseURL := fmt.Sprintf("http://%s:33030/api/rest/systemManagement/v1/elementType/%s/objectType/config/%s",
		ip, dto.ElementType, dto.Name)

	requests := make([]soap.HttpRequest, 0, 2)

	setURL := baseURL
	if dto.Index != nil {
		setURL += fmt.Sprintf("?loc=%d", *dto.Index)
	}
	requests = append(requests, soap.HttpRequest{
		URL:        setURL,
		HttpMethod: "PUT",
		Body:       string(body),
		RequestId:  requestID,
	})

	requests = append(requests, soap.HttpRequest{
		URL:        baseURL,
		HttpMethod: "GET",
		RequestId:  fmt.Sprintf("query:%s", dto.Name),
	})

	operationId := fmt.Sprintf("corenet_set_%d_%s", dto.CoreNetworkId, dto.Name)
	proxy := &soap.HttpRequestProxy{Requests: requests}
	if err := s.opSender.SendHttpRequestProxy(sn, proxy, operationId); err != nil {
		logger.Errorf("SetCoreNetworkParameters: failed to send HttpRequestProxy to %s: %v", sn, err)
		return 0, err
	}

	return s.logCoreNetOp(dto.CoreNetworkId, "Set Parameter", user, info, requestID), nil
}

// QueryCoreNetworkParameters reads config values from the element via CPE proxy
// (HttpRequestProxy SOAP) and logs the operation. Mirrors Java queryCoreNetworkParameters.
func (s *service) QueryCoreNetworkParameters(dto QueryCoreNetworkParametersDTO, user string) (int64, error) {
	if dto.CoreNetworkId == 0 || dto.Name == "" {
		return 0, fmt.Errorf("coreNetworkId and name are required")
	}
	requestID := fmt.Sprintf("query:%s", dto.Name)

	sn, err := s.resolveDeviceSN(dto.CoreNetworkId)
	if err != nil {
		return 0, err
	}

	ip := getElementIp(dto.ElementType)
	url := fmt.Sprintf("http://%s:33030/api/rest/systemManagement/v1/elementType/%s/objectType/config/%s",
		ip, dto.ElementType, dto.Name)

	requests := []soap.HttpRequest{
		{
			URL:        url,
			HttpMethod: "GET",
			RequestId:  requestID,
		},
	}

	operationId := fmt.Sprintf("corenet_query_%d_%s", dto.CoreNetworkId, dto.Name)
	proxy := &soap.HttpRequestProxy{Requests: requests}
	if err := s.opSender.SendHttpRequestProxy(sn, proxy, operationId); err != nil {
		logger.Errorf("QueryCoreNetworkParameters: failed to send HttpRequestProxy to %s: %v", sn, err)
		return 0, err
	}

	return s.logCoreNetOp(dto.CoreNetworkId, "Query Parameter", user, dto.Name, requestID), nil
}

// DeleteCoreNetworkParameter removes a config array element on the element via
// CPE proxy (HttpRequestProxy SOAP) and logs the operation. Mirrors Java
// deleteCoreNetworkParameter.
func (s *service) DeleteCoreNetworkParameter(dto DeleteCoreNetworkParameterDTO, user string) (int64, error) {
	if dto.Name == "" || dto.Index == 0 {
		return 0, fmt.Errorf("name and index are required")
	}
	idx := dto.Index
	requestID := fmt.Sprintf("delete:%s", dto.Name)
	info := fmt.Sprintf("%s:%d", dto.Name, dto.Index)

	sn, err := s.resolveDeviceSN(dto.CoreNetworkId)
	if err != nil {
		return 0, err
	}

	ip := getElementIp(dto.ElementType)
	url := fmt.Sprintf("http://%s:33030/api/rest/systemManagement/v1/elementType/%s/objectType/config/%s?loc=%d",
		ip, dto.ElementType, dto.Name, idx)

	requests := []soap.HttpRequest{
		{
			URL:        url,
			HttpMethod: "DELETE",
			RequestId:  requestID,
		},
	}

	operationId := fmt.Sprintf("corenet_delete_%d_%s", dto.CoreNetworkId, dto.Name)
	proxy := &soap.HttpRequestProxy{Requests: requests}
	if err := s.opSender.SendHttpRequestProxy(sn, proxy, operationId); err != nil {
		logger.Errorf("DeleteCoreNetworkParameter: failed to send HttpRequestProxy to %s: %v", sn, err)
		return 0, err
	}

	return s.logCoreNetOp(dto.CoreNetworkId, "Delete Parameter", user, info, requestID), nil
}

// AddCoreNetworkParameter adds a config array element on the element via CPE
// proxy (HttpRequestProxy SOAP) and logs the operation. Mirrors Java addCoreNetworkParameter.
func (s *service) AddCoreNetworkParameter(dto SetCoreNetworkParametersDTO, user string) (int64, error) {
	if dto.Name == "" || dto.Index == nil {
		return 0, fmt.Errorf("name and index are required")
	}
	body, _ := json.Marshal(dto.Data)
	requestID := fmt.Sprintf("add:%s", dto.Name)
	info := dto.Name + ":" + string(body)

	sn, err := s.resolveDeviceSN(dto.CoreNetworkId)
	if err != nil {
		return 0, err
	}

	ip := getElementIp(dto.ElementType)
	url := fmt.Sprintf("http://%s:33030/api/rest/systemManagement/v1/elementType/%s/objectType/config/%s?loc=%d",
		ip, dto.ElementType, dto.Name, *dto.Index)

	requests := []soap.HttpRequest{
		{
			URL:        url,
			HttpMethod: "POST",
			Body:       string(body),
			RequestId:  requestID,
		},
	}

	operationId := fmt.Sprintf("corenet_add_%d_%s", dto.CoreNetworkId, dto.Name)
	proxy := &soap.HttpRequestProxy{Requests: requests}
	if err := s.opSender.SendHttpRequestProxy(sn, proxy, operationId); err != nil {
		logger.Errorf("AddCoreNetworkParameter: failed to send HttpRequestProxy to %s: %v", sn, err)
		return 0, err
	}

	return s.logCoreNetOp(dto.CoreNetworkId, "Add Parameter", user, info, requestID), nil
}

// resolveDeviceSN resolves the device serial number linked to a core network
// via core_network.element_id -> cpe_element.ne_neid.
func (s *service) resolveDeviceSN(coreNetworkId int) (string, error) {
	cn, err := s.repo.FindByID(coreNetworkId)
	if err != nil || cn == nil || cn.ElementId == nil {
		return "", fmt.Errorf("core network %d not found or no linked device", coreNetworkId)
	}
	var sn string
	if err := s.db.Table("cpe_element").
		Select("serial_number").
		Where("ne_neid = ? AND deleted = ?", *cn.ElementId, false).
		Scan(&sn).Error; err != nil || sn == "" {
		return "", fmt.Errorf("no device serial for core network %d", coreNetworkId)
	}
	return sn, nil
}

// getElementIp maps a core-network element type to its loopback management IP.
// Mirrors Java CoreNetworkManagementServiceImpl.getElementIp.
func getElementIp(elementType string) string {
	switch elementType {
	case "ims":
		return "127.0.0.110"
	case "amf":
		return "127.0.0.120"
	case "ausf":
		return "127.0.0.130"
	case "udm":
		return "127.0.0.140"
	case "smf":
		return "127.0.0.150"
	case "pcf":
		return "127.0.0.160"
	case "upf":
		return "127.0.0.190"
	default:
		return ""
	}
}

// GetCoreNetworkElementSystemState reads core_network_data and parses each
// non-empty per-element *Info column into a CoreNetworkSystemState. Mirrors
// Java getCoreNetworkElementSystemState (no device call).
func (s *service) GetCoreNetworkElementSystemState(coreNetworkId int) (*GetCoreNetworkElementSystemStateVO, error) {
	if coreNetworkId <= 0 {
		return nil, fmt.Errorf("coreNetworkId is required")
	}
	data, err := s.repo.FindCoreNetworkData(coreNetworkId)
	if err != nil || data == nil {
		return nil, fmt.Errorf("core network data not found")
	}
	vo := &GetCoreNetworkElementSystemStateVO{}
	if str := data.ImsInfo; str != nil && *str != "" {
		vo.Ims = parseSystemState(*str)
	}
	if str := data.AmfInfo; str != nil && *str != "" {
		vo.Amf = parseSystemState(*str)
	}
	if str := data.AusfInfo; str != nil && *str != "" {
		vo.Ausf = parseSystemState(*str)
	}
	if str := data.UdmInfo; str != nil && *str != "" {
		vo.Udm = parseSystemState(*str)
	}
	if str := data.SmfInfo; str != nil && *str != "" {
		vo.Smf = parseSystemState(*str)
	}
	if str := data.PcfInfo; str != nil && *str != "" {
		vo.Pcf = parseSystemState(*str)
	}
	if str := data.UpfInfo; str != nil && *str != "" {
		vo.Upf = parseSystemState(*str)
	}
	return vo, nil
}

// resolveRmUid returns the device serial number linked to a core network via
// core_network.element_id -> cpe_element.ne_neid (mirrors the rm_uid scoping
// Java uses in getBuiltInCoreNetworkUpfTraffic).
func (s *service) resolveRmUid(coreNetworkId int) (string, error) {
	var rmUid string
	if err := s.db.Table("cpe_element").
		Select("cpe_element.serial_number").
		Joins("JOIN core_network cn ON cpe_element.ne_neid = cn.element_id").
		Where("cn.id = ? AND cpe_element.deleted = ?", coreNetworkId, false).
		Scan(&rmUid).Error; err != nil {
		return "", err
	}
	return rmUid, nil
}

// GetCoreNetworkUserInfo returns aggregated user counts derived from the
// core network's KPI rows (count of device reports scoped by rm_uid).
// Mirrors Java getCoreNetworkUserInfo (which uses an in-memory user-count
// cache fed by UDM/IMS KPI; Go approximates with the KPI-row count).
func (s *service) GetCoreNetworkUserInfo(coreNetworkId int) (*CoreNetworkUserInfoVo, error) {
	rmUid, _ := s.resolveRmUid(coreNetworkId)
	var count int64
	if rmUid != "" {
		if err := s.db.Table("core_network_kpi").
			Where("rm_uid = ?", rmUid).
			Count(&count).Error; err != nil {
			return nil, err
		}
	}
	return &CoreNetworkUserInfoVo{
		TotalUsers:  count,
		ActiveUsers: count,
		IdleUsers:   0,
		ByCoreNet:   map[string]int64{fmt.Sprintf("%d", coreNetworkId): count},
		GeneratedAt: time.Now().Format(time.RFC3339),
	}, nil
}

// GetCoreNetworkUpfTraffic returns aggregated UPF traffic derived from the
// core network's KPI rows (sum of N6 + SGI uplink/downlink). Mirrors Java
// getCoreNetworkUpfTraffic, which reads the n6_* / sgi_* traffic columns and
// scopes by rm_uid. The previous implementation queried non-existent columns
// (uplink_bps/downlink_bps) and a non-existent core_network_id column, so
// traffic was always 0.
func (s *service) GetCoreNetworkUpfTraffic(coreNetworkId int) (*CoreNetworkUpfTrafficVo, error) {
	rmUid, _ := s.resolveRmUid(coreNetworkId)
	vo := &CoreNetworkUpfTrafficVo{GeneratedAt: time.Now().Format(time.RFC3339)}
	if rmUid == "" {
		return vo, nil
	}
	type row struct {
		N6UlTraffic  *float64 `gorm:"column:n6_ul_traffic"`
		N6DlTraffic  *float64 `gorm:"column:n6_dl_traffic"`
		SgiUlTraffic *float64 `gorm:"column:sgi_ul_traffic"`
		SgiDlTraffic *float64 `gorm:"column:sgi_dl_traffic"`
	}
	var rows []row
	if err := s.db.Table("core_network_kpi").
		Select("n6_ul_traffic, n6_dl_traffic, sgi_ul_traffic, sgi_dl_traffic").
		Where("rm_uid = ?", rmUid).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	var up, down float64
	for _, r := range rows {
		if r.N6UlTraffic != nil {
			up += *r.N6UlTraffic
		}
		if r.SgiUlTraffic != nil {
			up += *r.SgiUlTraffic
		}
		if r.N6DlTraffic != nil {
			down += *r.N6DlTraffic
		}
		if r.SgiDlTraffic != nil {
			down += *r.SgiDlTraffic
		}
	}
	vo.UplinkBps = up
	vo.DownlinkBps = down
	vo.TotalBytes = int64((up + down) / 8)
	return vo, nil
}

// GetBuiltInCoreNetworkUpfTraffic returns the built-in UPF traffic
// view. The Go side has no separate "built-in" vs "user" distinction;
// we return the same data as GetCoreNetworkUpfTraffic. Mirrors Java
// getBuiltInCoreNetworkUpfTraffic.
func (s *service) GetBuiltInCoreNetworkUpfTraffic(coreNetworkId int) (*CoreNetworkUpfTrafficVo, error) {
	return s.GetCoreNetworkUpfTraffic(coreNetworkId)
}

// GetBuiltInCoreNetworkUserInfo returns the built-in user-info view.
// Same as GetCoreNetworkUserInfo (no separate "built-in" table in Go).
// Mirrors Java getBuiltInCoreNetworkUserInfo.
func (s *service) GetBuiltInCoreNetworkUserInfo(coreNetworkId int) (*CoreNetworkUserInfoVo, error) {
	return s.GetCoreNetworkUserInfo(coreNetworkId)
}

// GetKpiReport returns a time-series report for the given KPI index
// (0=user, 1=traffic, 2=alarm, ...). The Go side maps index 0/1 to the
// core_network_statistic_data rows; other indices return empty. Mirrors
// Java kpiReport/{index}.
func (s *service) GetKpiReport(coreNetworkId int, index int, startTime, endTime string) ([]KpiReportRow, error) {
	if index != 0 && index != 1 {
		return []KpiReportRow{}, nil
	}
	st, et, err := parseKpiReportTimeRange(startTime, endTime)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.FindCoreNetworkStatisticData(coreNetworkId, st, et)
	if err != nil {
		return nil, err
	}
	out := make([]KpiReportRow, 0, len(rows))
	for _, r := range rows {
		metrics := map[string]interface{}{}
		if r.StatisticTime != nil {
			metrics["time"] = r.StatisticTime.Format(time.RFC3339)
		}
		metrics["id"] = r.Id
		metrics["core_network_id"] = r.CoreNetworkId
		// The Go statistic table has only time + id + core_network_id
		// (the Java side has per-metric columns that Go does not). The
		// caller maps additional metric fields by index.
		metrics["index"] = index
		out = append(out, KpiReportRow{
			Timestamp: *r.StatisticTime,
			Metrics:   metrics,
		})
	}
	return out, nil
}

// parseKpiReportTimeRange parses two RFC3339 strings and returns them
// as time.Time. Empty strings default to "last 24h" / "now".
func parseKpiReportTimeRange(startTime, endTime string) (time.Time, time.Time, error) {
	now := time.Now()
	if endTime == "" {
		endTime = now.Format(time.RFC3339)
	}
	if startTime == "" {
		startTime = now.Add(-24 * time.Hour).Format(time.RFC3339)
	}
	st, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	et, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return st, et, nil
}

// ---------------------------------------------------------------------------
// PCF UE management (mirrors Java downloadUPFUETemplate / importPCFUE /
// updatePCFUE / deletePCFUE). Forwards to the PCF element's 33030
// ueManagement API (http://127.0.0.160:33030/api/rest/ueManagement/v1/
// elementType/pcf/objectType/ueInfo) plus operation logging.
// ---------------------------------------------------------------------------

const pcfUEAPI = "ueManagement"
const pcfUEObjectType = "ueInfo"

// DownloadPCFUETemplate returns the embedded PCF UE import template xlsx,
// mirroring Java downloadUPFUETemplate (serves the classpath resource).
func (s *service) DownloadPCFUETemplate() ([]byte, error) {
	return pcfTemplateFS.ReadFile("templates/PCF UE Template.xlsx")
}

// ImportPCFUE parses the uploaded xlsx, validates it, forwards each UE to the
// PCF element and logs the operation. Mirrors Java importPCFUE.
func (s *service) ImportPCFUE(r io.Reader, coreNetworkId int, user string) error {
	if coreNetworkId == 0 {
		return fmt.Errorf("coreNetworkId is required")
	}
	rows, err := parsePCFUEExcel(r)
	if err != nil {
		return err
	}
	for _, d := range rows {
		if d.Imsi == "" {
			return fmt.Errorf("The 'IMSI' cannot be null")
		}
		if d.Msisdn == "" {
			return fmt.Errorf("The 'MSISDN' cannot be null")
		}
	}
	seen := make(map[string]int, len(rows))
	for _, d := range rows {
		seen[d.Imsi]++
		if seen[d.Imsi] > 1 {
			return fmt.Errorf("%s is duplicated", d.Imsi)
		}
	}
	if len(rows) > 100 {
		return fmt.Errorf("The maximum number of UE imported in a batch is 100")
	}
	for _, d := range rows {
		if _, err := callElementAPI("pcf", pcfUEAPI, pcfUEObjectType, nil, "POST", "", d); err != nil {
			return err
		}
	}
	if _, err := callElementAPI("pcf", pcfUEAPI, pcfUEObjectType, nil, "GET", "", nil); err != nil {
		return err
	}
	imsis := make([]string, 0, len(rows))
	for _, d := range rows {
		imsis = append(imsis, d.Imsi)
	}
	if b, e := json.Marshal(imsis); e == nil {
		s.logCoreNetOp(coreNetworkId, "Add PCF UE", user, string(b), "")
	}
	return nil
}

// UpdatePCFUE PUTs a UE to the PCF element and logs the operation.
// Mirrors Java updatePCFUE.
func (s *service) UpdatePCFUE(dto PCFUEVO, user string) error {
	if dto.Imsi == "" || dto.Msisdn == "" || dto.CoreNetworkId == 0 {
		return fmt.Errorf("imsi, msisdn and coreNetworkId are required")
	}
	q := "imsi=" + url.QueryEscape(dto.Imsi) + "&msisdn=" + url.QueryEscape(dto.Msisdn)
	if _, err := callElementAPI("pcf", pcfUEAPI, pcfUEObjectType, nil, "PUT", q, dto.ImportPCFUEDTO); err != nil {
		return err
	}
	if _, err := callElementAPI("pcf", pcfUEAPI, pcfUEObjectType, nil, "GET", "", nil); err != nil {
		return err
	}
	s.logCoreNetOp(dto.CoreNetworkId, "Modify PCF UE", user, dto.Imsi, "")
	return nil
}

// DeletePCFUE DELETEs a UE from the PCF element and logs the operation.
// Mirrors Java deletePCFUE.
func (s *service) DeletePCFUE(dto DeletePCFUEDTO, user string) error {
	if dto.Imsi == "" || dto.Msisdn == "" || dto.CoreNetworkId == 0 {
		return fmt.Errorf("imsi, msisdn and coreNetworkId are required")
	}
	q := "imsi=" + url.QueryEscape(dto.Imsi) + "&msisdn=" + url.QueryEscape(dto.Msisdn)
	if _, err := callElementAPI("pcf", pcfUEAPI, pcfUEObjectType, nil, "DELETE", q, nil); err != nil {
		return err
	}
	if _, err := callElementAPI("pcf", pcfUEAPI, pcfUEObjectType, nil, "GET", "", nil); err != nil {
		return err
	}
	s.logCoreNetOp(dto.CoreNetworkId, "Delete PCF UE", user, dto.Imsi, "")
	return nil
}
