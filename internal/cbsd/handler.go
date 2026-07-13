package cbsd

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for CBSD endpoints.
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

	licenseId := middleware.GetLicenseId(c)
	if err := h.svc.RegisterCbsd(&info, licenseId); err != nil {
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

// ---------------------------------------------------------------------------
// CBSD lifecycle
// ---------------------------------------------------------------------------

// EnableCBSD handles POST /cbsd/:id/enable
func (h *Handler) EnableCBSD(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.EnableCBSD(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// DisableCBSD handles POST /cbsd/:id/disable
func (h *Handler) DisableCBSD(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.DisableCBSD(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

// DeleteCBSD handles DELETE /cbsd/:id
func (h *Handler) DeleteCBSD(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.DeleteCBSD(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete CBSD")
		return
	}
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// SAS protocol operations
// ---------------------------------------------------------------------------

// SpectrumInquiry handles POST /cbsd/spectrum-inquiry
func (h *Handler) SpectrumInquiry(c *gin.Context) {
	var req SpectrumInquiryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	licenseId := middleware.GetLicenseId(c)
	result, err := h.svc.SpectrumInquiry(licenseId, &req)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, fmt.Sprintf("spectrum inquiry failed: %v", err))
		return
	}
	utils.Success(c, result)
}

// Grant handles POST /cbsd/:id/grant
func (h *Handler) Grant(c *gin.Context) {
	id := c.Param("id")

	var req GrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.svc.Grant(id, &req)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, fmt.Sprintf("grant failed: %v", err))
		return
	}
	utils.Success(c, result)
}

// Relinquishment handles POST /cbsd/:id/relinquishment
func (h *Handler) Relinquishment(c *gin.Context) {
	id := c.Param("id")

	var req RelinquishmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.svc.Relinquishment(id, &req)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, fmt.Sprintf("relinquishment failed: %v", err))
		return
	}
	utils.Success(c, result)
}

// ---------------------------------------------------------------------------
// Import / Template
// ---------------------------------------------------------------------------

// ImportCBSDs handles POST /cbsd/import (CSV file upload)
func (h *Handler) ImportCBSDs(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "CSV file is required")
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "failed to parse CSV file")
		return
	}

	// Skip header row if present
	if len(records) > 0 && strings.HasPrefix(records[0][0], "serial") {
		records = records[1:]
	}

	licenseId := middleware.GetLicenseId(c)
	count, err := h.svc.ImportCBSDs(licenseId, records)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, fmt.Sprintf("import failed: %v", err))
		return
	}
	utils.Success(c, map[string]interface{}{"imported": count})
}

// DownloadCBSDTemplate handles GET /cbsd/template
func (h *Handler) DownloadCBSDTemplate(c *gin.Context) {
	header := "serial_number,cbsd_category,latitude,longitude,height,vendor,model\n"
	c.Header("Content-Disposition", "attachment; filename=cbsd_import_template.csv")
	c.Data(http.StatusOK, "text/csv", []byte(header))
}

// ---------------------------------------------------------------------------
// Statistics
// ---------------------------------------------------------------------------

// ListCBSDStatusCount handles POST /cbsd/status-count
func (h *Handler) ListCBSDStatusCount(c *gin.Context) {
	licenseId := middleware.GetLicenseId(c)

	data, err := h.svc.ListCBSDStatusCount(licenseId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to get CBSD status count")
		return
	}
	utils.Success(c, data)
}

// ---------------------------------------------------------------------------
// SAS config
// ---------------------------------------------------------------------------

// ListSasConfig handles GET /cbsd/sas-config
func (h *Handler) ListSasConfig(c *gin.Context) {
	licenseId := middleware.GetLicenseId(c)

	data, err := h.svc.ListSasConfig(licenseId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list SAS config")
		return
	}
	utils.Success(c, data)
}

// UpdateSasConfig handles PUT /cbsd/sas-config
func (h *Handler) UpdateSasConfig(c *gin.Context) {
	var cfg SasConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.UpdateSasConfig(&cfg); err != nil {
		utils.Error(c, http.StatusInternalServerError, fmt.Sprintf("failed to update SAS config: %v", err))
		return
	}
	utils.Success(c, cfg)
}
