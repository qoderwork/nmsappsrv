package parameter

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for parameter endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// ---------------------------------------------------------------------------
// Parameter endpoints
// ---------------------------------------------------------------------------

// GetParameters handles GET /:elementId
func (h *Handler) GetParameters(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Param("elementId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element id")
		return
	}

	data, err := h.svc.GetParameters(elementId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to get parameters")
		return
	}
	utils.Success(c, data)
}

// SetParameter handles PUT /:elementId
func (h *Handler) SetParameter(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Param("elementId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element id")
		return
	}

	var body struct {
		ParamName string `json:"param_name"`
		Value     string `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.svc.SetParameter(elementId, body.ParamName, body.Value, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to set parameter")
		return
	}
	utils.Success(c, nil)
}

// BatchSetParameter handles POST /parameters/batch-set
// Sets multiple parameters atomically on a single device (aligns with Java batch SPV).
func (h *Handler) BatchSetParameter(c *gin.Context) {
	var req BatchSetParameterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Values) == 0 {
		utils.Error(c, http.StatusBadRequest, "values must not be empty")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.svc.BatchSetParameter(req.ElementId, req.Values, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ListParameterLogs handles GET /:elementId/logs?page=&pageSize=
func (h *Handler) ListParameterLogs(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Param("elementId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element id")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	data, total, err := h.svc.ListParameterLogs(elementId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list parameter logs")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// ---------------------------------------------------------------------------
// ParameterSet endpoints
// ---------------------------------------------------------------------------

// ListParameterSets handles GET /
func (h *Handler) ListParameterSets(c *gin.Context) {
	licenseId := middleware.GetLicenseId(c)

	data, err := h.svc.ListParameterSets(licenseId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list parameter sets")
		return
	}
	utils.Success(c, data)
}

// CreateParameterSet handles POST /
func (h *Handler) CreateParameterSet(c *gin.Context) {
	var ps ParameterSet
	if err := c.ShouldBindJSON(&ps); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateParameterSet(&ps); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create parameter set")
		return
	}
	utils.Success(c, ps)
}

// UpdateParameterSet handles PUT /:id
func (h *Handler) UpdateParameterSet(c *gin.Context) {
	id := c.Param("id")

	var ps ParameterSet
	if err := c.ShouldBindJSON(&ps); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	ps.Id = id

	if err := h.svc.UpdateParameterSet(&ps); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to update parameter set")
		return
	}
	utils.Success(c, ps)
}

// DeleteParameterSet handles DELETE /:id
func (h *Handler) DeleteParameterSet(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.DeleteParameterSet(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete parameter set")
		return
	}
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// ParameterTemplate endpoints
// ---------------------------------------------------------------------------

// ListParameterTemplates handles GET /
func (h *Handler) ListParameterTemplates(c *gin.Context) {
	tenancyIdStr := c.DefaultQuery("tenancy_id", "0")
	tenancyId, _ := strconv.Atoi(tenancyIdStr)

	data, err := h.svc.ListParameterTemplates(tenancyId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list parameter templates")
		return
	}
	utils.Success(c, data)
}

// CreateParameterTemplate handles POST /
func (h *Handler) CreateParameterTemplate(c *gin.Context) {
	var req ParameterTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateParameterTemplate(&req); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create parameter template")
		return
	}
	utils.Success(c, nil)
}

// UpdateParameterTemplate handles PUT /:id
func (h *Handler) UpdateParameterTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid template id")
		return
	}

	var req ParameterTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ID = id

	if err := h.svc.UpdateParameterTemplate(&req); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to update parameter template")
		return
	}
	utils.Success(c, nil)
}

// GetParameterTemplate handles GET /parameter/templates/:id
// Returns a single parameter template's metadata plus its full parameter list
// (with paths from the `parameter` table). Mirrors Java
// `getParameterDeployTemplateInfo`.
func (h *Handler) GetParameterTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid template id")
		return
	}
	vo, err := h.svc.GetParameterTemplate(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Error(c, http.StatusNotFound, "parameter template not found")
			return
		}
		utils.Error(c, http.StatusInternalServerError, "failed to get parameter template")
		return
	}
	utils.Success(c, vo)
}

// DeleteParameterTemplate handles DELETE /parameter/templates/:id
// Cascades to parameter_template_has_parameter. Mirrors Java
// `deleteParameterDeployTemplate`.
func (h *Handler) DeleteParameterTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid template id")
		return
	}
	if err := h.svc.DeleteParameterTemplate(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete parameter template")
		return
	}
	utils.Success(c, nil)
}

// DeployTemplate handles POST /parameter-templates/:id/deploy
func (h *Handler) DeployTemplate(c *gin.Context) {
	templateId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid template id")
		return
	}

	var body struct {
		ElementIds []int64 `json:"elementIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body: elementIds required")
		return
	}

	username := middleware.GetUsername(c)
	results, err := h.svc.DeployTemplate(templateId, body.ElementIds, username)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, results)
}

// ListDeployTemplateLogs handles GET /parameter-templates/:id/deploy-logs
func (h *Handler) ListDeployTemplateLogs(c *gin.Context) {
	templateId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid template id")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	data, total, err := h.svc.ListDeployTemplateLogs(templateId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list deploy template logs")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// TriggerBackup handles POST /parameter/:elementId/backup
func (h *Handler) TriggerBackup(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Param("elementId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element id")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.svc.TriggerBackup(elementId, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// ParameterBackupLog endpoints
// ---------------------------------------------------------------------------

// ListBackupLogs handles GET /:elementId/backups
func (h *Handler) ListBackupLogs(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Param("elementId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid element id")
		return
	}

	data, err := h.svc.ListBackupLogs(elementId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list backup logs")
		return
	}
	utils.Success(c, data)
}

// ---------------------------------------------------------------------------
// Batch Parameter Configuration endpoints
// ---------------------------------------------------------------------------

// BatchParameterConfigurationDirect handles POST /parameter-tasks
func (h *Handler) BatchParameterConfigurationDirect(c *gin.Context) {
	var req BatchParameterConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	username := middleware.GetUsername(c)
	tenancyId := getTenancyId(c)
	if err := h.svc.BatchParameterConfigurationDirect(&req, username, tenancyId); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ListBatchConfigurations handles GET /batch-configurations
func (h *Handler) ListBatchConfigurations(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	tenancyId := getTenancyId(c)
	data, total, err := h.svc.ListBatchConfigurations(tenancyId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list batch configurations")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// ListBatchConfigurationDetail handles GET /batch-configurations/:taskId/detail
func (h *Handler) ListBatchConfigurationDetail(c *gin.Context) {
	taskId, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid taskId")
		return
	}
	data, err := h.svc.ListBatchConfigurationDetail(taskId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list batch config detail")
		return
	}
	utils.Success(c, data)
}

// ---------------------------------------------------------------------------
// Export Parameter Template
// ---------------------------------------------------------------------------

// ExportParameterTemplate handles GET /parameter-templates/:id/export
// It downloads the parameter template as a CSV file.
func (h *Handler) ExportParameterTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid template id")
		return
	}

	data, filename, err := h.svc.ExportParameterTemplate(id)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to export parameter template")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", data)
}

// getTenancyId extracts tenancy_id from gin context as int.
func getTenancyId(c *gin.Context) int {
	v, ok := c.Get("tenancy_id")
	if !ok {
		return 0
	}
	s, ok := v.(string)
	if !ok {
		return 0
	}
	id, _ := strconv.Atoi(s)
	return id
}
