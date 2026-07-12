package restapi

import (
	"nmsappsrv/pkg/apperror"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// offsetResponse sends a response with X-Total-Count and Link headers for offset-based pagination
func (h *Handler) offsetResponse(c *gin.Context, data interface{}, total int64, offset, limit int, baseUrl string) {
	c.Header("X-Total-Count", fmt.Sprintf("%d", total))
	c.Header("Link", generateLinkHeader(baseUrl, offset, limit, int(total)))
	utils.Success(c, data)
}

// parseOffsetLimit extracts offset and limit from query params with defaults
func parseOffsetLimit(c *gin.Context) (int, int) {
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return offset, limit
}

// buildBaseUrl reconstructs the base URL for Link header generation
func buildBaseUrl(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s%s", scheme, c.Request.Host, c.Request.URL.Path)
}

// ============================
// Device endpoints
// ============================

// GET /v1/devices — list devices with offset pagination
func (h *Handler) ListDevices(c *gin.Context) {
	offset, limit := parseOffsetLimit(c)

	data, total, err := h.svc.ListDevices(c, offset, limit)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	h.offsetResponse(c, data, total, offset, limit, buildBaseUrl(c))
}

// GET /v1/device/:id — get single device
func (h *Handler) GetDevice(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, 400, "Invalid device ID")
		return
	}

	device, err := h.svc.GetDevice(c, id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, device)
}

// POST /v1/devices — add device
func (h *Handler) AddDevice(c *gin.Context) {
	var req AddRestDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	device, err := h.svc.AddDevice(c, &req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, device)
}

// PUT /v1/devices/:id — modify device by ID
func (h *Handler) ModifyDeviceById(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, 400, "Invalid device ID")
		return
	}

	var req ModifyRestDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.ModifyDeviceById(c, id, &req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// PUT /v1/devices — modify device by SN
func (h *Handler) ModifyDeviceBySN(c *gin.Context) {
	var req ModifyRestDeviceBySNRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.ModifyDeviceBySN(c, &req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// DELETE /v1/devices/:id — soft delete device
func (h *Handler) DeleteDevice(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, 400, "Invalid device ID")
		return
	}

	if err := h.svc.DeleteDevice(c, id); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// ============================
// Parameter endpoints
// ============================

// GET /v1/device/:id/parameters — get device parameters
func (h *Handler) GetDeviceParams(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, 400, "Invalid device ID")
		return
	}

	params, err := h.svc.GetDeviceParams(c, id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, params)
}

// PUT /v1/device/:id/parameters — set device parameters
func (h *Handler) SetDeviceParams(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, 400, "Invalid device ID")
		return
	}

	var req SetRestParameterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.SetDeviceParams(c, id, &req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// PUT /v1/device/:id/presetParameters — preset parameters
func (h *Handler) PresetDeviceParams(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, 400, "Invalid device ID")
		return
	}

	var req PresetParameterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	status, err := h.svc.PresetDeviceParams(c, id, &req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, status)
}

// ============================
// Request status endpoint
// ============================

// GET /v1/requests/:requestId — get request status
func (h *Handler) GetRequestStatus(c *gin.Context) {
	requestId := c.Param("requestId")
	if requestId == "" {
		utils.Error(c, 400, "Request ID is required")
		return
	}

	status, err := h.svc.GetRequestStatus(requestId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, status)
}

// ============================
// Alarm endpoints
// ============================

// GET /v1/alarms — list alarms with offset pagination
func (h *Handler) ListAlarms(c *gin.Context) {
	offset, limit := parseOffsetLimit(c)

	data, total, err := h.svc.ListAlarms(c, offset, limit)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	h.offsetResponse(c, data, total, offset, limit, buildBaseUrl(c))
}

// POST /v1/syncAlarm — sync alarms for specified devices
func (h *Handler) SyncAlarm(c *gin.Context) {
	var req SyncAlarmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	data, err := h.svc.SyncAlarms(c, &req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, data)
}

// POST /v1/clearAlarm — clear specified alarms
func (h *Handler) ClearAlarm(c *gin.Context) {
	var req ClearAlarmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.ClearAlarms(c, &req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// ============================
// Upgrade file endpoints
// ============================

// POST /v1/upgradeFile — upload upgrade file
func (h *Handler) UploadUpgradeFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, 400, "File is required")
		return
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".bin" && ext != ".img" && ext != ".zip" && ext != ".tar" && ext != ".gz" {
		utils.Error(c, 400, "Unsupported file type. Allowed: .bin, .img, .zip, .tar, .gz")
		return
	}

	// Save file to disk
	uploadDir := "/data/uploads/upgrade"
	fileName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), file.Filename)
	filePath := filepath.Join(uploadDir, fileName)

	if err := c.SaveUploadedFile(file, filePath); err != nil {
		logger.Errorf("Failed to save uploaded file: %v", err)
		utils.HandleError(c, apperror.ErrInternal.WithMessage("failed to save uploaded file"))
		return
	}

	vo, err := h.svc.UploadUpgradeFile(c, file.Filename, filePath, file.Size)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, vo)
}

// GET /v1/upgradeFile — list upgrade files with offset pagination
func (h *Handler) ListUpgradeFiles(c *gin.Context) {
	offset, limit := parseOffsetLimit(c)

	data, total, err := h.svc.ListUpgradeFiles(c, offset, limit)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	h.offsetResponse(c, data, total, offset, limit, buildBaseUrl(c))
}

// DELETE /v1/upgradeFile — delete upgrade file
func (h *Handler) DeleteUpgradeFile(c *gin.Context) {
	idStr := c.DefaultQuery("id", "0")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		utils.Error(c, 400, "Valid file ID is required")
		return
	}

	if err := h.svc.DeleteUpgradeFile(c, id); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// ============================
// Upgrade task endpoints
// ============================

// POST /v1/upgradeTask — create upgrade task
func (h *Handler) CreateUpgradeTask(c *gin.Context) {
	var req RestUpgradeTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	vo, err := h.svc.CreateUpgradeTask(c, &req)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, vo)
}

// GET /v1/upgradeTask — view upgrade task results
func (h *Handler) ListUpgradeTasks(c *gin.Context) {
	// If id query param is provided, return single task
	idStr := c.DefaultQuery("id", "")
	if idStr != "" {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			utils.Error(c, 400, "Invalid task ID")
			return
		}

		vo, err := h.svc.GetUpgradeTask(c, id)
		if err != nil {
			utils.HandleError(c, err)
			return
		}

		utils.Success(c, vo)
		return
	}

	// Otherwise list all tasks with offset pagination
	offset, limit := parseOffsetLimit(c)

	data, total, err := h.svc.ListUpgradeTasks(c, offset, limit)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	h.offsetResponse(c, data, total, offset, limit, buildBaseUrl(c))
}

// ============================
// TBG (femtocell) endpoints
// ============================

// POST /v1/femtos — add TBG devices (batch, max 100)
func (h *Handler) AddTBGs(c *gin.Context) {
	var reqs []AddTBGRequest
	if err := c.ShouldBindJSON(&reqs); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if len(reqs) == 0 {
		utils.Error(c, 400, "At least one TBG device is required")
		return
	}

	data, err := h.svc.AddTBGs(c, reqs)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, data)
}

// PUT /v1/femtos — modify TBG devices
func (h *Handler) ModifyTBGs(c *gin.Context) {
	var reqs []ModifyTBGRequest
	if err := c.ShouldBindJSON(&reqs); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.ModifyTBGs(c, reqs); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// POST /v1/femtos/delete — delete TBG devices
func (h *Handler) DeleteTBGs(c *gin.Context) {
	var req DeleteTBGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.DeleteTBGs(c, &req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// GET /v1/femtos — list TBG devices with offset pagination
func (h *Handler) ListTBGs(c *gin.Context) {
	offset, limit := parseOffsetLimit(c)

	data, total, err := h.svc.ListTBGs(c, offset, limit)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	h.offsetResponse(c, data, total, offset, limit, buildBaseUrl(c))
}

// GET /v1/femtos/:sn — get TBG by serial number
func (h *Handler) GetTBGBySN(c *gin.Context) {
	sn := c.Param("sn")
	if sn == "" {
		utils.Error(c, 400, "Serial number is required")
		return
	}

	tbg, err := h.svc.GetTBGBySN(sn)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, tbg)
}

// GET /v1/femtos/wan/:mac — get TBG by WAN MAC address
func (h *Handler) GetTBGByWanMac(c *gin.Context) {
	mac := c.Param("wanMac")
	if mac == "" {
		utils.Error(c, 400, "WAN MAC address is required")
		return
	}

	tbg, err := h.svc.GetTBGByWanMac(mac)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, tbg)
}

// ============================
// Device Online Status (Task 6.2)
// ============================

// GET /v1/device/online-status — list all devices with real-time online status
func (h *Handler) ListDeviceOnlineStatus(c *gin.Context) {
	data, err := h.svc.ListDeviceOnlineStatus(c)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, data)
}

// GET /v1/device/:elementId/online-status — single device online status
func (h *Handler) GetDeviceOnlineStatus(c *gin.Context) {
	elementId, err := strconv.ParseInt(c.Param("elementId"), 10, 64)
	if err != nil {
		utils.Error(c, 400, "Invalid element ID")
		return
	}

	data, err := h.svc.GetDeviceOnlineStatus(c, elementId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, data)
}

// ============================
// ACS Settings (Task 6.3)
// ============================

// GET /v1/settings/acs — get ACS configuration
func (h *Handler) GetACSSettings(c *gin.Context) {
	data, err := h.svc.GetACSSettings(c)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, data)
}

// PUT /v1/settings/acs — update ACS configuration
func (h *Handler) UpdateACSSettings(c *gin.Context) {
	var req RestUpdateACSConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.UpdateACSSettings(c, &req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// ============================
// SNMP Operations (Task 6.4)
// ============================

// POST /v1/snmp/get — trigger SNMP GET
func (h *Handler) SnmpGet(c *gin.Context) {
	var req SnmpGetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.SnmpGet(c, &req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// POST /v1/snmp/set — trigger SNMP SET
func (h *Handler) SnmpSet(c *gin.Context) {
	var req SnmpSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	if err := h.svc.SnmpSet(c, &req); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// GET /v1/snmp/operation-logs — list SNMP operation logs with pagination
func (h *Handler) ListSnmpOperationLogs(c *gin.Context) {
	offset, limit := parseOffsetLimit(c)

	data, total, err := h.svc.ListSnmpOperationLogs(c, offset, limit)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	h.offsetResponse(c, data, total, offset, limit, buildBaseUrl(c))
}
