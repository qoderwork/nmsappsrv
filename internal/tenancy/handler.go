package tenancy

import (
	"nmsappsrv/internal/device"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler provides HTTP handlers for tenancy management endpoints
type Handler struct {
	svc Service
}

// NewHandler creates a new Handler
func NewHandler(db *gorm.DB) *Handler {
	deviceSvc := device.NewService(db)
	return &Handler{svc: NewService(NewRepository(db), deviceSvc)}
}

// AddTenancy handles POST /api/v1/addTenancy
func (h *Handler) AddTenancy(c *gin.Context) {
	var req AddTenancyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	id, err := h.svc.AddTenancy(&req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, gin.H{"id": id})
}

// UpdateTenancy handles POST /api/v1/updateTenancy
func (h *Handler) UpdateTenancy(c *gin.Context) {
	var req UpdateTenancyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	result, err := h.svc.UpdateTenancy(&req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, result)
}

// ListTenancy handles POST /api/v1/listTenancy
func (h *Handler) ListTenancy(c *gin.Context) {
	var query ListTenancyQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		// Use defaults if no query provided
		query = ListTenancyQuery{Page: 1, PageSize: 10}
	}

	items, total, err := h.svc.ListTenancies(&query)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	utils.Paginated(c, items, total, page, pageSize)
}

// DeleteTenancy handles POST /api/v1/deleteTenancy
func (h *Handler) DeleteTenancy(c *gin.Context) {
	var req DeleteTenancyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	if err := h.svc.DeleteTenancy(req.Id); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// ViewTenancy handles POST /api/v1/viewTenancy
func (h *Handler) ViewTenancy(c *gin.Context) {
	var req struct {
		Id int `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	result, err := h.svc.ViewTenancy(req.Id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, result)
}
