package parammonitor

import (
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	svc *Service
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{
		svc: NewService(db),
	}
}

func (h *Handler) AddMonitorConfig(c *gin.Context) {
	var req AddMonitorConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.AddMonitorConfig(c, &req); err != nil {
		utils.Error(c, 500, "Failed to add monitor config: "+err.Error())
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
		utils.Error(c, 500, "Failed to delete monitor config: "+err.Error())
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
		utils.Error(c, 500, "Failed to view monitor config: "+err.Error())
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
		utils.Error(c, 500, "Failed to update monitor config: "+err.Error())
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
		utils.Error(c, 500, "Failed to list monitor configs: "+err.Error())
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
		utils.Error(c, 500, "Failed to toggle monitor config: "+err.Error())
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
		utils.Error(c, 500, "Failed to get realtime monitor data: "+err.Error())
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
		utils.Error(c, 500, "Failed to reload monitor parameters: "+err.Error())
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
		utils.Error(c, 500, "Failed to batch query device parameters: "+err.Error())
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
		utils.Error(c, 500, "Failed to batch query live: "+err.Error())
		return
	}

	utils.Success(c, result)
}
