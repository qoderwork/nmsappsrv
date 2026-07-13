package parameter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// ---------------------------------------------------------------------------
// ParameterTemplate
// ---------------------------------------------------------------------------

// deployParamValue is a template parameter's TR-069 path together with its
// DEFINED (target) value, as read from parameter_template_has_parameter.
type deployParamValue struct {
	ParamPath  string `gorm:"column:path"`
	ParamValue string `gorm:"column:parameter_value"`
}

// buildDeploySPV converts template-defined parameter values into the SPV
// entries/dispatched structs that are sent to the device. The values pushed are
// the template's DEFINED values (对齐 Java), never the device's current values.
func buildDeploySPV(params []deployParamValue) ([]setParamEntry, []soap.ParameterValueStruct) {
	entries := make([]setParamEntry, len(params))
	spv := make([]soap.ParameterValueStruct, len(params))
	for i, p := range params {
		entries[i] = setParamEntry{ParamName: p.ParamPath, ParamValue: p.ParamValue}
		spv[i] = soap.ParameterValueStruct{Name: p.ParamPath, Value: p.ParamValue, Type: "xsd:string"}
	}
	return entries, spv
}

// ListParameterTemplates returns all templates for the given tenancy.
func (s *service) ListParameterTemplates(tenancyId int) ([]ParameterTemplate, error) {
	return s.repo.FindParameterTemplates(tenancyId)
}

// CreateParameterTemplate persists a new parameter template together with its
// parameter list and DEFINED values.
func (s *service) CreateParameterTemplate(req *ParameterTemplateRequest) error {
	t := &ParameterTemplate{
		Name:        req.Name,
		Description: req.Description,
		TenancyId:   req.TenancyId,
		IsDefault:   req.IsDefault,
	}
	if err := s.repo.CreateParameterTemplate(t); err != nil {
		return err
	}
	return s.repo.SaveTemplateParameters(t.Id, req.Parameters)
}

// UpdateParameterTemplate persists changes to an existing parameter template
// and replaces its parameter list (with DEFINED values).
func (s *service) UpdateParameterTemplate(req *ParameterTemplateRequest) error {
	t := &ParameterTemplate{
		Id:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		TenancyId:   req.TenancyId,
		IsDefault:   req.IsDefault,
	}
	if err := s.repo.UpdateParameterTemplate(t); err != nil {
		return err
	}
	return s.repo.SaveTemplateParameters(req.ID, req.Parameters)
}

// ---------------------------------------------------------------------------
// DeployTemplate
// ---------------------------------------------------------------------------

// DeployTemplateStatus holds the per-device result of a template deployment.
type DeployTemplateStatus struct {
	ElementId    int64  `json:"elementId"`
	SerialNumber string `json:"serialNumber"`
	DeviceName   string `json:"deviceName"`
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	ParamCount   int    `json:"paramCount"`
}

// DeployTemplate deploys a parameter template to the specified target devices.
// It loads the template's parameter paths, reads the DEFINED (target) values
// stored in parameter_template_has_parameter, and sends SPV commands via TR-069.
// 对齐 Java ParameterDeploymentTemplate: 下发的是模板"定义值", 而非设备当前值.
func (s *service) DeployTemplate(templateId int64, elementIds []int64, username string) ([]DeployTemplateStatus, error) {
	if len(elementIds) == 0 {
		return nil, fmt.Errorf("no target devices specified")
	}

	// 1. Load template's parameter paths via parameter_template_has_parameter JOIN parameter
	var paramPaths []string
	err := s.repo.DB().Raw(`
		SELECT p.path FROM parameter_template_has_parameter pth
		JOIN parameter p ON p.id = pth.parameter_id
		WHERE pth.template_id = ? AND p.path IS NOT NULL AND p.path != ''
	`, templateId).Scan(&paramPaths).Error
	if err != nil {
		return nil, fmt.Errorf("load template parameters: %w", err)
	}
	if len(paramPaths) == 0 {
		return nil, fmt.Errorf("template %d has no parameters", templateId)
	}

	ctx := context.Background()
	now := time.Now()
	var results []DeployTemplateStatus

	for _, elementId := range elementIds {
		status := DeployTemplateStatus{ElementId: elementId}

		// 2. Resolve device SN and name
		var deviceInfo struct {
			SerialNumber string `gorm:"column:serial_number"`
			DeviceName   string `gorm:"column:device_name"`
		}
		if err := s.repo.DB().Table("cpe_element").
			Select("serial_number, device_name").
			Where("ne_neid = ? AND deleted = ?", elementId, false).
			Scan(&deviceInfo).Error; err != nil {
			status.Message = fmt.Sprintf("device not found: %v", err)
			results = append(results, status)
			s.insertDeployLog(templateId, elementId, 0, false, status.Message, now)
			continue
		}
		if deviceInfo.SerialNumber == "" {
			status.Message = "device has no serial number"
			results = append(results, status)
			s.insertDeployLog(templateId, elementId, 0, false, status.Message, now)
			continue
		}
		status.SerialNumber = deviceInfo.SerialNumber
		status.DeviceName = deviceInfo.DeviceName

		// 3. Read DEFINED (target) values from parameter_template_has_parameter.
		//    对齐 Java: 下发模板中"定义"的值, 而非设备当前值(element_basic_info_parameter).
		var paramValues []deployParamValue
		if err := s.repo.DB().Raw(`
			SELECT p.path AS path, COALESCE(pth.parameter_value, '') AS parameter_value
			FROM parameter_template_has_parameter pth
			JOIN parameter p ON p.id = pth.parameter_id
			WHERE pth.template_id = ? AND p.path IS NOT NULL AND p.path != ''
		`, templateId).Scan(&paramValues).Error; err != nil {
			status.Message = fmt.Sprintf("load template defined values: %v", err)
			results = append(results, status)
			s.insertDeployLog(templateId, elementId, 0, false, status.Message, now)
			continue
		}

		if len(paramValues) == 0 {
			status.Message = "template has no defined parameter values to deploy"
			results = append(results, status)
			s.insertDeployLog(templateId, elementId, 0, false, status.Message, now)
			continue
		}

		// 4. Build SPV entries from the template's DEFINED values
		entries, spvParams := buildDeploySPV(paramValues)
		opParamJSON, _ := json.Marshal(entries)

		// 5. Create event_log (status=1 pending)
		eventLogId, err := s.repo.InsertEventLog("SetParameterValues", elementId, username, 1, "")
		if err != nil {
			status.Message = fmt.Sprintf("create event_log failed: %v", err)
			results = append(results, status)
			s.insertDeployLog(templateId, elementId, 0, false, status.Message, now)
			continue
		}

		// 6. Build SOAP XML
		headerId := soap.GenerateHeaderID()
		soapXml := soap.BuildSetParameterValues(headerId, spvParams, "")

		// 7. Update event_log with tracking data
		trackData, _ := json.Marshal(map[string]interface{}{
			"header_id":      headerId,
			"serial_number":  deviceInfo.SerialNumber,
			"operation_type": "SET_PARAMETER_VALUES",
			"operationParam": string(opParamJSON),
			"event_log_id":   eventLogId,
			"template_id":    templateId,
			"issue_time":     now.Format(time.RFC3339),
		})
		s.repo.DB().Table("event_log").Where("id = ?", eventLogId).
			Updates(map[string]interface{}{
				"command_track_data": string(trackData),
				"command_issue_time": now,
			})

		// 8. Cache track data in Redis
		trackKey := fmt.Sprintf("tr069:track:%s", headerId)
		trackJson, _ := json.Marshal(map[string]interface{}{
			"header_id":      headerId,
			"sn":             deviceInfo.SerialNumber,
			"operation_type": "SET_PARAMETER_VALUES",
			"event_log_id":   eventLogId,
			"template_id":    templateId,
		})
		redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

		// 9. Push SOAP XML to device queue
		queueKey := fmt.Sprintf("tr069:queue:%s", deviceInfo.SerialNumber)
		if err := redis.LPush(ctx, queueKey, soapXml); err != nil {
			status.Message = fmt.Sprintf("push to device queue failed: %v", err)
			s.repo.DB().Table("event_log").Where("id = ?", eventLogId).Update("status", 4)
			results = append(results, status)
			s.insertDeployLog(templateId, elementId, eventLogId, false, status.Message, now)
			continue
		}
		redis.Expire(ctx, queueKey, 24*time.Hour)

		status.Success = true
		status.ParamCount = len(paramValues)
		status.Message = "SPV dispatched successfully"
		results = append(results, status)
		s.insertDeployLog(templateId, elementId, eventLogId, true, status.Message, now)

		logger.Infof("DeployTemplate: dispatched %d params to device %s (elementId=%d) from template %d",
			len(paramValues), deviceInfo.SerialNumber, elementId, templateId)
	}

	return results, nil
}

// insertDeployLog is a helper that creates a parameter_deployment_log entry.
// Errors are logged but not propagated since logging is non-fatal.
func (s *service) insertDeployLog(templateId, elementId, eventLogId int64, result bool, info string, opTime time.Time) {
	log := &ParameterDeploymentLog{
		TemplateId:    &templateId,
		ElementId:     &elementId,
		Result:        &result,
		Info:          &info,
		OperationTime: &opTime,
	}
	if eventLogId > 0 {
		log.EventLogId = &eventLogId
	}
	if err := s.repo.CreateDeployTemplateLog(log); err != nil {
		logger.Errorf("failed to create deploy template log: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ListDeployTemplateLogs
// ---------------------------------------------------------------------------

// ListDeployTemplateLogs returns a paginated list of deployment history logs
// for the given template, enriched with device and template names.
func (s *service) ListDeployTemplateLogs(templateId int64, page, pageSize int) ([]DeployTemplateLogVo, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	logs, total, err := s.repo.FindDeployTemplateLogs(templateId, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	// Resolve template name once
	templateName := ""
	var tpl ParameterTemplate
	if err := s.repo.DB().Where("id = ?", templateId).First(&tpl).Error; err == nil && tpl.Name != nil {
		templateName = *tpl.Name
	}

	// Build VOs, resolving device info per element
	vos := make([]DeployTemplateLogVo, 0, len(logs))
	for _, l := range logs {
		vo := DeployTemplateLogVo{
			Id:           l.Id,
			TemplateName: templateName,
			Result:       l.Result,
		}
		if l.Info != nil {
			vo.Info = *l.Info
		}
		if l.OperationTime != nil {
			vo.OperationTime = l.OperationTime.Format(time.RFC3339)
		}

		if l.ElementId != nil {
			var deviceInfo struct {
				SerialNumber string `gorm:"column:serial_number"`
				DeviceName   string `gorm:"column:device_name"`
			}
			if err := s.repo.DB().Table("cpe_element").
				Select("serial_number, device_name").
				Where("ne_neid = ?", *l.ElementId).
				Scan(&deviceInfo).Error; err == nil {
				vo.DeviceName = deviceInfo.DeviceName
				vo.SerialNumber = deviceInfo.SerialNumber
			}
		}

		vos = append(vos, vo)
	}

	return vos, total, nil
}
