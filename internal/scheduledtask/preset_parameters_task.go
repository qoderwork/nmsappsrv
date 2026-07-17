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

// PresetParametersTask 预置参数任务（每2分钟）
// 查询 preset_parameters_task 表中 status=1（待执行）的记录
// 对每个任务：
//   - 根据 element_id 获取设备 SN
//   - 检查设备在线状态
//   - 根据任务类型（set/get）：
//     - set: 通过 SPV 下发预置参数值
//     - get: 通过 GPV 查询参数值并更新到 result
//   - 更新任务状态为已完成或记录错误
type PresetParametersTask struct {
	db       *gorm.DB
	opSender *tr069.OperationSender
}

// presetParametersTaskRow 对应 preset_parameters_task 表
// 与 misc/model.go 中的 PresetParametersTask 定义对应
type presetParametersTaskRow struct {
	Id         int64   `gorm:"primaryKey;autoIncrement;column:id"`
	ElementId  *int64  `gorm:"column:element_id"`
	Parameters *string `gorm:"column:parameters;type:longtext"` // JSON 数组
	TaskType   *string `gorm:"column:task_type;type:varchar(255)"` // "set" 或 "get"
	Status     *int    `gorm:"column:status"` // 1=待执行, 2=执行中, 3=已完成, 4=失败
	Result     *string `gorm:"column:result;type:longtext"`
	EventLogId *int64  `gorm:"column:event_log_id"`
}

// presetParamItem 是 parameters JSON 中的单个参数项
type presetParamItem struct {
	ParamPath  string `json:"path"`
	ParamValue string `json:"value"`
}

// NewPresetParametersTask 创建 PresetParametersTask 实例
func NewPresetParametersTask(db *gorm.DB, opSender *tr069.OperationSender) *PresetParametersTask {
	return &PresetParametersTask{
		db:       db,
		opSender: opSender,
	}
}

// TriggerTasks 扫描并触发预置参数任务
// 1. 查询 status=1 的待执行任务
// 2. 对每个任务执行 set 或 get 操作
// 3. 更新任务状态
func (t *PresetParametersTask) TriggerTasks() {
	ctx := context.Background()

	// 1. 查询所有待执行任务 (status=1)
	var tasks []presetParametersTaskRow
	if err := t.db.Table("preset_parameters_task").
		Where("status = ?", 1).
		Find(&tasks).Error; err != nil {
		logger.Errorf("PresetParametersTask: query preset_parameters_task failed: %v", err)
		return
	}

	for _, task := range tasks {
		t.processTask(ctx, task)
	}
}

// processTask 处理单个预置参数任务
func (t *PresetParametersTask) processTask(ctx context.Context, task presetParametersTaskRow) {
	elementId := int64Val(task.ElementId)
	if elementId == 0 {
		t.markTaskFailed(task.Id, "element_id is empty")
		return
	}

	// 2. 获取设备 SN
	var deviceInfo struct {
		SerialNumber string `gorm:"column:serial_number"`
	}
	if err := t.db.Table("cpe_element").
		Select("serial_number").
		Where("ne_neid = ? AND deleted = ?", elementId, false).
		Scan(&deviceInfo).Error; err != nil {
		t.markTaskFailed(task.Id, fmt.Sprintf("device not found: %v", err))
		return
	}
	sn := deviceInfo.SerialNumber
	if sn == "" {
		t.markTaskFailed(task.Id, "device has no serial number")
		return
	}

	// 3. 检查设备在线状态
	onlineKey := fmt.Sprintf("online_%d", elementId)
	onlineVal, err := redis.Get(ctx, onlineKey)
	if err != nil || onlineVal != "yes" {
		// 设备不在线，跳过但不标记失败，等待下次轮询
		logger.Debugf("PresetParametersTask: device %d is offline, skipping", elementId)
		return
	}

	// 4. 根据任务类型执行操作
	taskType := strVal(task.TaskType)
	switch taskType {
	case "set":
		t.executeSetTask(ctx, task, sn, elementId)
	case "get":
		t.executeGetTask(ctx, task, sn, elementId)
	default:
		t.markTaskFailed(task.Id, fmt.Sprintf("unknown task type: %s", taskType))
	}
}

// executeSetTask 执行 set 类型任务（通过 SPV 下发参数）
func (t *PresetParametersTask) executeSetTask(ctx context.Context, task presetParametersTaskRow, sn string, elementId int64) {
	// 解析 parameters JSON
	var paramItems []presetParamItem
	if task.Parameters == nil || *task.Parameters == "" {
		t.markTaskFailed(task.Id, "parameters is empty")
		return
	}
	if err := json.Unmarshal([]byte(*task.Parameters), &paramItems); err != nil {
		t.markTaskFailed(task.Id, fmt.Sprintf("parse parameters JSON failed: %v", err))
		return
	}
	if len(paramItems) == 0 {
		t.markTaskFailed(task.Id, "no parameters to set")
		return
	}

	// 构建 SPV 参数列表
	params := make([]soap.ParameterValueStruct, 0, len(paramItems))
	for _, item := range paramItems {
		if item.ParamPath == "" {
			continue
		}
		params = append(params, soap.ParameterValueStruct{
			Name:  item.ParamPath,
			Value: item.ParamValue,
			Type:  "xsd:string",
		})
	}

	if len(params) == 0 {
		t.markTaskFailed(task.Id, "no valid parameters to set")
		return
	}

	// 标记任务为执行中
	t.db.Table("preset_parameters_task").Where("id = ?", task.Id).Update("status", 2)

	// 发送 SPV
	operationId := fmt.Sprintf("preset_set_%d_%d", elementId, task.Id)
	if err := t.opSender.SendSetParameterValues(sn, params, "", operationId); err != nil {
		t.markTaskFailed(task.Id, fmt.Sprintf("send SPV failed: %v", err))
		return
	}

	// 标记任务为已完成
	t.db.Table("preset_parameters_task").Where("id = ?", task.Id).Updates(map[string]interface{}{
		"status":      3,
		"result":      "SPV sent successfully",
		"event_log_id": nil, // 由 OperationSender 内部创建 event_log
	})

	logger.Infof("PresetParametersTask: sent SPV to device %s (element %d, task %d, %d params)",
		sn, elementId, task.Id, len(params))
}

// executeGetTask 执行 get 类型任务（通过 GPV 查询参数）
func (t *PresetParametersTask) executeGetTask(ctx context.Context, task presetParametersTaskRow, sn string, elementId int64) {
	// 解析 parameters JSON 获取要查询的参数路径列表
	var paramItems []presetParamItem
	if task.Parameters == nil || *task.Parameters == "" {
		t.markTaskFailed(task.Id, "parameters is empty")
		return
	}
	if err := json.Unmarshal([]byte(*task.Parameters), &paramItems); err != nil {
		t.markTaskFailed(task.Id, fmt.Sprintf("parse parameters JSON failed: %v", err))
		return
	}

	// 收集参数路径
	paramPaths := make([]string, 0, len(paramItems))
	for _, item := range paramItems {
		if item.ParamPath != "" {
			paramPaths = append(paramPaths, item.ParamPath)
		}
	}

	if len(paramPaths) == 0 {
		t.markTaskFailed(task.Id, "no parameters to get")
		return
	}

	// 标记任务为执行中
	t.db.Table("preset_parameters_task").Where("id = ?", task.Id).Update("status", 2)

	// 发送 GPV
	operationId := fmt.Sprintf("preset_get_%d_%d", elementId, task.Id)
	if err := t.opSender.SendGetParameterValues(sn, paramPaths, operationId); err != nil {
		t.markTaskFailed(task.Id, fmt.Sprintf("send GPV failed: %v", err))
		return
	}

	// GPV 结果会通过 event_log 异步返回，这里先标记为完成
	// 实际结果由 TR-069 响应处理器更新到 result 字段
	now := time.Now()
	resultJSON, _ := json.Marshal(map[string]interface{}{
		"status":      "GPV sent, waiting for response",
		"param_count": len(paramPaths),
		"sent_time":   now.Format(time.RFC3339),
	})
	t.db.Table("preset_parameters_task").Where("id = ?", task.Id).Updates(map[string]interface{}{
		"status": 3,
		"result": string(resultJSON),
	})

	logger.Infof("PresetParametersTask: sent GPV to device %s (element %d, task %d, %d params)",
		sn, elementId, task.Id, len(paramPaths))
}

// markTaskFailed 标记任务为失败
func (t *PresetParametersTask) markTaskFailed(taskId int64, reason string) {
	t.db.Table("preset_parameters_task").Where("id = ?", taskId).Updates(map[string]interface{}{
		"status": 4,
		"result": reason,
	})
	logger.Warnf("PresetParametersTask: task %d failed: %s", taskId, reason)
}

// strVal 安全获取 string 指针值
func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// int64Val 安全获取 int64 指针值
func int64Val(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}