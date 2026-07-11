package device

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for device and device-group endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a Handler backed by a fresh Service.
// The *gorm.DB is forwarded via dependency injection to avoid a circular
// import with pkg/database.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// ---------------------------------------------------------------------------
// Device endpoints
// ---------------------------------------------------------------------------

// ListDevices godoc
// @Summary List devices
// @Description Returns a paginated list of devices for the authenticated tenant
// @Tags devices
// @Accept json
// @Produce json
// @Param page query int false "Page number (default 1)"
// @Param pageSize query int false "Page size (default 20)"
// @Param keyword query string false "Search keyword (matches device_name or serial_number)"
// @Security BearerAuth
// @Success 200 {object} utils.PaginatedResponse
// @Failure 500 {object} utils.Response
// @Router /devices [get]
func (h *Handler) ListDevices(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	keyword := c.Query("keyword")
	licenseId := middleware.GetLicenseId(c)

	data, total, err := h.svc.ListDevices(licenseId, keyword, page, pageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// GetDevice godoc
// @Summary Get device by ID
// @Description Returns a single device by its primary key
// @Tags devices
// @Accept json
// @Produce json
// @Param id path int true "Device ID"
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /devices/{id} [get]
func (h *Handler) GetDevice(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid device id")
		return
	}

	elem, err := h.svc.GetDevice(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, elem)
}

// CreateDevice godoc
// @Summary Create a device
// @Description Creates a new device record
// @Tags devices
// @Accept json
// @Produce json
// @Param device body CpeElement true "Device data"
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Router /devices [post]
func (h *Handler) CreateDevice(c *gin.Context) {
	var elem CpeElement
	if err := c.ShouldBindJSON(&elem); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateDevice(&elem); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, elem)
}

// UpdateDevice godoc
// @Summary Update a device
// @Description Updates an existing device record
// @Tags devices
// @Accept json
// @Produce json
// @Param id path int true "Device ID"
// @Param device body CpeElement true "Device data"
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /devices/{id} [put]
func (h *Handler) UpdateDevice(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid device id")
		return
	}

	var elem CpeElement
	if err := c.ShouldBindJSON(&elem); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	elem.NeNeid = id

	if err := h.svc.UpdateDevice(&elem); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, elem)
}

// DeleteDevice godoc
// @Summary Delete a device
// @Description Soft-deletes a device (sets deleted=true)
// @Tags devices
// @Accept json
// @Produce json
// @Param id path int true "Device ID"
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /devices/{id} [delete]
func (h *Handler) DeleteDevice(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid device id")
		return
	}

	if err := h.svc.DeleteDevice(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// DeviceGroup endpoints
// ---------------------------------------------------------------------------

// ListGroups handles GET /device-groups
func (h *Handler) ListGroups(c *gin.Context) {
	licenseId := middleware.GetLicenseId(c)

	groups, err := h.svc.ListGroups(licenseId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list groups")
		return
	}
	utils.Success(c, groups)
}

// CreateGroup handles POST /device-groups
func (h *Handler) CreateGroup(c *gin.Context) {
	var g DeviceGroup
	if err := c.ShouldBindJSON(&g); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateGroup(&g); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create group")
		return
	}
	utils.Success(c, g)
}

// UpdateGroup handles PUT /device-groups/:id
func (h *Handler) UpdateGroup(c *gin.Context) {
	id := c.Param("id")

	var g DeviceGroup
	if err := c.ShouldBindJSON(&g); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	g.Id = id

	if err := h.svc.UpdateGroup(&g); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to update group")
		return
	}
	utils.Success(c, g)
}

// DeleteGroup handles DELETE /device-groups/:id
func (h *Handler) DeleteGroup(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.DeleteGroup(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete group")
		return
	}
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// Device Import
// ---------------------------------------------------------------------------

// ImportDevices godoc
// @Summary Import devices from Excel
// @Description Upload an Excel file to batch-import devices. Supports Add/Modify/Delete operations per row.
// @Tags devices
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Excel file (.xlsx)"
// @Param deviceGroupId formData string false "Target device group ID"
// @Param deviceType formData string false "Device type (gnb/enb/cpe)" Enums(gnb, enb, cpe)
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Router /devices/import [post]
func (h *Handler) ImportDevices(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	deviceGroupId := c.PostForm("deviceGroupId")
	deviceType := c.DefaultPostForm("deviceType", "gnb")
	licenseId := middleware.GetLicenseId(c)

	rows, err := ParseImportExcel(file)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if len(rows) == 0 {
		utils.Error(c, http.StatusBadRequest, "no valid device data found in file")
		return
	}

	result, err := h.svc.ImportDevices(rows, deviceType, deviceGroupId, licenseId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.Success(c, result)
}
