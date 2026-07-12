package pm

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"
)

// Handler exposes PM HTTP endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a new PM handler.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// ---------- PerformanceKpi ----------

func (h *Handler) ListKPIs(c *gin.Context) {
	tenancyId := middleware.GetLicenseId(c)
	items, err := h.svc.ListKPIs(tenancyId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

func (h *Handler) GetKPI(c *gin.Context) {
	id := c.Param("id")
	item, err := h.svc.GetKPI(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

func (h *Handler) CreateKPI(c *gin.Context) {
	var item PerformanceKpi
	if err := c.ShouldBindJSON(&item); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	if err := h.svc.CreateKPI(&item); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

func (h *Handler) UpdateKPI(c *gin.Context) {
	id := c.Param("id")
	var item PerformanceKpi
	if err := c.ShouldBindJSON(&item); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	item.Id = id
	if err := h.svc.UpdateKPI(&item); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

func (h *Handler) DeleteKPI(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteKPI(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ---------- PerformanceKpiSet ----------

func (h *Handler) ListKPISets(c *gin.Context) {
	tenancyId := middleware.GetLicenseId(c)
	items, err := h.svc.ListKPISets(tenancyId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

func (h *Handler) CreateKPISet(c *gin.Context) {
	var item PerformanceKpiSet
	if err := c.ShouldBindJSON(&item); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	if err := h.svc.CreateKPISet(&item); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

// ---------- PerformanceKpiTemplate ----------

func (h *Handler) ListKPITemplates(c *gin.Context) {
	tenancyId := middleware.GetLicenseId(c)
	items, err := h.svc.ListKPITemplates(tenancyId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

func (h *Handler) CreateKPITemplate(c *gin.Context) {
	var item PerformanceKpiTemplate
	if err := c.ShouldBindJSON(&item); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	if err := h.svc.CreateKPITemplate(&item); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

func (h *Handler) UpdateKPITemplate(c *gin.Context) {
	id := c.Param("id")
	intID, err := strconv.Atoi(id)
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	var item PerformanceKpiTemplate
	if err := c.ShouldBindJSON(&item); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	item.Id = intID
	if err := h.svc.UpdateKPITemplate(&item); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

func (h *Handler) DeleteKPITemplate(c *gin.Context) {
	id := c.Param("id")
	intID, err := strconv.Atoi(id)
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	if err := h.svc.DeleteKPITemplate(intID); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ---------- PMFileLog ----------

func (h *Handler) ListPMFileLogs(c *gin.Context) {
	tenancyId := middleware.GetLicenseId(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	items, total, err := h.svc.ListPMFileLogs(tenancyId, page, pageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Paginated(c, items, total, page, pageSize)
}

// ---------- KpiAlarmTemplate ----------

func (h *Handler) ListKPIAlarms(c *gin.Context) {
	tenancyId := middleware.GetLicenseId(c)
	items, err := h.svc.ListKPIAlarmTemplates(tenancyId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

func (h *Handler) CreateKPIAlarm(c *gin.Context) {
	var item KpiAlarmTemplate
	if err := c.ShouldBindJSON(&item); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	if err := h.svc.CreateKPIAlarmTemplate(&item); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

func (h *Handler) UpdateKPIAlarm(c *gin.Context) {
	id := c.Param("id")
	intID, err := strconv.Atoi(id)
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	var item KpiAlarmTemplate
	if err := c.ShouldBindJSON(&item); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	item.Id = intID
	if err := h.svc.UpdateKPIAlarmTemplate(&item); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

func (h *Handler) DeleteKPIAlarm(c *gin.Context) {
	id := c.Param("id")
	intID, err := strconv.Atoi(id)
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	if err := h.svc.DeleteKPIAlarmTemplate(intID); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ---------- Dashboard ----------

func (h *Handler) GetDashboardData(c *gin.Context) {
	tenancyId := middleware.GetLicenseId(c)
	startTime := c.Query("start_time")
	endTime := c.Query("end_time")
	items, err := h.svc.GetDashboardData(tenancyId, startTime, endTime)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

// ---------- PDCPTraffic ----------

func (h *Handler) GetPDCPTraffic(c *gin.Context) {
	tenancyId := middleware.GetLicenseId(c)
	startTime := c.Query("start_time")
	endTime := c.Query("end_time")
	items, err := h.svc.GetPDCPTraffic(tenancyId, startTime, endTime)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

// ---------- Dashboard: Device Online Info ----------

func (h *Handler) GetDeviceOnlineInfo(c *gin.Context) {
	tenancyId := middleware.GetLicenseId(c)
	info, err := h.svc.GetDeviceOnlineInfo(tenancyId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, info)
}

// ---------- Dashboard: Product Type & Device Count ----------

func (h *Handler) GetProductTypeAndDeviceCount(c *gin.Context) {
	tenancyId := middleware.GetLicenseId(c)
	mode := c.DefaultQuery("mode", "")
	items, err := h.svc.GetProductTypeAndDeviceCount(tenancyId, mode)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}
