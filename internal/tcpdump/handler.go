package tcpdump

import (
	"net/http"

	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler exposes HTTP handlers for tcpdump endpoints. The contract mirrors
// Java's TcpdumpManagementController (/api/v2/...): listNetworkCards,
// doTcpdump, listTcpdumpFiles, downloadTcpdumpFile, deleteTcpdumpFile,
// batchDeleteTcpdumpFile — expressed in nmsappsrv's RESTful /api/v1 style.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler. db is forwarded for signature compatibility
// (tcpdump is file-based and does not use the database).
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{svc: NewService(db)}
}

// ListNetworkCards handles GET /tcpdump/network-cards
// (Java GET /api/v2/listNetworkCards).
func (h *Handler) ListNetworkCards(c *gin.Context) {
	cards, err := h.svc.ListNetworkCards()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, cards)
}

// DoCapture handles POST /tcpdump/capture (Java POST /api/v2/doTcpdump).
func (h *Handler) DoCapture(c *gin.Context) {
	var req DoCaptureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: duration and container are required")
		return
	}
	if err := h.svc.DoCapture(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ListFiles handles GET /tcpdump/files (Java POST /api/v2/listTcpdumpFiles).
func (h *Handler) ListFiles(c *gin.Context) {
	files, err := h.svc.ListFiles()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, files)
}

// DownloadFile handles GET /tcpdump/files/:name/download
// (Java GET /api/v2/downloadTcpdumpFile?fileName=...).
func (h *Handler) DownloadFile(c *gin.Context) {
	name := c.Param("name")
	if !isValidFileName(name) {
		utils.Error(c, http.StatusBadRequest, "invalid file name")
		return
	}
	path, err := h.svc.DownloadPath(name)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	c.Header("Content-Type", "application/vnd.tcpdump.pcap")
	c.Header("Content-Disposition", "attachment; filename="+name)
	c.File(path)
}

// DeleteFile handles DELETE /tcpdump/files/:name
// (Java POST /api/v2/deleteTcpdumpFile).
func (h *Handler) DeleteFile(c *gin.Context) {
	name := c.Param("name")
	if err := h.svc.DeleteFile(name); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, nil)
}

// BatchDeleteFiles handles POST /tcpdump/files/batch-delete
// (Java POST /api/v2/batchDeleteTcpdumpFile).
func (h *Handler) BatchDeleteFiles(c *gin.Context) {
	var req BatchDeleteFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: fileNames are required")
		return
	}
	if err := h.svc.BatchDelete(req.FileNames); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, nil)
}
