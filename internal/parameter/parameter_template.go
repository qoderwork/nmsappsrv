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

// ListParameterTemplates returns all templates for the given tenancy.
func (s *Service) ListParameterTemplates(tenancyId int) ([]ParameterTemplate, error) {
	return s.repo.FindParameterTemplates(tenancyId)
}

// CreateParameterTemplate persists a new parameter template.
func (s *Service) CreateParameterTemplate(t *ParameterTemplate) error {
	return s.repo.CreateParameterTemplate(t)
}

// UpdateParameterTemplate persists changes to an existing parameter template.
func (s *Service) UpdateParameterTemplate(t *ParameterTemplate) error {
	return s.repo.UpdateParameterTemplate(t)
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
// It loads the template's parameter paths, reads the desired values from
// element_basic_info_parameter for each device, and sends SPV commands via TR-069.
func (s *Service) DeployTemplate(templateId int64, elementIds []int64, username string) ([]DeployTemplateStatus, error) {
	if len(elementIds) == 0 {
		return nil, fmt.Errorf("no target devices specified")
	}

	// 1. Load template's parameter paths via parameter_template_has_parameter JOIN parameter
	var paramPaths []string
	err := s.repo.db.Raw(`
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
		if err := s.repo.db.Table("cpe_element").
			Select("serial_number, device_name").
			Where("ne_neid = ? AND deleted = ?", elementId, false).
			Scan(&deviceInfo).Error; err != nil {
			status.Message = fmt.Sprintf("device not found: %v", err)
			results = append(results, status)
			continue
		}
		if deviceInfo.SerialNumber == "" {
			status.Message = "device has no serial number"
			results = append(results, status)
			continue
		}
		status.SerialNumber = deviceInfo.SerialNumber
		status.DeviceName = deviceInfo.DeviceName

		// 3. Read desired parameter values from element_basic_info_parameter
		var paramValues []struct {
			ParamName  string `gorm:"column:param_name"`
			ParamValue string `gorm:"column:param_value"`
		}
		s.repo.db.Table("element_basic_info_parameter").
			Select("param_name, param_value").
			Where("element_id = ? AND param_name IN ?", elementId, paramPaths).
			Scan(&paramValues)

		if len(paramValues) == 0 {
			status.Message = "no parameter values found for device"
			results = append(results, status)
			continue
		}

		// 4. Build SPV entries
		entries := make([]setParamEntry, len(paramValues))
		spvParams := make([]soap.ParameterValueStruct, len(paramValues))
		for i, pv := range paramValues {
			entries[i] = setParamEntry{ParamName: pv.ParamName, ParamValue: pv.ParamValue}
			spvParams[i] = soap.ParameterValueStruct{Name: pv.ParamName, Value: pv.ParamValue, Type: "xsd:string"}
		}
		opParamJSON, _ := json.Marshal(entries)

		// 5. Create event_log (status=1 pending)
		eventLogId, err := s.repo.InsertEventLog("SetParameterValues", elementId, username, 1, "")
		if err != nil {
			status.Message = fmt.Sprintf("create event_log failed: %v", err)
			results = append(results, status)
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
		s.repo.db.Table("event_log").Where("id = ?", eventLogId).
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
			s.repo.db.Table("event_log").Where("id = ?", eventLogId).Update("status", 4)
			results = append(results, status)
			continue
		}
		redis.Expire(ctx, queueKey, 24*time.Hour)

		status.Success = true
		status.ParamCount = len(paramValues)
		status.Message = "SPV dispatched successfully"
		results = append(results, status)

		logger.Infof("DeployTemplate: dispatched %d params to device %s (elementId=%d) from template %d",
			len(paramValues), deviceInfo.SerialNumber, elementId, templateId)
	}

	return results, nil
}
