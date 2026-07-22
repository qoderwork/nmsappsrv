package blacklist

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for blacklist endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// ListDeviceBlackList handles POST /blacklist/list
func (h *Handler) ListDeviceBlackList(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	var req struct {
		SN       string `json:"sn"`
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// fallback to query params
		req.SN = c.Query("sn")
		req.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
		req.PageSize, _ = strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	query := ListBlackListQuery{
		SN:       req.SN,
		Page:     req.Page,
		PageSize: req.PageSize,
	}
	data, total, err := h.svc.ListBlackList(tenantId, query)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Paginated(c, data, total, query.Page, query.PageSize)
}

// AddDeviceToBlackList handles POST /blacklist/add
func (h *Handler) AddDeviceToBlackList(c *gin.Context) {
	var req AddDeviceToBlackListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	tenantId := middleware.GetTenantId(c)
	username := middleware.GetUsername(c)

	id, err := h.svc.AddDeviceToBlackList(&req, tenantId, username)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, map[string]int{"id": id})
}

// DeleteDeviceFromBlackList handles POST /blacklist/delete
func (h *Handler) DeleteDeviceFromBlackList(c *gin.Context) {
	var req struct {
		Id int `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.svc.DeleteDeviceFromBlackList(req.Id, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// BatchDeleteDeviceFromBlackList handles POST /blacklist/batch-delete
func (h *Handler) BatchDeleteDeviceFromBlackList(c *gin.Context) {
	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.svc.BatchDeleteDeviceFromBlackList(req.Ids, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// ListBlackListOperationLog handles POST /blacklist/operation-logs
func (h *Handler) ListBlackListOperationLog(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	var req ListBlackListOperationLogQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		// fallback to query params
		req.SearchText = c.Query("searchText")
		req.DeviceSN = c.Query("deviceSn")
		req.OperationType = c.Query("operationType")
		req.DeviceType = c.Query("deviceType")
		req.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
		req.PageSize, _ = strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	data, total, err := h.svc.ListOperationLogs(tenantId, req)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Paginated(c, data, total, req.Page, req.PageSize)
}
