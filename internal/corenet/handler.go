package corenet

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for core network endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
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

// ListCoreNetworks handles GET /core-networks
func (h *Handler) ListCoreNetworks(c *gin.Context) {
	tenancyId := getTenancyId(c)

	networks, err := h.svc.ListCoreNetworks(tenancyId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list core networks")
		return
	}
	utils.Success(c, networks)
}

// GetCoreNetwork handles GET /core-networks/:id
func (h *Handler) GetCoreNetwork(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid core network id")
		return
	}

	cn, err := h.svc.GetCoreNetwork(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "core network not found")
		return
	}
	utils.Success(c, cn)
}

// CreateCoreNetwork handles POST /core-networks
func (h *Handler) CreateCoreNetwork(c *gin.Context) {
	var cn CoreNetwork
	if err := c.ShouldBindJSON(&cn); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateCoreNetwork(&cn); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create core network")
		return
	}
	utils.Success(c, cn)
}

// UpdateCoreNetwork handles PUT /core-networks/:id
func (h *Handler) UpdateCoreNetwork(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid core network id")
		return
	}

	var cn CoreNetwork
	if err := c.ShouldBindJSON(&cn); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	cn.Id = id

	if err := h.svc.UpdateCoreNetwork(&cn); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to update core network")
		return
	}
	utils.Success(c, cn)
}

// DeleteCoreNetwork handles DELETE /core-networks/:id
func (h *Handler) DeleteCoreNetwork(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid core network id")
		return
	}

	if err := h.svc.DeleteCoreNetwork(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete core network")
		return
	}
	utils.Success(c, nil)
}

// GetCoreNetworkData handles GET /core-networks/:id/data
func (h *Handler) GetCoreNetworkData(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid core network id")
		return
	}

	data, err := h.svc.GetCoreNetworkData(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "core network data not found")
		return
	}
	utils.Success(c, data)
}

// SaveCoreNetworkData handles PUT /core-networks/:id/data
func (h *Handler) SaveCoreNetworkData(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid core network id")
		return
	}

	var data CoreNetworkData
	if err := c.ShouldBindJSON(&data); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	data.CoreNetworkId = &id

	if err := h.svc.SaveCoreNetworkData(&data); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to save core network data")
		return
	}
	utils.Success(c, data)
}

// GetCoreNetworkKpis handles GET /core-networks/:id/kpis?start_time=&end_time=
func (h *Handler) GetCoreNetworkKpis(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid core network id")
		return
	}

	startTime := c.Query("start_time")
	endTime := c.Query("end_time")
	if startTime == "" || endTime == "" {
		utils.Error(c, http.StatusBadRequest, "start_time and end_time are required")
		return
	}

	kpis, err := h.svc.GetCoreNetworkKpis(id, startTime, endTime)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to get KPIs")
		return
	}
	utils.Success(c, kpis)
}

// GetStatisticData handles GET /core-networks/:id/statistics?start_time=&end_time=
func (h *Handler) GetStatisticData(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid core network id")
		return
	}

	startTime := c.Query("start_time")
	endTime := c.Query("end_time")
	if startTime == "" || endTime == "" {
		utils.Error(c, http.StatusBadRequest, "start_time and end_time are required")
		return
	}

	data, err := h.svc.GetStatisticData(id, startTime, endTime)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to get statistic data")
		return
	}
	utils.Success(c, data)
}

// ListOperationLogs handles GET /core-networks/:id/logs?page=&pageSize=
func (h *Handler) ListOperationLogs(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid core network id")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	data, total, err := h.svc.ListOperationLogs(id, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list operation logs")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// ---------------------------------------------------------------------------
// Tier 1 corenet KPI batch
// ---------------------------------------------------------------------------

// GetCoreNetworkAlarms handles POST /core-networks/alarms
// Body: {coreNetworkId}. Mirrors Java getCoreNetworkAlarms.
func (h *Handler) GetCoreNetworkAlarms(c *gin.Context) {
	var body struct {
		CoreNetworkId int `json:"coreNetworkId"`
	}
	_ = c.ShouldBindJSON(&body)
	data, err := h.svc.GetCoreNetworkAlarms(body.CoreNetworkId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// ListUEList handles POST /core-networks/ue-list
// Body: {coreNetworkId}. Mirrors Java listUEList.
func (h *Handler) ListUEList(c *gin.Context) {
	var body struct {
		CoreNetworkId int `json:"coreNetworkId"`
	}
	_ = c.ShouldBindJSON(&body)
	data, err := h.svc.ListUEList(body.CoreNetworkId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// ListUENumberStatistic handles POST /core-networks/ue-number-statistic
// Body: {coreNetworkId}. Mirrors Java listUENumberStatistic.
func (h *Handler) ListUENumberStatistic(c *gin.Context) {
	var body struct {
		CoreNetworkId int `json:"coreNetworkId"`
	}
	_ = c.ShouldBindJSON(&body)
	data, err := h.svc.ListUENumberStatistic(body.CoreNetworkId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// GetUeInfos handles POST /core-networks/ue-infos
// Body: {coreNetworkId}. Mirrors Java getUeInfos.
func (h *Handler) GetUeInfos(c *gin.Context) {
	var body struct {
		CoreNetworkId int `json:"coreNetworkId"`
	}
	_ = c.ShouldBindJSON(&body)
	data, err := h.svc.GetUeInfos(body.CoreNetworkId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// ChangeCoreNetworkSwitch handles POST /core-networks/switch
// Body: {coreNetworkId, enable}. Mirrors Java changeCoreNetworkSwitch.
func (h *Handler) ChangeCoreNetworkSwitch(c *gin.Context) {
	var body struct {
		CoreNetworkId int  `json:"coreNetworkId"`
		Enable        bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.CoreNetworkId <= 0 {
		utils.Error(c, 400, "coreNetworkId is required")
		return
	}
	if err := h.svc.ChangeCoreNetworkSwitch(body.CoreNetworkId, body.Enable, middleware.GetUsername(c)); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// IngestCoreNetworkKpi handles POST /core-networks/kpi/ingest
// Body: Task{Period{StartTime,EndTime},Ne{NeType,RmUid,Kpis[{kpiId/kpiId1,value}]}}.
// Mirrors Java kpi()/dealUPFKPI device-reported KPI collector. This is the
// ingest half that makes core_network_kpi non-empty (the rewrite previously
// had no writer, so traffic/user KPIs were always empty).
func (h *Handler) IngestCoreNetworkKpi(c *gin.Context) {
	var dto IngestCoreNetworkKpiDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid ingest body")
		return
	}
	if err := h.svc.IngestCoreNetworkKpi(dto); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// GetCoreNetworkUserInfo handles POST /core-networks/kpi/user-info
// Body: {coreNetworkId}. Mirrors Java getCoreNetworkUserInfo.
func (h *Handler) GetCoreNetworkUserInfo(c *gin.Context) {
	var body struct {
		CoreNetworkId int `json:"coreNetworkId"`
	}
	_ = c.ShouldBindJSON(&body)
	data, err := h.svc.GetCoreNetworkUserInfo(body.CoreNetworkId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// GetCoreNetworkUpfTraffic handles POST /core-networks/kpi/upf-traffic
// Body: {coreNetworkId}. Mirrors Java getCoreNetworkUpfTraffic.
func (h *Handler) GetCoreNetworkUpfTraffic(c *gin.Context) {
	var body struct {
		CoreNetworkId int `json:"coreNetworkId"`
	}
	_ = c.ShouldBindJSON(&body)
	data, err := h.svc.GetCoreNetworkUpfTraffic(body.CoreNetworkId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// GetBuiltInCoreNetworkUpfTraffic handles POST /core-networks/kpi/upf-traffic/built-in
// Body: {coreNetworkId}. Mirrors Java getBuiltInCoreNetworkUpfTraffic.
func (h *Handler) GetBuiltInCoreNetworkUpfTraffic(c *gin.Context) {
	var body struct {
		CoreNetworkId int `json:"coreNetworkId"`
	}
	_ = c.ShouldBindJSON(&body)
	data, err := h.svc.GetBuiltInCoreNetworkUpfTraffic(body.CoreNetworkId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// GetBuiltInCoreNetworkUserInfo handles POST /core-networks/kpi/user-info/built-in
// Body: {coreNetworkId}. Mirrors Java getBuiltInCoreNetworkUserInfo.
func (h *Handler) GetBuiltInCoreNetworkUserInfo(c *gin.Context) {
	var body struct {
		CoreNetworkId int `json:"coreNetworkId"`
	}
	_ = c.ShouldBindJSON(&body)
	data, err := h.svc.GetBuiltInCoreNetworkUserInfo(body.CoreNetworkId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// GetKpiReport handles POST /core-networks/kpi/report
// Body: {coreNetworkId, index, startTime, endTime}. Mirrors Java
// kpiReport/{index}.
func (h *Handler) GetKpiReport(c *gin.Context) {
	var body struct {
		CoreNetworkId int    `json:"coreNetworkId"`
		Index         int    `json:"index"`
		StartTime     string `json:"startTime"`
		EndTime       string `json:"endTime"`
	}
	_ = c.ShouldBindJSON(&body)
	data, err := h.svc.GetKpiReport(body.CoreNetworkId, body.Index, body.StartTime, body.EndTime)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// ---------------------------------------------------------------------------
// Tier 1.5 corenet parameter CRUD (mirrors Java CoreNetworkManagementController
// getCoreNetworkParameters / setCoreNetworkParameters / queryCoreNetworkParameters /
// deleteCoreNetworkParameter / addCoreNetworkParameter).
// ---------------------------------------------------------------------------

// GetCoreNetworkParameters handles POST /core-networks/parameters
// Body: {coreNetworkId, elementType}. Returns the param catalog for the
// element type, enriched with stored values.
func (h *Handler) GetCoreNetworkParameters(c *gin.Context) {
	var q GetCoreNetworkParametersQuery
	if err := c.ShouldBindJSON(&q); err != nil || q.CoreNetworkId <= 0 || q.ElementType == "" {
		utils.Error(c, http.StatusBadRequest, "coreNetworkId and elementType are required")
		return
	}
	data, err := h.svc.GetCoreNetworkParameters(q)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// SetCoreNetworkParameters handles POST /core-networks/parameters/set
// Body: {coreNetworkId, name, index?, data, elementType}. Pushes config to
// the element's 33030 API and logs the operation.
func (h *Handler) SetCoreNetworkParameters(c *gin.Context) {
	var dto SetCoreNetworkParametersDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	id, err := h.svc.SetCoreNetworkParameters(dto, middleware.GetUsername(c))
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, map[string]int64{"id": id})
}

// QueryCoreNetworkParameters handles POST /core-networks/parameters/query
// Body: {coreNetworkId, name, elementType}. Reads config from the element's
// 33030 API and logs the operation.
func (h *Handler) QueryCoreNetworkParameters(c *gin.Context) {
	var dto QueryCoreNetworkParametersDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	id, err := h.svc.QueryCoreNetworkParameters(dto, middleware.GetUsername(c))
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, map[string]int64{"id": id})
}

// DeleteCoreNetworkParameter handles POST /core-networks/parameters/delete
// Body: {coreNetworkId, name, index, elementType}. Deletes a config array
// element on the element's 33030 API and logs the operation.
func (h *Handler) DeleteCoreNetworkParameter(c *gin.Context) {
	var dto DeleteCoreNetworkParameterDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	id, err := h.svc.DeleteCoreNetworkParameter(dto, middleware.GetUsername(c))
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, map[string]int64{"id": id})
}

// AddCoreNetworkParameter handles POST /core-networks/parameters/add
// Body: {coreNetworkId, name, index, data, elementType}. Adds a config array
// element on the element's 33030 API and logs the operation.
func (h *Handler) AddCoreNetworkParameter(c *gin.Context) {
	var dto SetCoreNetworkParametersDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	id, err := h.svc.AddCoreNetworkParameter(dto, middleware.GetUsername(c))
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, map[string]int64{"id": id})
}
