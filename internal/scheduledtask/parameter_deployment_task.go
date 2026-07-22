package scheduledtask

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/internal/parameter"
	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

type ParameterDeploymentTask struct {
	db       *gorm.DB
	opSender *tr069.OperationSender
}

type deploymentParamItem struct {
	ParameterNames  string `json:"parameterNames"`
	ParameterValue  string `json:"parameterValue"`
}

func NewParameterDeploymentTask(db *gorm.DB, opSender *tr069.OperationSender) *ParameterDeploymentTask {
	return &ParameterDeploymentTask{
		db:       db,
		opSender: opSender,
	}
}

func (t *ParameterDeploymentTask) DeployParameters() {
	ctx := context.Background()

	var templates []parameter.ParameterDeploymentTemplate
	if err := t.db.Find(&templates).Error; err != nil {
		logger.Errorf("ParameterDeploymentTask: query parameter_deployment_template failed: %v", err)
		return
	}

	for _, tpl := range templates {
		t.deployTemplate(ctx, tpl)
	}
}

func (t *ParameterDeploymentTask) deployTemplate(ctx context.Context, tpl parameter.ParameterDeploymentTemplate) {
	if tpl.Parameters == nil || *tpl.Parameters == "" {
		return
	}

	var paramItems []deploymentParamItem
	if err := json.Unmarshal([]byte(*tpl.Parameters), &paramItems); err != nil {
		logger.Warnf("ParameterDeploymentTask: failed to parse parameters for template %d: %v", tpl.Id, err)
		return
	}
	if len(paramItems) == 0 {
		return
	}

	elementIds, err := t.resolveTargetElements(tpl)
	if err != nil {
		logger.Warnf("ParameterDeploymentTask: failed to resolve targets for template %d: %v", tpl.Id, err)
		return
	}
	if len(elementIds) == 0 {
		return
	}

	paramNames := make([]string, 0, len(paramItems))
	for _, item := range paramItems {
		paramNames = append(paramNames, item.ParameterNames)
	}

	for _, elementId := range elementIds {
		t.deployToDevice(ctx, tpl, elementId, paramItems, paramNames)
	}
}

func (t *ParameterDeploymentTask) resolveTargetElements(tpl parameter.ParameterDeploymentTemplate) ([]int64, error) {
	scope := ""
	if tpl.Scope != nil {
		scope = *tpl.Scope
	}

	if scope == "deviceGroup" {
		if tpl.DeviceGroupIds == nil || *tpl.DeviceGroupIds == "" {
			return nil, nil
		}
		var groupIds []string
		if err := json.Unmarshal([]byte(*tpl.DeviceGroupIds), &groupIds); err != nil {
			return nil, err
		}
		if len(groupIds) == 0 {
			return nil, nil
		}
		var ids []int64
		if err := t.db.Table("group_has_element").
			Select("element_id").
			Where("group_id IN ?", groupIds).
			Pluck("element_id", &ids).Error; err != nil {
			return nil, err
		}
		return ids, nil
	}

	var ids []int64
	if err := t.db.Table("parameter_deployment_template_has_element").
		Select("element_id").
		Where("template_id = ?", tpl.Id).
		Pluck("element_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (t *ParameterDeploymentTask) deployToDevice(ctx context.Context, tpl parameter.ParameterDeploymentTemplate, elementId int64, paramItems []deploymentParamItem, paramNames []string) {
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

	onlineKey := fmt.Sprintf("online_%d", elementId)
	onlineVal, err := redis.Get(ctx, onlineKey)
	if err != nil || onlineVal != "yes" {
		return
	}

	var blackListCount int64
	t.db.Table("element_black_list").
		Where("sn = ?", sn).
		Count(&blackListCount)
	if blackListCount > 0 {
		return
	}

	type elemParamRow struct {
		ParamName  string `gorm:"column:param_name"`
		ParamValue string `gorm:"column:param_value"`
		ParamType  string `gorm:"column:param_type"`
	}
	var elemParams []elemParamRow
	if err := t.db.Table("element_basic_info_parameter").
		Select("param_name, param_value, param_type").
		Where("element_id = ? AND param_name IN ?", elementId, paramNames).
		Find(&elemParams).Error; err != nil {
		return
	}

	if len(elemParams) != len(paramNames) {
		tplName := ""
		if tpl.TemplateName != nil {
			tplName = *tpl.TemplateName
		}
		logger.Debugf("ParameterDeploymentTask: %s doesn't trigger parameter deployment for %s", tplName, sn)
		return
	}

	nameToValue := make(map[string]string)
	nameToType := make(map[string]string)
	for _, p := range elemParams {
		if p.ParamValue != "" {
			nameToValue[p.ParamName] = p.ParamValue
		}
		if p.ParamType != "" {
			nameToType[p.ParamName] = p.ParamType
		}
	}

	var diffParams []soap.ParameterValueStruct
	for _, item := range paramItems {
		if item.ParameterValue != nameToValue[item.ParameterNames] {
			paramType := nameToType[item.ParameterNames]
			if paramType == "" {
				paramType = "xsd:string"
			}
			diffParams = append(diffParams, soap.ParameterValueStruct{
				Name:  item.ParameterNames,
				Value: item.ParameterValue,
				Type:  paramType,
			})
		}
	}

	if len(diffParams) == 0 {
		return
	}

	operationId := fmt.Sprintf("param_deploy_%d_%d", tpl.Id, elementId)
	if err := t.opSender.SendSetParameterValues(sn, diffParams, "", operationId); err != nil {
		logger.Errorf("ParameterDeploymentTask: failed to send SPV to %s for template %d: %v", sn, tpl.Id, err)
		return
	}

	now := time.Now()
	tenantId := 0
	if tpl.TenantId != nil {
		tenantId = *tpl.TenantId
	}
	result := true
	info := fmt.Sprintf("deployed %d params from template %d", len(diffParams), tpl.Id)
	t.db.Table("parameter_deployment_log").Create(map[string]interface{}{
		"template_id":     tpl.Id,
		"element_id":      elementId,
		"result":          result,
		"info":            info,
		"operation_time":  now,
		"tenant_id":      tenantId,
	})

	logger.Infof("ParameterDeploymentTask: deployed %d params to %s (element %d, template %d)",
		len(diffParams), sn, elementId, tpl.Id)
}
