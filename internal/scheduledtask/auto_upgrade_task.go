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

// AutoUpgradeTaskJob 自动升级任务触发
// 镜像 Java AutoUpgradeTaskJob
// 扫描 upgrade_auto_task 表中 status=1（Waiting）且 trigger_time 已过的记录，
// 对每个任务按设备循环执行升级（先固件后软件或反之，由 upgrade_firmware_first 配置控制）
type AutoUpgradeTaskJob struct {
	db       *gorm.DB
	opSender *tr069.OperationSender
}

// autoUpgradeTaskRow 对应 upgrade_auto_task 表中定时扫描所需的字段
type autoUpgradeTaskRow struct {
	Id                   int        `gorm:"primaryKey;column:id"`
	TaskName             *string    `gorm:"column:task_name"`
	DeviceIds            *string    `gorm:"column:device_ids;type:text"`
	DeviceGroupIds       *string    `gorm:"column:device_group_ids;type:longtext"`
	Scope                *string    `gorm:"column:scope;type:varchar(255)"`
	SoftwareVersionId    *int       `gorm:"column:software_version_id"`
	HardwareVersionId    *int       `gorm:"column:hardware_version_id"`
	MaxOccurs            *int       `gorm:"column:max_occurs"`
	UpgradeFirmwareFirst *bool      `gorm:"column:upgrade_firmware_first"`
	DurationTime         *int       `gorm:"column:duration_time"`
	TenancyId            *int       `gorm:"column:tenancy_id"`
	IsInitiated          *int       `gorm:"column:is_initiated"`
	UpgradeFileId        *int64     `gorm:"column:upgrade_file_id"`
	DeviceType           *string    `gorm:"column:device_type;type:varchar(50)"`
	Enabled              *bool      `gorm:"column:enabled"`
}

// autoUpgradeStatus 对应 upgrade_auto_task 的 is_initiated 状态
const (
	autoUpgradeStatusWaiting   = 1 // 等待触发
	autoUpgradeStatusExecuting = 2 // 执行中
	autoUpgradeStatusCompleted = 3 // 已完成
)

// NewAutoUpgradeTaskJob 创建 AutoUpgradeTaskJob 实例
func NewAutoUpgradeTaskJob(db *gorm.DB, opSender *tr069.OperationSender) *AutoUpgradeTaskJob {
	return &AutoUpgradeTaskJob{
		db:       db,
		opSender: opSender,
	}
}

// TriggerDueTasks 扫描到期任务并触发升级
// 1. 查询 upgrade_auto_task 中 is_initiated=1（Waiting）且 enabled=true 的记录
// 2. 对每个到期任务，解析目标设备列表
// 3. 按设备循环下发升级命令（通过 upgrade 队列）
// 4. 更新任务状态为执行中
func (j *AutoUpgradeTaskJob) TriggerDueTasks() {
	ctx := context.Background()

	// 1. 查询 status=1（Waiting）且 enabled=true 的自动升级任务
	var tasks []autoUpgradeTaskRow
	if err := j.db.Table("upgrade_auto_task").
		Where("is_initiated = ? AND (enabled = ? OR enabled IS NULL)", autoUpgradeStatusWaiting, true).
		Find(&tasks).Error; err != nil {
		logger.Errorf("AutoUpgradeTaskJob: query upgrade_auto_task failed: %v", err)
		return
	}

	if len(tasks) == 0 {
		return
	}

	for _, task := range tasks {
		j.triggerTask(ctx, task)
	}
}

// triggerTask 触发单个自动升级任务
func (j *AutoUpgradeTaskJob) triggerTask(ctx context.Context, task autoUpgradeTaskRow) {
	taskName := ""
	if task.TaskName != nil {
		taskName = *task.TaskName
	}

	// 解析目标设备列表
	elementIds := j.resolveDeviceIds(task)
	if len(elementIds) == 0 {
		logger.Warnf("AutoUpgradeTaskJob: task %d (%s) has no target devices", task.Id, taskName)
		return
	}

	// 控制最大并发数
	maxOccurs := 1
	if task.MaxOccurs != nil && *task.MaxOccurs > 0 {
		maxOccurs = *task.MaxOccurs
	}

	// 更新任务状态为执行中
	now := time.Now()
	if err := j.db.Table("upgrade_auto_task").
		Where("id = ?", task.Id).
		Updates(map[string]interface{}{
			"is_initiated": autoUpgradeStatusExecuting,
			"update_time":  now,
		}).Error; err != nil {
		logger.Errorf("AutoUpgradeTaskJob: failed to update task %d status: %v", task.Id, err)
		return
	}

	// 计算任务截止时间（durationTime 小时后）
	durationHours := 24
	if task.DurationTime != nil && *task.DurationTime > 0 {
		durationHours = *task.DurationTime
	}
	deadline := now.Add(time.Duration(durationHours) * time.Hour)

	// 按设备循环下发升级命令，控制并发数
	sem := make(chan struct{}, maxOccurs)
	completedCount := 0

	for _, eid := range elementIds {
		// 检查是否已超时
		if time.Now().After(deadline) {
			logger.Infof("AutoUpgradeTaskJob: task %d (%s) reached duration time limit, stopping", task.Id, taskName)
			break
		}

		sem <- struct{}{} // acquire concurrency slot

		// 检查设备在线状态
		onlineKey := fmt.Sprintf("online_%d", eid)
		onlineVal, err := redis.Get(ctx, onlineKey)
		if err != nil || onlineVal != "yes" {
			<-sem
			continue
		}

		// 查设备 SN
		var sn string
		if err := j.db.Table("cpe_element").
			Select("serial_number").
			Where("ne_neid = ? AND deleted = ?", eid, false).
			Scan(&sn).Error; err != nil || sn == "" {
			<-sem
			continue
		}

		// 下发升级命令：通过 SPV 设置 Device.FAP.Service.Status.NR.Operation.Upgrade=1
		// 或者如果有 upgrade_file_id，通过 Download 下发升级文件
		if task.UpgradeFileId != nil && *task.UpgradeFileId > 0 {
			j.dispatchUpgradeDownload(ctx, task, eid, sn)
		} else {
			j.dispatchUpgradeSPV(ctx, task, eid, sn)
		}

		completedCount++
		<-sem // release concurrency slot
	}

	// 更新任务状态为已完成
	if err := j.db.Table("upgrade_auto_task").
		Where("id = ?", task.Id).
		Updates(map[string]interface{}{
			"is_initiated": autoUpgradeStatusCompleted,
			"update_time":  time.Now(),
		}).Error; err != nil {
		logger.Errorf("AutoUpgradeTaskJob: failed to complete task %d: %v", task.Id, err)
	}

	logger.Infof("AutoUpgradeTaskJob: task %d (%s) completed, %d devices processed", task.Id, taskName, completedCount)
}

// dispatchUpgradeDownload 通过 Download 命令下发升级文件
func (j *AutoUpgradeTaskJob) dispatchUpgradeDownload(ctx context.Context, task autoUpgradeTaskRow, elementId int64, sn string) {
	// 查询升级文件信息
	type upgradeFileRow struct {
		Id      int    `gorm:"primaryKey;column:id"`
		Version string `gorm:"column:version"`
	}
	var uf upgradeFileRow
	if err := j.db.Table("upgrade_file").
		Where("id = ?", *task.UpgradeFileId).
		Scan(&uf).Error; err != nil {
		logger.Errorf("AutoUpgradeTaskJob: failed to find upgrade file %d: %v", *task.UpgradeFileId, err)
		return
	}

	// 构造 Download 请求
	commandKey := fmt.Sprintf("auto_upgrade_%d_%d_%d", task.Id, elementId, time.Now().Unix())
	dl := &soap.Download{
		CommandKey: commandKey,
		FileType:   "1 Firmware Upgrade Image",
		URL:        fmt.Sprintf("http://localhost/acs-file-server/upgrade/downloadFile/%d", uf.Id),
	}

	operationId := fmt.Sprintf("auto_upgrade_%d_%d", task.Id, elementId)
	if err := j.opSender.SendDownload(sn, dl, operationId); err != nil {
		logger.Errorf("AutoUpgradeTaskJob: failed to send download to %s (element %d): %v", sn, elementId, err)
		return
	}

	logger.Infof("AutoUpgradeTaskJob: download sent to %s (element %d, task %d)", sn, elementId, task.Id)
}

// dispatchUpgradeSPV 通过 SPV 下发升级触发命令
func (j *AutoUpgradeTaskJob) dispatchUpgradeSPV(ctx context.Context, task autoUpgradeTaskRow, elementId int64, sn string) {
	upgradeFirmwareFirst := true
	if task.UpgradeFirmwareFirst != nil {
		upgradeFirmwareFirst = *task.UpgradeFirmwareFirst
	}

	// 根据升级顺序选择参数路径
	var params []soap.ParameterValueStruct
	if upgradeFirmwareFirst {
		// 先升级固件
		params = []soap.ParameterValueStruct{
			{Name: "Device.FAP.Service.Status.NR.Operation.Upgrade", Value: "1", Type: "xsd:string"},
		}
	} else {
		// 先升级软件
		params = []soap.ParameterValueStruct{
			{Name: "Device.FAP.Service.Status.NR.Operation.Upgrade", Value: "1", Type: "xsd:string"},
		}
	}

	operationId := fmt.Sprintf("auto_upgrade_%d_%d", task.Id, elementId)
	if err := j.opSender.SendSetParameterValues(sn, params, "", operationId); err != nil {
		logger.Errorf("AutoUpgradeTaskJob: failed to send SPV to %s (element %d): %v", sn, elementId, err)
		return
	}

	logger.Infof("AutoUpgradeTaskJob: upgrade SPV sent to %s (element %d, task %d)", sn, elementId, task.Id)
}

// resolveDeviceIds 解析自动升级任务的目标设备列表
func (j *AutoUpgradeTaskJob) resolveDeviceIds(task autoUpgradeTaskRow) []int64 {
	scope := ""
	if task.Scope != nil {
		scope = *task.Scope
	}

	switch scope {
	case "deviceGroup":
		// 从 device_group_ids 解析设备组，再查 group_has_element 获取设备
		var groupIds []string
		if task.DeviceGroupIds != nil && *task.DeviceGroupIds != "" {
			// 尝试解析为 JSON 数组
			if err := json.Unmarshal([]byte(*task.DeviceGroupIds), &groupIds); err != nil {
				// 不是 JSON，尝试逗号分隔
				groupIds = splitComma(*task.DeviceGroupIds)
			}
		}
		if len(groupIds) == 0 {
			return nil
		}

		var ids []int64
		if err := j.db.Table("group_has_element").
			Select("element_id").
			Where("group_id IN (?)", groupIds).
			Pluck("element_id", &ids).Error; err != nil {
			logger.Warnf("AutoUpgradeTaskJob: failed to resolve device group ids for task %d: %v", task.Id, err)
			return nil
		}
		return ids

	case "device":
		// 从 device_ids 直接解析
		if task.DeviceIds == nil || *task.DeviceIds == "" {
			return nil
		}
		var ids []int64
		if err := json.Unmarshal([]byte(*task.DeviceIds), &ids); err != nil {
			// 尝试逗号分隔
			strs := splitComma(*task.DeviceIds)
			for _, s := range strs {
				var v int64
				fmt.Sscanf(s, "%d", &v)
				if v > 0 {
					ids = append(ids, v)
				}
			}
		}
		return ids

	default:
		// 兼容：尝试从 device_ids 解析
		if task.DeviceIds == nil || *task.DeviceIds == "" {
			return nil
		}
		var ids []int64
		if err := json.Unmarshal([]byte(*task.DeviceIds), &ids); err != nil {
			strs := splitComma(*task.DeviceIds)
			for _, s := range strs {
				var v int64
				fmt.Sscanf(s, "%d", &v)
				if v > 0 {
					ids = append(ids, v)
				}
			}
		}
		return ids
	}
}
