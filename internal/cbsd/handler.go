package cbsd

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for CBSD endpoints.
type Handler struct {
	svc *Service
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

// ListCBSD handles GET /cbsd?page=&pageSize=
func (h *Handler) ListCBSD(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	licenseId := middleware.GetLicenseId(c)

	data, total, err := h.svc.ListCbsdInfos(licenseId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list CBSD infos")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// GetCBSD handles GET /cbsd?serial_number=
func (h *Handler) GetCBSD(c *gin.Context) {
	sn := c.Query("serial_number")
	if sn == "" {
		utils.Error(c, http.StatusBadRequest, "serial_number is required")
		return
	}
	licenseId := middleware.GetLicenseId(c)

	info, err := h.svc.GetCbsdInfo(sn, licenseId)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "CBSD not found")
		return
	}
	utils.Success(c, info)
}

// RegisterCBSD handles POST /cbsd
func (h *Handler) RegisterCBSD(c *gin.Context) {
	var info CbsdInfo
	if err := c.ShouldBindJSON(&info); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.RegisterCbsd(&info); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to register CBSD")
		return
	}
	utils.Success(c, info)
}

// UpdateCBSD handles PUT /cbsd/:id
func (h *Handler) UpdateCBSD(c *gin.Context) {
	id := c.Param("id")

	var info CbsdInfo
	if err := c.ShouldBindJSON(&info); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	info.Id = id

	if err := h.svc.UpdateCbsdInfo(&info); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to update CBSD")
		return
	}
	utils.Success(c, info)
}

// DeregisterCBSD handles DELETE /cbsd/:id
func (h *Handler) DeregisterCBSD(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.DeregisterCbsd(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to deregister CBSD")
		return
	}
	utils.Success(c, nil)
}

// ListCBSDLogs handles GET /cbsd/logs?cbsd_id=&log_type=&page=&pageSize=
func (h *Handler) ListCBSDLogs(c *gin.Context) {
	cbsdId := c.Query("cbsd_id")
	logType := c.Query("log_type")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	data, total, err := h.svc.ListCbrsLogs(cbsdId, logType, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list CBRS logs")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// CreateCertFileSendTask handles POST /cbsd/cert-tasks
func (h *Handler) CreateCertFileSendTask(c *gin.Context) {
	var task CBSDCertFileSendTask
	if err := c.ShouldBindJSON(&task); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateCertFileSendTask(&task); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create cert file send task")
		return
	}
	utils.Success(c, task)
}

// ListCertFileSendTasks handles GET /cbsd/cert-tasks?page=&pageSize=
func (h *Handler) ListCertFileSendTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	tenancyId := getTenancyId(c)

	data, total, err := h.svc.ListCertFileSendTasks(tenancyId, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list cert file send tasks")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}
