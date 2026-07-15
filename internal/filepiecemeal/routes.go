package filepiecemeal

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts the chunked-upload management endpoints under the
// authenticated API group (Java's /api/v2/ prefix → nmsappsrv /api/v1).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/getPiecemealFileId", h.GetPiecemealFileId)
	rg.POST("/uploadPiecemealFile", h.UploadPiecemealFile)
	rg.POST("/checkPiecemealFileNeedToUpload", h.CheckPiecemealFileNeedToUpload)
	rg.POST("/assemblePiecemealFiles", h.AssemblePiecemealFiles)
}
