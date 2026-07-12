package ntp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"nmsappsrv/pkg/utils"
)

// Handler exposes HTTP handlers for NTP endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// ListNTPConfig handles POST /listNTPConfig
func (h *Handler) ListNTPConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, cfg)
}

// UpdateNTPConfig handles POST /updateNTPConfig
func (h *Handler) UpdateNTPConfig(c *gin.Context) {
	var req NTPConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.UpdateConfig(&req); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// GetNTPStatus handles POST /getNTPStatus
func (h *Handler) GetNTPStatus(c *gin.Context) {
	status, err := h.svc.GetStatus()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, NTPStatusResponse{Status: status})
}
