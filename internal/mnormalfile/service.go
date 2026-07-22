package mnormalfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"nmsappsrv/internal/config"
	"nmsappsrv/internal/device"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Service defines the business-logic contract for MNormal file operations.
type Service interface {
	InitUpload(tenantId int, username string, req *InitUploadRequest) (*InitUploadResponse, error)
	UploadChunk(fileId string, chunkIndex int, chunkMd5 string, data io.Reader) error
	CheckChunk(fileId string, chunkIndex int) (bool, error)
	Assemble(fileId string) error
	Upload(tenantId int, username string, fileName string, data io.Reader, size int64) (string, error)
	Delete(fileId string) error
	List(tenantId int, req *ListRequest) ([]MNormalFile, int64, error)
	Detail(fileId string) (*MNormalFile, error)
	DownloadToDevice(fileId string, elementIds []int64, opSender OperationSender) error
	DownloadResults(fileId string, page, pageSize int) ([]MNormalFileDownloadLog, int64, error)
}

// OperationSender is a minimal interface for sending TR-069 Download commands.
// This avoids a direct import of tr069.OperationSender (which would create a
// cycle when the handler is wired from main.go).
type OperationSender interface {
	SendDownload(sn string, dl *soap.Download, operationId string) error
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
	cfg  *config.Config
	db   *gorm.DB
}

// NewService creates a Service backed by the given repository and config.
func NewService(db *gorm.DB, cfg *config.Config) Service {
	return &service{repo: NewRepository(db), cfg: cfg, db: db}
}

// baseDir returns the root directory for MNormal file storage.
func (s *service) baseDir() string {
	dir := s.cfg.FileServer.DeviceMNormalDir
	if dir == "" {
		dir = filepath.Join(s.cfg.FileServer.Root, "mnormal")
	}
	return dir
}

// chunkDir returns the temp directory for chunk storage of a given fileId.
func (s *service) chunkDir(fileId string) string {
	return filepath.Join(s.baseDir(), "chunks", fileId)
}

// assembledPath returns the path for the assembled file.
func (s *service) assembledPath(fileId string) string {
	return filepath.Join(s.baseDir(), fileId)
}

// InitUpload creates a new MNormal file record and returns the fileId.
func (s *service) InitUpload(tenantId int, username string, req *InitUploadRequest) (*InitUploadResponse, error) {
	fileId := uuid.New().String()
	now := time.Now()
	status := 1 // uploading
	chunkCount := req.ChunkCount
	f := &MNormalFile{
		FileId:     fileId,
		FileName:   &req.FileName,
		FileSize:   &req.FileSize,
		FileMd5:    &req.FileMd5,
		ChunkCount: &chunkCount,
		Status:     &status,
		TenantId:  &tenantId,
		CreateTime: &now,
		UpdateTime: &now,
		Username:   &username,
	}
	if err := s.repo.CreateFile(f); err != nil {
		return nil, err
	}
	return &InitUploadResponse{FileId: fileId}, nil
}

// UploadChunk saves a chunk to the temp directory.
func (s *service) UploadChunk(fileId string, chunkIndex int, chunkMd5 string, data io.Reader) error {
	// Verify the file exists
	_, err := s.repo.FindFileByFileId(fileId)
	if err != nil {
		return fmt.Errorf("file not found: %s", fileId)
	}

	dir := s.chunkDir(fileId)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create chunk directory: %w", err)
	}

	chunkPath := filepath.Join(dir, fmt.Sprintf("%d", chunkIndex))
	f, err := os.Create(chunkPath)
	if err != nil {
		return fmt.Errorf("failed to create chunk file: %w", err)
	}
	defer f.Close()

	size, err := io.Copy(f, data)
	if err != nil {
		return fmt.Errorf("failed to write chunk data: %w", err)
	}

	now := time.Now()
	chunk := &MNormalFileChunk{
		FileId:     fileId,
		ChunkIndex: chunkIndex,
		ChunkMd5:   &chunkMd5,
		ChunkSize:  &size,
		CreateTime: &now,
	}
	return s.repo.CreateChunk(chunk)
}

// CheckChunk checks if a chunk has already been uploaded.
func (s *service) CheckChunk(fileId string, chunkIndex int) (bool, error) {
	_, err := s.repo.FindChunk(fileId, chunkIndex)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Assemble merges all chunks into the final file.
func (s *service) Assemble(fileId string) error {
	f, err := s.repo.FindFileByFileId(fileId)
	if err != nil {
		return fmt.Errorf("file not found: %s", fileId)
	}

	chunks, err := s.repo.FindChunksByFileId(fileId)
	if err != nil {
		return fmt.Errorf("failed to list chunks: %w", err)
	}

	chunkCount := 0
	if f.ChunkCount != nil {
		chunkCount = *f.ChunkCount
	}
	if len(chunks) < chunkCount {
		return fmt.Errorf("not all chunks uploaded: got %d, expected %d", len(chunks), chunkCount)
	}

	// Create assembled file
	assembledPath := s.assembledPath(fileId)
	if err := os.MkdirAll(filepath.Dir(assembledPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	out, err := os.Create(assembledPath)
	if err != nil {
		return fmt.Errorf("failed to create assembled file: %w", err)
	}
	defer out.Close()

	chunkDir := s.chunkDir(fileId)
	for _, chunk := range chunks {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%d", chunk.ChunkIndex))
		cf, err := os.Open(chunkPath)
		if err != nil {
			return fmt.Errorf("failed to open chunk %d: %w", chunk.ChunkIndex, err)
		}
		_, err = io.Copy(out, cf)
		cf.Close()
		if err != nil {
			return fmt.Errorf("failed to copy chunk %d: %w", chunk.ChunkIndex, err)
		}
	}

	// Update status to assembled
	now := time.Now()
	status := 2 // assembled
	f.Status = &status
	f.UpdateTime = &now
	if err := s.repo.UpdateFile(f); err != nil {
		return fmt.Errorf("failed to update file status: %w", err)
	}

	// Clean up chunk files
	os.RemoveAll(chunkDir)

	return nil
}

// Upload handles a whole-file upload (no chunking).
func (s *service) Upload(tenantId int, username string, fileName string, data io.Reader, size int64) (string, error) {
	fileId := uuid.New().String()
	now := time.Now()
	status := 3 // complete
	chunkCount := 0
	f := &MNormalFile{
		FileId:     fileId,
		FileName:   &fileName,
		FileSize:   &size,
		ChunkCount: &chunkCount,
		Status:     &status,
		TenantId:  &tenantId,
		CreateTime: &now,
		UpdateTime: &now,
		Username:   &username,
	}
	if err := s.repo.CreateFile(f); err != nil {
		return "", err
	}

	assembledPath := s.assembledPath(fileId)
	if err := os.MkdirAll(filepath.Dir(assembledPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	out, err := os.Create(assembledPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, data); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fileId, nil
}

// Delete removes a file and its record.
func (s *service) Delete(fileId string) error {
	// Remove file from disk
	assembledPath := s.assembledPath(fileId)
	os.Remove(assembledPath)

	// Remove chunks
	os.RemoveAll(s.chunkDir(fileId))

	// Remove DB records
	if err := s.repo.DeleteFile(fileId); err != nil {
		return err
	}
	return nil
}

// List returns a paginated list of MNormal files.
func (s *service) List(tenantId int, req *ListRequest) ([]MNormalFile, int64, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindFiles(tenantId, req.FileName, offset, pageSize)
}

// Detail returns the file metadata.
func (s *service) Detail(fileId string) (*MNormalFile, error) {
	return s.repo.FindFileByFileId(fileId)
}

// DownloadToDevice sends the file to specified devices via TR-069 Download.
func (s *service) DownloadToDevice(fileId string, elementIds []int64, opSender OperationSender) error {
	f, err := s.repo.FindFileByFileId(fileId)
	if err != nil {
		return fmt.Errorf("file not found: %s", fileId)
	}

	for _, elementId := range elementIds {
		sn, err := s.getDeviceSerialNumber(elementId)
		if err != nil {
			logger.Warnf("mnormal download: device %d not found: %v", elementId, err)
			continue
		}
		if sn == "" {
			logger.Warnf("mnormal download: device %d has no serial number", elementId)
			continue
		}

		// Build the download URL — the file is served by the device-facing
		// /acs-file-server/** endpoint. The URL matches the pattern used by
		// the upgrade module.
		fileName := ""
		if f.FileName != nil {
			fileName = *f.FileName
		}
		downloadURL := fmt.Sprintf("/acs-file-server/mnormal/%s", fileId)
		dl := &soap.Download{
			CommandKey:     fmt.Sprintf("mnormal_download_%s_%d", fileId, elementId),
			FileType:       "MNormal File",
			URL:            downloadURL,
			TargetFileName: fileName,
		}

		opId := fmt.Sprintf("mnormal_%s_%d_%d", fileId, elementId, time.Now().UnixNano())
		if err := opSender.SendDownload(sn, dl, opId); err != nil {
			logger.Errorf("mnormal download: failed to send Download to device %d: %v", elementId, err)
			continue
		}

		// Record download log
		now := time.Now()
		pendingStatus := 1 // pending
		log := &MNormalFileDownloadLog{
			FileId:       fileId,
			ElementId:    &elementId,
			SerialNumber: &sn,
			Status:       &pendingStatus,
			CreateTime:   &now,
			UpdateTime:   &now,
		}
		if err := s.repo.CreateDownloadLog(log); err != nil {
			logger.Errorf("mnormal download: failed to create download log for device %d: %v", elementId, err)
		}
	}

	return nil
}

// DownloadResults returns paginated download results.
func (s *service) DownloadResults(fileId string, page, pageSize int) ([]MNormalFileDownloadLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindDownloadLogs(fileId, offset, pageSize)
}

// getDeviceSerialNumber returns the serial number for a device element.
func (s *service) getDeviceSerialNumber(elementId int64) (string, error) {
	var dev device.CpeElement
	if err := s.db.Select("ne_neid, serial_number").Where("ne_neid = ? AND deleted = ?", elementId, false).First(&dev).Error; err != nil {
		return "", err
	}
	if dev.SerialNumber == nil {
		return "", nil
	}
	return *dev.SerialNumber, nil
}
