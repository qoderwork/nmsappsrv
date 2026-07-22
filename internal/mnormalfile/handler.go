package mnormalfile

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/config"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for MNormal file management endpoints.
type Handler struct {
	svc       Service
	db        *gorm.DB
	opSender  OperationSender
}

// NewHandler creates a Handler backed by a fresh Service.
// The opSender is injected from main to avoid a direct import of tr069.
func NewHandler(db *gorm.DB, cfg *config.Config, opSender OperationSender) *Handler {
	return &Handler{svc: NewService(db, cfg), db: db, opSender: opSender}
}

func (h *Handler) InitUpload(c *gin.Context) {
	var req InitUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	tenantId := middleware.GetTenantId(c)
	username := middleware.GetUsername(c)
	resp, err := h.svc.InitUpload(tenantId, username, &req)
	if err != nil {
		logger.Errorf("InitUpload error: %v", err)
		utils.Error(c, http.StatusInternalServerError, "failed to init upload")
		return
	}
	utils.Success(c, resp)
}

func (h *Handler) UploadChunk(c *gin.Context) {
	var req UploadChunkRequest
	if err := c.ShouldBind(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request parameters")
		return
	}
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	if err := h.svc.UploadChunk(req.FileId, req.ChunkIndex, req.ChunkMd5, file); err != nil {
		logger.Errorf("UploadChunk error: %v", err)
		utils.Error(c, http.StatusInternalServerError, "failed to upload chunk")
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) CheckChunk(c *gin.Context) {
	var req CheckChunkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	uploaded, err := h.svc.CheckChunk(req.FileId, req.ChunkIndex)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to check chunk")
		return
	}
	utils.Success(c, &CheckChunkResponse{Uploaded: uploaded})
}

func (h *Handler) Assemble(c *gin.Context) {
	var req AssembleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.Assemble(req.FileId); err != nil {
		logger.Errorf("Assemble error: %v", err)
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	tenantId := middleware.GetTenantId(c)
	username := middleware.GetUsername(c)

	fileId, err := h.svc.Upload(tenantId, username, header.Filename, file, header.Size)
	if err != nil {
		logger.Errorf("Upload error: %v", err)
		utils.Error(c, http.StatusInternalServerError, "failed to upload file")
		return
	}
	utils.Success(c, map[string]string{"fileId": fileId})
}

func (h *Handler) Delete(c *gin.Context) {
	var req DeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.Delete(req.FileId); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete file")
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) List(c *gin.Context) {
	var req ListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	tenantId := middleware.GetTenantId(c)
	data, total, err := h.svc.List(tenantId, &req)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list files")
		return
	}
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	utils.Paginated(c, data, total, page, pageSize)
}

func (h *Handler) Detail(c *gin.Context) {
	var req DetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	data, err := h.svc.Detail(req.FileId)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to get file detail")
		return
	}
	utils.Success(c, data)
}

func (h *Handler) DownloadToDevice(c *gin.Context) {
	var req DownloadToDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.DownloadToDevice(req.FileId, req.ElementIds, h.opSender); err != nil {
		logger.Errorf("DownloadToDevice error: %v", err)
		utils.Error(c, http.StatusInternalServerError, "failed to download to device")
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) DownloadResults(c *gin.Context) {
	var req DownloadResultsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Fall back to query params
		fileId := c.Query("fileId")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
		data, total, err := h.svc.DownloadResults(fileId, page, pageSize)
		if err != nil {
			utils.Error(c, http.StatusInternalServerError, "failed to get download results")
			return
		}
		utils.Paginated(c, data, total, page, pageSize)
		return
	}
	data, total, err := h.svc.DownloadResults(req.FileId, req.Page, req.PageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to get download results")
		return
	}
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	utils.Paginated(c, data, total, page, pageSize)
}
