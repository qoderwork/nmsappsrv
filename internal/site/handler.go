package site

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"
)

// Handler exposes site HTTP endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a new site handler.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// ---------- SiteInfo ----------

// ListSites handles GET /sites?page=1&pageSize=20&search=xxx
func (h *Handler) ListSites(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	search := c.Query("search")
	tenantId := middleware.GetTenantId(c)

	data, total, err := h.svc.ListSites(tenantId, search, page, pageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// CreateSite handles POST /sites
func (h *Handler) CreateSite(c *gin.Context) {
	var site SiteInfo
	if err := c.ShouldBindJSON(&site); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	tenantId := middleware.GetTenantId(c)
	if err := h.svc.CreateSite(&site, tenantId); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, site)
}

// UpdateSite handles PUT /sites/:id
func (h *Handler) UpdateSite(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.Error(c, 400, "invalid site id")
		return
	}

	var site SiteInfo
	if err := c.ShouldBindJSON(&site); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	tenantId := middleware.GetTenantId(c)
	if err := h.svc.UpdateSite(id, &site, tenantId); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, site)
}

// DeleteSite handles DELETE /sites/:id
func (h *Handler) DeleteSite(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.Error(c, 400, "invalid site id")
		return
	}

	if err := h.svc.DeleteSite(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ListSiteBasicInfo handles GET /sites/basic
func (h *Handler) ListSiteBasicInfo(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	data, err := h.svc.ListSiteBasicInfo(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// ---------- SysArea ----------

func (h *Handler) ListAreas(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	items, err := h.svc.ListAreas(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

func (h *Handler) GetArea(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	item, err := h.svc.GetArea(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

func (h *Handler) CreateArea(c *gin.Context) {
	var item SysArea
	if err := c.ShouldBindJSON(&item); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	if err := h.svc.CreateArea(&item); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

func (h *Handler) UpdateArea(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	var item SysArea
	if err := c.ShouldBindJSON(&item); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	item.Id = id
	if err := h.svc.UpdateArea(&item); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}

func (h *Handler) DeleteArea(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	if err := h.svc.DeleteArea(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ---------- SystemConfig ----------

func (h *Handler) GetSystemConfig(c *gin.Context) {
	cfg, err := h.svc.GetSystemConfig()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, cfg)
}

func (h *Handler) UpdateSystemConfig(c *gin.Context) {
	var body struct {
		Config string `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	if err := h.svc.UpdateSystemConfig(body.Config); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ---------- SystemParameter ----------

func (h *Handler) ListSystemParameters(c *gin.Context) {
	items, err := h.svc.ListSystemParameters()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, items)
}

func (h *Handler) UpdateSystemParameter(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, 400, "invalid id")
		return
	}
	var item SystemParameter
	if err := c.ShouldBindJSON(&item); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}
	item.Id = id
	if err := h.svc.UpdateSystemParameter(&item); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, item)
}
