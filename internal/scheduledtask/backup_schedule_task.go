package scheduledtask

import (
	"context"
	"fmt"

	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// BackupScheduleTask 每日设备配置自动备份
// 镜像 Java BackupScheduleJob
// 每天凌晨 00:00 执行，扫描所有未删除的网元，
// 对在线且不在黑名单中的设备通过 SPV 设置
// Device.Services.FAPService.1.FAPControl.NR.Operation.Backup=1
type BackupScheduleTask struct {
	db       *gorm.DB
	opSender *tr069.OperationSender
}

// NewBackupScheduleTask 创建 BackupScheduleTask 实例
func NewBackupScheduleTask(db *gorm.DB, opSender *tr069.OperationSender) *BackupScheduleTask {
	return &BackupScheduleTask{
		db:       db,
		opSender: opSender,
	}
}

// RunBackup 执行每日自动备份
// 1. 查询所有未删除的网元（cpe_element）
// 2. 对在线且不在黑名单中的设备
// 3. 通过 SPV 下发备份命令
func (t *BackupScheduleTask) RunBackup() {
	ctx := context.Background()

	// 1. 查询所有未删除的网元
	type cpeElementRow struct {
		NeNeid       int64  `gorm:"column:ne_neid"`
		SerialNumber string `gorm:"column:serial_number"`
	}
	var elements []cpeElementRow
	if err := t.db.Table("cpe_element").
		Select("ne_neid, serial_number").
		Where("deleted = ? AND serial_number IS NOT NULL AND serial_number != ''", false).
		Find(&elements).Error; err != nil {
		logger.Errorf("BackupScheduleTask: query cpe_element failed: %v", err)
		return
	}

	if len(elements) == 0 {
		return
	}

	backupCount := 0

	for _, elem := range elements {
		elementId := elem.NeNeid
		sn := elem.SerialNumber

		// 2a. 检查在线状态
		onlineKey := fmt.Sprintf("online_%d", elementId)
		onlineVal, err := redis.Get(ctx, onlineKey)
		if err != nil || onlineVal != "yes" {
			continue
		}

		// 2b. 检查黑名单
		var blackListCount int64
		t.db.Table("element_black_list").
			Where("sn = ?", sn).
			Count(&blackListCount)
		if blackListCount > 0 {
			continue
		}

		// 3. 通过 SPV 下发备份命令
		params := []soap.ParameterValueStruct{
			{Name: "Device.Services.FAPService.1.FAPControl.NR.Operation.Backup", Value: "1", Type: "xsd:string"},
		}
		operationId := fmt.Sprintf("backup_schedule_%d", elementId)
		if err := t.opSender.SendSetParameterValues(sn, params, "", operationId); err != nil {
			logger.Errorf("BackupScheduleTask: failed to send backup SPV to %s (element %d): %v", sn, elementId, err)
			continue
		}

		backupCount++
	}

	if backupCount > 0 {
		logger.Infof("BackupScheduleTask: backup command sent to %d devices", backupCount)
	}
}
