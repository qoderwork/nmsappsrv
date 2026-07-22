package scheduledtask

import (
	"context"
	"encoding/json"
	"fmt"

	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

type AlarmSyncTask struct {
	db       *gorm.DB
	opSender *tr069.OperationSender
}

type alarmSyncConfig struct {
	Enable *bool `json:"enable"`
	Period *int  `json:"period"`
}

func NewAlarmSyncTask(db *gorm.DB, opSender *tr069.OperationSender) *AlarmSyncTask {
	return &AlarmSyncTask{
		db:       db,
		opSender: opSender,
	}
}

func (t *AlarmSyncTask) SyncAlarms() {
	ctx := context.Background()

	type tenancyRow struct {
		Id int `gorm:"column:id"`
	}
	var tenancies []tenancyRow
	if err := t.db.Table("license").Select("id").Find(&tenancies).Error; err != nil {
		logger.Errorf("AlarmSyncTask: query tenancies failed: %v", err)
		return
	}

	totalSynced := 0
	for _, tenancy := range tenancies {
		config := t.loadAlarmSyncConfig(tenancy.Id)
		if config.Enable != nil && !*config.Enable {
			continue
		}

		syncCount := t.syncAlarmsForTenancy(ctx, tenancy.Id)
		totalSynced += syncCount
	}

	if totalSynced > 0 {
		logger.Infof("AlarmSyncTask: synced alarms for %d enb devices across %d tenancies", totalSynced, len(tenancies))
	}
}

func (t *AlarmSyncTask) syncAlarmsForTenancy(ctx context.Context, tenancyId int) int {
	type enbElementRow struct {
		NeNeid       int64  `gorm:"column:ne_neid"`
		SerialNumber string `gorm:"column:serial_number"`
	}
	var elements []enbElementRow
	if err := t.db.Table("cpe_element").
		Select("ne_neid, serial_number").
		Where("deleted = ? AND serial_number IS NOT NULL AND serial_number != '' AND device_type = ? AND license_id = ?", false, "enb", tenancyId).
		Find(&elements).Error; err != nil {
		logger.Errorf("AlarmSyncTask: query enb devices for tenancy %d failed: %v", tenancyId, err)
		return 0
	}

	if len(elements) == 0 {
		return 0
	}

	rateLimiter := make(chan struct{}, 14)
	syncCount := 0

	for _, elem := range elements {
		elementId := elem.NeNeid
		sn := elem.SerialNumber

		onlineKey := fmt.Sprintf("online_%d", elementId)
		onlineVal, err := redis.Get(ctx, onlineKey)
		if err != nil || onlineVal != "yes" {
			continue
		}

		var blackListCount int64
		t.db.Table("element_black_list").
			Where("sn = ?", sn).
			Count(&blackListCount)
		if blackListCount > 0 {
			continue
		}

		rateLimiter <- struct{}{}

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

	return syncCount
}

func (t *AlarmSyncTask) loadAlarmSyncConfig(tenancyId int) *alarmSyncConfig {
	configKey := fmt.Sprintf("alarm_sync_%d", tenancyId)

	var configStr *string
	if err := t.db.Table("system_config").
		Select("config").
		Where("id = ?", configKey).
		Scan(&configStr).Error; err != nil {
		logger.Warnf("AlarmSyncTask: failed to load %s, using defaults: %v", configKey, err)
		return t.defaultConfig()
	}

	if configStr == nil || *configStr == "" {
		return t.defaultConfig()
	}

	var cfg alarmSyncConfig
	if err := json.Unmarshal([]byte(*configStr), &cfg); err != nil {
		logger.Errorf("AlarmSyncTask: failed to unmarshal %s, using defaults: %v", configKey, err)
		return t.defaultConfig()
	}

	if cfg.Enable == nil {
		enable := true
		cfg.Enable = &enable
	}
	if cfg.Period == nil || *cfg.Period <= 0 {
		period := 60
		cfg.Period = &period
	}

	return &cfg
}

func (t *AlarmSyncTask) defaultConfig() *alarmSyncConfig {
	enable := true
	period := 60
	return &alarmSyncConfig{
		Enable: &enable,
		Period: &period,
	}
}
