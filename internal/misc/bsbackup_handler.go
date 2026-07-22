package misc

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
)

// ListBaseStationBackupInfo handles POST /listBaseStationBackupLatestFileInfo.
// Returns a paginated list of devices with their latest config file / backup info.
func (h *Handler) ListBaseStationBackupInfo(c *gin.Context) {
	var req ListBaseStationBackupInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	tenantId := middleware.GetTenantId(c)

	list, total, err := h.svc.ListBaseStationBackupInfo(&req, tenantId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list backup info: "+err.Error())
		return
	}

	utils.Paginated(c, list, total, req.Page, req.PageSize)
}

// ImportConfigFile handles POST /importConfigFile.
// Accepts a multipart file upload together with an elementId form field.
func (h *Handler) ImportConfigFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	fileData, err := io.ReadAll(file)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to read file: "+err.Error())
		return
	}

	elementIdStr := c.PostForm("elementId")
	elementId, err := strconv.ParseInt(elementIdStr, 10, 64)
	if err != nil || elementId == 0 {
		utils.Error(c, http.StatusBadRequest, "valid elementId is required")
		return
	}

	tenantId := middleware.GetTenantId(c)

	result, err := h.svc.ImportConfigFile(elementId, header.Filename, fileData, tenantId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to import config file: "+err.Error())
		return
	}

	utils.Success(c, result)
}

// ExportConfigFile handles POST /exportConfigFile.
// Collects config files for the requested devices and returns them as a zip archive.
func (h *Handler) ExportConfigFile(c *gin.Context) {
	var req ExportConfigFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if len(req.ElementIds) == 0 {
		utils.Error(c, http.StatusBadRequest, "elementIds is required")
		return
	}

	tenantId := middleware.GetTenantId(c)

	zipPath, err := h.svc.ExportConfigFile(req.ElementIds, tenantId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to export config files: "+err.Error())
		return
	}

	c.Header("Content-Disposition", "attachment; filename=config_files.zip")
	c.Header("Content-Type", "application/zip")
	c.File(zipPath)
}

// AddBSBackupTask handles POST /addBackupTask.
// Creates a new device-specific backup task.
func (h *Handler) AddBSBackupTask(c *gin.Context) {
	var req AddBSBackupTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	username := middleware.GetUsername(c)
	tenantId := middleware.GetTenantId(c)

	if err := h.svc.CreateBSBackupTask(&req, username, tenantId); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create backup task: "+err.Error())
		return
	}

	utils.OK(c, nil)
}

// CancelBackupTask handles POST /cancelBackupTask.
// Sets a backup task's status to 4 (cancelled).
func (h *Handler) CancelBackupTask(c *gin.Context) {
	var req CancelTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if err := h.svc.CancelTask(req.TaskId); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to cancel backup task: "+err.Error())
		return
	}

	utils.OK(c, nil)
}

// CancelRestoreTask handles POST /cancelRestoreTask.
// Sets a restore task's status to 4 (cancelled).
func (h *Handler) CancelRestoreTask(c *gin.Context) {
	var req CancelTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if err := h.svc.CancelTask(req.TaskId); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to cancel restore task: "+err.Error())
		return
	}

	utils.OK(c, nil)
}

// AddBSRestoreTask handles POST /addRestoreTask.
// Creates a new device-specific restore task.
func (h *Handler) AddBSRestoreTask(c *gin.Context) {
	var req AddBSRestoreTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	username := middleware.GetUsername(c)
	tenantId := middleware.GetTenantId(c)

	if err := h.svc.CreateBSRestoreTask(&req, username, tenantId); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create restore task: "+err.Error())
		return
	}

	utils.OK(c, nil)
}

// StartBackupOrRestoreTask handles POST /startBackupOrRestoreTask.
// Manually starts a task that is in the awaiting state (status 1 -> 2).
func (h *Handler) StartBackupOrRestoreTask(c *gin.Context) {
	var req StartTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	username := middleware.GetUsername(c)

	if err := h.svc.StartBSBackupRestoreTask(req.TaskId, username); err != nil {
		utils.Error(c, http.StatusInternalServerError, fmt.Sprintf("failed to start task: %v", err))
		return
	}

	utils.OK(c, nil)
}

// ListBSBackupTasks handles POST /listBackupOrRestoreTask.
// Returns a paginated list of backup/restore tasks.
func (h *Handler) ListBSBackupTasks(c *gin.Context) {
	var req ListBSBackupTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	tenantId := middleware.GetTenantId(c)

	list, total, err := h.svc.ListBSBackupTasks(tenantId, req.Page, req.PageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list tasks: "+err.Error())
		return
	}

	utils.Paginated(c, list, total, req.Page, req.PageSize)
}

// ListDeviceBackupResult handles POST /listDeviceBackupAndRestoreResult.
// Returns per-device execution results for a given task.
func (h *Handler) ListDeviceBackupResult(c *gin.Context) {
	var req ListDeviceBackupResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	list, total, err := h.svc.ListDeviceBackupResult(req.TaskId, req.Page, req.PageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list device results: "+err.Error())
		return
	}

	utils.Paginated(c, list, total, req.Page, req.PageSize)
}

// DownloadConfigFile handles GET /downloadConfigFile.
// Downloads a single configuration file identified by its config_upload_log id.
func (h *Handler) DownloadConfigFile(c *gin.Context) {
	logId := c.Query("logId")
	if logId == "" {
		utils.Error(c, http.StatusBadRequest, "logId is required")
		return
	}

	filePath, err := h.svc.GetConfigFilePath(logId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to get config file: "+err.Error())
		return
	}

	c.FileAttachment(filePath, logId+".cfg")
}
