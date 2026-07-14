package license

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/qoderwork/go-infra/licensing"

	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for license-related endpoints.
type Handler struct {
	svc Service
	enf *Enforcer
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB, enf *Enforcer) *Handler {
	return &Handler{svc: NewService(db), enf: enf}
}

// ---------------------------------------------------------------------------
// License endpoints
// ---------------------------------------------------------------------------

// GetLicense handles GET /licenses/:id
func (h *Handler) GetLicense(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid license id")
		return
	}

	data, err := h.svc.GetLicense(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "license not found")
		return
	}
	utils.Success(c, data)
}

// ListLicenses handles GET /licenses
func (h *Handler) ListLicenses(c *gin.Context) {
	data, err := h.svc.ListLicenses()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list licenses")
		return
	}
	utils.Success(c, data)
}

// UpdateLicense handles PUT /licenses/:id
func (h *Handler) UpdateLicense(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid license id")
		return
	}

	var l License
	if err := c.ShouldBindJSON(&l); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	l.Id = id

	if err := h.svc.UpdateLicense(&l); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to update license")
		return
	}
	utils.Success(c, &l)
}

// ---------------------------------------------------------------------------
// SASConfig endpoints
// ---------------------------------------------------------------------------

// GetSASConfig handles GET /licenses/:id/sas
func (h *Handler) GetSASConfig(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid license id")
		return
	}

	data, err := h.svc.GetSASConfig(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "sas config not found")
		return
	}
	utils.Success(c, data)
}

// SaveSASConfig handles PUT /licenses/:id/sas
func (h *Handler) SaveSASConfig(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid license id")
		return
	}

	var cfg SASConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	cfg.LicenseId = &id

	if err := h.svc.SaveSASConfig(&cfg); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to save sas config")
		return
	}
	utils.Success(c, &cfg)
}

// ---------------------------------------------------------------------------
// EntraEndpoint endpoints
// ---------------------------------------------------------------------------

// ListEntraEndpoints handles GET /entra-endpoints
func (h *Handler) ListEntraEndpoints(c *gin.Context) {
	data, err := h.svc.ListEntraEndpoints()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list entra endpoints")
		return
	}
	utils.Success(c, data)
}

// CreateEntraEndpoint handles POST /entra-endpoints
func (h *Handler) CreateEntraEndpoint(c *gin.Context) {
	var e EntraEndpoint
	if err := c.ShouldBindJSON(&e); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateEntraEndpoint(&e); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create entra endpoint")
		return
	}
	utils.Success(c, &e)
}

// UpdateEntraEndpoint handles PUT /entra-endpoints/:id
func (h *Handler) UpdateEntraEndpoint(c *gin.Context) {
	id := c.Param("id")

	var e EntraEndpoint
	if err := c.ShouldBindJSON(&e); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	e.Id = id

	if err := h.svc.UpdateEntraEndpoint(&e); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to update entra endpoint")
		return
	}
	utils.Success(c, &e)
}

// DeleteEntraEndpoint handles DELETE /entra-endpoints/:id
func (h *Handler) DeleteEntraEndpoint(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.DeleteEntraEndpoint(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete entra endpoint")
		return
	}
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// L-2 license enforcement endpoints (public — must stay open before activation)
// ---------------------------------------------------------------------------

// UploadLicenseFile handles POST /license/upload (public).
// It accepts a multipart .lic envelope, verifies it, and activates it.
func (h *Handler) UploadLicenseFile(c *gin.Context) {
	if h.enf == nil {
		utils.Error(c, http.StatusServiceUnavailable, "license enforcer not configured")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "missing license file (multipart field 'file')")
		return
	}
	f, err := file.Open()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "cannot open uploaded file")
		return
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "cannot read uploaded file")
		return
	}

	lic, err := h.enf.VerifyAndStore(raw, file.Filename)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{
		"message":    "license activated",
		"subject":    lic.Subject,
		"issuer":     lic.Issuer,
		"product":    lic.Product,
		"expiry":     lic.Expiry,
		"not_before": lic.NotBefore,
		"features":   lic.Features,
		"capacity":   lic.Capacity,
		"machine":    machineFP(lic),
	})
}

// GetLicenseInfo handles GET /license/info (public) — reports enforcement state.
func (h *Handler) GetLicenseInfo(c *gin.Context) {
	if h.enf == nil {
		utils.Error(c, http.StatusServiceUnavailable, "license enforcer not configured")
		return
	}
	lic, status, detail := h.enf.GetActive()
	resp := gin.H{
		"required": h.enf.Required(),
		"status":   status,
		"detail":   detail,
	}
	if lic != nil {
		resp["subject"] = lic.Subject
		resp["issuer"] = lic.Issuer
		resp["product"] = lic.Product
		resp["expiry"] = lic.Expiry
		resp["not_before"] = lic.NotBefore
		resp["features"] = lic.Features
		resp["capacity"] = lic.Capacity
		resp["machine_fingerprint"] = machineFP(lic)
	}
	utils.Success(c, resp)
}

// machineFP returns the license's bound machine fingerprint, or "".
func machineFP(lic *licensing.License) string {
	if lic == nil || lic.Machine == nil {
		return ""
	}
	return lic.Machine.Fingerprint
}
