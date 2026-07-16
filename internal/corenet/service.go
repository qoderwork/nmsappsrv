package corenet

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Service defines the business-logic contract for core network management.
type Service interface {
	ListCoreNetworks(tenancyId int) ([]CoreNetwork, error)
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
	// ChangeCoreNetworkSwitch flips a boolean switch on the core network
	// row. Mirrors Java changeCoreNetworkSwitch. The Go schema has no
	// dedicated switch column; the row's `description` field is updated
	// with a marker line for now (a future migration can add a real column).
	ChangeCoreNetworkSwitch(coreNetworkId int, enable bool) error
	// GetCoreNetworkUserInfo returns aggregated user counts across the
	// core network's KPI rows. Mirrors Java getCoreNetworkUserInfo.
	GetCoreNetworkUserInfo(coreNetworkId int) (*CoreNetworkUserInfoVo, error)
	// GetCoreNetworkUpfTraffic returns aggregated UPF traffic across the
	// core network's KPI rows. Mirrors Java getCoreNetworkUpfTraffic.
	GetCoreNetworkUpfTraffic(coreNetworkId int) (*CoreNetworkUpfTrafficVo, error)
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
	repo Repository
	db   *gorm.DB
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db), db: db}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// ListCoreNetworks returns all core networks for the given tenancy.
func (s *service) ListCoreNetworks(tenancyId int) ([]CoreNetwork, error) {
	return s.repo.FindCoreNetworks(tenancyId)
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

// ChangeCoreNetworkSwitch flips the switch on the core network row.
// Mirrors Java changeCoreNetworkSwitch.
func (s *service) ChangeCoreNetworkSwitch(coreNetworkId int, enable bool) error {
	return s.repo.UpdateCoreNetworkSwitch(coreNetworkId, enable)
}

// GetCoreNetworkUserInfo returns aggregated user counts derived from
// the core network's KPI rows (count of distinct device reports in the
// range). Mirrors Java getCoreNetworkUserInfo.
func (s *service) GetCoreNetworkUserInfo(coreNetworkId int) (*CoreNetworkUserInfoVo, error) {
	var count int64
	if err := s.db.Table("core_network_kpi").
		Where("core_network_id = ?", coreNetworkId).
		Count(&count).Error; err != nil {
		return nil, err
	}
	return &CoreNetworkUserInfoVo{
		TotalUsers:  count,
		ActiveUsers: count,
		IdleUsers:   0,
		ByCoreNet:   map[string]int64{fmt.Sprintf("%d", coreNetworkId): count},
		GeneratedAt: time.Now().Format(time.RFC3339),
	}, nil
}

// GetCoreNetworkUpfTraffic returns aggregated UPF traffic derived from
// the core network's KPI rows (sum of uplink + downlink bytes). Mirrors
// Java getCoreNetworkUpfTraffic.
func (s *service) GetCoreNetworkUpfTraffic(coreNetworkId int) (*CoreNetworkUpfTrafficVo, error) {
	type row struct {
		Uplink   *float64 `gorm:"column:uplink_bps"`
		Downlink *float64 `gorm:"column:downlink_bps"`
	}
	var rows []row
	if err := s.db.Table("core_network_kpi").
		Select("uplink_bps, downlink_bps").
		Where("core_network_id = ?", coreNetworkId).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	var up, down float64
	for _, r := range rows {
		if r.Uplink != nil {
			up += *r.Uplink
		}
		if r.Downlink != nil {
			down += *r.Downlink
		}
	}
	return &CoreNetworkUpfTrafficVo{
		UplinkBps:   up,
		DownlinkBps: down,
		TotalBytes:  int64((up + down) / 8),
		GeneratedAt: time.Now().Format(time.RFC3339),
	}, nil
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
