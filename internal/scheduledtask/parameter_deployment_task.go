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

// ParameterDeploymentTask 定时检查参数部署模板并下发不一致的参数
// 镜像 Java ParameterDeploymentTask
type ParameterDeploymentTask struct {
	db       *gorm.DB
	opSender *tr069.OperationSender
}

// deploymentTemplate 对应 parameter_deployment_template 表
type deploymentTemplate struct {
	Id          int64  `gorm:"primaryKey;autoIncrement;column:id"`
	Name        string `gorm:"column:name"`
	Parameters  string `gorm:"column:parameters;type:longtext"` // JSON 数组
	Scope       string `gorm:"column:scope;type:varchar(255)"`  // "deviceGroup" 或 "device"
	ScopeIds    string `gorm:"column:scope_ids;type:longtext"`  // 逗号分隔的 ID 列表
	TenancyId   int    `gorm:"column:tenancy_id"`
}

// deploymentParamItem 是模板 parameters JSON 中的单个参数项
type deploymentParamItem struct {
	ParamPath  string `json:"path"`
	ParamValue string `json:"value"`
	ParamId    string `json:"parameterId"`
}

// NewParameterDeploymentTask 创建 ParameterDeploymentTask 实例
func NewParameterDeploymentTask(db *gorm.DB, opSender *tr069.OperationSender) *ParameterDeploymentTask {
	return &ParameterDeploymentTask{
		db:       db,
		opSender: opSender,
	}
}

// DeployParameters 执行参数部署检查
// 1. 查询所有 parameter_deployment_template
// 2. 对每个模板解析 parameters JSON
// 3. 根据 scope 获取目标 elementId 列表
// 4. 对每个在线且不在黑名单的设备比较参数值
// 5. 有差异则通过 SPV 下发
func (t *ParameterDeploymentTask) DeployParameters() {
	ctx := context.Background()

	// 1. 查询所有 parameter_deployment_template
	var templates []deploymentTemplate
	if err := t.db.Table("parameter_deployment_template").Find(&templates).Error; err != nil {
		logger.Errorf("ParameterDeploymentTask: query parameter_deployment_template failed: %v", err)
		return
	}

	for _, tpl := range templates {
		t.deployTemplate(ctx, tpl)
	}
}

// deployTemplate 部署单个模板
func (t *ParameterDeploymentTask) deployTemplate(ctx context.Context, tpl deploymentTemplate) {
	// 2. 解析 parameters JSON
	var paramItems []deploymentParamItem
	if err := json.Unmarshal([]byte(tpl.Parameters), &paramItems); err != nil {
		logger.Warnf("ParameterDeploymentTask: failed to parse parameters for template %d: %v", tpl.Id, err)
		return
	}
	if len(paramItems) == 0 {
		return
	}

	// 3. 根据 scope 获取目标 elementId 列表
	elementIds, err := t.resolveTargetElements(tpl)
	if err != nil {
		logger.Warnf("ParameterDeploymentTask: failed to resolve targets for template %d: %v", tpl.Id, err)
		return
	}
	if len(elementIds) == 0 {
		return
	}

	// 4. 对每个在线且不在黑名单的设备进行比较和下发
	for _, elementId := range elementIds {
		t.deployToDevice(ctx, tpl, elementId, paramItems)
	}
}

// resolveTargetElements 根据 scope 解析目标设备 elementId 列表
func (t *ParameterDeploymentTask) resolveTargetElements(tpl deploymentTemplate) ([]int64, error) {
	switch tpl.Scope {
	case "deviceGroup":
		// 从 group_has_element 关联表获取
		var ids []int64
		if err := t.db.Table("group_has_element").
			Select("element_id").
			Where("group_id IN (SELECT id FROM device_group WHERE id IN (?))", parseScopeIds(tpl.ScopeIds)).
			Pluck("element_id", &ids).Error; err != nil {
			return nil, err
		}
		return ids, nil
	case "device":
		// 直接使用 scope_ids 作为 elementId 列表
		return parseScopeIdsAsInt64(tpl.ScopeIds), nil
	default:
		return nil, fmt.Errorf("unsupported scope: %s", tpl.Scope)
	}
}

// deployToDevice 对单个设备进行参数部署
func (t *ParameterDeploymentTask) deployToDevice(ctx context.Context, tpl deploymentTemplate, elementId int64, paramItems []deploymentParamItem) {
	// 查设备 SN 和在线状态
	var deviceInfo struct {
		SerialNumber string `gorm:"column:serial_number"`
	}
	if err := t.db.Table("cpe_element").
		Select("serial_number").
		Where("ne_neid = ? AND deleted = ?", elementId, false).
		Scan(&deviceInfo).Error; err != nil || deviceInfo.SerialNumber == "" {
		return
	}
	sn := deviceInfo.SerialNumber

	// 检查在线状态
	onlineKey := fmt.Sprintf("online_%d", elementId)
	onlineVal, err := redis.Get(ctx, onlineKey)
	if err != nil || onlineVal != "yes" {
		return
	}

	// 检查黑名单
	var blackListCount int64
	t.db.Table("element_black_list").
		Where("sn = ?", sn).
		Count(&blackListCount)
	if blackListCount > 0 {
		return
	}

	// 5. 查当前参数值，收集不一致的参数
	var diffParams []soap.ParameterValueStruct
	for _, item := range paramItems {
		if item.ParamPath == "" {
			continue
		}

		var currentValue string
		row := struct {
			ParamValue *string `gorm:"column:param_value"`
		}{}
		if err := t.db.Table("element_basic_info_parameter").
			Select("param_value").
			Where("element_id = ? AND param_name = ?", elementId, item.ParamPath).
			Scan(&row).Error; err != nil {
			// 查不到当前值，视为不一致
			diffParams = append(diffParams, soap.ParameterValueStruct{
				Name:  item.ParamPath,
				Value: item.ParamValue,
				Type:  "xsd:string",
			})
			continue
		}

		currentValue = ""
		if row.ParamValue != nil {
			currentValue = *row.ParamValue
		}

		if currentValue != item.ParamValue {
			diffParams = append(diffParams, soap.ParameterValueStruct{
				Name:  item.ParamPath,
				Value: item.ParamValue,
				Type:  "xsd:string",
			})
		}
	}

	if len(diffParams) == 0 {
		return
	}

	// 通过 SPV 下发
	operationId := fmt.Sprintf("param_deploy_%d_%d", tpl.Id, elementId)
	if err := t.opSender.SendSetParameterValues(sn, diffParams, "", operationId); err != nil {
		logger.Errorf("ParameterDeploymentTask: failed to send SPV to %s for template %d: %v", sn, tpl.Id, err)
		return
	}

	// 记录部署日志
	now := time.Now()
	result := true
	info := fmt.Sprintf("deployed %d params from template %d", len(diffParams), tpl.Id)
	t.db.Table("parameter_deployment_log").Create(map[string]interface{}{
		"template_id":     tpl.Id,
		"element_id":      elementId,
		"result":          result,
		"info":            info,
		"operation_time":  now,
		"tenancy_id":      tpl.TenancyId,
	})

	logger.Infof("ParameterDeploymentTask: deployed %d params to %s (element %d, template %d)",
		len(diffParams), sn, elementId, tpl.Id)
}

// parseScopeIds 解析逗号分隔的 scope ID 字符串
func parseScopeIds(ids string) []string {
	if ids == "" {
		return nil
	}
	var result []string
	for _, id := range splitComma(ids) {
		if id != "" {
			result = append(result, id)
		}
	}
	return result
}

// parseScopeIdsAsInt64 解析逗号分隔的 scope ID 字符串为 int64 列表
func parseScopeIdsAsInt64(ids string) []int64 {
	strs := parseScopeIds(ids)
	var result []int64
	for _, s := range strs {
		var v int64
		fmt.Sscanf(s, "%d", &v)
		if v > 0 {
			result = append(result, v)
		}
	}
	return result
}

// splitComma 按逗号分隔字符串
func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := s[start:i]
			if part != "" {
				result = append(result, part)
			}
			start = i + 1
		}
	}
	return result
}
