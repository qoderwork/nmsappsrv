package scheduledtask

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// AlarmSyncTask 告警同步定时任务
// 镜像 Java AlarmSyncTaskJob
// 对在线 enb 设备通过 SPV 设置 Device.FAP.Service.Status.AlarmSync=1 触发设备上报告警
type AlarmSyncTask struct {
	db       *gorm.DB
	opSender *tr069.OperationSender
}

// alarmSyncConfigRow 对应 system_config 表中 alarm_sync_config 的结构
type alarmSyncConfigRow struct {
	Enabled      *bool   `json:"enabled"`
	SyncInterval *int    `json:"syncInterval"`
	LastSyncTime *string `json:"lastSyncTime"`
}

// NewAlarmSyncTask 创建 AlarmSyncTask 实例
func NewAlarmSyncTask(db *gorm.DB, opSender *tr069.OperationSender) *AlarmSyncTask {
	return &AlarmSyncTask{
		db:       db,
		opSender: opSender,
	}
}

// SyncAlarms 执行告警同步
// 1. 读取 alarm_sync_config 配置，检查是否启用
// 2. 查询所有在线的 enb 设备
// 3. 对在线设备下发 AlarmSync 参数（SPV 设置 Device.FAP.Service.Status.AlarmSync=1）
// 4. 使用限流控制下发速率（Java 使用 RateLimiter(14)）
func (t *AlarmSyncTask) SyncAlarms() {
	ctx := context.Background()

	// 1. 读取 alarm_sync_config 配置
	config := t.loadAlarmSyncConfig()
	if config == nil {
		logger.Warnf("AlarmSyncTask: no alarm_sync_config found, skipping")
		return
	}
	if config.Enabled != nil && !*config.Enabled {
		return
	}

	// 2. 查询所有在线的 enb 设备
	type enbElementRow struct {
		NeNeid       int64  `gorm:"column:ne_neid"`
		SerialNumber string `gorm:"column:serial_number"`
	}
	var elements []enbElementRow
	if err := t.db.Table("cpe_element").
		Select("ne_neid, serial_number").
		Where("deleted = ? AND serial_number IS NOT NULL AND serial_number != '' AND device_type = ?", false, "enb").
		Find(&elements).Error; err != nil {
		logger.Errorf("AlarmSyncTask: query enb devices failed: %v", err)
		return
	}

	if len(elements) == 0 {
		return
	}

	// 3. 对在线设备下发 AlarmSync 参数，限流控制
	// Java 使用 RateLimiter(14)，即每秒最多 14 个请求
	// 使用带缓冲的 channel 模拟限流
	rateLimiter := make(chan struct{}, 14)
	syncCount := 0

	for _, elem := range elements {
		elementId := elem.NeNeid
		sn := elem.SerialNumber

		// 检查在线状态
		onlineKey := fmt.Sprintf("online_%d", elementId)
		onlineVal, err := redis.Get(ctx, onlineKey)
		if err != nil || onlineVal != "yes" {
			continue
		}

		// 限流
		rateLimiter <- struct{}{}

		// 下发 AlarmSync 参数
		params := []soap.ParameterValueStruct{
			{Name: "Device.FAP.Service.Status.AlarmSync", Value: "1", Type: "xsd:string"},
		}
		operationId := fmt.Sprintf("alarm_sync_%d_%d", elementId, syncCount)
		if err := t.opSender.SendSetParameterValues(sn, params, "", operationId); err != nil {
			logger.Errorf("AlarmSyncTask: failed to send AlarmSync SPV to %s (element %d): %v", sn, elementId, err)
			<-rateLimiter
			continue
		}

		syncCount++
		<-rateLimiter
	}

	// 4. 更新最后同步时间
	if syncCount > 0 {
		t.updateLastSyncTime()
		logger.Infof("AlarmSyncTask: synced alarms for %d enb devices", syncCount)
	}
}

// loadAlarmSyncConfig 从 system_config 表读取告警同步配置
func (t *AlarmSyncTask) loadAlarmSyncConfig() *alarmSyncConfigRow {
	var configStr *string
	if err := t.db.Table("system_config").
		Select("config").
		Where("id = ?", "alarm_sync_config").
		Scan(&configStr).Error; err != nil {
		logger.Errorf("AlarmSyncTask: failed to load alarm_sync_config: %v", err)
		return nil
	}

	if configStr == nil || *configStr == "" {
		return nil
	}

	var cfg alarmSyncConfigRow
	if err := json.Unmarshal([]byte(*configStr), &cfg); err != nil {
		logger.Errorf("AlarmSyncTask: failed to unmarshal alarm_sync_config: %v", err)
		return nil
	}

	return &cfg
}

// updateLastSyncTime 更新告警同步的最后同步时间
func (t *AlarmSyncTask) updateLastSyncTime() {
	// 读取当前配置
	cfg := t.loadAlarmSyncConfig()
	if cfg == nil {
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	cfg.LastSyncTime = &now

	data, err := json.Marshal(cfg)
	if err != nil {
		logger.Errorf("AlarmSyncTask: failed to marshal alarm_sync_config: %v", err)
		return
	}

	configStr := string(data)
	if err := t.db.Table("system_config").
		Where("id = ?", "alarm_sync_config").
		Update("config", configStr).Error; err != nil {
		logger.Errorf("AlarmSyncTask: failed to update last sync time: %v", err)
	}
}
