package device

import (
	"context"
	"time"

	"nmsappsrv/pkg/constants"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// FillDeviceStatistic snapshots the current online/active/amf_active status of
// every non-deleted device into the device_statistic table for the given
// statistic-time bucket. It is registered as a scheduled job so the dashboard's
// online-statistics endpoints (which read device_statistic) have data to
// aggregate.
//
// The dashboard buckets by hour, so callers should pass an hour-aligned bucket
// (one row per device per hour). The write is idempotent for a given bucket:
// any existing rows for that exact statistic_time are deleted before insert, so
// re-running (hourly job, restart backfill) replaces the snapshot instead of
// duplicating it and inflating the per-bucket counts.
//
// Online determination mirrors offline_worker's liveness detection:
//   - primary:  the TR-069 online key (constants.RedisKeyDeviceOnline), refreshed
//     on every Inform by the tr069 layer;
//   - fallback: the heartbeat last-seen key (constants.RedisKeyHeartbeat, TTL-scoped)
//     for SAS/CBSD devices that report via heartbeat instead of TR-069.
//
// active and amf_active are currently mapped to the online signal. Refining
// amf_active to a distinct 5G AMF-registration state is a follow-up (requires a
// dedicated liveness source).
func FillDeviceStatistic(ctx context.Context, db *gorm.DB, bucket time.Time) (int, error) {
	var elems []CpeElement
	if err := db.WithContext(ctx).
		Select("ne_neid", "serial_number").
		Where("deleted = ?", false).
		Find(&elems).Error; err != nil {
		return 0, err
	}

	rows := make([]DeviceStatistic, 0, len(elems))
	for i := range elems {
		online := isDeviceOnline(ctx, elems[i].SerialNumber)
		eid := elems[i].NeNeid
		o := online
		a := online
		amf := online
		bt := bucket
		rows = append(rows, DeviceStatistic{
			ElementId:     &eid,
			Online:        &o,
			Active:        &a,
			AmfActive:     &amf,
			StatisticTime: &bt,
		})
	}

	if len(rows) == 0 {
		return 0, nil
	}

	// Idempotent per bucket: drop the previous snapshot for this hour first.
	if err := db.WithContext(ctx).
		Where("statistic_time = ?", bucket).
		Delete(&DeviceStatistic{}).Error; err != nil {
		return 0, err
	}

	if err := db.WithContext(ctx).Create(&rows).Error; err != nil {
		return 0, err
	}

	logger.Infof("device-statistic: wrote %d device snapshots for bucket %s", len(rows), bucket.Format(time.RFC3339))
	return len(rows), nil
}

// isDeviceOnline reports whether a device is currently reachable. Mirrors
// offline_worker's liveness detection (TR-069 online key first, heartbeat key
// as fallback).
func isDeviceOnline(ctx context.Context, sn *string) bool {
	if sn == nil || *sn == "" {
		return false
	}
	if redis.Exists(ctx, constants.RedisKeyDeviceOnline+*sn) {
		return true
	}
	if redis.Exists(ctx, constants.RedisKeyHeartbeat+*sn) {
		return true
	}
	return false
}
