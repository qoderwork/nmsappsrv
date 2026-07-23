package mr

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts the MR management/report endpoints under the
// authenticated API group (Java's /api/v2/ prefix → nmsappsrv /api/v1).
// Routes mirror the Java backend MRManagementController.
// The device upload (/acs-file-server/mr) lives in filebase and calls the
// mr Service via the filebase.MRIngester interface.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/getMRStatisticDataForData", h.GetStatisticData)
	rg.POST("/generateMRExcelForNR", h.GenerateCSV)
	rg.GET("/downloadMRReportExcel", h.DownloadReport)
	// TODO: rg.POST("/addMRUploadTask", h.AddMRUploadTask) // handler not yet implemented
	// TODO: rg.POST("/deleteMRUploadTask", h.DeleteMRUploadTask) // handler not yet implemented
	// TODO: rg.POST("/viewMRUploadTask", h.ViewMRUploadTask) // handler not yet implemented
	// TODO: rg.POST("/listMRUploadTask", h.ListMRUploadTask) // handler not yet implemented
	rg.GET("/downloadMRFile", h.DownloadRaw)
	// TODO: rg.POST("/listMRUploadLatestTime", h.ListMRUploadLatestTime) // handler not yet implemented
	rg.POST("/listMRLogs", h.ListLogs)
}
