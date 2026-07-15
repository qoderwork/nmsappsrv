package parameter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
)

// ---------------------------------------------------------------------------
// Model Tree - Service implementation
// ---------------------------------------------------------------------------

// resolveDeviceSN looks up the serial number for a given element ID.
func (s *service) resolveDeviceSN(elementId int64) (string, error) {
	var deviceInfo struct {
		SerialNumber string `gorm:"column:serial_number"`
	}
	if err := s.repo.DB().Table("cpe_element").
		Select("serial_number").
		Where("ne_neid = ? AND deleted = ?", elementId, false).
		Scan(&deviceInfo).Error; err != nil {
		return "", fmt.Errorf("device not found: %w", err)
	}
	if deviceInfo.SerialNumber == "" {
		return "", fmt.Errorf("device %d has no serial number", elementId)
	}
	return deviceInfo.SerialNumber, nil
}

// dispatchSoapCommand pushes a SOAP XML command to the device Redis queue and
// creates the corresponding event_log tracking entry.
func (s *service) dispatchSoapCommand(elementId int64, serialNumber string, eventType string, soapXml string, headerId string, username string) error {
	ctx := context.Background()
	now := time.Now()

	// Create event_log (status=1 pending)
	eventLogId, err := s.repo.InsertEventLog(eventType, elementId, username, 1, "")
	if err != nil {
		return fmt.Errorf("create event_log: %w", err)
	}

	// Update event_log with tracking data
	trackData, _ := json.Marshal(map[string]interface{}{
		"header_id":      headerId,
		"serial_number":  serialNumber,
		"operation_type": eventType,
		"event_log_id":   eventLogId,
		"issue_time":     now.Format(time.RFC3339),
	})
	s.repo.DB().Table("event_log").Where("id = ?", eventLogId).
		Updates(map[string]interface{}{
			"command_track_data": string(trackData),
			"command_issue_time": now,
		})

	// Cache track data in Redis
	trackKey := fmt.Sprintf("tr069:track:%s", headerId)
	trackJson, _ := json.Marshal(map[string]interface{}{
		"header_id":      headerId,
		"sn":             serialNumber,
		"operation_type": eventType,
		"event_log_id":   eventLogId,
	})
	redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour)

	// Push SOAP XML to device queue
	queueKey := fmt.Sprintf("tr069:queue:%s", serialNumber)
	if err := redis.LPush(ctx, queueKey, soapXml); err != nil {
		s.repo.DB().Table("event_log").Where("id = ?", eventLogId).Update("status", 4)
		return fmt.Errorf("push to device queue: %w", err)
	}
	redis.Expire(ctx, queueKey, 24*time.Hour)

	return nil
}

// GetModelTree builds a parameter tree for the given device element.
func (s *service) GetModelTree(elementId int64) (*ModelTreeNode, error) {
	_, err := s.resolveDeviceSN(elementId)
	if err != nil {
		return nil, err
	}

	params, err := s.repo.FindParameterVosByElementId(elementId)
	if err != nil {
		return nil, fmt.Errorf("get parameters: %w", err)
	}

	root := &ModelTreeNode{
		Name:     "InternetGatewayDevice",
		Path:     "InternetGatewayDevice.",
		IsObject: true,
	}

	for _, p := range params {
		path := p.Tr069Name
		if path == "" {
			path = p.ParamName
		}
		if path == "" {
			continue
		}
		insertIntoTree(root, path, p)
	}

	return root, nil
}

func insertIntoTree(root *ModelTreeNode, fullPath string, p ParameterVo) {
	parts := splitParamPath(fullPath)
	if len(parts) == 0 {
		return
	}

	current := root
	for i := 0; i < len(parts)-1; i++ {
		found := false
		for j := range current.Children {
			if current.Children[j].Name == parts[i] {
				current = &current.Children[j]
				found = true
				break
			}
		}
		if !found {
			child := ModelTreeNode{
				Name:     parts[i],
				Path:     buildPath(parts[:i+1]),
				IsObject: true,
			}
			current.Children = append(current.Children, child)
			current = &current.Children[len(current.Children)-1]
		}
	}

	leaf := ModelTreeNode{
		Name:      parts[len(parts)-1],
		Path:      fullPath,
		Value:     p.Value,
		ParamType: p.Type,
		Writable:  p.Writable,
		IsObject:  false,
	}
	current.Children = append(current.Children, leaf)
}

func splitParamPath(path string) []string {
	var parts []string
	current := ""
	for _, ch := range path {
		if ch == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func buildPath(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "."
		}
		result += p
	}
	return result + "."
}

// RefreshParameter sends GetParameterNames to discover child parameters.
func (s *service) RefreshParameter(elementId int64, paramPath string, username string) error {
	sn, err := s.resolveDeviceSN(elementId)
	if err != nil {
		return err
	}

	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetParameterNames(headerId, paramPath, true)

	logger.Infof("RefreshParameter: GPN dispatched to device %s (elementId=%d) for path %s",
		sn, elementId, paramPath)
	return s.dispatchSoapCommand(elementId, sn, "GetParameterNames", soapXml, headerId, username)
}

// ReloadParameter sends GetParameterValues to reload current values.
func (s *service) ReloadParameter(elementId int64, paramPaths []string, username string) error {
	if len(paramPaths) == 0 {
		return fmt.Errorf("parameterPaths must not be empty")
	}

	sn, err := s.resolveDeviceSN(elementId)
	if err != nil {
		return err
	}

	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetParameterValues(headerId, paramPaths)

	logger.Infof("ReloadParameter: GPV dispatched to device %s (elementId=%d) for %d params",
		sn, elementId, len(paramPaths))
	return s.dispatchSoapCommand(elementId, sn, "GetParameterValues", soapXml, headerId, username)
}

// AddObject sends a TR-069 AddObject RPC to the device.
func (s *service) AddObject(elementId int64, objectName string, username string) error {
	sn, err := s.resolveDeviceSN(elementId)
	if err != nil {
		return err
	}

	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildAddObject(headerId, objectName, "")

	logger.Infof("AddObject: dispatched to device %s (elementId=%d) for object %s",
		sn, elementId, objectName)
	return s.dispatchSoapCommand(elementId, sn, "AddObject", soapXml, headerId, username)
}

// DeleteObject sends a TR-069 DeleteObject RPC to the device.
func (s *service) DeleteObject(elementId int64, objectName string, username string) error {
	sn, err := s.resolveDeviceSN(elementId)
	if err != nil {
		return err
	}

	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildDeleteObject(headerId, objectName, "")

	logger.Infof("DeleteObject: dispatched to device %s (elementId=%d) for object %s",
		sn, elementId, objectName)
	return s.dispatchSoapCommand(elementId, sn, "DeleteObject", soapXml, headerId, username)
}

// BatchDeleteObject sends multiple TR-069 DeleteObject RPCs to the device.
func (s *service) BatchDeleteObject(elementId int64, objectNames []string, username string) error {
	if len(objectNames) == 0 {
		return fmt.Errorf("objectNames must not be empty")
	}

	sn, err := s.resolveDeviceSN(elementId)
	if err != nil {
		return err
	}

	for _, objectName := range objectNames {
		headerId := soap.GenerateHeaderID()
		soapXml := soap.BuildDeleteObject(headerId, objectName, "")

		if err := s.dispatchSoapCommand(elementId, sn, "DeleteObject", soapXml, headerId, username); err != nil {
			logger.Errorf("BatchDeleteObject: failed to delete %s on device %s: %v",
				objectName, sn, err)
		}
	}

	logger.Infof("BatchDeleteObject: dispatched %d delete operations to device %s (elementId=%d)",
		len(objectNames), sn, elementId)
	return nil
}

// DeleteObjectAfterNeedReboot calls BatchDeleteObject then dispatches a
// SoftReboot command to the device. Mirrors Java
// ModelTreeManagementController.deleteObjectAfterNeedReboot.
func (s *service) DeleteObjectAfterNeedReboot(elementId int64, objectNames []string, username string) error {
	if err := s.BatchDeleteObject(elementId, objectNames, username); err != nil {
		return fmt.Errorf("delete objects: %w", err)
	}

	sn, err := s.resolveDeviceSN(elementId)
	if err != nil {
		return fmt.Errorf("resolve SN for reboot: %w", err)
	}

	// Dispatch a SoftReboot after deletion.
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildSoftReboot(headerId, "delete+reboot")

	logger.Infof("DeleteObjectAfterNeedReboot: dispatching reboot to device %s (elementId=%d) after deleting %d objects",
		sn, elementId, len(objectNames))
	return s.dispatchSoapCommand(elementId, sn, "Reboot", soapXml, headerId, username)
}

// ---------------------------------------------------------------------------
// Export Parameter Template - Service implementation
// ---------------------------------------------------------------------------

// ExportParameterTemplate generates a CSV file from the template parameters.
func (s *service) ExportParameterTemplate(templateId int64) ([]byte, string, error) {
	var tpl ParameterTemplate
	if err := s.repo.DB().Where("id = ?", templateId).First(&tpl).Error; err != nil {
		return nil, "", fmt.Errorf("template not found: %w", err)
	}

	templateName := "parameter_template"
	if tpl.Name != nil && *tpl.Name != "" {
		templateName = *tpl.Name
	}

	var rows []struct {
		Name       string `gorm:"column:name"`
		Path       string `gorm:"column:path"`
		ParamType  string `gorm:"column:param_type"`
		Remark     string `gorm:"column:remark"`
		Unit       string `gorm:"column:unit"`
		Length     *int   `gorm:"column:length"`
		IsWritable *bool  `gorm:"column:is_writable"`
	}
	err := s.repo.DB().Raw(`
		SELECT p.name, COALESCE(p.path, '') AS path,
		       COALESCE(p.param_type, '') AS param_type,
		       COALESCE(p.remark, '') AS remark,
		       COALESCE(p.unit, '') AS unit,
		       p.length, p.is_writable
		FROM parameter_template_has_parameter pth
		JOIN parameter p ON p.id = pth.parameter_id
		WHERE pth.template_id = ?
		ORDER BY p.sort, p.name
	`, templateId).Scan(&rows).Error
	if err != nil {
		return nil, "", fmt.Errorf("load template parameters: %w", err)
	}

	csv := "name,path,type,remark,unit,length,writable\n"
	for _, r := range rows {
		writable := "false"
		if r.IsWritable != nil && *r.IsWritable {
			writable = "true"
		}
		length := ""
		if r.Length != nil {
			length = strconv.Itoa(*r.Length)
		}
		csv += fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s\n",
			escapeCSV(r.Name), escapeCSV(r.Path), escapeCSV(r.ParamType),
			escapeCSV(r.Remark), escapeCSV(r.Unit), length, writable)
	}

	filename := fmt.Sprintf("%s_%d.csv", templateName, templateId)
	return []byte(csv), filename, nil
}

func escapeCSV(s string) string {
	needsQuote := false
	for _, ch := range s {
		if ch == ',' || ch == '"' || ch == '\n' || ch == '\r' {
			needsQuote = true
			break
		}
	}
	if !needsQuote {
		return s
	}
	escaped := ""
	for _, ch := range s {
		if ch == '"' {
			escaped += `""`
		} else {
			escaped += string(ch)
		}
	}
	return `"` + escaped + `"`
}

// ---------------------------------------------------------------------------
// Model Tree - HTTP handlers
// ---------------------------------------------------------------------------

// GetModelTree handles GET /model-tree/:elementId
func (h *Handler) GetModelTree(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Param("elementId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element id")
		return
	}

	tree, err := h.svc.GetModelTree(elementId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, tree)
}

// RefreshParameter handles POST /model-tree/:elementId/refresh
func (h *Handler) RefreshParameter(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Param("elementId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element id")
		return
	}

	var req RefreshParameterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.svc.RefreshParameter(elementId, req.ParameterPath, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ReloadParameter handles POST /model-tree/:elementId/reload
func (h *Handler) ReloadParameter(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Param("elementId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element id")
		return
	}

	var req ReloadParameterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.svc.ReloadParameter(elementId, req.ParameterPaths, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// AddObject handles POST /model-tree/:elementId/add-object
func (h *Handler) AddObject(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Param("elementId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element id")
		return
	}

	var req AddObjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.svc.AddObject(elementId, req.ObjectName, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// DeleteObject handles POST /model-tree/:elementId/delete-object
func (h *Handler) DeleteObject(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Param("elementId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element id")
		return
	}

	var req DeleteObjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.svc.DeleteObject(elementId, req.ObjectName, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// BatchDeleteObject handles POST /model-tree/:elementId/batch-delete-object
func (h *Handler) BatchDeleteObject(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Param("elementId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element id")
		return
	}

	var req BatchDeleteObjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.svc.BatchDeleteObject(elementId, req.ObjectNames, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// DeleteObjectAfterNeedReboot handles POST /model-tree/:elementId/delete-object-after-need-reboot
func (h *Handler) DeleteObjectAfterNeedReboot(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Param("elementId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element id")
		return
	}

	var req BatchDeleteObjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.svc.DeleteObjectAfterNeedReboot(elementId, req.ObjectNames, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}
