package pmfile

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"nmsappsrv/internal/mq"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const defaultPMFileDir = "./data/pm-files"

// Handler exposes HTTP handlers for PM file endpoints.
type Handler struct {
	db      *gorm.DB
	fileDir string
}

// NewHandler creates a Handler. It ensures the file upload directory exists.
func NewHandler(db *gorm.DB) *Handler {
	dir := defaultPMFileDir
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Errorf("pmfile: failed to create file dir %s: %v", dir, err)
	}
	return &Handler{db: db, fileDir: dir}
}

// Upload handles POST /pm-files/upload (multipart form).
// It saves the file to disk, creates a PMFile record, and enqueues a message
// to the queue:pm Redis queue for background processing.
func (h *Handler) Upload(c *gin.Context) {
	var req UploadRequest
	if err := c.ShouldBind(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: elementId is required")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "file field is required")
		return
	}

	// Determine file type from extension
	ext := filepath.Ext(file.Filename)
	fileType := "unknown"
	switch ext {
	case ".xml":
		fileType = "xml"
	case ".csv":
		fileType = "csv"
	}

	// Build the storage path
	ts := time.Now().Format("20060102150405")
	storedName := fmt.Sprintf("%d_%s_%s", req.ElementId, ts, file.Filename)
	storedPath := filepath.Join(h.fileDir, storedName)

	if err := c.SaveUploadedFile(file, storedPath); err != nil {
		utils.Error(c, http.StatusInternalServerError, fmt.Sprintf("failed to save file: %v", err))
		return
	}

	// Create the database record
	pmFile := &PMFile{
		ElementId:  req.ElementId,
		FileName:   file.Filename,
		FilePath:   storedPath,
		FileSize:   file.Size,
		FileType:   fileType,
		Status:     StatusUploaded,
		CreateTime: time.Now(),
	}
	if err := h.db.Create(pmFile).Error; err != nil {
		// Clean up the saved file
		_ = os.Remove(storedPath)
		utils.Error(c, http.StatusInternalServerError, fmt.Sprintf("failed to create record: %v", err))
		return
	}

	// Enqueue for background processing
	msg := PMQueueMessage{
		PMFileId:  pmFile.Id,
		ElementId: req.ElementId,
		FileName:  file.Filename,
		FilePath:  storedPath,
	}
	if err := mq.Enqueue(context.Background(), mq.PMQueue, msg); err != nil {
		logger.Errorf("pmfile: failed to enqueue file %d for processing: %v", pmFile.Id, err)
		// Not fatal: the file is saved and can be re-queued manually
	}

	utils.Success(c, pmFile)
}

// ListFiles handles GET /pm-files
func (h *Handler) ListFiles(c *gin.Context) {
	var files []PMFile
	if err := h.db.Order("create_time DESC").Find(&files).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, files)
}

// GetFile handles GET /pm-files/:id
func (h *Handler) GetFile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	var file PMFile
	if err := h.db.First(&file, id).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "file not found")
		return
	}

	utils.Success(c, file)
}

// DownloadFile handles GET /pm-files/:id/download
func (h *Handler) DownloadFile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	var file PMFile
	if err := h.db.First(&file, id).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "file not found")
		return
	}

	if _, statErr := os.Stat(file.FilePath); statErr != nil {
		utils.Error(c, http.StatusNotFound, "file not found on disk")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", file.FileName))
	c.File(file.FilePath)
}

// DeleteFile handles DELETE /pm-files/:id
func (h *Handler) DeleteFile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	var file PMFile
	if err := h.db.First(&file, id).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "file not found")
		return
	}

	// Refuse to delete while processing
	if file.Status == StatusProcessing {
		utils.Error(c, http.StatusConflict, "cannot delete file while it is being processed")
		return
	}

	// Remove the file from disk
	if file.FilePath != "" {
		_ = os.Remove(file.FilePath)
	}

	// Delete associated KPI measurements
	h.db.Where("pm_file_id = ?", id).Delete(&PMKPIMeasurement{})

	// Delete the file record
	if err := h.db.Delete(&PMFile{}, id).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.OK(c, "deleted")
}
