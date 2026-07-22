package cbsd

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"
)

// CbsdCertificate 对应 cbsd_certificate 表 (CBSD 证书文件管理)
type CbsdCertificate struct {
	Id         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementId  int64      `gorm:"column:element_id" json:"elementId"`
	FileName   *string    `gorm:"column:file_name;type:varchar(255)" json:"fileName"`
	FilePath   *string    `gorm:"column:file_path;type:varchar(500)" json:"filePath"`
	UploadTime *time.Time `gorm:"column:upload_time" json:"uploadTime"`
	Status     *string    `gorm:"column:status;type:varchar(50)" json:"status"`
	TenantId  *int       `gorm:"column:tenant_id" json:"tenant_id"`
}

func (CbsdCertificate) TableName() string { return "cbsd_certificate" }

// ---------- CBSD certificate CRUD ----------

// ListCbsdCertificates handles GET /cbsd/certificate
func (h *Handler) ListCbsdCertificates(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	var certs []CbsdCertificate
	if err := h.db.Where("tenant_id = ?", tenantId).Order("id DESC").Find(&certs).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list certificates")
		return
	}
	if certs == nil {
		certs = []CbsdCertificate{}
	}
	utils.Success(c, certs)
}

// UploadCbsdCertificate handles POST /cbsd/certificate (multipart upload)
func (h *Handler) UploadCbsdCertificate(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)
	elementIdStr := c.PostForm("elementId")
	elementId, err := strconv.ParseInt(elementIdStr, 10, 64)
	if err != nil || elementId == 0 {
		utils.Error(c, http.StatusBadRequest, "elementId is required")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "file is required")
		return
	}

	fileName := file.Filename
	dir := filepath.Join(h.cfg.FileServer.Root, "cbsd", "certs")
	os.MkdirAll(dir, 0o755)
	savedPath := filepath.Join(dir, file.Filename)
	if err := c.SaveUploadedFile(file, savedPath); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to save certificate file")
		return
	}

	now := time.Now()
	status := "uploaded"
	cert := CbsdCertificate{
		ElementId:  elementId,
		FileName:   &fileName,
		FilePath:   &savedPath,
		UploadTime: &now,
		Status:     &status,
		TenantId:  &tenantId,
	}
	if err := h.db.Create(&cert).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to create certificate record")
		return
	}
	utils.Success(c, cert)
}

// DeleteCbsdCertificate handles DELETE /cbsd/certificate/:id
func (h *Handler) DeleteCbsdCertificate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid certificate id")
		return
	}
	tenantId := middleware.GetTenantId(c)
	if err := h.db.Where("id = ? AND tenant_id = ?", id, tenantId).Delete(&CbsdCertificate{}).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete certificate")
		return
	}
	utils.Success(c, nil)
}
