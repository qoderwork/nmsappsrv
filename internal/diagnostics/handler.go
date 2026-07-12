package diagnostics

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/config"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for CPE diagnostics endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// ListDiagnosticsResult handles POST /diagnostics/result
func (h *Handler) ListDiagnosticsResult(c *gin.Context) {
	var req IdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: id is required")
		return
	}

	result, err := h.svc.GetDiagnosticsResult(req.Id)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, result)
}

// ListDiagnosticsStatus handles POST /diagnostics/status
func (h *Handler) ListDiagnosticsStatus(c *gin.Context) {
	var req IdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: id is required")
		return
	}

	inDiag := h.svc.GetDiagnosticsStatus(req.Id)
	utils.Success(c, inDiag)
}

// DiagnosticsPing handles POST /diagnostics/ping
func (h *Handler) DiagnosticsPing(c *gin.Context) {
	var req PingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: server, count and elementId are required")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.svc.TriggerPing(&req, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// DiagnosticsTraceRoute handles POST /diagnostics/trace-route
func (h *Handler) DiagnosticsTraceRoute(c *gin.Context) {
	var req TraceRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: server and elementId are required")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.svc.TriggerTraceRoute(&req, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// DiagnosticsDownload handles POST /diagnostics/download
func (h *Handler) DiagnosticsDownload(c *gin.Context) {
	var req DownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: elementId is required")
		return
	}

	username := middleware.GetUsername(c)
	fileServerIp := ""
	if config.Cfg != nil {
		fileServerIp = config.Cfg.TR069.FileServerIp
	}
	if err := h.svc.TriggerDownload(&req, username, fileServerIp); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}

// DiagnosticsUpload handles POST /diagnostics/upload
func (h *Handler) DiagnosticsUpload(c *gin.Context) {
	var req UploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: elementId is required")
		return
	}

	username := middleware.GetUsername(c)
	fileServerIp := ""
	if config.Cfg != nil {
		fileServerIp = config.Cfg.TR069.FileServerIp
	}
	if err := h.svc.TriggerUpload(&req, username, fileServerIp); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.OK(c, "ok")
}
