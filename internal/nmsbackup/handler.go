package nmsbackup

import (
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// AddNMSBackupTask creates a new backup schedule
func (h *Handler) AddNMSBackupTask(c *gin.Context) {
	var req AddNMSBackupTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	schedule, err := h.svc.AddBackupSchedule(c, &req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, schedule)
}

// ListNMSBackupTask returns paginated list of backup schedules
func (h *Handler) ListNMSBackupTask(c *gin.Context) {
	var req ListNMSBackupTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	data, total, err := h.svc.ListBackupSchedules(c, &req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	utils.Paginated(c, data, total, page, pageSize)
}

// ModifyNMSBackupTask updates an existing backup schedule
func (h *Handler) ModifyNMSBackupTask(c *gin.Context) {
	var req ModifyNMSBackupTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.ModifyBackupSchedule(c, &req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// RunNMSBackupTask triggers immediate backup execution
func (h *Handler) RunNMSBackupTask(c *gin.Context) {
	var req RunNMSBackupTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.RunBackup(c, &req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// DeleteNMSBackupTask deletes a backup schedule
func (h *Handler) DeleteNMSBackupTask(c *gin.Context) {
	var req DeleteNMSBackupTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.DeleteBackupSchedule(c, &req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// RevertNMSBackupTask triggers restore from backup
func (h *Handler) RevertNMSBackupTask(c *gin.Context) {
	var req RevertNMSBackupTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.RevertBackup(c, &req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// GetBackupAndRestoreConfig reads retention configuration
func (h *Handler) GetBackupAndRestoreConfig(c *gin.Context) {
	config, err := h.svc.GetBackupRetentionConfig()
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, config)
}

// UpdateBackupAndRestoreConfig updates retention configuration
func (h *Handler) UpdateBackupAndRestoreConfig(c *gin.Context) {
	var req UpdateBackupRetentionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.UpdateBackupRetentionConfig(&req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// ListNMSBackupLogs returns paginated list of backup/revert logs
func (h *Handler) ListNMSBackupLogs(c *gin.Context) {
	var req ListNMSBackupLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	data, total, err := h.svc.ListBackupLogs(&req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	utils.Paginated(c, data, total, page, pageSize)
}

// GetNMSBackupLogDetail returns a single log detail
func (h *Handler) GetNMSBackupLogDetail(c *gin.Context) {
	var req GetNMSBackupLogDetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	log, err := h.svc.GetBackupLogDetail(&req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, log)
}
