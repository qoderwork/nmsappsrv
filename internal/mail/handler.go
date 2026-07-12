package mail

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"
)

// Handler exposes HTTP handlers for mail endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB, aesKeyHex string) *Handler {
	return &Handler{svc: NewService(db, aesKeyHex)}
}

// ListMailConfig handles POST /listMailConfig
func (h *Handler) ListMailConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, cfg)
}

// UpdateMailConfig handles POST /updateMailConfig
func (h *Handler) UpdateMailConfig(c *gin.Context) {
	var req UpdateMailConfigRequest
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

// TestMail handles POST /testMail
func (h *Handler) TestMail(c *gin.Context) {
	if err := h.svc.SendTestMail(); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// GetEmailCode handles POST /getEmailCode
func (h *Handler) GetEmailCode(c *gin.Context) {
	var req GetEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.SendEmailCode(req.Username, req.GrantType); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, EmailCodeResponse{Sent: true})
}

// CheckEmailCode handles POST /checkEmailCode
func (h *Handler) CheckEmailCode(c *gin.Context) {
	var req CheckEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	username := middleware.GetUsername(c)
	ok, err := h.svc.CheckEmailCode(username, req.EmailCode)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		utils.Error(c, http.StatusBadRequest, "invalid or expired verification code")
		return
	}
	utils.OK(c, "ok")
}

// IsEnabledEmailAuthentication handles POST /isEnabledEmailAuthentication
func (h *Handler) IsEnabledEmailAuthentication(c *gin.Context) {
	enabled, err := h.svc.IsEmailAuthEnabled()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, IsEnabledResponse{Enabled: enabled})
}
