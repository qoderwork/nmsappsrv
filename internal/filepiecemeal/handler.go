package filepiecemeal

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/pkg/utils"
)

// Handler implements Java's FilePiecemealUploadManagementController.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type getIDRequest struct {
	MD5 string `json:"md5"`
}

type getIDResponse struct {
	FileID string `json:"fileId"`
}

type checkRequest struct {
	FileID  string `json:"fileId"`
	PartMD5 string `json:"partMd5"`
	Index   int    `json:"index"`
}

type checkResponse struct {
	NeedUpload bool `json:"needUpload"`
}

type assembleRequest struct {
	FileID   string `json:"fileId"`
	Total    int    `json:"total"`
	FileName string `json:"fileName"`
	MD5      string `json:"md5"`
}

// GetPiecemealFileId handles POST /getPiecemealFileId.
func (h *Handler) GetPiecemealFileId(c *gin.Context) {
	var req getIDRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.MD5 == "" {
		utils.Error(c, http.StatusBadRequest, "md5 (whole-file hash) is required")
		return
	}
	utils.Success(c, getIDResponse{FileID: h.svc.GetPiecemealFileId(req.MD5)})
}

// UploadPiecemealFile handles POST /uploadPiecemealFile (multipart: index,
// fileId, fileName form fields + the chunk as the file part).
func (h *Handler) UploadPiecemealFile(c *gin.Context) {
	fileID := c.PostForm("fileId")
	fileName := c.PostForm("fileName")
	indexStr := c.PostForm("index")
	if fileID == "" || fileName == "" || indexStr == "" || !safeFileName(fileName) {
		utils.Error(c, http.StatusBadRequest, "index, fileId and fileName are required")
		return
	}
	index, err := strconv.Atoi(indexStr)
	if err != nil || index < 1 {
		utils.Error(c, http.StatusBadRequest, "index must be a positive integer")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		// Java also supports the chunk as the raw request body stream.
		raw, rerr := c.GetRawData()
		if rerr != nil || len(raw) == 0 {
			utils.Error(c, http.StatusBadRequest, "missing chunk payload")
			return
		}
		if err := h.svc.UploadPiecemealFile(fileID, index, fileName, bytes.NewReader(raw)); err != nil {
			utils.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		utils.Success(c, true)
		return
	}
	f, err := file.Open()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()
	if err := h.svc.UploadPiecemealFile(fileID, index, fileName, f); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, true)
}

// CheckPiecemealFileNeedToUpload handles POST /checkPiecemealFileNeedToUpload.
func (h *Handler) CheckPiecemealFileNeedToUpload(c *gin.Context) {
	var req checkRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.FileID == "" || req.PartMD5 == "" || req.Index < 1 {
		utils.Error(c, http.StatusBadRequest, "fileId, partMd5 and index are required")
		return
	}
	need, err := h.svc.CheckNeedUpload(req.FileID, req.Index, req.PartMD5)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, checkResponse{NeedUpload: need})
}

// AssemblePiecemealFiles handles POST /assemblePiecemealFiles.
func (h *Handler) AssemblePiecemealFiles(c *gin.Context) {
	var req assembleRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.FileID == "" || req.FileName == "" ||
		req.Total < 1 || req.MD5 == "" || !safeFileName(req.FileName) {
		utils.Error(c, http.StatusBadRequest, "fileId, total, fileName and md5 are required")
		return
	}
	path, err := h.svc.Assemble(req.FileID, req.Total, req.FileName, req.MD5)
	if err != nil {
		// Java returns 10176 (assembly failure) / 10177 (digest mismatch).
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = path
	utils.Success(c, true)
}
