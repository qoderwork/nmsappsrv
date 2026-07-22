package deviceauth

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"time"

	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler provides HTTP handlers for device auth config management.
type Handler struct {
	svc Service
}

// NewHandler creates a new device auth config handler.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// GetConfig returns the current device auth configuration.
func (h *Handler) GetConfig(c *gin.Context) {
	tenantId, _ := c.Get("tenant_id")
	lid, _ := tenantId.(string)

	cfg, err := h.svc.GetConfig(lid)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if cfg == nil {
		cfg = &DeviceAuthConfig{}
	}
	utils.Success(c, cfg)
}

// SaveConfig saves the device auth configuration for the current tenant.
func (h *Handler) SaveConfig(c *gin.Context) {
	var cfg DeviceAuthConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantId, _ := c.Get("tenant_id")
	lid, _ := tenantId.(string)

	if err := h.svc.SaveConfig(&cfg, lid); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, cfg)
}

// GetAuthInfo returns the auth challenge info for the current tenant.
// Used by the frontend to know what auth type is configured.
func (h *Handler) GetAuthInfo(c *gin.Context) {
	tenantId, _ := c.Get("tenant_id")
	lid, _ := tenantId.(string)

	cfg, err := h.svc.GetConfig(lid)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if cfg == nil {
		cfg = &DeviceAuthConfig{}
	}

	info := gin.H{
		"enable":    cfg.Enable,
		"algorithm": cfg.Algorithm,
	}
	if cfg.Enable && cfg.Algorithm == "Digest" {
		nonce := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%d:%s", time.Now().UnixNano(), "TR-069 ACS"))))
		info["nonce"] = nonce
		info["realm"] = "TR-069 ACS"
		info["qop"] = "auth"
	}
	utils.Success(c, info)
}
