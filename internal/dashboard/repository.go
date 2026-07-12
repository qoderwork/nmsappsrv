package dashboard

import (
	"context"
	"strings"
	"time"

	"nmsappsrv/pkg/apperror"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for Dashboard module.
type Repository interface {
	ListDevicesByMode(ctx context.Context, mode string, tenancyId *int) ([]map[string]interface{}, error)
	ListAllDevices(ctx context.Context, tenancyId *int) ([]map[string]interface{}, error)
	ListPDCPTraffic(ctx context.Context, startTime, endTime string, tenancyId *int) ([]map[string]interface{}, error)
	GetDeviceByIds(ctx context.Context, ids []int64) ([]map[string]interface{}, error)
	QueryOnlineStatistics(ctx context.Context, elementIds []int64, startTime, endTime time.Time, deviceType string) ([]map[string]interface{}, error)
	QueryDashboardPmData(ctx context.Context, tenancyId *int, startTime, endTime time.Time) ([]map[string]interface{}, error)
	QueryDeviceGroupsByIds(ctx context.Context, groupIds []string) ([]map[string]interface{}, error)
	QueryGroupElementIds(ctx context.Context, groupId string) ([]int64, error)
	QueryDeviceStatisticForGroup(ctx context.Context, elementIds []int64, startTime, endTime time.Time) ([]map[string]interface{}, error)
}

// repository is the concrete GORM-backed implementation of Repository.
type repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// ListDevicesByMode returns devices filtered by mode and tenancyId
func (r *repository) ListDevicesByMode(ctx context.Context, mode string, tenancyId *int) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	q := r.db.WithContext(ctx).Table("cpe_element").
		Select("model_name, ne_neid").
		Where("deleted = ?", false)

	if tenancyId != nil {
		q = q.Where("license_id = ?", *tenancyId)
	}

	switch mode {
	case "CPE":
		q = q.Where("device_type = ?", "cpe")
	case "eNB":
		q = q.Where("device_type = ? AND generation = ?", "enb", "LTE")
	case "gNB":
		q = q.Where("device_type = ? AND generation = ?", "enb", "NR")
	default:
		return nil, apperror.New("INVALID_MODE", 400, "invalid mode: "+mode)
	}

	err := q.Scan(&results).Error
	return results, err
}

// ListAllDevices returns all non-deleted devices with type info
func (r *repository) ListAllDevices(ctx context.Context, tenancyId *int) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	q := r.db.WithContext(ctx).Table("cpe_element").
		Select("ne_neid, device_type, generation").
		Where("deleted = ?", false)

	if tenancyId != nil {
		q = q.Where("license_id = ?", *tenancyId)
	}

	err := q.Scan(&results).Error
	return results, err
}

// ListPDCPTraffic returns PDCP traffic statistics grouped by PLMN
func (r *repository) ListPDCPTraffic(ctx context.Context, startTime, endTime string, tenancyId *int) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	sql := `SELECT plmn, SUM(dl_traffic), SUM(ul_traffic) FROM pdcp_traffic WHERE `
	args := []interface{}{}

	if tenancyId != nil {
		sql += "tenancy_id = ? AND "
		args = append(args, *tenancyId)
	} else {
		sql += "tenancy_id IS NULL AND "
	}

	if startTime != "" {
		sql += "statistic_time >= ? AND "
		args = append(args, startTime)
	}
	if endTime != "" {
		sql += "statistic_time < ? AND "
		args = append(args, endTime)
	}

	// Remove trailing " AND "
	sql = strings.TrimSuffix(sql, " AND ")
	sql += " GROUP BY plmn"

	err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&results).Error
	return results, err
}

// GetDeviceByIds returns devices by their IDs
func (r *repository) GetDeviceByIds(ctx context.Context, ids []int64) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	err := r.db.WithContext(ctx).Table("cpe_element").
		Select("ne_neid, serial_number").
		Where("ne_neid IN ?", ids).
		Scan(&results).Error
	return results, err
}

// QueryOnlineStatistics queries device_statistic joined with cpe_element to get online/offline
// records for devices of a given type within a time range.
func (r *repository) QueryOnlineStatistics(ctx context.Context, elementIds []int64, startTime, endTime time.Time, deviceType string) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	// Determine the device_type and generation filter for cpe_element
	var typeFilter string
	var genFilter string
	switch deviceType {
	case "cpe":
		typeFilter = "cpe"
	case "gnb":
		typeFilter = "enb"
		genFilter = "NR"
	default:
		typeFilter = deviceType
	}

	sql := `SELECT ds.statistic_time, ds.online
		FROM device_statistic ds
		INNER JOIN cpe_element ce ON ds.element_id = ce.ne_neid
		WHERE ce.deleted = ?
		AND ce.device_type = ?
		AND ds.statistic_time >= ?
		AND ds.statistic_time <= ?`

	args := []interface{}{false, typeFilter, startTime, endTime}

	if genFilter != "" {
		sql += " AND ce.generation = ?"
		args = append(args, genFilter)
	}

	if len(elementIds) > 0 {
		sql += " AND ds.element_id IN ?"
		args = append(args, elementIds)
	}

	sql += " ORDER BY ds.statistic_time ASC"

	err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&results).Error
	return results, err
}

// QueryDashboardPmData queries dashboard_pm_statistic_data for a time range and optional tenancyId
func (r *repository) QueryDashboardPmData(ctx context.Context, tenancyId *int, startTime, endTime time.Time) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	q := r.db.WithContext(ctx).Table("dashboard_pm_statistic_data").
		Select("`time`, cell_available_rate, pdcp_ul_rate, pdcp_dl_rate").
		Where("`time` >= ? AND `time` <= ?", startTime, endTime)

	if tenancyId != nil {
		q = q.Where("tenancy_id = ?", *tenancyId)
	}

	q = q.Order("`time` ASC")

	err := q.Scan(&results).Error
	return results, err
}

// QueryDeviceGroupsByIds queries device_group by group IDs
func (r *repository) QueryDeviceGroupsByIds(ctx context.Context, groupIds []string) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	err := r.db.WithContext(ctx).Table("device_group").
		Select("id, group_name, license_id").
		Where("id IN ?", groupIds).
		Scan(&results).Error

	return results, err
}

// QueryGroupElementIds queries element IDs belonging to a device group via group_has_element join
func (r *repository) QueryGroupElementIds(ctx context.Context, groupId string) ([]int64, error) {
	var elementIds []int64

	err := r.db.WithContext(ctx).Table("group_has_element").
		Where("group_id = ?", groupId).
		Pluck("element_id", &elementIds).Error

	return elementIds, err
}

// QueryDeviceStatisticForGroup queries device_statistic for elements in a group within a time range.
// Returns rows with statistic_time, online, element_id.
func (r *repository) QueryDeviceStatisticForGroup(ctx context.Context, elementIds []int64, startTime, endTime time.Time) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	if len(elementIds) == 0 {
		return results, nil
	}

	sql := `SELECT ds.statistic_time, ds.online, ds.element_id
		FROM device_statistic ds
		WHERE ds.element_id IN ?
		AND ds.statistic_time >= ?
		AND ds.statistic_time <= ?
		ORDER BY ds.statistic_time ASC`

	err := r.db.WithContext(ctx).Raw(sql, elementIds, startTime, endTime).Scan(&results).Error
	return results, err
}
