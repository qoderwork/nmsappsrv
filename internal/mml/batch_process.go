package mml

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Models
// ---------------------------------------------------------------------------

// BatchProcessFile represents an uploaded MML script file for batch processing.
type BatchProcessFile struct {
	Id         int        `gorm:"primaryKey;autoIncrement" json:"id"`
	FileName   *string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	FilePath   *string    `gorm:"column:file_path;type:varchar(500)" json:"file_path"`
	FileSize   *int64     `gorm:"column:file_size" json:"file_size"`
	Status     int        `gorm:"column:status" json:"status"` // 0=uploaded, 1=sending, 2=sent, 3=completed, 4=failed
	UploadUser *string    `gorm:"column:upload_user;type:varchar(255)" json:"upload_user"`
	TenantId  *int       `gorm:"column:tenant_id" json:"tenant_id"`
	UploadTime *time.Time `gorm:"column:upload_time" json:"upload_time"`
	UpdateTime *time.Time `gorm:"column:update_time" json:"update_time"`
}

func (BatchProcessFile) TableName() string { return "batch_process_file" }

// BatchProcessLog represents an execution log entry for a batch process file.
type BatchProcessLog struct {
	Id          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	BatchFileId *int       `gorm:"column:batch_file_id" json:"batch_file_id"`
	ElementId   *int64     `gorm:"column:element_id" json:"element_id"`
	Command     *string    `gorm:"column:command;type:varchar(500)" json:"command"`
	Status      int        `gorm:"column:status" json:"status"` // 0=pending, 1=executing, 2=success, 3=failed
	Result      *string    `gorm:"column:result;type:longtext" json:"result"`
	FaultString *string    `gorm:"column:fault_string;type:text" json:"fault_string"`
	SendTime    *time.Time `gorm:"column:send_time" json:"send_time"`
	ResultTime  *time.Time `gorm:"column:result_time" json:"result_time"`
}

func (BatchProcessLog) TableName() string { return "batch_process_log" }

// ---------------------------------------------------------------------------
// Repository methods (added to the existing repository struct)
// ---------------------------------------------------------------------------

// FindBatchProcessFiles returns all batch process files for the given license.
func (r *repository) FindBatchProcessFiles(tenantId int) ([]BatchProcessFile, error) {
	var files []BatchProcessFile
	query := r.db.Model(&BatchProcessFile{})
	if tenantId > 0 {
		query = query.Where("tenant_id = ?", tenantId)
	}
	if err := query.Order("upload_time DESC").
		Find(&files).Error; err != nil {
		logger.Errorf("FindBatchProcessFiles error: %v", err)
		return nil, err
	}
	return files, nil
}

// FindBatchProcessFileByID returns a single batch process file by ID.
func (r *repository) FindBatchProcessFileByID(id int) (*BatchProcessFile, error) {
	var file BatchProcessFile
	if err := r.db.Where("id = ?", id).First(&file).Error; err != nil {
		return nil, err
	}
	return &file, nil
}

// CreateBatchProcessFile inserts a new batch process file record.
func (r *repository) CreateBatchProcessFile(file *BatchProcessFile) error {
	return r.db.Create(file).Error
}

// UpdateBatchProcessFile saves changes to an existing batch process file.
func (r *repository) UpdateBatchProcessFile(file *BatchProcessFile) error {
	return r.db.Save(file).Error
}

// DeleteBatchProcessFile removes a batch process file by ID.
func (r *repository) DeleteBatchProcessFile(id int) error {
	return r.db.Where("id = ?", id).Delete(&BatchProcessFile{}).Error
}

// FindBatchProcessLogs returns all logs for a given batch file.
func (r *repository) FindBatchProcessLogs(batchFileId int) ([]BatchProcessLog, error) {
	var logs []BatchProcessLog
	if err := r.db.Where("batch_file_id = ?", batchFileId).
		Order("id ASC").
		Find(&logs).Error; err != nil {
		logger.Errorf("FindBatchProcessLogs error: %v", err)
		return nil, err
	}
	return logs, nil
}

// CreateBatchProcessLog inserts a new batch process log entry.
func (r *repository) CreateBatchProcessLog(log *BatchProcessLog) error {
	return r.db.Create(log).Error
}

// FindBatchProcessExecuteResults returns the execution results associated
// with a batch process file by collecting element IDs from its logs.
func (r *repository) FindBatchProcessExecuteResults(batchFileId int) ([]MmlExecuteResult, error) {
	var logs []BatchProcessLog
	if err := r.db.Where("batch_file_id = ?", batchFileId).Find(&logs).Error; err != nil {
		logger.Errorf("FindBatchProcessExecuteResults error: %v", err)
		return nil, err
	}
	if len(logs) == 0 {
		return []MmlExecuteResult{}, nil
	}

	elemSet := make(map[int64]bool)
	var elementIds []int64
	for _, log := range logs {
		if log.ElementId != nil && !elemSet[*log.ElementId] {
			elemSet[*log.ElementId] = true
			elementIds = append(elementIds, *log.ElementId)
		}
	}
	if len(elementIds) == 0 {
		return []MmlExecuteResult{}, nil
	}

	var results []MmlExecuteResult
	if err := r.db.Where("element_id IN ?", elementIds).
		Order("id DESC").
		Find(&results).Error; err != nil {
		logger.Errorf("FindBatchProcessExecuteResults query error: %v", err)
		return nil, err
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Service interface extension
// ---------------------------------------------------------------------------

// BatchProcessService defines the business-logic contract for MML batch
// processing operations.
type BatchProcessService interface {
	UploadBatchProcessFile(fileName, filePath string, fileSize int64, username string, tenantId int) (*BatchProcessFile, error)
	ListBatchProcessFiles(tenantId int) ([]BatchProcessFile, error)
	SendBatchProcessFile(id int, tenantId int) (*BatchProcessFile, error)
	CheckBatchProcessFile(id int) (*BatchProcessFile, error)
	ListBatchProcessLogs(batchFileId int) ([]BatchProcessLog, error)
	ListBatchExecuteResults(batchFileId int) ([]MmlExecuteResult, error)
	GetBatchProcessFile(id int) (*BatchProcessFile, error)
	DeleteBatchProcessFile(id int) error
}

const uploadBatchProcessDir = "/data/uploads/batch_process"

func (s *service) UploadBatchProcessFile(fileName, filePath string, fileSize int64, username string, tenantId int) (*BatchProcessFile, error) {
	now := time.Now()
	file := &BatchProcessFile{FileName: &fileName, FilePath: &filePath, FileSize: &fileSize, Status: 0, UploadUser: &username, TenantId: &tenantId, UploadTime: &now, UpdateTime: &now}
	if err := s.repo.CreateBatchProcessFile(file); err != nil {
		return nil, apperror.Wrap(err, "UPLOAD_BATCH_PROCESS_FAILED", 500, "failed to save batch process file record")
	}
	return file, nil
}

func (s *service) ListBatchProcessFiles(tenantId int) ([]BatchProcessFile, error) {
	data, err := s.repo.FindBatchProcessFiles(tenantId)
	if err != nil {
		return nil, apperror.Wrap(err, "LIST_BATCH_PROCESS_FILES_FAILED", 500, "failed to list batch process files")
	}
	return data, nil
}

func (s *service) SendBatchProcessFile(id int, tenantId int) (*BatchProcessFile, error) {
	file, err := s.repo.FindBatchProcessFileByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperror.ErrNotFound.WithMessage("batch process file not found")
		}
		return nil, apperror.Wrap(err, "SEND_BATCH_PROCESS_FAILED", 500, "failed to find batch process file")
	}
	now := time.Now()
	file.Status = 1
	file.UpdateTime = &now
	if err := s.repo.UpdateBatchProcessFile(file); err != nil {
		return nil, apperror.Wrap(err, "SEND_BATCH_PROCESS_FAILED", 500, "failed to update status")
	}
	if file.FilePath == nil {
		return nil, apperror.ErrInvalidInput.WithMessage("batch process file has no path")
	}
	content, err := os.ReadFile(*file.FilePath)
	if err != nil {
		file.Status = 4
		file.UpdateTime = &now
		_ = s.repo.UpdateBatchProcessFile(file)
		return nil, apperror.Wrap(err, "SEND_BATCH_PROCESS_FAILED", 500, "failed to read batch process file")
	}
	cmdLines := splitBatchLines(string(content))
	for _, line := range cmdLines {
		if line == "" {
			continue
		}
		cmd := line
		logEntry := &BatchProcessLog{BatchFileId: &id, Command: &cmd, Status: 0, SendTime: &now}
		if createErr := s.repo.CreateBatchProcessLog(logEntry); createErr != nil {
			logger.Errorf("SendBatchProcessFile: failed to create log: %v", createErr)
		}
	}
	file.Status = 2
	file.UpdateTime = &now
	if err := s.repo.UpdateBatchProcessFile(file); err != nil {
		logger.Errorf("SendBatchProcessFile: failed to update status: %v", err)
	}
	return file, nil
}

func (s *service) CheckBatchProcessFile(id int) (*BatchProcessFile, error) {
	file, err := s.repo.FindBatchProcessFileByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperror.ErrNotFound.WithMessage("batch process file not found")
		}
		return nil, apperror.Wrap(err, "CHECK_BATCH_PROCESS_FAILED", 500, "failed to find batch process file")
	}
	logs, err := s.repo.FindBatchProcessLogs(id)
	if err != nil {
		return nil, apperror.Wrap(err, "CHECK_BATCH_PROCESS_FAILED", 500, "failed to query logs")
	}
	allCompleted := true
	hasFailure := false
	for _, log := range logs {
		if log.Status == 0 || log.Status == 1 {
			allCompleted = false
			break
		}
		if log.Status == 3 {
			hasFailure = true
		}
	}
	now := time.Now()
	if allCompleted && len(logs) > 0 {
		if hasFailure {
			file.Status = 4
		} else {
			file.Status = 3
		}
	}
	file.UpdateTime = &now
	if err := s.repo.UpdateBatchProcessFile(file); err != nil {
		logger.Errorf("CheckBatchProcessFile: failed to update status: %v", err)
	}
	return file, nil
}

func (s *service) ListBatchProcessLogs(batchFileId int) ([]BatchProcessLog, error) {
	data, err := s.repo.FindBatchProcessLogs(batchFileId)
	if err != nil {
		return nil, apperror.Wrap(err, "LIST_BATCH_PROCESS_LOGS_FAILED", 500, "failed to list batch process logs")
	}
	return data, nil
}

func (s *service) ListBatchExecuteResults(batchFileId int) ([]MmlExecuteResult, error) {
	data, err := s.repo.FindBatchProcessExecuteResults(batchFileId)
	if err != nil {
		return nil, apperror.Wrap(err, "LIST_BATCH_EXECUTE_RESULTS_FAILED", 500, "failed to list batch execute results")
	}
	return data, nil
}

func (s *service) GetBatchProcessFile(id int) (*BatchProcessFile, error) {
	file, err := s.repo.FindBatchProcessFileByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperror.ErrNotFound.WithMessage("batch process file not found")
		}
		return nil, apperror.Wrap(err, "GET_BATCH_PROCESS_FILE_FAILED", 500, "failed to get batch process file")
	}
	return file, nil
}

func (s *service) DeleteBatchProcessFile(id int) error {
	if delErr := s.repo.(*repository).db.Where("batch_file_id = ?", id).Delete(&BatchProcessLog{}).Error; delErr != nil {
		logger.Errorf("DeleteBatchProcessFile: failed to delete logs: %v", delErr)
	}
	if err := s.repo.DeleteBatchProcessFile(id); err != nil {
		return apperror.Wrap(err, "DELETE_BATCH_PROCESS_FILE_FAILED", 500, "failed to delete batch process file")
	}
	return nil
}

func splitBatchLines(text string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == 10 {
			line := text[start:i]
			if len(line) > 0 && line[len(line)-1] == 13 {
				line = line[:len(line)-1]
			}
			trimmed := trimBatchSpace(line)
			if trimmed != "" {
				lines = append(lines, trimmed)
			}
			start = i + 1
		}
	}
	if start < len(text) {
		line := text[start:]
		if len(line) > 0 && line[len(line)-1] == 13 {
			line = line[:len(line)-1]
		}
		trimmed := trimBatchSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func trimBatchSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == 32 || s[start] == 9) {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == 32 || s[end-1] == 9) {
		end--
	}
	return s[start:end]
}

// ---------------------------------------------------------------------------
// Handler methods
// ---------------------------------------------------------------------------

func (h *Handler) UploadBatchProcessFile(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "file is required")
		return
	}
	if err := os.MkdirAll(uploadBatchProcessDir, 0755); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create upload directory")
		return
	}
	fileName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), fileHeader.Filename)
	filePath := filepath.Join(uploadBatchProcessDir, fileName)
	if err := c.SaveUploadedFile(fileHeader, filePath); err != nil {
		logger.Errorf("UploadBatchProcessFile: failed to save file: %v", err)
		utils.Error(c, http.StatusInternalServerError, "failed to save uploaded file")
		return
	}
	username := middleware.GetUsername(c)
	tenantId := middleware.GetTenantId(c)
	result, err := h.svc.UploadBatchProcessFile(fileHeader.Filename, filePath, fileHeader.Size, username, tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, result)
}

func (h *Handler) ListBatchProcessFiles(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	data, err := h.svc.ListBatchProcessFiles(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

func (h *Handler) SendBatchProcessFile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid batch process file id")
		return
	}
	tenantId := middleware.GetTenantId(c)
	result, err := h.svc.SendBatchProcessFile(id, tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, result)
}

func (h *Handler) CheckBatchProcessFile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid batch process file id")
		return
	}
	result, err := h.svc.CheckBatchProcessFile(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, result)
}

func (h *Handler) ListBatchProcessLogs(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid batch process file id")
		return
	}
	data, err := h.svc.ListBatchProcessLogs(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

func (h *Handler) ListBatchExecuteResults(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid batch process file id")
		return
	}
	data, err := h.svc.ListBatchExecuteResults(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

func (h *Handler) DownloadBatchProcessFile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid batch process file id")
		return
	}
	file, err := h.svc.GetBatchProcessFile(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	if file.FilePath == nil {
		utils.Error(c, http.StatusNotFound, "file path not found")
		return
	}
	fileName := "batch_process_file"
	if file.FileName != nil {
		fileName = *file.FileName
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	c.File(*file.FilePath)
}

func (h *Handler) DeleteBatchProcessFile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid batch process file id")
		return
	}
	if err := h.svc.DeleteBatchProcessFile(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

func (h *Handler) DownloadExecuteResultFile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid batch process file id")
		return
	}
	results, err := h.svc.ListBatchExecuteResults(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	var content string
	content += fmt.Sprintf("Batch Process Execute Results (ID: %d)\n", id)
	content += fmt.Sprintf("Generated: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	content += "========================================\n\n"
	for i, r := range results {
		content += fmt.Sprintf("--- Result %d ---\n", i+1)
		if r.Command != nil {
			content += fmt.Sprintf("Command: %s\n", *r.Command)
		}
		content += fmt.Sprintf("Status: %d\n", r.Status)
		if r.Result != nil {
			content += fmt.Sprintf("Result: %s\n", *r.Result)
		}
		if r.FaultString != nil {
			content += fmt.Sprintf("Fault: %s\n", *r.FaultString)
		}
		if r.OperationTime != nil {
			content += fmt.Sprintf("Time: %s\n", r.OperationTime.Format("2006-01-02 15:04:05"))
		}
		content += "\n"
	}
	fileName := fmt.Sprintf("batch_result_%d.txt", id)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(content))
}

// ---------------------------------------------------------------------------
// Route registration
// ---------------------------------------------------------------------------

// RegisterBatchProcessRoutes registers batch process routes on the given router group.
func RegisterBatchProcessRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/batch-process/upload", h.UploadBatchProcessFile)
	rg.GET("/batch-process/files", h.ListBatchProcessFiles)
	rg.POST("/batch-process/send/:id", h.SendBatchProcessFile)
	rg.POST("/batch-process/check/:id", h.CheckBatchProcessFile)
	rg.GET("/batch-process/logs/:id", h.ListBatchProcessLogs)
	rg.GET("/batch-process/results/:id", h.ListBatchExecuteResults)
	rg.GET("/batch-process/download/:id", h.DownloadBatchProcessFile)
	rg.DELETE("/batch-process/:id", h.DeleteBatchProcessFile)
	rg.GET("/batch-process/results/:id/download", h.DownloadExecuteResultFile)
}
