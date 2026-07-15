package mr

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts the MR management/report endpoints under the
// authenticated API group (Java's /api/v2/ prefix → nmsappsrv /api/v1).
// The device upload (/acs-file-server/mr) lives in filebase and calls the
// mr Service via the filebase.MRIngester interface.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/getMRStatisticDataForData", h.GetStatisticData)
	rg.POST("/generateMRCSVForNR", h.GenerateCSV)
	rg.GET("/downloadMRReportExcel", h.DownloadReport)
	rg.POST("/listMRLogs", h.ListLogs)
	rg.GET("/downloadMRFile", h.DownloadRaw)
}
