package monitor

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/internal/parameter"
	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
)

// monitorOpPrefix is the operationId prefix the collector uses when issuing
// GetParameterValues for a monitor task, so the tr069 response handler can route
// the response back here (via tr069.MonitorGPVCallback).
const monitorOpPrefix = "monitor:"

// Collector drives the periodic monitor-data ingestion: for each enabled monitor
// task it resolves the target devices and their parameter TR-069 paths, issues a
// GetParameterValues to each device, and persists the responses into monitor_data
// via the tr069 MonitorGPVCallback hook. This mirrors Java's MonitorValueTask
// (5-min cron) + GetCpeStatisticMessageProcessor (response -> MonitorData rows).
type Collector struct {
	repo Repository
	db   *gorm.DB
}

// NewCollector creates a monitor ingestion collector bound to the given DB.
func NewCollector(db *gorm.DB) *Collector {
	return &Collector{repo: NewRepository(db), db: db}
}

// RunOnce executes a single collection pass across all enabled monitor tasks.
func (c *Collector) RunOnce() {
	if tr069.DefaultSender == nil {
		logger.Errorf("monitor collector: tr069.DefaultSender not wired; skipping collection")
		return
	}
	tasks, err := c.repo.FindEnabledMonitorTasks()
	if err != nil {
		logger.Errorf("monitor collector: failed to load enabled tasks: %v", err)
		return
	}
	for i := range tasks {
		c.collectTask(tasks[i])
	}
}

func (c *Collector) collectTask(t MonitorTask) {
	taskID := t.Id
	params, err := c.repo.FindMonitorParameters(taskID)
	if err != nil || len(params) == 0 {
		return
	}
	paramIDs := make([]string, 0, len(params))
	for _, p := range params {
		if p.ParameterId != nil {
			paramIDs = append(paramIDs, *p.ParameterId)
		}
	}
	paths, err := c.parameterPaths(paramIDs)
	if err != nil || len(paths) == 0 {
		return
	}
	elements, err := c.resolveElements(t)
	if err != nil || len(elements) == 0 {
		return
	}
	for _, eid := range elements {
		sn, ok := c.elementSerial(eid)
		if !ok {
			continue
		}
		opID := fmt.Sprintf("%s%d:%d", monitorOpPrefix, taskID, eid)
		if err := tr069.DefaultSender.SendGetParameterValues(sn, paths, opID); err != nil {
			logger.Warnf("monitor collector: failed to send GPV task=%d elem=%d sn=%s: %v", taskID, eid, sn, err)
		}
	}
}

// parameterPaths returns the TR-069 paths for the given parameter IDs.
func (c *Collector) parameterPaths(paramIDs []string) ([]string, error) {
	var params []parameter.Parameter
	if err := c.db.Where("id IN ?", paramIDs).Find(&params).Error; err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(params))
	for _, p := range params {
		if p.Path != nil && *p.Path != "" {
			paths = append(paths, *p.Path)
		}
	}
	return paths, nil
}

// resolveElements resolves the target device element IDs for a monitor task based on
// execution_scope + scope_data (mirrors Java MonitorValueTask semantics):
//   0 -> all elements of the tenant
//   1 -> by device group (scope_data = JSON list of group IDs)
//   2/3/4 -> explicit element IDs (scope_data = JSON list of ne_neid)
func (c *Collector) resolveElements(t MonitorTask) ([]int64, error) {
	scope := 0
	if t.ExecutionScope != nil {
		scope = *t.ExecutionScope
	}
	tenantID := 0
	if t.TenantId != nil {
		tenantID = *t.TenantId
	}
	switch scope {
	case 0:
		var ids []int64
		q := c.db.Table("cpe_element").Select("ne_neid").Where("deleted = ?", false)
		if tenantID != 0 {
			q = q.Where("tenant_id = ?", tenantID)
		}
		if err := q.Pluck("ne_neid", &ids).Error; err != nil {
			return nil, err
		}
		return ids, nil
	case 1:
		groupIDs := parseStringList(t.ScopeData)
		if len(groupIDs) == 0 {
			return nil, nil
		}
		var ids []int64
		if err := c.db.Table("group_has_element").Select("element_id").
			Where("group_id IN ?", groupIDs).Pluck("element_id", &ids).Error; err != nil {
			return nil, err
		}
		return ids, nil
	default:
		return parseInt64List(t.ScopeData), nil
	}
}

func (c *Collector) elementSerial(eid int64) (string, bool) {
	var sn string
	err := c.db.Table("cpe_element").Select("serial_number").
		Where("ne_neid = ? AND deleted = ?", eid, false).Pluck("serial_number", &sn).Error
	if err != nil || sn == "" {
		return "", false
	}
	return sn, true
}

// HandleGPVResponse is registered as tr069.MonitorGPVCallback. It maps the device
// response (path->value) back to the monitor task's parameterIds, averages
// multi-instance values (mirrors Java {i} expansion), and writes monitor_data rows.
func (c *Collector) HandleGPVResponse(sn, operationId string, values []soap.ParameterValueStruct) {
	rest := strings.TrimPrefix(operationId, monitorOpPrefix)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return
	}
	taskID, err1 := strconv.Atoi(parts[0])
	elemID, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil {
		return
	}

	params, err := c.repo.FindMonitorParameters(taskID)
	if err != nil || len(params) == 0 {
		return
	}
	pids := make([]string, 0, len(params))
	for _, p := range params {
		if p.ParameterId != nil {
			pids = append(pids, *p.ParameterId)
		}
	}
	var dbParams []parameter.Parameter
	if err := c.db.Where("id IN ?", pids).Find(&dbParams).Error; err != nil {
		return
	}
	pathToPID := make(map[string]string, len(dbParams))
	for _, p := range dbParams {
		if p.Path != nil && *p.Path != "" {
			// Strip the {i} placeholder so multi-instance base paths (e.g.
			// "Device.boats.{i}" -> "Device.boats.") can prefix-match instance names.
			base := strings.ReplaceAll(*p.Path, "{i}", "")
			pathToPID[base] = p.Id
		}
	}

	agg := aggregateMonitorValues(values, pathToPID)
	if len(agg) == 0 {
		return
	}

	now := time.Now()
	rows := make([]MonitorData, 0, len(agg))
	for pid, avg := range agg {
		pidC := pid
		eidC := elemID
		nowC := now
		rows = append(rows, MonitorData{
			ElementId:   &eidC,
			SampleTime:  &nowC,
			ParameterId: &pidC,
			Value:       &avg,
		})
	}
	if err := c.repo.SaveMonitorData(rows); err != nil {
		logger.Errorf("monitor collector: failed to save monitor_data task=%d elem=%d: %v", taskID, elemID, err)
	}
}

// aggregateMonitorValues maps device response values (path->value) to parameterIds
// and averages multi-instance values (path "X.i" grouped under parameterId of "X"),
// mirroring Java's running-mean for {i}-expanded parameters.
func aggregateMonitorValues(values []soap.ParameterValueStruct, pathToPID map[string]string) map[string]float64 {
	sums := make(map[string]float64)
	counts := make(map[string]int)
	for _, v := range values {
		pid, ok := matchParam(v.Name, pathToPID)
		if !ok {
			continue
		}
		f, err := strconv.ParseFloat(v.Value, 64)
		if err != nil {
			continue
		}
		sums[pid] += f
		counts[pid]++
	}
	out := make(map[string]float64, len(sums))
	for pid, s := range sums {
		out[pid] = s / float64(counts[pid])
	}
	return out
}

// matchParam returns the parameterId whose path equals name, or (for multi-instance
// base paths ending in ".") whose path is a prefix of name.
func matchParam(name string, pathToPID map[string]string) (string, bool) {
	if pid, ok := pathToPID[name]; ok {
		return pid, true
	}
	for path, pid := range pathToPID {
		// Multi-instance base path (e.g. "Device.boats." after {i} is stripped)
		// matches instance names like "Device.boats.1".
		if strings.HasSuffix(path, ".") && strings.HasPrefix(name, path) && len(name) > len(path) {
			return pid, true
		}
	}
	return "", false
}

// Cleanup deletes monitor_data samples older than the given time (retention pruning).
func (c *Collector) Cleanup(before time.Time) (int64, error) {
	return c.repo.DeleteMonitorDataBefore(before)
}

func parseStringList(s *string) []string {
	if s == nil || *s == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(*s), &out); err != nil {
		return nil
	}
	return out
}

func parseInt64List(s *string) []int64 {
	if s == nil || *s == "" {
		return nil
	}
	var out []int64
	if err := json.Unmarshal([]byte(*s), &out); err != nil {
		return nil
	}
	return out
}
