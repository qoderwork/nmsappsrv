package cacert

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all CA/certificate routes that require authentication.
// Routes mirror the Java backend controllers:
//   - CaTaskController (all POST)
//   - CaFileConsoleController (all POST)
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// CaTaskController (all POST)
	rg.POST("/catask/save", h.SaveCaTask)
	rg.POST("/catask/list", h.ListCaTasks)
	rg.POST("/catask/detail", h.GetCaTaskDetail)
	rg.POST("/catask/delete", h.DeleteCaTask)
	rg.POST("/catask/queryDeviceSendCaLog", h.QueryDeviceSendCaLog)

	// CaFileConsoleController (all POST)
	rg.POST("/caFile/list", h.ListCaFiles)
	rg.POST("/caFile/delete", h.DeleteCaFile)
	rg.POST("/caFile/queryCaList", h.QueryCaList)
}
