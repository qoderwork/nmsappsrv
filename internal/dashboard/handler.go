package dashboard

import (
	"context"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler provides HTTP handlers for Dashboard endpoints
type Handler struct {
	svc Service
}

// NewHandler creates a new Handler
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(NewRepository(db))}
}

// ListCpeOnlineStatistics handles POST /api/v1/listCpeOnlineStatistics
func (h *Handler) ListCpeOnlineStatistics(c *gin.Context) {
	var req ListCpeOnlineStatisticsQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	ctx := context.Background()
	data, err := h.svc.ListCpeOnlineStatistics(ctx, &req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// ListGNBOnlineStatistics handles POST /api/v1/listGNBOnlineStatistics
func (h *Handler) ListGNBOnlineStatistics(c *gin.Context) {
	var req ListCpeOnlineStatisticsQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	ctx := context.Background()
	data, err := h.svc.ListGNBOnlineStatistics(ctx, &req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// ListProductTypeAndDeviceCount handles POST /api/v1/listProductTypeAndDeviceCount
func (h *Handler) ListProductTypeAndDeviceCount(c *gin.Context) {
	var req ListProductTypeAndDeviceCountQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	tenantId := middleware.GetTenantId(c)
	var tenantIdPtr *int
	if tenantId > 0 {
		tenantIdPtr = &tenantId
	}

	ctx := context.Background()
	data, err := h.svc.ListProductTypeAndDeviceCount(ctx, req.Mode, tenantIdPtr)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, data)
}

// ListBaseStationStatistics handles POST /api/v1/listBaseStationStatistics
func (h *Handler) ListBaseStationStatistics(c *gin.Context) {
	var req ListCpeOnlineStatisticsQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	if req.StartTime == nil || req.EndTime == nil {
		utils.Error(c, 400, "startTime and endTime are required")
		return
	}

	tenantId := middleware.GetTenantId(c)
	var tenantIdPtr *int
	if tenantId > 0 {
		tenantIdPtr = &tenantId
	}

	ctx := context.Background()
	data, err := h.svc.ListBaseStationStatistics(ctx, tenantIdPtr, req.ElementIds, *req.StartTime, *req.EndTime)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, data)
}

// ListPDCPTrafficStatistic handles POST /api/v1/listPDCPTrafficStatistic
func (h *Handler) ListPDCPTrafficStatistic(c *gin.Context) {
	var req ListCpeOnlineStatisticsQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	if req.StartTime == nil || req.EndTime == nil {
		utils.Error(c, 400, "startTime and endTime are required")
		return
	}

	tenantId := middleware.GetTenantId(c)
	var tenantIdPtr *int
	if tenantId > 0 {
		tenantIdPtr = &tenantId
	}

	startTimeStr := req.StartTime.Format("2006-01-02 15:04:05")
	endTimeStr := req.EndTime.Format("2006-01-02 15:04:05")

	ctx := context.Background()
	data, err := h.svc.ListPDCPTrafficStatistic(ctx, startTimeStr, endTimeStr, tenantIdPtr)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, data)
}

// ListDeviceOnlineInfo handles POST /api/v1/listDeviceOnlineInfo
func (h *Handler) ListDeviceOnlineInfo(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	var tenantIdPtr *int
	if tenantId > 0 {
		tenantIdPtr = &tenantId
	}

	ctx := context.Background()
	data, err := h.svc.ListDeviceOnlineInfo(ctx, tenantIdPtr)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, data)
}

// StatisticKPIForDevicelop handles POST /api/v1/statisticKPIForDevicelop
func (h *Handler) StatisticKPIForDevicelop(c *gin.Context) {
	var req StatisticKPIForDevicelopQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	if len(req.DeviceGroupId) == 0 {
		utils.Error(c, 400, "deviceGroupId is required")
		return
	}

	tenantId := middleware.GetTenantId(c)
	var tenantIdPtr *int
	if tenantId > 0 {
		tenantIdPtr = &tenantId
	}

	ctx := context.Background()
	data, err := h.svc.StatisticKPIForDevicelop(ctx, tenantIdPtr, req.DeviceGroupId, req.Granularity, req.Gmt, req.Timestamp)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, data)
}
