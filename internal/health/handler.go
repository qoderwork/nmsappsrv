package health

import (
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler provides HTTP handlers for health endpoints
type Handler struct {
	svc Service
}

// NewHandler creates a new Handler
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// HealthCheck handles GET /healthCheck
func (h *Handler) HealthCheck(c *gin.Context) {
	utils.Success(c, h.svc.HealthCheck())
}

// Liveness handles GET /healthz — process is alive, no dependency checks.
// Intended for Kubernetes liveness probes.
func (h *Handler) Liveness(c *gin.Context) {
	utils.Success(c, h.svc.HealthCheck())
}

// Readiness handles GET /readyz — checks DB + Redis. Returns HTTP 503 with a
// status body when any dependency is down (Kubernetes readiness probe contract).
func (h *Handler) Readiness(c *gin.Context) {
	st := h.svc.Readiness()
	if st.Status == "down" {
		c.JSON(503, utils.Response{Code: 503, Message: "service not ready", Data: st})
		return
	}
	utils.Success(c, st)
}

// GetMysqlInfo handles GET /api/v1/getMysqlInfo
func (h *Handler) GetMysqlInfo(c *gin.Context) {
	info, err := h.svc.GetMysqlInfo()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, info)
}

// GetRedisInfo handles GET /api/v1/getRedisInfo
func (h *Handler) GetRedisInfo(c *gin.Context) {
	info, err := h.svc.GetRedisInfo()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, info)
}

// GetQueueInfo handles GET /api/v1/getQueueInfo
func (h *Handler) GetQueueInfo(c *gin.Context) {
	info := h.svc.GetQueueInfo()
	utils.Success(c, info)
}

// ReportHAStatus handles POST /reportHAStatus
func (h *Handler) ReportHAStatus(c *gin.Context) {
	var status HAComponentStatus
	if err := c.ShouldBindJSON(&status); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	if err := h.svc.ReportHAStatus(status); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// ReportHAStatusHead handles HEAD /reportHAStatus (keepalive)
func (h *Handler) ReportHAStatusHead(c *gin.Context) {
	c.Status(200)
}
