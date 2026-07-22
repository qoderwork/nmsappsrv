package filebase

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/systemsettings"
	"nmsappsrv/pkg/utils"
)

// BasicAuthMiddleware enforces HTTP Basic auth on /acs-file-server/** using the
// device-facing credentials from system_config ("nms_config" FileServerUsername/
// Password). Java lets the API gateway bypass Spring Security for this path and
// relies on network trust + ACS-injected credentials; nmsappsrv has no such
// gateway trust, so it enforces Basic auth itself to honour the contract.
func BasicAuthMiddleware(sysSvc *systemsettings.SystemSettingsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, pass, err := sysSvc.GetFileServerCredentials()
		if err != nil || user == "" {
			utils.Error(c, http.StatusUnauthorized, "file server credentials not configured")
			c.Abort()
			return
		}
		reqUser, reqPass, ok := c.Request.BasicAuth()
		if !ok || reqUser != user || reqPass != pass {
			c.Header("WWW-Authenticate", `Basic realm="acs-file-server"`)
			utils.Error(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RegisterRoutes mounts the device-facing file server at the root (no /api/v1
// prefix, mirroring Java's @RequestMapping("/acs-file-server/") which the
// gateway reaches after stripping the /api prefix). Basic auth is enforced.
func RegisterRoutes(router *gin.Engine, sysSvc *systemsettings.SystemSettingsService, svc *Service, ingester MRIngester) {
	h := NewHandler(svc, sysSvc, ingester)
	rg := router.Group("/acs-file-server")
	rg.Use(BasicAuthMiddleware(sysSvc))
	{
		// ---- generic downloads ----
		rg.GET("/download/:type/:fileName", h.DownloadBpf)
		rg.GET("/testDownload", h.TestDownload)

		// ---- mr upload (triggers parse via ingester) ----
		rg.PUT("/mr", h.UploadMrNoName)
		rg.POST("/mr", h.UploadMrNoName)
		rg.PUT("/mr/:fileName", h.UploadMrNamed)
		rg.POST("/mr/:fileName", h.UploadMrNamed)

		// ---- device log upload ----
		rg.PUT("/log/:fileName", h.UploadLogNamed)
		rg.POST("/log/:fileName", h.UploadLogNamed)
		rg.PUT("/log", h.UploadLog)
		rg.POST("/log", h.UploadLog)
		rg.PUT("/upload/log/:requestId/:fileName", h.UploadLogReqNamed)
		rg.POST("/upload/log/:requestId/:fileName", h.UploadLogReqNamed)
		rg.PUT("/upload/log/:requestId", h.UploadLogReq)
		rg.POST("/upload/log/:requestId", h.UploadLogReq)

		// ---- capture upload ----
		rg.PUT("/capture/:fileName", h.UploadCaptureNamed)
		rg.POST("/capture/:fileName", h.UploadCaptureNamed)
		rg.PUT("/capture", h.UploadCapture)
		rg.POST("/capture", h.UploadCapture)

		// ---- config upload (per tenantId/elementId) ----
		rg.PUT("/upload/config/:tenantId/:elementId/:fileName", h.UploadConfigNamed)
		rg.POST("/upload/config/:tenantId/:elementId/:fileName", h.UploadConfigNamed)
		rg.PUT("/upload/config/:tenantId/:elementId", h.UploadConfig)
		rg.POST("/upload/config/:tenantId/:elementId", h.UploadConfig)

		// ---- mml execute-result upload (per elementId) ----
		rg.PUT("/mml/:elementId", h.UploadMml)
		rg.POST("/mml/:elementId", h.UploadMml)
		rg.PUT("/mml/:elementId/:fileName", h.UploadMmlNamed)
		rg.POST("/mml/:elementId/:fileName", h.UploadMmlNamed)

		// ---- callTrace / pm upload ----
		rg.PUT("/callTrace/:fileName", h.UploadCallTraceNamed)
		rg.POST("/callTrace/:fileName", h.UploadCallTraceNamed)
		rg.PUT("/callTrace", h.UploadCallTrace)
		rg.POST("/callTrace", h.UploadCallTrace)

		// ---- test upload ----
		rg.PUT("/testUpload", h.TestUpload)
		rg.POST("/testUpload", h.TestUpload)

		// ---- domain downloads (FileDownload phase) ----
		// The six providers below were stubbed (→ 404) until the FileDownload
		// phase. They are now registered so DownloadByKind resolves them to real
		// handlers that mirror Java's BaseStationLicenseFileController +
		// CaFileController path resolution (config-rooted + DB record lookup).
		RegisterDownloadProvider("license", h.DownloadLicense)
		RegisterDownloadProvider("ztpFile", h.DownloadZtpFile)
		RegisterDownloadProvider("configFile", h.DownloadConfigFile)
		RegisterDownloadProvider("upgradeFile", h.DownloadUpgradeFile)
		RegisterDownloadProvider("mNormalFile", h.DownloadMNormalFile)
		RegisterDownloadProvider("ca", h.DownloadCaFile)

		rg.GET("/license", h.DownloadByKind("license"))
		rg.GET("/ztpFile", h.DownloadByKind("ztpFile"))
		rg.GET("/configFile", h.DownloadByKind("configFile"))
		rg.GET("/upgradeFile", h.DownloadByKind("upgradeFile"))
		rg.GET("/mNormalFile", h.DownloadByKind("mNormalFile"))
		rg.GET("/ca/downloadFile/:fileId", h.DownloadByKind("ca"))
	}
}
