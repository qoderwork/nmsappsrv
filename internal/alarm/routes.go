package alarm

import "github.com/gin-gonic/gin"

// RegisterRoutes registers ALL alarm management endpoints in Java AlarmManagementController
// wire-compatible shape — BASE PATH is /api/v2 (attached from main.go), every endpoint
// name is byte-for-byte the Java @PostMapping/@GetMapping value.
//
// Java Controller reference:
// smallcell_nms_v4_appserv/api/station-new/src/main/java/com/waveoss/stationapinew/controller/AlarmManagementController.java
//   @RestController
//   @RequestMapping("/api/v2/")
//   public class AlarmManagementController { … }
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// ----- Alarm CRUD / state transitions -----
	rg.POST("listAlarm", h.ListAlarms)
	rg.POST("confirmAlarm", h.ConfirmAlarm)
	rg.POST("unconfirmAlarm", h.UnconfirmAlarm)
	rg.POST("clearAlarm", h.ClearAlarm)
	rg.POST("deleteAlarm", h.DeleteAlarm)

	// ----- Alarm statistics & count -----
	rg.POST("queryAlarmStatisticResult", h.QueryAlarmStatisticResult)
	rg.POST("queryAlarmStatisticTopN", h.QueryAlarmStatisticTopN)
	rg.POST("getCountOfSeverity", h.GetSeverityCount)

	// ----- Alarm filter tasks -----
	rg.POST("addAlarmFilterTask", h.AddAlarmFilterTask)
	rg.POST("listAlarmFilterTask", h.ListAlarmFilters)
	rg.POST("updateAlarmFilterTask", h.UpdateAlarmFilter)
	rg.POST("deleteAlarmFilterTask", h.DeleteAlarmFilter)
	rg.POST("viewAlarmFilterTask", h.GetAlarmFilter)
	rg.POST("enableAlarmFilterTask", h.EnableAlarmFilterTask)
	rg.POST("disableAlarmFilterTask", h.DisableAlarmFilterTask)

	// ----- Alarm library (import / list / delete) + template download -----
	rg.POST("importAlarmLibrary", h.ImportAlarmLibrary)
	rg.POST("deleteAlarmLibrary", h.DeleteAlarmLibrary)
	rg.POST("listAlarmLibrary", h.ListAlarmLibrary)
	rg.GET("downloadAlarmLibraryTemplate", h.DownloadAlarmLibraryTemplate)

	// ----- Alarm template -----
	rg.POST("addAlarmTemplate", h.CreateAlarmTemplate)
	rg.POST("listAlarmTemplate", h.ListAlarmTemplates)
	rg.POST("viewAlarmTemplate", h.GetAlarmTemplate)
	rg.POST("updateAlarmTemplate", h.UpdateAlarmTemplate)
	rg.POST("deleteAlarmTemplate", h.DeleteAlarmTemplate)
	rg.POST("updateEmailNotificationEnableInTemplate", h.UpdateEmailNotificationEnableInTemplate)

	// ----- Email notification config -----
	rg.POST("updateEmailNotificationConfig", h.UpdateEmailNotificationConfig)
	rg.POST("listEmailNotificationConfig", h.ListEmailNotificationConfig)
	rg.POST("listEmailNoticeResult", h.ListEmailNoticeResult)

	// ----- Misc alarm helpers -----
	rg.POST("listActiveAlarmProbableCause", h.ListActiveAlarmProbableCause)
	rg.POST("getAlarmEventType", h.GetAlarmEventType)
	rg.POST("getAlarmSyncConfig", h.GetAlarmSyncConfig)
	rg.POST("updateAlarmSyncConfig", h.UpdateAlarmSyncConfig)
	rg.POST("addCommentForAlarm", h.AddCommentForAlarm)

	// Note: Alarm sync (old Go REST /alarms/sync) has no Java counterpart in
	// AlarmManagementController, so it is intentionally removed per route B-plan.
}
