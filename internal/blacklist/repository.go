package blacklist

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/pkg/baserepo"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// Repository provides data access for blacklist management.
// It embeds BaseRepository[ElementBlackList, int] for standard CRUD,
// and retains module-specific methods for custom queries.
type Repository struct {
	*baserepo.BaseRepository[ElementBlackList, int]
	db *gorm.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		BaseRepository: baserepo.New[ElementBlackList, int](db, "id"),
		db:             db,
	}
}

// FindBySNAndDeviceType returns an existing entry (duplicate check).
func (r *Repository) FindBySNAndDeviceType(licenseId int, sn, deviceType string) (*ElementBlackList, error) {
	var entry ElementBlackList
	err := r.db.Where("license_id = ? AND sn = ? AND device_type = ?", licenseId, sn, deviceType).First(&entry).Error
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// List returns paginated blacklist entries.
func (r *Repository) List(licenseId int, query ListBlackListQuery) ([]ListDeviceBlackListVO, int64, error) {
	page, pageSize := query.Page, query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	baseWhere := "license_id = ?"
	args := []interface{}{licenseId}
	if query.SN != "" {
		baseWhere += " AND sn LIKE ?"
		args = append(args, "%"+query.SN+"%")
	}

	var total int64
	r.db.Model(&ElementBlackList{}).Where(baseWhere, args...).Count(&total)

	var entries []ElementBlackList
	r.db.Where(baseWhere, args...).
		Order("add_time DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&entries)

	tenancyNames := r.getTenancyNames()

	vos := make([]ListDeviceBlackListVO, len(entries))
	for i, e := range entries {
		vos[i] = ListDeviceBlackListVO{
			Id:          e.Id,
			SN:          e.SN,
			Username:    e.Username,
			AddTime:     e.AddTime,
			DeviceType:  e.DeviceType,
			TenancyName: tenancyNames[e.LicenseId],
			Reason:      e.Reason,
		}
	}
	return vos, total, nil
}

// InsertOperationLog creates a blacklist operation log entry.
func (r *Repository) InsertOperationLog(log *BlackListOperationLog) error {
	return r.db.Create(log).Error
}

// ListOperationLogs returns paginated operation logs.
func (r *Repository) ListOperationLogs(licenseId int, query ListBlackListOperationLogQuery) ([]ListBlackListOperationLogVO, int64, error) {
	page, pageSize := query.Page, query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	baseWhere := "license_id = ?"
	args := []interface{}{licenseId}
	if query.DeviceSN != "" {
		baseWhere += " AND device_sn LIKE ?"
		args = append(args, "%"+query.DeviceSN+"%")
	}
	if query.OperationType != "" {
		baseWhere += " AND operation_type = ?"
		args = append(args, query.OperationType)
	}
	if query.DeviceType != "" {
		baseWhere += " AND device_type = ?"
		args = append(args, query.DeviceType)
	}
	if query.SearchText != "" {
		baseWhere += " AND (device_sn LIKE ? OR operator_username LIKE ?)"
		args = append(args, "%"+query.SearchText+"%", "%"+query.SearchText+"%")
	}
	if query.StartTime != nil {
		if t, err := time.Parse(time.RFC3339, *query.StartTime); err == nil {
			baseWhere += " AND operation_time >= ?"
			args = append(args, t)
		}
	}
	if query.EndTime != nil {
		if t, err := time.Parse(time.RFC3339, *query.EndTime); err == nil {
			baseWhere += " AND operation_time <= ?"
			args = append(args, t)
		}
	}

	var total int64
	r.db.Model(&BlackListOperationLog{}).Where(baseWhere, args...).Count(&total)

	var logs []BlackListOperationLog
	r.db.Where(baseWhere, args...).
		Order("operation_time DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs)

	tenancyNames := r.getTenancyNames()

	vos := make([]ListBlackListOperationLogVO, len(logs))
	for i, l := range logs {
		vos[i] = ListBlackListOperationLogVO{
			Id:               l.Id,
			DeviceSN:         l.DeviceSN,
			DeviceType:       l.DeviceType,
			OperationType:    l.OperationType,
			OperatorUsername: l.OperatorUsername,
			OperationTime:    l.OperationTime,
			OperationReason:  l.OperationReason,
			TenancyName:      tenancyNames[l.LicenseId],
		}
	}
	return vos, total, nil
}

// SetRedisBlackListKey sets the Redis blacklist key for a device.
func SetRedisBlackListKey(deviceType, sn string) {
	ctx := context.Background()
	key := fmt.Sprintf("black_list_%s%s", deviceType, sn)
	if err := redis.Set(ctx, key, "y", 0); err != nil {
		logger.Errorf("blacklist: set redis key %s: %v", key, err)
	}
}

// DeleteRedisBlackListKey removes the Redis blacklist key for a device.
func DeleteRedisBlackListKey(deviceType, sn string) {
	ctx := context.Background()
	key := fmt.Sprintf("black_list_%s%s", deviceType, sn)
	if err := redis.Del(ctx, key); err != nil {
		logger.Errorf("blacklist: delete redis key %s: %v", key, err)
	}
}

// LoadAllToRedis loads all blacklist entries from DB into Redis (for startup warm-up).
func (r *Repository) LoadAllToRedis() error {
	var entries []ElementBlackList
	if err := r.db.Find(&entries).Error; err != nil {
		return err
	}
	for _, e := range entries {
		SetRedisBlackListKey(e.DeviceType, e.SN)
	}
	logger.Infof("blacklist: loaded %d entries to Redis", len(entries))
	return nil
}

// ---------- helpers ----------

func (r *Repository) getTenancyNames() map[int]string {
	m := make(map[int]string)
	type row struct {
		Id   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	var rows []row
	r.db.Table("tenancy").Select("id, name").Scan(&rows)
	for _, row := range rows {
		m[row.Id] = row.Name
	}
	return m
}
