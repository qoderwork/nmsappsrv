package corenet

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

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
