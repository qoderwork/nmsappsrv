package security

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"nmsappsrv/pkg/utils"
)

// Handler exposes HTTP handlers for security rule endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// GetSecurityRule handles POST /getSecurityRule
func (h *Handler) GetSecurityRule(c *gin.Context) {
	rule, err := h.svc.GetRule()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, rule)
}

// UpdateSecurityRule handles POST /updateSecurityRule
func (h *Handler) UpdateSecurityRule(c *gin.Context) {
	var req UpdateSecurityRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.UpdateRule(&req); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// GetPasswordStrategy handles GET /getPasswordStrategy
func (h *Handler) GetPasswordStrategy(c *gin.Context) {
	strategy, err := h.svc.GetPasswordStrategy()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, strategy)
}
