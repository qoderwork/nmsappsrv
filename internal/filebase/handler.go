package filebase

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/systemsettings"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
)

// Handler exposes the device-facing /acs-file-server/** endpoints. The contract
// mirrors Java's station-new controller.file.* (FileDownloadController,
// MrController, LogController, CaptureFileController, ConfigFileController,
// MmlExecuteResultController, CallTraceFileController) plus the core
// FilePiecemealUpload endpoints are served by the filepiecemeal package.
type Handler struct {
	svc      *Service
	sysSvc   *systemsettings.SystemSettingsService
	ingester MRIngester
}

func NewHandler(svc *Service, sysSvc *systemsettings.SystemSettingsService, ingester MRIngester) *Handler {
	return &Handler{svc: svc, sysSvc: sysSvc, ingester: ingester}
}

// requestBodyReader returns the upload payload as a reader. Java accepts either
// a raw request body (PUT /mr/{fileName}) or a multipart form (POST /mr). We
// transparently support both.
func requestBodyReader(c *gin.Context) (io.Reader, error) {
	ct := c.ContentType()
	if strings.HasPrefix(ct, "multipart/") {
		if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
			return nil, err
		}
		for _, files := range c.Request.MultipartForm.File {
			if len(files) > 0 {
				return files[0].Open()
			}
		}
		return nil, io.EOF
	}
	return c.Request.Body, nil
}

// firstMultipartName returns the original filename of the first multipart part.
func firstMultipartName(c *gin.Context) string {
	if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
		return ""
	}
	for _, files := range c.Request.MultipartForm.File {
		if len(files) > 0 {
			return files[0].Filename
		}
	}
	return ""
}

func elementIDParam(c *gin.Context) int64 {
	if v := c.Query("elementId"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return 0
}

// ---- downloads ----

// DownloadBpf handles GET /acs-file-server/download/{type}/{fileName}?tenantId=
// (Java FileDownloadController.download, type "bpf" only).
func (h *Handler) DownloadBpf(c *gin.Context) {
	typ := c.Param("type")
	fileName := c.Param("fileName")
	if typ != "bpf" {
		utils.Error(c, http.StatusBadRequest, "illegal type: "+typ)
		return
	}
	if !safeFileName(fileName) {
		utils.Error(c, http.StatusBadRequest, "invalid file name")
		return
	}
	path := h.svc.BpfPath(c.Query("tenantId"), fileName)
	serveFileAs(c, path, fileName)
}

// serveFile streams a file with Accept-Ranges / Range support (Java uses
// DownloadFileUtil.download which sets Accept-Ranges: bytes).
func serveFile(c *gin.Context, path string) {
	info, err := os.Stat(path)
	if err != nil {
		// Java returns an empty 200 when the file is missing; we mirror that.
		c.Status(http.StatusOK)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()
	c.Header("Accept-Ranges", "bytes")
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), f)
}

// serveFileAs streams a file with the same contract as Java's
// DownloadFileUtil.download: application/octet-stream, an explicit
// Content-Disposition attachment with the given download name, and
// Content-Length. Missing files yield an empty 200, mirroring Java.
func serveFileAs(c *gin.Context, path, downloadName string) {
	info, err := os.Stat(path)
	if err != nil {
		c.Status(http.StatusOK)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", downloadName))
	c.Header("Access-Control-Expose-Headers", "Content-Disposition")
	c.Header("Content-Length", strconv.FormatInt(info.Size(), 10))
	c.Header("Accept-Ranges", "bytes")
	io.Copy(c.Writer, f)
}

// fileIdParam reads the Integer fileId query parameter (used by upgradeFile /
// mNormalFile). Returns 0 when absent/invalid.
func fileIdParam(c *gin.Context) int {
	if v := c.Query("fileId"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

// ---- domain downloads (FileDownload phase) ----

// DownloadLicense serves GET /acs-file-server/license?elementId= (Java
// BaseStationLicenseFileController.downloadLicense).
func (h *Handler) DownloadLicense(c *gin.Context) {
	id := elementIDParam(c)
	if id == 0 {
		utils.Error(c, http.StatusBadRequest, "elementId required")
		return
	}
	path, name, ok := h.svc.resolveLicense(id)
	if !ok {
		c.Status(http.StatusOK)
		return
	}
	serveFileAs(c, path, name)
}

// DownloadZtpFile serves GET /acs-file-server/ztpFile?elementId= (Java
// AOSManagementServiceImpl.downloadZTPFile).
func (h *Handler) DownloadZtpFile(c *gin.Context) {
	id := elementIDParam(c)
	if id == 0 {
		utils.Error(c, http.StatusBadRequest, "elementId required")
		return
	}
	path, name, ok := h.svc.resolveZtpFile(id)
	if !ok {
		c.Status(http.StatusOK)
		return
	}
	serveFileAs(c, path, name)
}

// DownloadConfigFile serves GET /acs-file-server/configFile?elementId= (Java
// BaseStationBackupAndRestoreManagementServiceImpl.downloadConfigFile).
func (h *Handler) DownloadConfigFile(c *gin.Context) {
	id := elementIDParam(c)
	if id == 0 {
		utils.Error(c, http.StatusBadRequest, "elementId required")
		return
	}
	path, name, ok := h.svc.resolveConfigFile(id)
	if !ok {
		c.Status(http.StatusOK)
		return
	}
	serveFileAs(c, path, name)
}

// DownloadUpgradeFile serves GET /acs-file-server/upgradeFile?fileId= (Java
// UpgradeManagementServiceImpl.downloadUpgradeFile).
func (h *Handler) DownloadUpgradeFile(c *gin.Context) {
	id := fileIdParam(c)
	if id == 0 {
		utils.Error(c, http.StatusBadRequest, "fileId required")
		return
	}
	path, name, ok := h.svc.resolveUpgradeFile(id)
	if !ok {
		c.Status(http.StatusOK)
		return
	}
	serveFileAs(c, path, name)
}

// DownloadMNormalFile serves GET /acs-file-server/mNormalFile?fileId= (Java
// DeviceMNormalFileManagementServiceImpl.downloadMNormalFile). nmsappsrv has no
// device_m_normal_file table yet, so it mirrors Java's empty-200 on absence.
func (h *Handler) DownloadMNormalFile(c *gin.Context) {
	id := fileIdParam(c)
	if id == 0 {
		utils.Error(c, http.StatusBadRequest, "fileId required")
		return
	}
	if _, _, ok := h.svc.resolveMNormalFile(id); !ok {
		c.Status(http.StatusOK)
		return
	}
}

// DownloadCaFile serves GET /acs-file-server/ca/downloadFile/:fileId (Java
// CaFileController.downloadZtpFile → caFileService.downloadcaFile).
func (h *Handler) DownloadCaFile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("fileId"))
	if err != nil || id == 0 {
		utils.Error(c, http.StatusBadRequest, "fileId required")
		return
	}
	path, name, ok := h.svc.resolveCa(id)
	if !ok {
		c.Status(http.StatusOK)
		return
	}
	serveFileAs(c, path, name)
}

// TestDownload handles GET /acs-file-server/testDownload?size= (MB).
func (h *Handler) TestDownload(c *gin.Context) {
	sizeMB := 500
	if v := c.Query("size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sizeMB = n
		}
	}
	total := sizeMB * 1024 * 1024
	c.Header("Content-Type", "application/x-download")
	c.Header("Content-Disposition", "attachment;filename=test.temp")
	c.Header("Accept-Ranges", "bytes")
	w := c.Writer
	buf := make([]byte, 1024*1024)
	for i := 0; i < sizeMB; i++ {
		if _, err := w.Write(buf); err != nil {
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	_ = total
}

// TestUpload handles PUT/POST /acs-file-server/testUpload (drains the body).
func (h *Handler) TestUpload(c *gin.Context) {
	_, _ = io.Copy(io.Discard, c.Request.Body)
	c.Status(http.StatusOK)
}

// ---- mr upload ----

func (h *Handler) UploadMrNamed(c *gin.Context) {
	fileName := c.Param("fileName")
	if !safeFileName(fileName) {
		utils.Error(c, http.StatusBadRequest, "invalid file name")
		return
	}
	h.saveMr(c, h.svc.cfg.MrDir, fileName, elementIDParam(c))
}

func (h *Handler) UploadMrNoName(c *gin.Context) {
	name := firstMultipartName(c)
	if name == "" || !safeFileName(name) {
		utils.Error(c, http.StatusBadRequest, "missing file name")
		return
	}
	h.saveMr(c, h.svc.cfg.MrDir, name, elementIDParam(c))
}

func (h *Handler) saveMr(c *gin.Context, dir, fileName string, elementID int64) {
	r, err := requestBodyReader(c)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "failed to read upload: "+err.Error())
		return
	}
	path, err := saveBody(r, dir, fileName)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if h.ingester != nil {
		if err := h.ingester.IngestMR(path, elementID); err != nil {
			utils.Error(c, http.StatusInternalServerError, "mr ingest failed: "+err.Error())
			return
		}
	}
	utils.Success(c, true)
}

// ---- log upload ----

func (h *Handler) UploadLogNamed(c *gin.Context) {
	h.uploadTo(c, h.svc.cfg.LogDir, c.Param("fileName"))
}
func (h *Handler) UploadLog(c *gin.Context) {
	h.uploadTo(c, h.svc.cfg.LogDir, firstMultipartName(c))
}
func (h *Handler) UploadLogReqNamed(c *gin.Context) {
	h.uploadLogWithRequestId(c, c.Param("fileName"), c.Param("requestId"))
}
func (h *Handler) UploadLogReq(c *gin.Context) {
	h.uploadLogWithRequestId(c, firstMultipartName(c), c.Param("requestId"))
}

// uploadLogWithRequestId saves the uploaded log file to disk and then stores
// the file name in Redis keyed by requestId. This mirrors Java LogController's
// `redisService.setKeyAndValue("LogFileName_" + requestId, fileName)`, allowing
// processTransferComplete to retrieve the file name when the device reports
// upload completion via TransferComplete.
func (h *Handler) uploadLogWithRequestId(c *gin.Context, fileName, requestId string) {
	if fileName == "" || !safeFileName(fileName) {
		utils.Error(c, http.StatusBadRequest, "invalid file name")
		return
	}
	r, err := requestBodyReader(c)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "failed to read upload: "+err.Error())
		return
	}
	if _, err := saveBody(r, h.svc.cfg.LogDir, fileName); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if requestId != "" {
		redis.Set(context.Background(), "LogFileName_"+requestId, fileName, 30*time.Minute)
	}
	utils.Success(c, true)
}

// ---- capture upload ----

func (h *Handler) UploadCaptureNamed(c *gin.Context) {
	h.uploadTo(c, h.svc.cfg.CaptureDir, c.Param("fileName"))
}
func (h *Handler) UploadCapture(c *gin.Context) {
	h.uploadTo(c, h.svc.cfg.CaptureDir, firstMultipartName(c))
}

// ---- callTrace (pm) upload ----

func (h *Handler) UploadCallTraceNamed(c *gin.Context) {
	h.uploadTo(c, h.svc.cfg.PmDir, c.Param("fileName"))
}
func (h *Handler) UploadCallTrace(c *gin.Context) {
	h.uploadTo(c, h.svc.cfg.PmDir, firstMultipartName(c))
}

// ---- config upload ----

func (h *Handler) UploadConfigNamed(c *gin.Context) {
	tenantID, _ := strconv.ParseInt(c.Param("tenantId"), 10, 64)
	elementID, _ := strconv.ParseInt(c.Param("elementId"), 10, 64)
	base := filepath.Join(h.svc.cfg.ConfigDir, itoa(tenantID), itoa(elementID))
	h.uploadTo(c, base, c.Param("fileName"))
}
func (h *Handler) UploadConfig(c *gin.Context) {
	tenantID, _ := strconv.ParseInt(c.Param("tenantId"), 10, 64)
	elementID, _ := strconv.ParseInt(c.Param("elementId"), 10, 64)
	base := filepath.Join(h.svc.cfg.ConfigDir, itoa(tenantID), itoa(elementID))
	h.uploadTo(c, base, firstMultipartName(c))
}

// ---- mml result upload ----

func (h *Handler) UploadMml(c *gin.Context) {
	elementID, _ := strconv.ParseInt(c.Param("elementId"), 10, 64)
	base := filepath.Join(h.svc.cfg.MmlResultDir, itoa(elementID))
	h.uploadTo(c, base, firstMultipartName(c))
}
func (h *Handler) UploadMmlNamed(c *gin.Context) {
	elementID, _ := strconv.ParseInt(c.Param("elementId"), 10, 64)
	base := filepath.Join(h.svc.cfg.MmlResultDir, itoa(elementID))
	h.uploadTo(c, base, c.Param("fileName"))
}

// uploadTo is the shared multipart/raw save path for non-MR uploads.
func (h *Handler) uploadTo(c *gin.Context, dir, fileName string) {
	if fileName == "" || !safeFileName(fileName) {
		utils.Error(c, http.StatusBadRequest, "invalid file name")
		return
	}
	r, err := requestBodyReader(c)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "failed to read upload: "+err.Error())
		return
	}
	if _, err := saveBody(r, dir, fileName); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, true)
}

// ---- domain downloads (FileDownload phase: wired via registry) ----

var downloadProviders = map[string]func(c *gin.Context){}

// RegisterDownloadProvider lets other modules (upgrade, cacert, license, ...)
// serve their files through /acs-file-server/** without creating an import
// cycle. Not registered in Phase 1 → 404 until the FileDownload phase wires them.
func RegisterDownloadProvider(kind string, fn func(c *gin.Context)) {
	downloadProviders[kind] = fn
}

func (h *Handler) DownloadByKind(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if fn, ok := downloadProviders[kind]; ok {
			fn(c)
			return
		}
		utils.Error(c, http.StatusNotFound,
			"file download provider '"+kind+"' not wired in this phase (FileDownload phase TBD)")
	}
}
