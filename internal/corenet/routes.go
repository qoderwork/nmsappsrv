package corenet

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all core network routes on the given router group.
// Mirrors Java CoreNetworkManagementController (base @RequestMapping("/api/v2/"),
// all POST) and CoreNetworkKPIManagementController (absolute paths).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// CoreNetworkManagementController (base @RequestMapping("/api/v2/")) - ALL POST
	rg.POST("/api/v2/addCoreNetwork", h.CreateCoreNetwork)
	rg.POST("/api/v2/listCoreNetwork", h.ListCoreNetworks)
	rg.POST("/api/v2/deleteCoreNetwork", h.DeleteCoreNetwork)
	rg.POST("/api/v2/getCoreNetworkElementSystemState", h.GetCoreNetworkElementSystemState)
	rg.POST("/api/v2/getCoreNetworkAlarms", h.GetCoreNetworkAlarms)
	rg.POST("/api/v2/getCoreNetworkParameters", h.GetCoreNetworkParameters)
	rg.POST("/api/v2/setCoreNetworkParameters", h.SetCoreNetworkParameters)
	rg.POST("/api/v2/queryCoreNetworkParameters", h.QueryCoreNetworkParameters)
	rg.POST("/api/v2/deleteCoreNetworkParameter", h.DeleteCoreNetworkParameter)
	rg.POST("/api/v2/addCoreNetworkParameter", h.AddCoreNetworkParameter)
	rg.POST("/api/v2/listCoreNetworkLogs", h.ListOperationLogs)
	rg.POST("/api/v2/listUEList", h.ListUEList)
	rg.POST("/api/v2/listUENumberStatistic", h.ListUENumberStatistic)
	rg.POST("/api/v2/downloadPCFUETemplate", h.DownloadPCFUETemplate)
	rg.POST("/api/v2/importPCFUE", h.ImportPCFUE)
	rg.POST("/api/v2/updatePCFUE", h.UpdatePCFUE)
	rg.POST("/api/v2/deletePCFUE", h.DeletePCFUE)
	rg.POST("/api/v2/getUeInfos", h.GetUeInfos)
	rg.POST("/api/v2/changeCoreNetworkSwitch", h.ChangeCoreNetworkSwitch)

	// CoreNetworkKPIManagementController (no class-level base path, absolute paths)
	rg.POST("/rest/performanceManagement/v1/elementType/:elementTypeValue/objectType/kpiReport/:index", h.GetKpiReport)
	rg.POST("/api/v2/getCoreNetworkUserInfo", h.GetCoreNetworkUserInfo)
	rg.POST("/api/v2/getCoreNetworkUpfTraffic", h.GetCoreNetworkUpfTraffic)
	rg.POST("/api/v2/getBuiltInCoreNetworkUpfTraffic", h.GetBuiltInCoreNetworkUpfTraffic)
	rg.POST("/api/v2/getBuiltInCoreNetworkUserInfo", h.GetBuiltInCoreNetworkUserInfo)
}
