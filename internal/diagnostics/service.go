package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// Service defines the business-logic contract for diagnostics.
type Service interface {
	TriggerPing(req *PingRequest, username string) error
	TriggerTraceRoute(req *TraceRouteRequest, username string) error
	TriggerDownload(req *DownloadRequest, username string, fileServerIp string) error
	TriggerUpload(req *UploadRequest, username string, fileServerIp string) error
	GetDiagnosticsStatus(elementId int64) bool
	GetDiagnosticsResult(elementId int64) (*DiagnosticsResultVO, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// ---------- TR-069 parameter name constants ----------

var (
	routeHopPattern = regexp.MustCompile(`\.TraceRouteDiagnostics\.RouteHops\.(\d+)\.HopHost$`)
)

// ---------- Dispatch helper ----------

// dispatchDiagnostics builds params, creates EventLog, pushes to Redis queue, sets diagnostics status.
func (s *service) dispatchDiagnostics(elementId int64, rootNode string, username string, params []paramVo, diagType string) error {
	// Query param type info from DB
	paramNames := make([]string, len(params))
	for i, p := range params {
		paramNames[i] = p.ParamName
	}
	existingParams, _ := s.repo.FindParamsByElementIdAndNames(elementId, paramNames)
	typeMap := make(map[string]string)
	for _, ep := range existingParams {
		if ep.ParamName != nil && ep.Type != nil && *ep.Type != "" {
			typeMap[*ep.ParamName] = *ep.Type
		}
	}
	for i := range params {
		if t, ok := typeMap[params[i].ParamName]; ok {
			params[i].ParamType = t
		} else {
			params[i].ParamType = "xsd:string"
		}
	}

	opParamJSON, _ := json.Marshal(params)

	eventLogId, err := s.repo.InsertEventLog("SetParameterValues", elementId, username, 1, string(opParamJSON))
	if err != nil {
		return fmt.Errorf("create event_log: %w", err)
	}

	now := time.Now()
	msg := operationMessage{
		EventType:      "SetParameterValues",
		NeNeid:         elementId,
		Operation:      "SetParameterValues",
		OperationParam: string(opParamJSON),
		OperationUser:  username,
		CommandTrackId: eventLogId,
		ExpiredAt:      now.Add(5 * time.Minute).UnixMilli(),
	}
	msgJSON, _ := json.Marshal(msg)

	ctx := context.Background()
	if err := redis.LPush(ctx, "operation_queue", string(msgJSON)); err != nil {
		return fmt.Errorf("push to operation_queue: %w", err)
	}

	redis.Set(ctx, fmt.Sprintf("cpe_in_diagnostics_%d", elementId), "yes", 5*time.Minute)
	redis.Set(ctx, fmt.Sprintf("dia_%s_%d", diagType, elementId), strconv.FormatInt(now.UnixMilli(), 10), 0)

	return nil
}

// ---------- Trigger diagnostics ----------

func (s *service) TriggerPing(req *PingRequest, username string) error {
	rootNode, err := s.repo.FindElementRootNode(req.ElementId)
	if err != nil {
		return err
	}
	if rootNode == "" {
		return fmt.Errorf("device not found or deleted")
	}

	var params []paramVo
	if rootNode == "Device" {
		params = []paramVo{
			{ParamName: "Device.IPPingDiagnostics.Host", ParamValue: req.Server},
			{ParamName: "Device.IPPingDiagnostics.DiagnosticsState", ParamValue: "Requested"},
			{ParamName: "Device.IPPingDiagnostics.NumberOfRepetitions", ParamValue: strconv.Itoa(req.Count)},
			{ParamName: "Device.IPPingDiagnostics.Timeout", ParamValue: "2000"},
			{ParamName: "Device.IPPingDiagnostics.DataBlockSize", ParamValue: "56"},
		}
	} else {
		p := "InternetGatewayDevice"
		params = []paramVo{
			{ParamName: p + ".IPPingDiagnostics.Host", ParamValue: req.Server},
			{ParamName: p + ".IPPingDiagnostics.DiagnosticsState", ParamValue: "Requested"},
			{ParamName: p + ".IPPingDiagnostics.NumberOfRepetitions", ParamValue: strconv.Itoa(req.Count)},
			{ParamName: p + ".IPPingDiagnostics.Timeout", ParamValue: "2000"},
			{ParamName: p + ".IPPingDiagnostics.DataBlockSize", ParamValue: "56"},
		}
	}
	return s.dispatchDiagnostics(req.ElementId, rootNode, username, params, "ping")
}

func (s *service) TriggerTraceRoute(req *TraceRouteRequest, username string) error {
	rootNode, err := s.repo.FindElementRootNode(req.ElementId)
	if err != nil {
		return err
	}
	if rootNode == "" {
		return fmt.Errorf("device not found or deleted")
	}

	var params []paramVo
	if rootNode == "Device" {
		params = []paramVo{
			{ParamName: "Device.TraceRouteDiagnostics.Host", ParamValue: req.Server},
			{ParamName: "Device.TraceRouteDiagnostics.DiagnosticsState", ParamValue: "Requested"},
			{ParamName: "Device.TraceRouteDiagnostics.NumberOfTries", ParamValue: "3"},
			{ParamName: "Device.TraceRouteDiagnostics.Timeout", ParamValue: "2000"},
			{ParamName: "Device.TraceRouteDiagnostics.DataBlockSize", ParamValue: "56"},
		}
	} else {
		p := "InternetGatewayDevice"
		params = []paramVo{
			{ParamName: p + ".TraceRouteDiagnostics.Host", ParamValue: req.Server},
			{ParamName: p + ".TraceRouteDiagnostics.DiagnosticsState", ParamValue: "Requested"},
			{ParamName: p + ".TraceRouteDiagnostics.NumberOfTries", ParamValue: "3"},
			{ParamName: p + ".TraceRouteDiagnostics.Timeout", ParamValue: "2000"},
			{ParamName: p + ".TraceRouteDiagnostics.DataBlockSize", ParamValue: "56"},
		}
	}
	return s.dispatchDiagnostics(req.ElementId, rootNode, username, params, "trace")
}

func (s *service) TriggerDownload(req *DownloadRequest, username string, fileServerIp string) error {
	rootNode, err := s.repo.FindElementRootNode(req.ElementId)
	if err != nil {
		return err
	}
	if rootNode == "" {
		return fmt.Errorf("device not found or deleted")
	}

	url := req.DownloadUrl
	if url == "" || strings.EqualFold(url, "http://") {
		url = fileServerIp + "/api/acs-file-server/testDownload?size=500"
	}

	var params []paramVo
	if rootNode == "Device" {
		params = []paramVo{
			{ParamName: "Device.DownloadDiagnostics.DownloadURL", ParamValue: url},
			{ParamName: "Device.DownloadDiagnostics.DiagnosticsState", ParamValue: "Requested"},
		}
	} else {
		params = []paramVo{
			{ParamName: "InternetGatewayDevice.DownloadDiagnostics.DownloadURL", ParamValue: url},
			{ParamName: "InternetGatewayDevice.DownloadDiagnostics.DiagnosticsState", ParamValue: "Requested"},
		}
	}
	return s.dispatchDiagnostics(req.ElementId, rootNode, username, params, "download")
}

func (s *service) TriggerUpload(req *UploadRequest, username string, fileServerIp string) error {
	rootNode, err := s.repo.FindElementRootNode(req.ElementId)
	if err != nil {
		return err
	}
	if rootNode == "" {
		return fmt.Errorf("device not found or deleted")
	}

	url := req.DownloadUrl
	if url == "" || strings.EqualFold(url, "http://") {
		url = fileServerIp + "/api/acs-file-server/testUpload"
	}
	fileSize := int64(10)
	if req.FileSize != nil {
		fileSize = *req.FileSize
	}
	fileLengthBytes := fileSize * 1024 * 1024

	var params []paramVo
	if rootNode == "Device" {
		params = []paramVo{
			{ParamName: "Device.UploadDiagnostics.UploadURL", ParamValue: url},
			{ParamName: "Device.UploadDiagnostics.DiagnosticsState", ParamValue: "Requested"},
			{ParamName: "Device.UploadDiagnostics.TestFileLength", ParamValue: strconv.FormatInt(fileLengthBytes, 10)},
		}
	} else {
		params = []paramVo{
			{ParamName: "InternetGatewayDevice.UploadDiagnostics.UploadURL", ParamValue: url},
			{ParamName: "InternetGatewayDevice.UploadDiagnostics.DiagnosticsState", ParamValue: "Requested"},
			{ParamName: "InternetGatewayDevice.UploadDiagnostics.TestFileLength", ParamValue: strconv.FormatInt(fileLengthBytes, 10)},
		}
	}
	return s.dispatchDiagnostics(req.ElementId, rootNode, username, params, "upload")
}

// ---------- Query results ----------

func (s *service) GetDiagnosticsStatus(elementId int64) bool {
	ctx := context.Background()
	val, err := redis.Get(ctx, fmt.Sprintf("cpe_in_diagnostics_%d", elementId))
	if err != nil {
		return false
	}
	return val == "yes"
}

func (s *service) GetDiagnosticsResult(elementId int64) (*DiagnosticsResultVO, error) {
	rootNode, err := s.repo.FindElementRootNode(elementId)
	if err != nil {
		return nil, err
	}
	if rootNode == "" {
		return nil, fmt.Errorf("device not found or deleted")
	}

	ctx := context.Background()
	result := &DiagnosticsResultVO{}
	result.Ping = s.buildPingResult(elementId, rootNode, ctx)
	result.Download = s.buildDownloadResult(elementId, rootNode, ctx)
	result.Upload = s.buildUploadResult(elementId, rootNode, ctx)
	result.Trace = s.buildTraceResult(elementId, rootNode, ctx)
	return result, nil
}

// ---------- Result builders ----------

func (s *service) buildPingResult(elementId int64, rootNode string, ctx context.Context) *PingResultVO {
	params, err := s.repo.FindParamsByElementIdAndNameLike(elementId, rootNode+".IPPingDiagnostics.%")
	if err != nil {
		logger.Errorf("find ping params: %v", err)
		return &PingResultVO{}
	}
	m := paramMap(params)
	ping := &PingResultVO{
		Server: m[rootNode+".IPPingDiagnostics.Host"],
		Status: m[rootNode+".IPPingDiagnostics.DiagnosticsState"],
	}
	if v := m[rootNode+".IPPingDiagnostics.SuccessCount"]; v != "" {
		if n, e := strconv.Atoi(v); e == nil {
			ping.SuccessCount = &n
		}
	}
	if v := m[rootNode+".IPPingDiagnostics.FailureCount"]; v != "" {
		if n, e := strconv.Atoi(v); e == nil {
			ping.FailureCount = &n
		}
	}
	if v := m[rootNode+".IPPingDiagnostics.AverageResponseTime"]; v != "" {
		if n, e := strconv.Atoi(v); e == nil {
			ping.AverageResponseTime = &n
		}
	}
	if v, _ := redis.Get(ctx, fmt.Sprintf("dia_ping_%d", elementId)); v != "" {
		if n, e := strconv.ParseInt(v, 10, 64); e == nil {
			ping.TestTime = n
		}
	}
	return ping
}

func (s *service) buildDownloadResult(elementId int64, rootNode string, ctx context.Context) *DownloadResultVO {
	params, err := s.repo.FindParamsByElementIdAndNameLike(elementId, rootNode+".DownloadDiagnostics.%")
	if err != nil {
		logger.Errorf("find download params: %v", err)
		return &DownloadResultVO{}
	}
	m := paramMap(params)
	dl := &DownloadResultVO{
		Server:  m[rootNode+".DownloadDiagnostics.DownloadURL"],
		Statues: m[rootNode+".DownloadDiagnostics.DiagnosticsState"],
	}
	if v, _ := redis.Get(ctx, fmt.Sprintf("dia_download_%d", elementId)); v != "" {
		if n, e := strconv.ParseInt(v, 10, 64); e == nil {
			dl.TestTime = n
		}
	}
	if v := m[rootNode+".DownloadDiagnostics.TotalBytesReceived"]; v != "" {
		if n, e := strconv.ParseFloat(v, 64); e == nil {
			fsize := n / 1024 / 1024
			dl.FileSize = &fsize
		}
	}
	dl.ConnectionTimes = computeTimeDiffMs(
		m[rootNode+".DownloadDiagnostics.TCPOpenRequestTime"],
		m[rootNode+".DownloadDiagnostics.TCPOpenResponseTime"],
	)
	dl.DownloadTimes = computeTimeDiffMs(
		m[rootNode+".DownloadDiagnostics.ROMTime"],
		m[rootNode+".DownloadDiagnostics.EOMTime"],
	)
	if dl.FileSize != nil && dl.DownloadTimes != nil && *dl.DownloadTimes > 0 {
		speed := *dl.FileSize / (float64(*dl.DownloadTimes) / 1000.0) / 0.125
		dl.SpeedSize = &speed
	}
	return dl
}

func (s *service) buildUploadResult(elementId int64, rootNode string, ctx context.Context) *UploadResultVO {
	params, err := s.repo.FindParamsByElementIdAndNameLike(elementId, rootNode+".UploadDiagnostics.%")
	if err != nil {
		logger.Errorf("find upload params: %v", err)
		return &UploadResultVO{}
	}
	m := paramMap(params)
	ul := &UploadResultVO{
		Server:  m[rootNode+".UploadDiagnostics.UploadURL"],
		Statues: m[rootNode+".UploadDiagnostics.DiagnosticsState"],
	}
	if v, _ := redis.Get(ctx, fmt.Sprintf("dia_upload_%d", elementId)); v != "" {
		if n, e := strconv.ParseInt(v, 10, 64); e == nil {
			ul.TestTime = n
		}
	}
	if v := m[rootNode+".UploadDiagnostics.TotalBytesSent"]; v != "" {
		if n, e := strconv.ParseFloat(v, 64); e == nil {
			fsize := n / 1024 / 1024
			ul.FileSize = &fsize
		}
	}
	ul.ConnectionTimes = computeTimeDiffMs(
		m[rootNode+".UploadDiagnostics.TCPOpenRequestTime"],
		m[rootNode+".UploadDiagnostics.TCPOpenResponseTime"],
	)
	ul.UploadTimes = computeTimeDiffMs(
		m[rootNode+".UploadDiagnostics.ROMTime"],
		m[rootNode+".UploadDiagnostics.EOMTime"],
	)
	if ul.FileSize != nil && ul.UploadTimes != nil && *ul.UploadTimes > 0 {
		speed := *ul.FileSize / (float64(*ul.UploadTimes) / 1000.0) / 0.125
		ul.SpeedSize = &speed
	}
	return ul
}

func (s *service) buildTraceResult(elementId int64, rootNode string, ctx context.Context) *TraceResultVO {
	params, err := s.repo.FindParamsByElementIdAndNameLike(elementId, rootNode+".TraceRouteDiagnostics.%")
	if err != nil {
		logger.Errorf("find trace params: %v", err)
		return &TraceResultVO{RouteVOS: []RouteVO{}}
	}
	m := paramMap(params)
	trace := &TraceResultVO{
		Server: m[rootNode+".TraceRouteDiagnostics.Host"],
		Status: m[rootNode+".TraceRouteDiagnostics.DiagnosticsState"],
	}
	if v, _ := redis.Get(ctx, fmt.Sprintf("dia_trace_%d", elementId)); v != "" {
		if n, e := strconv.ParseInt(v, 10, 64); e == nil {
			trace.TestTime = n
		}
	}

	indexSet := make(map[int]bool)
	for _, p := range params {
		if p.ParamName == nil {
			continue
		}
		matches := routeHopPattern.FindStringSubmatch(*p.ParamName)
		if len(matches) == 2 {
			if idx, e := strconv.Atoi(matches[1]); e == nil {
				indexSet[idx] = true
			}
		}
	}
	indexes := make([]int, 0, len(indexSet))
	for idx := range indexSet {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)

	routes := make([]RouteVO, 0, len(indexes))
	for _, idx := range indexes {
		pfx := fmt.Sprintf("%s.TraceRouteDiagnostics.RouteHops.%d.", rootNode, idx)
		host := m[pfx+"HopHost"]
		if host == "" {
			continue
		}
		rv := RouteVO{
			HopHost:        host,
			HopHostAddress: m[pfx+"HopHostAddress"],
			HopRTTimes:     m[pfx+"HopRTTimes"],
		}
		if v := m[pfx+"HopErrorCode"]; v != "" {
			if n, e := strconv.Atoi(v); e == nil {
				rv.HopErrorCode = &n
			}
		}
		routes = append(routes, rv)
	}
	trace.RouteVOS = routes
	return trace
}

// ---------- Helpers ----------

func paramMap(params []ParameterValue) map[string]string {
	m := make(map[string]string, len(params))
	for _, p := range params {
		if p.ParamName != nil && p.ParamValue != nil {
			m[*p.ParamName] = *p.ParamValue
		}
	}
	return m
}

func computeTimeDiffMs(startStr, endStr string) *int64 {
	if startStr == "" || endStr == "" {
		return nil
	}
	layout := "2006-01-02T15:04:05.000000"
	start, err1 := time.Parse(layout, startStr)
	end, err2 := time.Parse(layout, endStr)
	if err1 != nil || err2 != nil {
		return nil
	}
	diff := end.Sub(start).Milliseconds()
	return &diff
}
