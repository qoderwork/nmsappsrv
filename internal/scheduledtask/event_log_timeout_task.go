package scheduledtask

import (
	"time"

	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// EventLogTimeoutTask 镜像 Java EventLogTimeoutTask：将超时的操作日志标记为
// 超时（成功失败/原因填充）。和 Java 实现一致，所有更新都是基于 event_log
// 的 status=1 + operation_time 进行子查询关联（不是简单按各表 status 字段
// 更新），所以这里不依赖 upgrade_log/manual_upgrade_log/... 自身的 status
// 字段存在性。
//
// Java 原生 SQL 参考（见 EventLogTimeoutTask.java + 各 ServiceImpl）:
//   - upgrade_log send_timeout:
//       UPDATE upgrade_log SET success=0, message='Command delivery timeout',
//         done_time=now() WHERE command_track_id IN
//         (SELECT id FROM event_log WHERE status=1 AND operation_time < ?)
//   - upgrade_log response_timeout (24h):
//       upgrade=1: ...SET is_done=1, success=0, message='Upgrade timeout'...
//                  WHERE upgrade=1 AND success IS NULL AND creation_time < ?
//       upgrade=0: ...SET is_done=1, success=0, message='Rollback timeout'...
//                  WHERE upgrade=0 AND success IS NULL AND creation_time < ?
//   - manual_upgrade_log send_timeout:
//       UPDATE manual_upgrade_log SET success=0, info='Command delivery timeout'
//         WHERE event_log_id IN (SELECT id FROM event_log WHERE status=1
//         AND operation_time < ?)
//   - manual_upgrade_log response_timeout (24h):
//       UPDATE manual_upgrade_log SET success=0, info='Download timeout'
//         WHERE success IS NULL AND start_time < ?
//   - restore_and_back_up_device_log send_timeout:
//       UPDATE restore_and_back_up_device_log SET results=2,
//         failure_reason='Command delivery timeout' WHERE event_log_id IN
//         (SELECT id FROM event_log WHERE status=1 AND operation_time < ?)
//   - restore_and_back_up_device_log response_timeout (10min):
//       UPDATE restore_and_back_up_device_log SET results=2,
//         failure_reason='Response timeout' WHERE results IS NULL
//         AND start_time < ?
//   - device_send_ca_log (send_timeout, event_type='UpdateCertificate'):
//       UPDATE device_send_ca_log SET result=3 WHERE event_log_id IN
//         (SELECT id FROM event_log WHERE status=1
//          AND event_type='UpdateCertificate' AND operation_time < ?)
//   - event_log (send_timeout):
//       UPDATE event_log SET status=6 WHERE status=1 AND operation_time < ?
//   - mml_execute_result (send_timeout):
//       UPDATE mml_execute_result SET status=6 WHERE status IS NULL
//         AND operation_time < ?
type EventLogTimeoutTask struct {
	db           *gorm.DB
	timeoutHours int // 响应超时小时数，默认 24（对应 Java deviceUpgradeTimeout）
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

// UpdateTimeoutStatus 执行超时状态更新（对齐 Java updateTimeoutStatus）
func (t *EventLogTimeoutTask) UpdateTimeoutStatus() {
	now := time.Now()
	sendTimeout := now.Add(-6 * time.Minute)         // 发送超时阈值
	responseTimeout := now.Add(-time.Duration(t.timeoutHours) * time.Hour) // 响应超时阈值
	restoreResponseTimeout := now.Add(-10 * time.Minute)                   // 备份恢复响应超时 10min

	// ---- 阶段1: 发送超时（6 分钟） ----
	t.markUpgradeLogSendTimeout(sendTimeout)
	t.markManualUpgradeLogSendTimeout(sendTimeout)
	t.markRestoreLogSendTimeout(sendTimeout)
	t.markDeviceSendCaLogTimeout(sendTimeout)
	t.markEventLogTimeout(sendTimeout)
	t.markMmlExecuteResultTimeout(sendTimeout)

	// ---- 阶段2: 响应超时 ----
	t.markUpgradeLogResponseTimeout(responseTimeout)
	t.markManualUpgradeLogResponseTimeout(responseTimeout)
	t.markRestoreLogResponseTimeout(restoreResponseTimeout)
}

// ---- 阶段1: 发送超时 ----

func (t *EventLogTimeoutTask) markUpgradeLogSendTimeout(sendTimeout time.Time) {
	sql := `UPDATE upgrade_log SET success = 0, message = 'Command delivery timeout', done_time = NOW()
		WHERE command_track_id IN (SELECT id FROM event_log WHERE status = 1 AND operation_time < ?)`
	if err := t.db.Exec(sql, sendTimeout).Error; err != nil {
		logger.Errorf("EventLogTimeoutTask: upgrade_log send timeout failed: %v", err)
	}
}

func (t *EventLogTimeoutTask) markManualUpgradeLogSendTimeout(sendTimeout time.Time) {
	sql := `UPDATE manual_upgrade_log SET success = 0, info = 'Command delivery timeout'
		WHERE event_log_id IN (SELECT id FROM event_log WHERE status = 1 AND operation_time < ?)`
	if err := t.db.Exec(sql, sendTimeout).Error; err != nil {
		logger.Errorf("EventLogTimeoutTask: manual_upgrade_log send timeout failed: %v", err)
	}
}

func (t *EventLogTimeoutTask) markRestoreLogSendTimeout(sendTimeout time.Time) {
	sql := `UPDATE restore_and_back_up_device_log SET results = 2, failure_reason = 'Command delivery timeout'
		WHERE event_log_id IN (SELECT id FROM event_log WHERE status = 1 AND operation_time < ?)`
	if err := t.db.Exec(sql, sendTimeout).Error; err != nil {
		logger.Errorf("EventLogTimeoutTask: restore_and_back_up_device_log send timeout failed: %v", err)
	}
}

func (t *EventLogTimeoutTask) markDeviceSendCaLogTimeout(sendTimeout time.Time) {
	sql := `UPDATE device_send_ca_log SET result = 3
		WHERE event_log_id IN (SELECT id FROM event_log WHERE status = 1 AND event_type = 'UpdateCertificate' AND operation_time < ?)`
	if err := t.db.Exec(sql, sendTimeout).Error; err != nil {
		logger.Errorf("EventLogTimeoutTask: device_send_ca_log timeout failed: %v", err)
	}
}

func (t *EventLogTimeoutTask) markEventLogTimeout(sendTimeout time.Time) {
	sql := `UPDATE event_log SET status = 6 WHERE status = 1 AND operation_time < ?`
	if err := t.db.Exec(sql, sendTimeout).Error; err != nil {
		logger.Errorf("EventLogTimeoutTask: event_log timeout failed: %v", err)
	}
}

func (t *EventLogTimeoutTask) markMmlExecuteResultTimeout(sendTimeout time.Time) {
	sql := `UPDATE mml_execute_result SET status = 6 WHERE status IS NULL AND operation_time < ?`
	if err := t.db.Exec(sql, sendTimeout).Error; err != nil {
		logger.Errorf("EventLogTimeoutTask: mml_execute_result timeout failed: %v", err)
	}
}

// ---- 阶段2: 响应超时 ----

func (t *EventLogTimeoutTask) markUpgradeLogResponseTimeout(responseTimeout time.Time) {
	// upgrade=1: Upgrade timeout
	sqlUpgrade := `UPDATE upgrade_log SET is_done = 1, success = 0, message = 'Upgrade timeout', done_time = NOW()
		WHERE upgrade = 1 AND success IS NULL AND creation_time < ?`
	if err := t.db.Exec(sqlUpgrade, responseTimeout).Error; err != nil {
		logger.Errorf("EventLogTimeoutTask: upgrade_log (upgrade) response timeout failed: %v", err)
	}
	// upgrade=0: Rollback timeout
	sqlRollback := `UPDATE upgrade_log SET is_done = 1, success = 0, message = 'Rollback timeout', done_time = NOW()
		WHERE upgrade = 0 AND success IS NULL AND creation_time < ?`
	if err := t.db.Exec(sqlRollback, responseTimeout).Error; err != nil {
		logger.Errorf("EventLogTimeoutTask: upgrade_log (rollback) response timeout failed: %v", err)
	}
}

func (t *EventLogTimeoutTask) markManualUpgradeLogResponseTimeout(responseTimeout time.Time) {
	sql := `UPDATE manual_upgrade_log SET success = 0, info = 'Download timeout'
		WHERE success IS NULL AND start_time < ?`
	if err := t.db.Exec(sql, responseTimeout).Error; err != nil {
		logger.Errorf("EventLogTimeoutTask: manual_upgrade_log response timeout failed: %v", err)
	}
}

func (t *EventLogTimeoutTask) markRestoreLogResponseTimeout(restoreResponseTimeout time.Time) {
	sql := `UPDATE restore_and_back_up_device_log SET results = 2, failure_reason = 'Response timeout'
		WHERE results IS NULL AND start_time < ?`
	if err := t.db.Exec(sql, restoreResponseTimeout).Error; err != nil {
		logger.Errorf("EventLogTimeoutTask: restore_and_back_up_device_log response timeout failed: %v", err)
	}
}
