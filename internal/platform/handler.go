package platform

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"

	"nmsappsrv/internal/config"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler provides HTTP handlers for platform settings endpoints
type Handler struct {
	svc           *Service
	platformFiles config.PlatformFilesConfig
}

// NewHandler creates a new Handler
func NewHandler(db *gorm.DB, aesKeyHex string, pfCfg config.PlatformFilesConfig) *Handler {
	return &Handler{
		svc:           NewService(NewRepository(db), aesKeyHex),
		platformFiles: pfCfg,
	}
}

// GetDate handles POST /api/v1/getDate
func (h *Handler) GetDate(c *gin.Context) {
	utils.Success(c, h.svc.GetTime())
}

// GetSupportedZone handles GET /api/v1/getSupportedZone
func (h *Handler) GetSupportedZone(c *gin.Context) {
	utils.Success(c, h.svc.GetSupportedZone())
}

// GetLogo handles GET /api/v1/getLogo
func (h *Handler) GetLogo(c *gin.Context) {
	// Try to get logo from license table
	logo := ""
	// The logo can come from the license table; for now return empty
	// In Java, it reads from tenancy.license_logo_base64
	utils.Success(c, logo)
}

// ListLogConfig handles GET /api/v1/listLogConfig
func (h *Handler) ListLogConfig(c *gin.Context) {
	cfg, err := h.svc.GetLogConfig()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, cfg)
}

// UpdateLogConfig handles POST /api/v1/updateLogConfig
func (h *Handler) UpdateLogConfig(c *gin.Context) {
	var cfg LogConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}
	if err := h.svc.UpdateLogConfig(&cfg); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// GetFTPTransferLogConfig handles GET /api/v1/getFTPTransferLogConfig
func (h *Handler) GetFTPTransferLogConfig(c *gin.Context) {
	cfg, err := h.svc.GetFTPTransferLogConfig()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, cfg)
}

// UpdateFTPTransferLogConfig handles POST /api/v1/updateFTPTransferLogConfig
func (h *Handler) UpdateFTPTransferLogConfig(c *gin.Context) {
	var cfg FTPTransferLogConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}
	if err := h.svc.UpdateFTPTransferLogConfig(&cfg); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// GetHECConfig handles GET /api/v1/getHECConfig
func (h *Handler) GetHECConfig(c *gin.Context) {
	cfg, err := h.svc.GetHECConfig()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, cfg)
}

// UpdateHECConfig handles POST /api/v1/updateHECConfig
func (h *Handler) UpdateHECConfig(c *gin.Context) {
	var cfg HECConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}
	if err := h.svc.UpdateHECConfig(&cfg); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ListNMSSecret handles GET /api/v1/listNMSSecret
func (h *Handler) ListNMSSecret(c *gin.Context) {
	secret, err := h.svc.ListNMSSecret()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, secret)
}

// UpdateNMSSecret handles POST /api/v1/updateNMSSecret
func (h *Handler) UpdateNMSSecret(c *gin.Context) {
	var secret NMSSecret
	if err := c.ShouldBindJSON(&secret); err != nil {
		utils.Error(c, 400, "invalid request body")
		return
	}
	if err := h.svc.UpdateNMSSecret(&secret); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// DownloadPasswordRSAPublicKey handles GET /api/v1/downloadPasswordRSAPublicKey
func (h *Handler) DownloadPasswordRSAPublicKey(c *gin.Context) {
	filePath := h.platformFiles.RSAPublicKeyPath
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		utils.HandleError(c, apperror.ErrNotFound.WithMessage("RSA public key file not found"))
		return
	}
	c.File(filePath)
}

// DownloadPlatformLogs handles POST /api/v1/downloadPlatformLogs
func (h *Handler) DownloadPlatformLogs(c *gin.Context) {
	logDir := h.platformFiles.PlatformLogDir

	// Check if the platform log directory exists
	dirInfo, err := os.Stat(logDir)
	if err != nil || !dirInfo.IsDir() {
		utils.HandleError(c, apperror.ErrNotFound.WithMessage("platform log directory not found"))
		return
	}

	// Create a temp file for the ZIP
	tmpFile, err := os.CreateTemp("", "platform-logs-*.zip")
	if err != nil {
		utils.HandleError(c, apperror.Wrap(err, "INTERNAL", 500, "failed to create temp file for log archive"))
		return
	}
	tmpPath := tmpFile.Name()
	// Ensure cleanup of the temp file after serving
	defer os.Remove(tmpPath)

	// Create a ZIP writer
	zipWriter := zip.NewWriter(tmpFile)

	// Walk the log directory and add files to the ZIP
	err = filepath.Walk(logDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		// Compute relative path for the entry name
		relPath, err := filepath.Rel(logDir, path)
		if err != nil {
			return err
		}
		// Create ZIP entry
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		// Open the source file and copy its contents
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})

	if err != nil {
		zipWriter.Close()
		tmpFile.Close()
		utils.HandleError(c, apperror.Wrap(err, "INTERNAL", 500, "failed to create log archive"))
		return
	}

	// Close the ZIP writer and temp file
	if err := zipWriter.Close(); err != nil {
		tmpFile.Close()
		utils.HandleError(c, apperror.Wrap(err, "INTERNAL", 500, "failed to finalize log archive"))
		return
	}
	tmpFile.Close()

	// Serve the ZIP file as an attachment download
	c.FileAttachment(tmpPath, "platform-logs.zip")
}

// DownloadNMSManualDocument handles GET /api/v1/downloadNMSManualDocument
func (h *Handler) DownloadNMSManualDocument(c *gin.Context) {
	filePath := h.platformFiles.NMSManualDocPath
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		utils.HandleError(c, apperror.ErrNotFound.WithMessage("NMS manual document not found"))
		return
	}
	c.FileAttachment(filePath, "nms_manual.pdf")
}
