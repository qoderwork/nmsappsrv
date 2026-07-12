package monitor

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"
)

// Handler exposes monitor HTTP endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a new monitor handler.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// ---------- MonitorTask ----------

func (h *Handler) ListMonitorTasks(c *gin.Context) {
	licenseId := middleware.GetLicenseId(c)
	items, err := h.svc.ListMonitorTasks(licenseId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

func (h *Handler) GetMonitorTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	item, err := h.svc.GetMonitorTask(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

func (h *Handler) CreateMonitorTask(c *gin.Context) {
	var item MonitorTask
	if err := c.ShouldBindJSON(&item); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	if err := h.svc.CreateMonitorTask(&item); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

func (h *Handler) UpdateMonitorTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	var item MonitorTask
	if err := c.ShouldBindJSON(&item); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	item.Id = id
	if err := h.svc.UpdateMonitorTask(&item); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

func (h *Handler) DeleteMonitorTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	if err := h.svc.DeleteMonitorTask(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ---------- MonitorData ----------

func (h *Handler) GetMonitorData(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Query("element_id"), 10, 64)
	if err != nil {
		utils.Error(c, 400, "invalid element_id")
		return
	}
	parameterId := c.Query("parameter_id")
	startTime := c.Query("start_time")
	endTime := c.Query("end_time")
	items, err := h.svc.GetMonitorData(elementId, parameterId, startTime, endTime)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

// ---------- MonitorElements ----------

func (h *Handler) GetMonitorElements(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid task id")
		return
	}
	items, err := h.svc.GetMonitorElements(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

func (h *Handler) SaveMonitorElements(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid task id")
		return
	}
	var body struct {
		ElementIds []int64 `json:"element_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	if err := h.svc.SaveMonitorElements(id, body.ElementIds); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ---------- MonitorParameters ----------

func (h *Handler) GetMonitorParameters(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid task id")
		return
	}
	items, err := h.svc.GetMonitorParameters(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

func (h *Handler) SaveMonitorParameters(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid task id")
		return
	}
	var body struct {
		ParameterIds []string `json:"parameter_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	if err := h.svc.SaveMonitorParameters(id, body.ParameterIds); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}
