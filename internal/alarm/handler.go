package alarm

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for alarm-related endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// ---------------------------------------------------------------------------
// Alarm endpoints
// ---------------------------------------------------------------------------

// ListAlarms handles GET /alarms?page=1&pageSize=20&severity=&alarm_type=
func (h *Handler) ListAlarms(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	severity := c.Query("severity")
	alarmType, _ := strconv.Atoi(c.DefaultQuery("alarm_type", "0"))
	licenseId := middleware.GetLicenseId(c)

	data, total, err := h.svc.ListAlarms(licenseId, severity, alarmType, page, pageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// GetAlarm handles GET /alarms/:id
func (h *Handler) GetAlarm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm id")
		return
	}

	alarm, err := h.svc.GetAlarm(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, alarm)
}

// ClearAlarm handles PUT /alarms/:id/clear
func (h *Handler) ClearAlarm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm id")
		return
	}

	if err := h.svc.ClearAlarm(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// BatchClearRequest is the JSON body for PUT /alarms/batch-clear.
type BatchClearRequest struct {
	AlarmIds  []int64 `json:"alarmIds" binding:"required"`
	ClearUser string  `json:"clearUser"`
}

// BatchClearAlarms handles PUT /alarms/batch-clear
func (h *Handler) BatchClearAlarms(c *gin.Context) {
	var req BatchClearRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body: alarmIds is required")
		return
	}

	cleared, notFound, err := h.svc.BatchClearAlarms(req.AlarmIds, req.ClearUser)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.OK(c, map[string]interface{}{
		"clearedCount": cleared,
		"notFoundIds":  notFound,
	})
}

// ---------------------------------------------------------------------------
// AlarmLibrary endpoints
// ---------------------------------------------------------------------------

// ListAlarmLibrary handles GET /alarm-library
func (h *Handler) ListAlarmLibrary(c *gin.Context) {
	tenancyId := middleware.GetLicenseId(c)

	data, err := h.svc.ListAlarmLibrary(tenancyId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// ---------------------------------------------------------------------------
// AlarmTemplate endpoints
// ---------------------------------------------------------------------------

// ListAlarmTemplates handles GET /alarm-templates
func (h *Handler) ListAlarmTemplates(c *gin.Context) {
	tenancyId := middleware.GetLicenseId(c)

	data, err := h.svc.ListAlarmTemplates(tenancyId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// CreateAlarmTemplate handles POST /alarm-templates
func (h *Handler) CreateAlarmTemplate(c *gin.Context) {
	var t AlarmTemplate
	if err := c.ShouldBindJSON(&t); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateAlarmTemplate(&t); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, &t)
}

// UpdateAlarmTemplate handles PUT /alarm-templates/:id
func (h *Handler) UpdateAlarmTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm template id")
		return
	}

	var t AlarmTemplate
	if err := c.ShouldBindJSON(&t); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	t.Id = id

	if err := h.svc.UpdateAlarmTemplate(&t); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, &t)
}

// ---------------------------------------------------------------------------
// AlarmFilter endpoints
// ---------------------------------------------------------------------------

// ListAlarmFilters handles GET /alarm-filters
func (h *Handler) ListAlarmFilters(c *gin.Context) {
	licenseId := middleware.GetLicenseId(c)

	data, err := h.svc.ListAlarmFilters(licenseId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// CreateAlarmFilter handles POST /alarm-filters
func (h *Handler) CreateAlarmFilter(c *gin.Context) {
	var f AlarmFilter
	if err := c.ShouldBindJSON(&f); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateAlarmFilter(&f); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, &f)
}

// UpdateAlarmFilter handles PUT /alarm-filters/:id
func (h *Handler) UpdateAlarmFilter(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm filter id")
		return
	}

	var f AlarmFilter
	if err := c.ShouldBindJSON(&f); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	f.Id = id

	if err := h.svc.UpdateAlarmFilter(&f); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, &f)
}

// DeleteAlarmFilter handles DELETE /alarm-filters/:id
func (h *Handler) DeleteAlarmFilter(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid alarm filter id")
		return
	}

	if err := h.svc.DeleteAlarmFilter(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}
