package cacert

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler provides HTTP handlers for CA certificate endpoints
type Handler struct {
	svc Service
}

// NewHandler creates a new Handler
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(NewRepository(db))}
}

// ---------- CaFile endpoints ----------

// ListCaFiles handles POST /api/v1/caFile/list
func (h *Handler) ListCaFiles(c *gin.Context) {
	var req CaFileListQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	ctx := context.Background()
	data, total, err := h.svc.ListCaFiles(ctx, req.Page, req.PageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Paginated(c, data, total, req.Page, req.PageSize)
}

// DeleteCaFile handles POST /api/v1/caFile/delete
func (h *Handler) DeleteCaFile(c *gin.Context) {
	var req CaFileDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	ctx := context.Background()
	if err := h.svc.DeleteCaFiles(ctx, []int{req.Id}); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// QueryCaList handles POST /api/v1/caFile/queryCaList
func (h *Handler) QueryCaList(c *gin.Context) {
	ctx := context.Background()
	data, err := h.svc.ListAllCaFiles(ctx)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, data)
}

// UploadCaFile handles POST /api/v1/caFile/upload
func (h *Handler) UploadCaFile(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		utils.Error(c, 400, "failed to parse multipart form")
		return
	}

	files := form.File["files"]
	description := ""
	if descs := form.Value["description"]; len(descs) > 0 {
		description = descs[0]
	}

	if len(files) == 0 {
		utils.Error(c, 400, "no files uploaded")
		return
	}

	// Validate file names
	validFileNames := map[string]bool{
		"H_Auth.cer":               true,
		"TMO_ISSUING_CA_NMS.pem":   true,
		"TMO_ISSUING_CA_SEGW.pem":  true,
		"public_key.pem":           true,
	}
	forbiddenFileNames := map[string]bool{
		"HARMAN_ROOT_CA.pem":    true,
		"TMO_ROOT_CA_NMS.pem":   true,
		"TMO_ROOT_CA_SEGW.pem":  true,
	}

	for _, file := range files {
		if forbiddenFileNames[file.Filename] {
			utils.Error(c, 10322, fmt.Sprintf("%s cannot be updated", file.Filename))
			return
		}
		if !validFileNames[file.Filename] {
			validList := []string{"H_Auth.cer", "TMO_ISSUING_CA_NMS.pem", "TMO_ISSUING_CA_SEGW.pem", "public_key.pem"}
			utils.Error(c, 10322, fmt.Sprintf("Certs that support replacement: %v", validList))
			return
		}
	}

	// Generate zip filename
	timestamp := time.Now().Format("20060102150405")
	randomNum := time.Now().UnixNano() % 10000
	zipFileName := fmt.Sprintf("%s%d.zip", timestamp, randomNum)

	// Save files and create zip
	caFilePath := h.svc.GetCaFilePath()
	if err := os.MkdirAll(caFilePath, 0755); err != nil {
		logger.Errorf("Failed to create CA file directory: %v", err)
		utils.HandleError(c, apperror.Wrap(err, "CA_DIR_CREATE_FAILED", 500, "failed to create directory"))
		return
	}

	zipPath := filepath.Join(caFilePath, zipFileName)
	if err := h.createZipFile(zipPath, files); err != nil {
		logger.Errorf("Failed to create zip file: %v", err)
		utils.HandleError(c, apperror.Wrap(err, "CA_ZIP_CREATE_FAILED", 500, "failed to save file"))
		return
	}

	// Create database record
	username := middleware.GetUsername(c)
	if err := h.svc.CreateCaFileRecord(context.Background(), zipFileName, zipPath, description, username); err != nil {
		logger.Errorf("Failed to save CA file record: %v", err)
		utils.HandleError(c, apperror.Wrap(err, "CA_FILE_RECORD_CREATE_FAILED", 500, "failed to save file record"))
		return
	}

	utils.Success(c, gin.H{
		"fileName": zipFileName,
		"url":      zipPath,
	})
}

// DownloadCaFile handles GET /acs-file-server/ca/downloadFile/:fileId
func (h *Handler) DownloadCaFile(c *gin.Context) {
	fileIdStr := c.Param("fileId")
	fileId, err := strconv.Atoi(fileIdStr)
	if err != nil {
		utils.Error(c, 400, "invalid file ID")
		return
	}

	ctx := context.Background()
	caFile, err := h.svc.GetCaFileByID(ctx, fileId)
	if err != nil {
		utils.HandleError(c, apperror.ErrNotFound.WithMessage("CA file not found"))
		return
	}

	filePath := strOrEmpty(caFile.URL)
	if filePath == "" {
		utils.HandleError(c, apperror.ErrNotFound.WithMessage("file path not available"))
		return
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		utils.HandleError(c, apperror.ErrNotFound.WithMessage(fmt.Sprintf("CA Task file %s not found", strOrEmpty(caFile.FileName))))
		return
	}

	c.File(filePath)
}

// ---------- CaTask endpoints ----------

// SaveCaTask handles POST /api/v1/catask/save
func (h *Handler) SaveCaTask(c *gin.Context) {
	var req CaTaskSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	username := middleware.GetUsername(c)
	licenseId := middleware.GetLicenseId(c)
	ctx := context.Background()

	if err := h.svc.SaveCaTask(ctx, req.TaskName, req.CaFileId, req.Scope, req.DeviceIds, req.GroupIds, username, licenseId); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// ListCaTasks handles POST /api/v1/catask/list
func (h *Handler) ListCaTasks(c *gin.Context) {
	var req CaTaskListQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	// Get tenancy ID from JWT context
	tenancyId := middleware.GetLicenseId(c)
	var tenancyIdPtr *int
	if tenancyId > 0 {
		tenancyIdPtr = &tenancyId
	}

	ctx := context.Background()
	data, total, err := h.svc.ListCaTasks(ctx, req.Page, req.PageSize, tenancyIdPtr)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Paginated(c, data, total, req.Page, req.PageSize)
}

// GetCaTaskDetail handles POST /api/v1/catask/detail
func (h *Handler) GetCaTaskDetail(c *gin.Context) {
	var req CaTaskDetailQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	ctx := context.Background()
	data, err := h.svc.GetCaTaskDetail(ctx, req.Id)
	if err != nil {
		utils.HandleError(c, apperror.ErrNotFound.WithMessage("task not found"))
		return
	}

	utils.Success(c, data)
}

// DeleteCaTask handles POST /api/v1/catask/delete
func (h *Handler) DeleteCaTask(c *gin.Context) {
	var req CaTaskDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	ctx := context.Background()
	if err := h.svc.DeleteCaTasks(ctx, []int{req.Id}); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, nil)
}

// QueryDeviceSendCaLog handles POST /api/v1/catask/queryDeviceSendCaLog
func (h *Handler) QueryDeviceSendCaLog(c *gin.Context) {
	var req DeviceCaLogQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	ctx := context.Background()
	data, total, err := h.svc.ListDeviceSendCaLogs(ctx, req.TaskId, req.Page, req.PageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Paginated(c, data, total, req.Page, req.PageSize)
}

// ---------- Helper functions ----------

// createZipFile creates a zip archive from uploaded files
func (h *Handler) createZipFile(zipPath string, files []*multipart.FileHeader) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, fileHeader := range files {
		srcFile, err := fileHeader.Open()
		if err != nil {
			return err
		}

		zipEntry, err := zipWriter.Create(fileHeader.Filename)
		if err != nil {
			srcFile.Close()
			return err
		}

		if _, err := io.Copy(zipEntry, srcFile); err != nil {
			srcFile.Close()
			return err
		}
		srcFile.Close()
	}

	return nil
}
