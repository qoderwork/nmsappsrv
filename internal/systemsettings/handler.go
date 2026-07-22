package systemsettings

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"
)

// SystemSettingsHandler handles system settings endpoints.
type SystemSettingsHandler struct {
	svc *SystemSettingsService
}

// NewSystemSettingsHandler creates a new SystemSettingsHandler.
func NewSystemSettingsHandler(db *gorm.DB, aesKey string) *SystemSettingsHandler {
	return &SystemSettingsHandler{
		svc: NewSystemSettingsService(db, aesKey),
	}
}

// Service returns the underlying service so other modules (e.g. the device
// file server's Basic-auth) can share the same instance.
func (h *SystemSettingsHandler) Service() *SystemSettingsService { return h.svc }

// ListDeviceSettings returns the device configuration for the current tenancy.
func (h *SystemSettingsHandler) ListDeviceSettings(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)

	cfg, err := h.svc.GetDeviceSettings(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, cfg)
}

// UpdateDeviceSettings updates the device configuration for the current tenancy.
func (h *SystemSettingsHandler) UpdateDeviceSettings(c *gin.Context) {
	var req UpdateDeviceSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	tenantId := middleware.GetTenantId(c)

	if err := h.svc.UpdateDeviceSettings(&req, tenantId); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.OK(c, nil)
}

// ListACSSettings returns the ACS configuration.
func (h *SystemSettingsHandler) ListACSSettings(c *gin.Context) {
	cfg, err := h.svc.GetACSConfig()
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, cfg)
}

// UpdateACSSettings updates the ACS configuration.
func (h *SystemSettingsHandler) UpdateACSSettings(c *gin.Context) {
	var req UpdateACSConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.UpdateACSConfig(&req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.OK(c, nil)
}

// ListLogSettings returns the log configuration.
func (h *SystemSettingsHandler) ListLogSettings(c *gin.Context) {
	cfg, err := h.svc.GetLogConfig()
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, cfg)
}

// UpdateLogSettings updates the log configuration.
func (h *SystemSettingsHandler) UpdateLogSettings(c *gin.Context) {
	var req UpdateLogConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.UpdateLogConfig(&req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.OK(c, nil)
}

// GetNorthBoundConfig returns the northbound integration configuration for the
// current tenancy (Java NorthBoundManagementController.getNorthBoundConfig).
func (h *SystemSettingsHandler) GetNorthBoundConfig(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)

	cfg, err := h.svc.GetNorthBoundConfig(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, cfg)
}

// UpdateNorthBoundConfig upserts the northbound integration configuration for the
// current tenancy (Java NorthBoundManagementController.updateNorthBoundConfig).
// The request body is a flat NorthBoundConfig (not wrapped in a RequestDataDTO),
// consistent with the rest of this package's update handlers.
func (h *SystemSettingsHandler) UpdateNorthBoundConfig(c *gin.Context) {
	var req NorthBoundConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	tenantId := middleware.GetTenantId(c)

	if err := h.svc.UpdateNorthBoundConfig(&req, tenantId); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.OK(c, nil)
}
