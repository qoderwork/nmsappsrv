package paramcompare

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for parameter comparison endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// Compare handles POST /api/v1/param-compare/compare
// Body: { "device_id": 1, "template_id": 2 }
func (h *Handler) Compare(c *gin.Context) {
	var req CompareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body: device_id and template_id are required")
		return
	}

	result, err := h.svc.CompareDeviceWithTemplate(req.DeviceID, req.TemplateID)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, result)
}

// BatchCompare handles POST /api/v1/param-compare/batch
// Body: { "device_ids": [1, 2, 3], "template_id": 2 }
func (h *Handler) BatchCompare(c *gin.Context) {
	var req BatchCompareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body: device_ids and template_id are required")
		return
	}

	if len(req.DeviceIDs) == 0 {
		utils.Error(c, http.StatusBadRequest, "device_ids must not be empty")
		return
	}

	results, err := h.svc.BatchCompare(req.DeviceIDs, req.TemplateID)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, results)
}

// ListTemplates handles GET /api/v1/param-compare/templates
func (h *Handler) ListTemplates(c *gin.Context) {
	data, err := h.svc.ListTemplates()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list templates")
		return
	}
	utils.Success(c, data)
}
