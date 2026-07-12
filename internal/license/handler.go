package license

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for license-related endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
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
