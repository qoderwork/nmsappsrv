package parammonitor

import (
	"nmsappsrv/pkg/apperror"
	"strconv"

	"nmsappsrv/internal/alarm"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	svc     Service
	repo    Repository
	checker *ThresholdChecker
}

func NewHandler(db *gorm.DB) *Handler {
	alarmSvc := alarm.NewService(db)
	return &Handler{
		svc:     NewService(db),
		repo:    NewRepository(db),
		checker: NewThresholdChecker(db, redis.RDB, alarmSvc),
	}
}

// StartThresholdChecker launches the background threshold evaluation loop.
func (h *Handler) StartThresholdChecker() {
	h.checker.Start()
}

// StopThresholdChecker stops the background threshold evaluation loop.
func (h *Handler) StopThresholdChecker() {
	h.checker.Stop()
}

func (h *Handler) AddMonitorConfig(c *gin.Context) {
	var req AddMonitorConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.AddMonitorConfig(c, &req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

func (h *Handler) DeleteMonitorConfig(c *gin.Context) {
	var req DeleteMonitorConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.DeleteMonitorConfig(&req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

func (h *Handler) ViewMonitorConfig(c *gin.Context) {
	var req ViewMonitorConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	vo, err := h.svc.ViewMonitorConfig(&req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, vo)
}

func (h *Handler) UpdateMonitorConfig(c *gin.Context) {
	var req UpdateMonitorConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.UpdateMonitorConfig(&req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

func (h *Handler) ListMonitorConfigs(c *gin.Context) {
	var req ListMonitorConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	list, total, err := h.svc.ListMonitorConfigs(c, &req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	page := req.Page
	pageSize := req.PageSize
	utils.Paginated(c, list, total, page, pageSize)
}

func (h *Handler) ToggleMonitorConfig(c *gin.Context) {
	var req ToggleMonitorConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.ToggleMonitorConfig(&req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

func (h *Handler) GetRealtimeMonitorData(c *gin.Context) {
	var req RealtimeMonitorDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	data, err := h.svc.GetRealtimeMonitorData(&req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, data)
}

func (h *Handler) ReloadMonitorParameters(c *gin.Context) {
	var req ReloadMonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.ReloadMonitorParameters(&req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

func (h *Handler) BatchQueryDeviceParameters(c *gin.Context) {
	var req BatchQueryDeviceParamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	result, err := h.svc.BatchQueryDeviceParameters(&req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, result)
}

func (h *Handler) BatchQueryDeviceParametersLive(c *gin.Context) {
	var req BatchQueryLiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	username := c.GetString("username")
	result, err := h.svc.BatchQueryDeviceParametersLive(&req, username)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, result)
}

// ---------------------------------------------------------------------------
// Threshold Rule CRUD handlers
// ---------------------------------------------------------------------------

// CreateThresholdRule handles POST /api/v1/param-monitor/threshold
func (h *Handler) CreateThresholdRule(c *gin.Context) {
	var rule ThresholdRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	repo := h.repo
	if err := repo.CreateThresholdRule(&rule); err != nil {
		utils.HandleError(c, apperror.Wrap(err, "CREATE_THRESHOLD_FAILED", 500, "failed to create threshold rule"))
		return
	}

	utils.Success(c, rule)
}

// UpdateThresholdRule handles PUT /api/v1/param-monitor/threshold/:id
func (h *Handler) UpdateThresholdRule(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		utils.Error(c, 400, "Invalid rule ID")
		return
	}

	repo := h.repo
	existing, err := repo.GetThresholdRule(uint(id))
	if err != nil {
		utils.HandleError(c, apperror.ErrNotFound.WithMessage("threshold rule not found"))
		return
	}

	var updates ThresholdRule
	if err := c.ShouldBindJSON(&updates); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	// Apply non-zero updates.
	if updates.Name != "" {
		existing.Name = updates.Name
	}
	if updates.ParameterName != "" {
		existing.ParameterName = updates.ParameterName
	}
	if updates.Operator != "" {
		existing.Operator = updates.Operator
	}
	if updates.ThresholdValue != 0 {
		existing.ThresholdValue = updates.ThresholdValue
	}
	if updates.Severity != "" {
		existing.Severity = updates.Severity
	}
	if updates.Description != "" {
		existing.Description = updates.Description
	}
	existing.Enabled = updates.Enabled
	existing.DeviceGroupID = updates.DeviceGroupID

	if err := repo.UpdateThresholdRule(existing); err != nil {
		utils.HandleError(c, apperror.Wrap(err, "UPDATE_THRESHOLD_FAILED", 500, "failed to update threshold rule"))
		return
	}

	utils.Success(c, existing)
}

// DeleteThresholdRule handles DELETE /api/v1/param-monitor/threshold/:id
func (h *Handler) DeleteThresholdRule(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		utils.Error(c, 400, "Invalid rule ID")
		return
	}

	repo := h.repo
	if err := repo.DeleteThresholdRule(uint(id)); err != nil {
		utils.HandleError(c, apperror.Wrap(err, "DELETE_THRESHOLD_FAILED", 500, "failed to delete threshold rule"))
		return
	}

	utils.Success(c, nil)
}

// ListThresholdRules handles GET /api/v1/param-monitor/threshold
func (h *Handler) ListThresholdRules(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	var enabledFilter *bool
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		val := enabledStr == "true"
		enabledFilter = &val
	}

	repo := h.repo
	rules, total, err := repo.ListThresholdRules(enabledFilter, page, pageSize)
	if err != nil {
		utils.HandleError(c, apperror.Wrap(err, "LIST_THRESHOLD_FAILED", 500, "failed to list threshold rules"))
		return
	}

	utils.Paginated(c, rules, total, page, pageSize)
}

// GetThresholdRule handles GET /api/v1/param-monitor/threshold/:id
func (h *Handler) GetThresholdRule(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		utils.Error(c, 400, "Invalid rule ID")
		return
	}

	repo := h.repo
	rule, err := repo.GetThresholdRule(uint(id))
	if err != nil {
		utils.HandleError(c, apperror.ErrNotFound.WithMessage("threshold rule not found"))
		return
	}

	utils.Success(c, rule)
}

// TestThresholdRule handles POST /api/v1/param-monitor/threshold/test
// Dry-run: evaluates a rule against current values without creating alarms.
func (h *Handler) TestThresholdRule(c *gin.Context) {
	var req struct {
		RuleID uint `json:"ruleId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	repo := h.repo
	rule, err := repo.GetThresholdRule(req.RuleID)
	if err != nil {
		utils.HandleError(c, apperror.ErrNotFound.WithMessage("threshold rule not found"))
		return
	}

	violations, err := h.checker.evaluateRule(rule)
	if err != nil {
		utils.HandleError(c, apperror.Wrap(err, "EVALUATE_THRESHOLD_FAILED", 500, "failed to evaluate rule"))
		return
	}

	utils.Success(c, violations)
}
