package resources

import (
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler provides HTTP handlers for resources endpoints
type Handler struct {
	svc Service
}

// NewHandler creates a new Handler
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(NewRepository(db))}
}

// GetCpuAndMemUsage handles POST /api/v1/getCpuAndMemUsage
func (h *Handler) GetCpuAndMemUsage(c *gin.Context) {
	utils.Success(c, h.svc.GetCpuAndMemUsage())
}

// GetTableStatus handles POST /api/v1/getTableStatus
func (h *Handler) GetTableStatus(c *gin.Context) {
	data, err := h.svc.GetTableStatus()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// GetDiskUsage handles POST /api/v1/getDiskUsage
func (h *Handler) GetDiskUsage(c *gin.Context) {
	utils.Success(c, h.svc.GetDiskUsage())
}

// SetCPUAndMemThreshold handles POST /api/v1/setCPUAndMemThreshold
func (h *Handler) SetCPUAndMemThreshold(c *gin.Context) {
	var cfg ThresholdConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}
	if err := h.svc.UpdateThreshold(&cfg); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ListCPUAndMemThreshold handles POST /api/v1/listCPUAndMemThreshold
func (h *Handler) ListCPUAndMemThreshold(c *gin.Context) {
	cfg, err := h.svc.GetThreshold()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, cfg)
}
