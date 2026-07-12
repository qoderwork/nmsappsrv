package initserver

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"
)

// Handler exposes HTTP handlers for init-server endpoints.
//
// Two categories of endpoints:
//   - Config management (behind web UI auth): GetConfig, SaveConfig, ExportConfig, UploadConfig
//   - Device TR-069 communication (no auth, device-facing): HandleInitServer
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// GetConfig handles GET /initserver/getConfig — returns current config as JSON.
func (h *Handler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, cfg)
}

// SaveConfig handles POST /initserver/save — saves config from JSON body.
// Per Java behavior, saving resets initServerEnable to "Disable".
func (h *Handler) SaveConfig(c *gin.Context) {
	var cfg InitServerConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.SaveConfig(&cfg); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// ExportConfig handles GET /initserver/exportConfig — downloads config as JSON file.
func (h *Handler) ExportConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Header("Content-Disposition", "attachment; filename=init_server.cfg")
	c.Data(http.StatusOK, "application/json", data)
}

// UploadConfig handles POST /initserver/uploadConfig — uploads config JSON file.
// Per Java behavior, uploading resets initServerEnable to "Disable".
func (h *Handler) UploadConfig(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "file is required")
		return
	}
	f, err := file.Open()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to open uploaded file")
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to read uploaded file")
		return
	}

	var cfg InitServerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid config JSON")
		return
	}

	if err := h.svc.SaveConfig(&cfg); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// HandleInitServer handles POST /init-server — TR-069 device communication endpoint.
//
// This is the core init-server endpoint. Devices contact this during first-boot
// provisioning. The flow:
//  1. Device sends empty body → server responds with SetParameterValues (init params)
//  2. Device sends Inform → server extracts SN, responds with InformResponse
//  3. Device sends other (SPVResponse, TransferComplete) → session complete
//
// Device identity is tracked via HTTP Cookie "SN".
// No web UI auth required — this endpoint is device-facing.
func (h *Handler) HandleInitServer(c *gin.Context) {
	// Read SOAP body from request
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Errorf("initserver: read request body: %v", err)
		c.Status(http.StatusOK)
		return
	}
	soapBody := string(body)

	if soapBody != "" {
		logger.Debugf("initserver: received SOAP (%d bytes)", len(soapBody))
	}

	// Get device SN from cookie
	cookie, _ := c.Cookie("SN")

	// Process the init request
	responseSoap, sn, err := h.svc.DealInitRequest(soapBody, cookie)
	if err != nil {
		logger.Errorf("initserver: deal init request: %v", err)
		c.Status(http.StatusOK)
		return
	}

	// Set SN cookie for session tracking
	if sn != "" {
		c.SetCookie("SN", sn, 86400, "/", "", false, false)
	}

	// Write SOAP response (or empty to finish session)
	if responseSoap != "" {
		c.Data(http.StatusOK, "text/xml", []byte(responseSoap))
	} else {
		c.Status(http.StatusOK)
	}
}
