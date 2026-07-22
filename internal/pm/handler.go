package pm

import (
	"fmt"
	"io"
	"net/http"
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
	tenantId := middleware.GetTenantId(c)
	items, err := h.svc.ListKPIs(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

func (h *Handler) ListAllKPIs(c *gin.Context) {
	items, err := h.svc.ListAllKPIs()
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
	tenantId := middleware.GetTenantId(c)
	items, err := h.svc.ListKPISets(tenantId)
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

func (h *Handler) DeleteKPISet(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	if err := h.svc.DeleteKPISet(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ---------- PerformanceKpiTemplate ----------

func (h *Handler) ListKPITemplates(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	items, err := h.svc.ListKPITemplates(tenantId)
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

func (h *Handler) GetKPITemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	item, err := h.svc.GetKPITemplate(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

func (h *Handler) DownloadKPITemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	data, filename, err := h.svc.DownloadKPITemplate(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
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
	tenantId := middleware.GetTenantId(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	items, total, err := h.svc.ListPMFileLogs(tenantId, page, pageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Paginated(c, items, total, page, pageSize)
}

// ---------- KpiAlarmTemplate ----------

func (h *Handler) ListKPIAlarms(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	items, err := h.svc.ListKPIAlarmTemplates(tenantId)
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

func (h *Handler) GetKPIAlarmTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	item, err := h.svc.GetKPIAlarmTemplate(id)
	if err != nil {
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

func (h *Handler) UpdateKPIAlarmTemplateStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	var body struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	if err := h.svc.UpdateKPIAlarmTemplateStatus(id, body.Enable); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
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
	tenantId := middleware.GetTenantId(c)
	startTime := c.Query("start_time")
	endTime := c.Query("end_time")
	items, err := h.svc.GetDashboardData(tenantId, startTime, endTime)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

// ---------- PDCPTraffic ----------

func (h *Handler) GetPDCPTraffic(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	startTime := c.Query("start_time")
	endTime := c.Query("end_time")
	items, err := h.svc.GetPDCPTraffic(tenantId, startTime, endTime)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

// ---------- Dashboard: Device Online Info ----------

func (h *Handler) GetDeviceOnlineInfo(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	info, err := h.svc.GetDeviceOnlineInfo(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, info)
}

// ---------- Dashboard: Product Type & Device Count ----------

func (h *Handler) GetProductTypeAndDeviceCount(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	mode := c.DefaultQuery("mode", "")
	items, err := h.svc.GetProductTypeAndDeviceCount(tenantId, mode)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

// ExportPMExcel handles POST /pm/export-excel
// Body: {"startTime": RFC3339, "endTime": RFC3339, "elementId"?: int,
//        "templateId"?: int}. Mirrors Java exportPMExcel.
func (h *Handler) ExportPMExcel(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	var body struct {
		StartTime  string `json:"startTime"`
		EndTime    string `json:"endTime"`
		ElementId  int64  `json:"elementId"`
		TemplateId int    `json:"templateId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}
	data, filename, err := h.svc.ExportPMExcel(tenantId, body.StartTime, body.EndTime)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

// ImportKPIs handles POST /pm/kpis/import
// multipart/form-data: file=<xlsx>, version=<string>. Mirrors Java
// importKPI (Java uses Apache POI; Go uses excelize).
func (h *Handler) ImportKPIs(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, 400, "missing 'file' multipart field (xlsx)")
		return
	}
	f, err := fh.Open()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	version := c.PostForm("version")
	count, err := h.svc.ImportKPIsFromXLSX(data, version)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, map[string]int{"imported": count})
}

// DownloadPMFile handles GET /pm/file-logs/download?elementId=&startTime=&endTime=
// Mirrors Java downloadPMFile.
func (h *Handler) DownloadPMFile(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Query("elementId"), 10, 64)
	if err != nil || elementId <= 0 {
		utils.Error(c, 400, "invalid or missing elementId")
		return
	}
	startTime := c.Query("startTime")
	endTime := c.Query("endTime")
	data, filename, err := h.svc.DownloadPMFile(elementId, startTime, endTime)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "application/octet-stream", data)
}

// ListKPIMeas handles POST /pm/kpi-meas
// Body: {page, pageSize, searchText}. Mirrors Java listKPIMeas.
func (h *Handler) ListKPIMeas(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	var body struct {
		Page       int    `json:"page"`
		PageSize   int    `json:"pageSize"`
		SearchText string `json:"searchText"`
	}
	_ = c.ShouldBindJSON(&body) // optional body; defaults handled in service
	items, total, err := h.svc.ListKPIMeas(tenantId, body.SearchText, body.Page, body.PageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Paginated(c, items, total, body.Page, body.PageSize)
}

// UpdateMeasTaskSwitch handles POST /pm/kpi-meas/switch
// Body: {elementId, enable, username?}. Mirrors Java updateMeasTaskSwitch.
func (h *Handler) UpdateMeasTaskSwitch(c *gin.Context) {
	var body struct {
		ElementId int64  `json:"elementId"`
		Enable    bool   `json:"enable"`
		Username  string `json:"username"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}
	if body.ElementId <= 0 {
		utils.Error(c, 400, "elementId is required")
		return
	}
	username := body.Username
	if username == "" {
		username = "system"
	}
	if err := h.svc.UpdateMeasTaskSwitch(body.ElementId, body.Enable, username); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// AddReplenishTask handles POST /pm/replenish-tasks
// Body: PMReplenishTask JSON. Mirrors Java addReplenishTask.
func (h *Handler) AddReplenishTask(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	var t PMReplenishTask
	if err := c.ShouldBindJSON(&t); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}
	t.TenantId = &tenantId
	if err := h.svc.AddReplenishTask(&t); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, &t)
}

// ListReplenishTask handles POST /pm/replenish-tasks/list
// Body: {page, pageSize, name?}. Mirrors Java listReplenishTask.
func (h *Handler) ListReplenishTask(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	var body struct {
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
		Name     string `json:"name"`
	}
	_ = c.ShouldBindJSON(&body)
	items, total, err := h.svc.ListReplenishTask(tenantId, body.Name, body.Page, body.PageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Paginated(c, items, total, body.Page, body.PageSize)
}

// ViewReplenishTask handles POST /pm/replenish-tasks/view
// Body: {id}. Mirrors Java viewReplenishTask.
func (h *Handler) ViewReplenishTask(c *gin.Context) {
	var body struct {
		Id int `json:"id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Id <= 0 {
		utils.Error(c, 400, "id is required")
		return
	}
	t, err := h.svc.ViewReplenishTask(body.Id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, t)
}

// ListDeviceReplenish handles POST /pm/replenish-tasks/devices
// Body: {id}. Mirrors Java listDeviceReplenish.
func (h *Handler) ListDeviceReplenish(c *gin.Context) {
	var body struct {
		Id int `json:"id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Id <= 0 {
		utils.Error(c, 400, "id is required")
		return
	}
	rows, err := h.svc.ListDeviceReplenish(body.Id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, rows)
}
