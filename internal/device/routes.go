package device

import "github.com/gin-gonic/gin"

// RegisterRoutes registers device management routes aligned with Java
// GNBMonitorManagementController, ENBMonitorManagementController and
// CPEMonitorManagementController (all base @RequestMapping("/api/v2/")).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// ===== GNBMonitorManagementController =====
	rg.POST("/listGNB", h.ListDevices)
	rg.POST("/viewGnbDetail", h.GetDevice)
	rg.POST("/addGnb", h.CreateDevice)
	rg.POST("/updateBaseStationInfo", h.UpdateDevice)
	rg.POST("/deleteTR069Device", h.DeleteDevice)
	rg.POST("/importGnbs", h.ImportDevices)
	rg.POST("/emptyTR069DeviceCommand", h.EmptyCommands)

	// TODO: rg.POST("/listElementBasicInfo", h.ListElementBasicInfo) // handler not yet implemented
	// TODO: rg.POST("/addDeviceToTenancy", h.AddDeviceToTenancy) // handler not yet implemented
	// TODO: rg.POST("/removeDeviceFromTenancy", h.RemoveDeviceFromTenancy) // handler not yet implemented
	// TODO: rg.POST("/getElementBasicInfoByIdIn", h.GetElementBasicInfoByIdIn) // handler not yet implemented
	// TODO: rg.POST("/getAllSoftwareVersion", h.GetAllSoftwareVersion) // handler not yet implemented
	// TODO: rg.POST("/getAllHardwareVersion", h.GetAllHardwareVersion) // handler not yet implemented
	// TODO: rg.POST("/getAllProductType", h.GetAllProductType) // handler not yet implemented
	// TODO: rg.POST("/getAllModelName", h.GetAllModelName) // handler not yet implemented
	// TODO: rg.POST("/gnbStatistics", h.GnbStatistics) // handler not yet implemented
	// TODO: rg.POST("/downloadGNBImportTemplate", h.DownloadGNBImportTemplate) // handler not yet implemented
	// TODO: rg.POST("/deleteCPEDevice", h.DeleteCPEDevice) // handler not yet implemented
	// TODO: rg.POST("/emptyCPECommand", h.EmptyCPECommand) // handler not yet implemented
	// TODO: rg.POST("/syncAlarm", h.SyncAlarm) // handler not yet implemented
	// TODO: rg.POST("/manualSyncAlarm", h.ManualSyncAlarm) // handler not yet implemented
	// TODO: rg.POST("/getBaseStationInfo", h.GetBaseStationInfo) // handler not yet implemented
	// TODO: rg.POST("/gNBFactoryReset", h.GNBFactoryReset) // handler not yet implemented
	// TODO: rg.POST("/eNBFactoryReset", h.ENBFactoryReset) // handler not yet implemented
	// TODO: rg.POST("/cpeFactoryReset", h.CPEFactoryReset) // handler not yet implemented

	// ===== ENBMonitorManagementController =====
	// TODO: rg.POST("/listENB", h.ListENB) // handler not yet implemented
	// TODO: rg.POST("/enbStatistics", h.EnbStatistics) // handler not yet implemented
	// TODO: rg.POST("/addEnb", h.AddEnb) // handler not yet implemented
	// TODO: rg.POST("/importEnbs", h.ImportEnbs) // handler not yet implemented

	// ===== CPEMonitorManagementController =====
	// TODO: rg.POST("/listAllCPESoftwareVersion", h.ListAllCPESoftwareVersion) // handler not yet implemented
	// TODO: rg.POST("/listAllCPEProductType", h.ListAllCPEProductType) // handler not yet implemented
	// TODO: rg.POST("/listAllCPEProductModel", h.ListAllCPEProductModel) // handler not yet implemented
	// TODO: rg.POST("/listCPE", h.ListCPE) // handler not yet implemented
	// TODO: rg.POST("/addCPE", h.AddCPE) // handler not yet implemented
	// TODO: rg.POST("/downloadCPEImportTemplate", h.DownloadCPEImportTemplate) // handler not yet implemented
	// TODO: rg.POST("/importCPEs", h.ImportCPEs) // handler not yet implemented
	// TODO: rg.POST("/cpeStatistics", h.CPEStatistics) // handler not yet implemented
	// TODO: rg.POST("/viewCPEDetail", h.ViewCPEDetail) // handler not yet implemented
	// TODO: rg.POST("/cpeKpiDay", h.CpeKpiDay) // handler not yet implemented
	// TODO: rg.POST("/listCPEStatisticItem", h.ListCPEStatisticItem) // handler not yet implemented
	// TODO: rg.POST("/getCPEPCIInfo", h.GetCPEPCIInfo) // handler not yet implemented
	// TODO: rg.POST("/savePCILock", h.SavePCILock) // handler not yet implemented

	// ===== Device Group (Go-specific, not in Java controllers) =====
	rg.POST("/listDeviceGroup", h.ListGroups)
	rg.POST("/addDeviceGroup", h.CreateGroup)
	rg.POST("/updateDeviceGroup", h.UpdateGroup)
	rg.POST("/deleteDeviceGroup", h.DeleteGroup)
}