package scheduledtask

import (
	"time"

	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// EventLogTimeoutTask 将超时的操作日志标记为超时（status 从 1 改为 3）
// 镜像 Java EventLogTimeoutTask
type EventLogTimeoutTask struct {
	db           *gorm.DB
	timeoutHours int // 响应超时小时数，默认 24
}

// NewEventLogTimeoutTask 创建 EventLogTimeoutTask 实例
func NewEventLogTimeoutTask(db *gorm.DB, timeoutHours int) *EventLogTimeoutTask {
	if timeoutHours <= 0 {
		timeoutHours = 24
	}
	return &EventLogTimeoutTask{
		db:           db,
		timeoutHours: timeoutHours,
	}
}

// UpdateTimeoutStatus 执行超时状态更新
// 分为两个阶段：
// 1. 发送超时：6分钟前未发送的日志 → status=3
// 2. 响应超时：超过 timeoutHours 未响应的日志 → status=3
func (t *EventLogTimeoutTask) UpdateTimeoutStatus() {
	now := time.Now()
	sendTimeout := now.Add(-6 * time.Minute)
	responseTimeout := now.Add(-time.Duration(t.timeoutHours) * time.Hour)
	restoreResponseTimeout := now.Add(-10 * time.Minute)

	// ---- 阶段1: 发送超时（6 分钟） ----
	// upgrade_log: status=1 且 operation_time < now-6min → status=3
	t.markSendTimeout("upgrade_log", sendTimeout)

	// manual_upgrade_log: status=1 且 operation_time < now-6min → status=3
	t.markSendTimeout("manual_upgrade_log", sendTimeout)

	// restore_and_backup_device_log: status=1 且 operation_time < now-6min → status=3
	t.markSendTimeout("restore_and_back_up_device_log", sendTimeout)

	// device_send_ca_log: status=1 且 operation_time < now-6min → status=3
	t.markSendTimeout("device_send_ca_log", sendTimeout)

	// event_log: status=1 且 operation_time < now-6min → status=3
	t.markSendTimeout("event_log", sendTimeout)

	// mml_execute_result: status=1 且 create_time < now-6min → status=3
	t.markSendTimeoutByCreateTime("mml_execute_result", sendTimeout)

	// ---- 阶段2: 响应超时 ----
	// upgrade_log: status=1 且 operation_time < now-timeoutHours → status=3
	t.markResponseTimeout("upgrade_log", responseTimeout)

	// manual_upgrade_log: status=1 且 operation_time < now-timeoutHours → status=3
	t.markResponseTimeout("manual_upgrade_log", responseTimeout)

	// restore_and_backup_device_log: status=1 且 operation_time < now-10min → status=3
	t.markResponseTimeout("restore_and_back_up_device_log", restoreResponseTimeout)
}

// markSendTimeout 将 operation_time 超过 sendTimeout 且 status=1 的记录标记为 status=3
func (t *EventLogTimeoutTask) markSendTimeout(table string, sendTimeout time.Time) {
	result := t.db.Table(table).
		Where("status = ? AND operation_time < ?", 1, sendTimeout).
		Update("status", 3)
	if result.Error != nil {
		logger.Errorf("EventLogTimeoutTask: failed to mark send timeout for table %s: %v", table, result.Error)
		return
	}
	if result.RowsAffected > 0 {
		logger.Infof("EventLogTimeoutTask: marked %d rows as send timeout in %s", result.RowsAffected, table)
	}
}

// markSendTimeoutByCreateTime 将 create_time 超过 sendTimeout 且 status=1 的记录标记为 status=3
// 用于 mml_execute_result 表（使用 create_time 而非 operation_time）
func (t *EventLogTimeoutTask) markSendTimeoutByCreateTime(table string, sendTimeout time.Time) {
	result := t.db.Table(table).
		Where("status = ? AND create_time < ?", 1, sendTimeout).
		Update("status", 3)
	if result.Error != nil {
		logger.Errorf("EventLogTimeoutTask: failed to mark send timeout for table %s (by create_time): %v", table, result.Error)
		return
	}
	if result.RowsAffected > 0 {
		logger.Infof("EventLogTimeoutTask: marked %d rows as send timeout in %s (by create_time)", result.RowsAffected, table)
	}
}

// markResponseTimeout 将 operation_time 超过 responseTimeout 且 status=1 的记录标记为 status=3
func (t *EventLogTimeoutTask) markResponseTimeout(table string, responseTimeout time.Time) {
	result := t.db.Table(table).
		Where("status = ? AND operation_time < ?", 1, responseTimeout).
		Update("status", 3)
	if result.Error != nil {
		logger.Errorf("EventLogTimeoutTask: failed to mark response timeout for table %s: %v", table, result.Error)
		return
	}
	if result.RowsAffected > 0 {
		logger.Infof("EventLogTimeoutTask: marked %d rows as response timeout in %s", result.RowsAffected, table)
	}
}
