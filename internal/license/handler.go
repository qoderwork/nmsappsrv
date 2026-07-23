package license

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qoderwork/go-infra/licensing"

	"nmsappsrv/internal/middleware"
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

// GetSASConfig handles GET /license/sas-config
func (h *Handler) GetSASConfig(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)

	data, err := h.svc.GetSASConfig(tenantId)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "sas config not found")
		return
	}
	utils.Success(c, data)
}

// SaveSASConfig handles POST /license/sas-config
func (h *Handler) SaveSASConfig(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)

	var cfg SASConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	cfg.TenantId = &tenantId

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

	utils.Success(c, merge(gin.H{"message": "license activated"}, licenseFields(lic)))
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
		for k, v := range licenseFields(lic) {
			resp[k] = v
		}
	}
	utils.Success(c, resp)
}

// ---------------------------------------------------------------------------
// Backend-agnostic license → response projection.
//
// Enforces the same GET /license/info and POST /license/upload response
// shape regardless of which verifier produced the license. Supports both
// *licensing.License (go-infra backend) and *TrueLicenseContent (TrueLicense
// backend) via a single type-switch helper so handler code stays free of
// per-backend branches.
// ---------------------------------------------------------------------------

// licenseFields returns the common response fields for the given license
// raw value (as returned by Enforcer.GetActive / VerifyAndStore).
func licenseFields(lic interface{}) gin.H {
	if lic == nil {
		return gin.H{}
	}
	switch l := lic.(type) {
	case *licensing.License:
		fields := gin.H{
			"subject":  l.Subject,
			"issuer":   l.Issuer,
			"product":  l.Product,
			"expiry":   nullableTime(l.Expiry),
			"features": l.Features,
			"capacity": l.Capacity,
		}
		if !l.NotBefore.IsZero() {
			fields["not_before"] = l.NotBefore
		} else {
			fields["not_before"] = nil
		}
		if l.Machine != nil {
			fields["machine"] = l.Machine.Fingerprint
			fields["machine_fingerprint"] = l.Machine.Fingerprint
		} else {
			fields["machine"] = ""
			fields["machine_fingerprint"] = ""
		}
		return fields
	case *TrueLicenseContent:
		issuer := ""
		if l.Issuer != nil {
			issuer = l.Issuer["CN"]
			if issuer == "" {
				issuer = joinDN(l.Issuer)
			}
		}
		holder := ""
		if l.Holder != nil {
			holder = l.Holder["CN"]
			if holder == "" {
				holder = joinDN(l.Holder)
			}
		}
		capacity := int32(0)
		if l.ConsumerAmount > 0 {
			capacity = l.ConsumerAmount
		} else if n, ok := extraAsInt32(l.Extra, ExtraEnbQuantity); ok {
			capacity = n
		}
		product := holder
		if p, ok := l.Extra[ExtraLicenseName]; ok && p != "" {
			product = p
		}
		fields := gin.H{
			"subject":              l.Subject,
			"issuer":               issuer,
			"product":              product,
			"expiry":               nullableTime(l.NotAfter),
			"not_before":           nullableTime(l.NotBefore),
			"features":             l.Extra,
			"capacity":             capacity,
			"consumer_type":        l.ConsumerType,
			"consumer_amount":      l.ConsumerAmount,
			"info":                 l.Info,
			"issued":               nullableTime(l.Issued),
			"holder":               holder,
			"machine":              l.Extra[ExtraMachineFp],
			"machine_fingerprint":  l.Extra[ExtraMachineFp],
			"truelicense_backend":  true,
		}
		return fields
	default:
		return gin.H{}
	}
}

func merge(a, b gin.H) gin.H {
	for k, v := range b {
		a[k] = v
	}
	return a
}

func nullableTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

func joinDN(attrs map[string]string) string {
	if attrs == nil || len(attrs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(attrs))
	order := []string{"CN", "O", "OU", "L", "ST", "C"}
	used := map[string]struct{}{}
	for _, k := range order {
		if v, ok := attrs[k]; ok && v != "" {
			parts = append(parts, k+"="+v)
			used[k] = struct{}{}
		}
	}
	for k, v := range attrs {
		if _, ok := used[k]; ok {
			continue
		}
		if v == "" {
			continue
		}
		parts = append(parts, k+"="+v)
	}
	return join(parts, ", ")
}

func join(parts []string, sep string) string {
	res := ""
	for i, p := range parts {
		if i > 0 {
			res += sep
		}
		res += p
	}
	return res
}
