package heartbeat

import (
	"net/http"
	"strconv"

	"nmsappsrv/internal/config"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HeartbeatHandler exposes HTTP handlers for the heartbeat API.
type HeartbeatHandler struct {
	svc *HeartbeatService
}

// NewHandler creates a HeartbeatHandler with its service dependencies.
func NewHandler(db *gorm.DB, cfg *config.Config) *HeartbeatHandler {
	return &HeartbeatHandler{
		svc: NewHeartbeatService(db, cfg),
	}
}

// ProcessHeartbeat handles POST /api/v1/heartbeat/process
func (h *HeartbeatHandler) ProcessHeartbeat(c *gin.Context) {
	var req struct {
		DeviceSN string                 `json:"device_sn" binding:"required"`
		Payload  map[string]interface{} `json:"payload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "device_sn is required")
		return
	}

	if err := h.svc.ProcessHeartbeat(req.DeviceSN, req.Payload); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to process heartbeat")
		return
	}
	utils.Success(c, nil)
}

// ListHeartbeatStatus handles GET /api/v1/heartbeat/status
func (h *HeartbeatHandler) ListHeartbeatStatus(c *gin.Context) {
	query := c.Query("query")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	data, total, err := h.svc.ListHeartbeatStatus(query, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list heartbeat status")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// SendHeartbeat handles POST /api/v1/heartbeat/send/:sn
func (h *HeartbeatHandler) SendHeartbeat(c *gin.Context) {
	sn := c.Param("sn")
	if sn == "" {
		utils.Error(c, http.StatusBadRequest, "device serial number is required")
		return
	}

	if err := h.svc.SendHeartbeatRequest(sn); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to send heartbeat request")
		return
	}
	utils.Success(c, nil)
}
