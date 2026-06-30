package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nmsappsrv/internal/alarm"
	"nmsappsrv/internal/blacklist"
	"nmsappsrv/internal/cacert"
	"nmsappsrv/internal/cbsd"
	"nmsappsrv/internal/config"
	"nmsappsrv/internal/corenet"
	"nmsappsrv/internal/dashboard"
	"nmsappsrv/internal/device"
	"nmsappsrv/internal/devicelog"
	"nmsappsrv/internal/diagnostics"
	"nmsappsrv/internal/eventlog"
	"nmsappsrv/internal/health"
	"nmsappsrv/internal/heartbeat"
	"nmsappsrv/internal/ha"
	"nmsappsrv/internal/license"
	"nmsappsrv/internal/mail"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/mml"
	"nmsappsrv/internal/monitor"
	"nmsappsrv/internal/nmsbackup"
	"nmsappsrv/internal/ntp"
	"nmsappsrv/internal/parameter"
	"nmsappsrv/internal/paramcompare"
	"nmsappsrv/internal/parammonitor"
	"nmsappsrv/internal/platform"
	"nmsappsrv/internal/pm"
	"nmsappsrv/internal/reboot"
	"nmsappsrv/internal/reset"
	"nmsappsrv/internal/resources"
	"nmsappsrv/internal/scheduler"
	"nmsappsrv/internal/restapi"
	"nmsappsrv/internal/security"
	"nmsappsrv/internal/site"
	"nmsappsrv/internal/snmp"
	sshmod "nmsappsrv/internal/ssh"
	"nmsappsrv/internal/websocket"
	"nmsappsrv/internal/systemsettings"
	"nmsappsrv/internal/tenancy"
	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/upgrade"
	"nmsappsrv/internal/user"
	"nmsappsrv/pkg/database"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
)

func main() {
	// 命令行参数
	configPath := flag.String("config", "", "config file path")
	flag.Parse()

	// 1. 加载配置
	var cfg *config.Config
	var err error
	if *configPath != "" {
		cfg, err = config.Load(*configPath)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 2. 初始化数据库
	if err := database.Init(database.Config{
		Host:         cfg.DB.Host,
		Port:         cfg.DB.Port,
		User:         cfg.DB.User,
		Password:     cfg.DB.Password,
		DBName:       cfg.DB.DBName,
		Charset:      cfg.DB.Charset,
		MaxIdleConns: cfg.DB.MaxIdleConns,
		MaxOpenConns: cfg.DB.MaxOpenConns,
		LogLevel:     cfg.DB.LogLevel,
	}); err != nil {
		logger.Fatalf("database init failed: %v", err)
	}

	// 3. AutoMigrate 所有表
	if err := database.AutoMigrateAll(); err != nil {
		logger.Fatalf("auto migrate failed: %v", err)
	}

	// 4. 初始化Redis
	if err := redis.Init(redis.Config{
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
	}); err != nil {
		logger.Fatalf("redis init failed: %v", err)
	}

	// 5. 设置Gin
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// 全局中间件
	router.Use(gin.Recovery())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.TenancyMiddleware())

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		utils.OK(c, "ok")
	})

	// ========== 初始化所有模块Handler ==========
	db := database.DB
	deviceH := device.NewHandler(db)
	alarmH := alarm.NewHandler(db)
	userH := user.NewHandler(db)
	licenseH := license.NewHandler(db)
	parameterH := parameter.NewHandler(db)
	upgradeH := upgrade.NewHandler(db)
	eventlogH := eventlog.NewHandler(db)
	pmH := pm.NewHandler(db)
	monitorH := monitor.NewHandler(db)
	siteH := site.NewHandler(db)
	mmlH := mml.NewHandler(db)
	cbsdH := cbsd.NewHandler(db)
	corenetH := corenet.NewHandler(db)
	miscH := misc.NewHandler(db)
	miscH.EnqueueZTPFunc = tr069.EnqueueZTPProvision
	diagnosticsH := diagnostics.NewHandler(db)
	rebootH := reboot.NewHandler(db)
	resetH := reset.NewHandler(db)
	blacklistH := blacklist.NewHandler(db)
	ntpH := ntp.NewHandler(db)
	sshH := sshmod.NewHandler(db)
	mailH := mail.NewHandler(db, cfg.Mail.AESKey)
	mailSvc := mail.NewService(db, cfg.Mail.AESKey)
	alarmNotifier := alarm.NewAlarmNotifier(db, mailSvc)
	securityH := security.NewHandler(db)
	shutdownH := upgrade.NewShutdownHandler(db)
	systemsettingsH := systemsettings.NewSystemSettingsHandler(db, cfg.Mail.AESKey)
	parammonitorH := parammonitor.NewHandler(db)
	parammonitorH.StartThresholdChecker()
	paramcompareH := paramcompare.NewHandler(db)
	devicelogH := devicelog.NewHandler(db)
	nmsbackupRepo := nmsbackup.NewRepository(db)
	nmsbackupSvc := nmsbackup.NewService(nmsbackupRepo)
	nmsbackupH := nmsbackup.NewHandler(nmsbackupSvc)

	// Unified cron scheduler — manages all cron-based periodic jobs
	mainScheduler := scheduler.NewScheduler(db)

	// Register NMS backup cron jobs from stored cron_expr
	backupSched := nmsbackup.NewBackupScheduler(nmsbackupRepo, nmsbackupSvc)
	backupSched.RegisterBackupJobs(mainScheduler)
	restapiRepo := restapi.NewRepository(db)
	restapiSvc := restapi.NewService(restapiRepo)
	restapiH := restapi.NewHandler(restapiSvc)

	// Second batch modules
	healthH := health.NewHandler(db)
	heartbeatH := heartbeat.NewHandler(db, cfg)
	resourcesH := resources.NewHandler(db)
	platformH := platform.NewHandler(db, cfg.Mail.AESKey, cfg.PlatformFiles)
	tenancyH := tenancy.NewHandler(db)
	cacertH := cacert.NewHandler(db)
	dashboardH := dashboard.NewHandler(db)

	// ========== WebSocket ==========
	wsHub := websocket.NewHub()
	utils.SafeGo("ws-hub", func() { wsHub.Run() })
	wsH := websocket.NewWSHandler(wsHub)
	wsBridge := websocket.NewBridge(wsHub, db)
	wsBridge.Start()

	// WebSocket route (no auth required)
	router.GET("/ws", wsH.ServeWS)

	// 启动SSH Access Timer后台过期检查
	sshH.StartExpiredChecker()

	// TR069 ACS
	tr069MsgMgr := tr069.NewMessageManager()
	tr069EventProc := tr069.NewEventProcessor(db)
	tr069ACS := tr069.NewACSHandler(db, tr069MsgMgr, tr069EventProc, cfg.TR069)

	// ========== API路由组 ==========
	api := router.Group("/api/v1")
	{
		// ===== 认证（公开） =====
		api.POST("/login", userH.Login)
		api.POST("/logout", userH.Logout)

		// ===== 需要认证的路由 =====
		auth := api.Group("")
		auth.Use(middleware.AuthMiddleware())
		{
			// 设备管理
			auth.GET("/devices", deviceH.ListDevices)
			auth.GET("/devices/:id", deviceH.GetDevice)
			auth.POST("/devices", deviceH.CreateDevice)
			auth.PUT("/devices/:id", deviceH.UpdateDevice)
			auth.DELETE("/devices/:id", deviceH.DeleteDevice)
			auth.POST("/devices/import", deviceH.ImportDevices)

			// 设备组
			auth.GET("/device-groups", deviceH.ListGroups)
			auth.POST("/device-groups", deviceH.CreateGroup)
			auth.PUT("/device-groups/:id", deviceH.UpdateGroup)
			auth.DELETE("/device-groups/:id", deviceH.DeleteGroup)

			// 站点
			auth.GET("/sites", siteH.ListSites)
			auth.GET("/sites/basic", siteH.ListSiteBasicInfo)
			auth.POST("/sites", siteH.CreateSite)
			auth.PUT("/sites/:id", siteH.UpdateSite)
			auth.DELETE("/sites/:id", siteH.DeleteSite)

			// 告警
			auth.GET("/alarms", alarmH.ListAlarms)
			auth.GET("/alarms/:id", alarmH.GetAlarm)
			auth.POST("/alarms/:id/clear", alarmH.ClearAlarm)
			auth.PUT("/alarms/batch-clear", alarmH.BatchClearAlarms)
			auth.GET("/alarm-library", alarmH.ListAlarmLibrary)
			auth.GET("/alarm-templates", alarmH.ListAlarmTemplates)
			auth.POST("/alarm-templates", alarmH.CreateAlarmTemplate)
			auth.PUT("/alarm-templates/:id", alarmH.UpdateAlarmTemplate)
			auth.GET("/alarm-filters", alarmH.ListAlarmFilters)
			auth.POST("/alarm-filters", alarmH.CreateAlarmFilter)
			auth.PUT("/alarm-filters/:id", alarmH.UpdateAlarmFilter)
			auth.DELETE("/alarm-filters/:id", alarmH.DeleteAlarmFilter)

			// 参数管理
			auth.GET("/parameters/:elementId", parameterH.GetParameters)
			auth.PUT("/parameters/:elementId", parameterH.SetParameter)
			auth.GET("/parameter-logs", parameterH.ListParameterLogs)
			auth.GET("/parameter-sets", parameterH.ListParameterSets)
			auth.POST("/parameter-sets", parameterH.CreateParameterSet)
			auth.PUT("/parameter-sets/:id", parameterH.UpdateParameterSet)
			auth.DELETE("/parameter-sets/:id", parameterH.DeleteParameterSet)
			auth.GET("/parameter-templates", parameterH.ListParameterTemplates)
			auth.POST("/parameter-templates", parameterH.CreateParameterTemplate)
			auth.PUT("/parameter-templates/:id", parameterH.UpdateParameterTemplate)
			auth.POST("/parameter-templates/:templateId/deploy", parameterH.DeployTemplate)
			auth.GET("/parameter-backup-logs", parameterH.ListBackupLogs)
			auth.POST("/parameter-backup/:elementId", parameterH.TriggerBackup)
			auth.POST("/parameter-tasks", parameterH.BatchParameterConfigurationDirect)
			auth.GET("/batch-configurations", parameterH.ListBatchConfigurations)
			auth.GET("/batch-configurations/:taskId/detail", parameterH.ListBatchConfigurationDetail)

			// 升级管理
			auth.GET("/upgrade-files", upgradeH.ListUpgradeFiles)
			auth.POST("/upgrade-files", upgradeH.UploadUpgradeFile)
			auth.DELETE("/upgrade-files/:id", upgradeH.DeleteUpgradeFile)
			auth.POST("/upgrade-tasks", upgradeH.CreateUpgradeTask)
			auth.GET("/upgrade-tasks", upgradeH.ListUpgradeTasks)
			auth.GET("/upgrade-tasks/:id", upgradeH.GetUpgradeTask)
			auth.GET("/upgrade-logs", upgradeH.ListUpgradeLogs)
			auth.POST("/reboot-tasks", rebootH.AddRebootTask)
			auth.GET("/reboot-tasks", rebootH.ListRebootTasks)
			auth.DELETE("/reboot-tasks/:id", rebootH.DeleteRebootTask)
			auth.POST("/reboot-tasks/:id/start", rebootH.StartRebootTask)
			auth.POST("/reboot-tasks/:id/cancel", rebootH.CancelRebootTask)
			auth.GET("/reboot-tasks/:id/results", rebootH.ListRebootTaskResults)
			auth.POST("/rollback-tasks", upgradeH.CreateRollbackTask)
			auth.GET("/rollback-tasks", upgradeH.ListRollbackTasks)

			// 性能监控
			auth.GET("/pm/kpis", pmH.ListKPIs)
			auth.GET("/pm/kpis/:id", pmH.GetKPI)
			auth.POST("/pm/kpis", pmH.CreateKPI)
			auth.PUT("/pm/kpis/:id", pmH.UpdateKPI)
			auth.DELETE("/pm/kpis/:id", pmH.DeleteKPI)
			auth.GET("/pm/kpi-sets", pmH.ListKPISets)
			auth.POST("/pm/kpi-sets", pmH.CreateKPISet)
			auth.GET("/pm/kpi-templates", pmH.ListKPITemplates)
			auth.POST("/pm/kpi-templates", pmH.CreateKPITemplate)
			auth.PUT("/pm/kpi-templates/:id", pmH.UpdateKPITemplate)
			auth.DELETE("/pm/kpi-templates/:id", pmH.DeleteKPITemplate)
			auth.GET("/pm/data", pmH.GetDashboardData)
			auth.GET("/pm/file-logs", pmH.ListPMFileLogs)
			auth.GET("/pm/kpi-alarms", pmH.ListKPIAlarms)
			auth.POST("/pm/kpi-alarms", pmH.CreateKPIAlarm)
			auth.PUT("/pm/kpi-alarms/:id", pmH.UpdateKPIAlarm)
			auth.DELETE("/pm/kpi-alarms/:id", pmH.DeleteKPIAlarm)
			auth.GET("/pm/dashboard", pmH.GetDashboardData)
			auth.GET("/pm/pdcp-traffic", pmH.GetPDCPTraffic)
			auth.GET("/pm/device-online-info", pmH.GetDeviceOnlineInfo)
			auth.GET("/pm/product-type-device-count", pmH.GetProductTypeAndDeviceCount)

			// 监控任务
			auth.GET("/monitor-tasks", monitorH.ListMonitorTasks)
			auth.GET("/monitor-tasks/:id", monitorH.GetMonitorTask)
			auth.POST("/monitor-tasks", monitorH.CreateMonitorTask)
			auth.PUT("/monitor-tasks/:id", monitorH.UpdateMonitorTask)
			auth.DELETE("/monitor-tasks/:id", monitorH.DeleteMonitorTask)
			auth.GET("/monitor-data", monitorH.GetMonitorData)
			auth.GET("/monitor-elements", monitorH.GetMonitorElements)
			auth.PUT("/monitor-elements", monitorH.SaveMonitorElements)
			auth.GET("/monitor-parameters", monitorH.GetMonitorParameters)
			auth.PUT("/monitor-parameters", monitorH.SaveMonitorParameters)

			// MML命令
			auth.GET("/mml-sets", mmlH.ListMmlSets)
			auth.GET("/mml-commands", mmlH.ListMmlCommands)
			auth.GET("/mml-commands/:id/params", mmlH.GetMmlCommandParams)
			auth.POST("/mml-execute", mmlH.ExecuteMml)
			auth.GET("/mml-results", mmlH.ListMmlResults)
			auth.GET("/mml-results/:id", mmlH.GetMmlResult)

			// 核心网
			auth.GET("/core-networks", corenetH.ListCoreNetworks)
			auth.GET("/core-networks/:id", corenetH.GetCoreNetwork)
			auth.POST("/core-networks", corenetH.CreateCoreNetwork)
			auth.PUT("/core-networks/:id", corenetH.UpdateCoreNetwork)
			auth.DELETE("/core-networks/:id", corenetH.DeleteCoreNetwork)
			auth.GET("/core-networks/:id/data", corenetH.GetCoreNetworkData)
			auth.PUT("/core-networks/:id/data", corenetH.SaveCoreNetworkData)
			auth.GET("/core-networks/:id/kpis", corenetH.GetCoreNetworkKpis)
			auth.GET("/core-networks/:id/statistics", corenetH.GetStatisticData)
			auth.GET("/core-network-logs", corenetH.ListOperationLogs)

			// CBSD (SAS)
			auth.GET("/cbsd", cbsdH.ListCBSD)
			auth.GET("/cbsd/:id", cbsdH.GetCBSD)
			auth.POST("/cbsd/register", cbsdH.RegisterCBSD)
			auth.PUT("/cbsd/:id", cbsdH.UpdateCBSD)
			auth.POST("/cbsd/deregister", cbsdH.DeregisterCBSD)
			auth.GET("/cbsd-logs", cbsdH.ListCBSDLogs)
			auth.POST("/cbsd/cert-tasks", cbsdH.CreateCertFileSendTask)
			auth.GET("/cbsd/cert-tasks", cbsdH.ListCertFileSendTasks)

			// 批量操作
			auth.POST("/batch-backup", miscH.CreateBackup)
			auth.POST("/batch-restore", miscH.CreateRestore)
			auth.GET("/batch-tasks", miscH.ListBackupRestoreTasks)
			auth.POST("/batch-tasks/start", miscH.StartBackupRestoreTask)
			auth.POST("/batch-tasks/cancel", miscH.CancelBackupRestoreTask)
			auth.GET("/batch-tasks/:taskId/detail", miscH.ListBackupRestoreTaskDetail)
			auth.GET("/batch-logs", miscH.ListBatchConfigLogs)

			// Batch Add Object (TR069)
			auth.POST("/batch-add-object", miscH.BatchAddObject)
			auth.GET("/batch-add-object/tasks", miscH.ListBatchAddObjectTasks)
			auth.GET("/batch-add-object/tasks/:taskId/detail", miscH.ListBatchAddObjectTaskDetail)

			// ZTP
			auth.POST("/ztp/provision", miscH.ProvisionZTP)
			auth.GET("/ztp/logs", miscH.ListZTPLogs)
			auth.GET("/ztp/setting", miscH.GetZTPSetting)
			auth.POST("/ztp/setting", miscH.SaveZTPSetting)
			auth.POST("/ztp/results", miscH.ListZTPResults)
			auth.POST("/ztp/retry-logs", miscH.ListZTPRetryLogs)
			auth.POST("/ztp/history-files", miscH.ListHistoryZTPFiles)
			auth.POST("/ztp/status", miscH.SetZTPStatus)
			auth.POST("/ztp/batch-reztp", miscH.BatchReZTP)
			auth.POST("/ztp/delete-files", miscH.DeleteZTPFiles)

			// 用户管理
			auth.GET("/users", userH.ListUsers)
			auth.POST("/users", userH.CreateUser)
			auth.PUT("/users/:id", userH.UpdateUser)
			auth.DELETE("/users/:id", userH.DeleteUser)
			auth.POST("/users/kick-out", userH.KickOutUser)
			auth.POST("/users/unlock", userH.UnlockUser)
			auth.POST("/users/modify-password", userH.ModifyPassword)
			auth.POST("/users/enable", userH.EnableUser)
			auth.POST("/users/disable", userH.DisableUser)
			auth.POST("/users/reset-password", userH.ResetPassword)
			auth.POST("/users/reset-password-by-link", userH.ResetPasswordByLink)
			auth.POST("/users/set-tenancy", userH.SetTenancyForUser)
			auth.POST("/users/login-failed-times", userH.GetLoginFailedTimes)
			auth.GET("/users/need-change-password", userH.NeedChangePassword)
			auth.GET("/roles", userH.ListRoles)
			auth.POST("/roles", userH.CreateRole)
			auth.PUT("/roles/:id", userH.UpdateRole)
			auth.DELETE("/roles/:id", userH.DeleteRole)
			auth.GET("/roles/:id/permissions", userH.GetRolePermissions)
			auth.PUT("/roles/:id/permissions", userH.UpdateRolePermissions)

			// License
			auth.GET("/license", licenseH.GetLicense)
			auth.GET("/licenses", licenseH.ListLicenses)
			auth.PUT("/license", licenseH.UpdateLicense)
			auth.GET("/license/sas-config", licenseH.GetSASConfig)
			auth.POST("/license/sas-config", licenseH.SaveSASConfig)
			auth.GET("/license/entra-endpoints", licenseH.ListEntraEndpoints)
			auth.POST("/license/entra-endpoints", licenseH.CreateEntraEndpoint)
			auth.PUT("/license/entra-endpoints/:id", licenseH.UpdateEntraEndpoint)
			auth.DELETE("/license/entra-endpoints/:id", licenseH.DeleteEntraEndpoint)

			// 事件日志
			auth.GET("/event-logs", eventlogH.ListEventLogs)
			auth.GET("/event-logs/:id", eventlogH.GetEventLog)
			auth.GET("/event-logs/task/:taskId", eventlogH.ListTaskEventLogs)

			// 系统
			auth.GET("/system/config", siteH.GetSystemConfig)
			auth.PUT("/system/config", siteH.UpdateSystemConfig)
			auth.GET("/system/areas", siteH.ListAreas)
			auth.GET("/system/areas/:id", siteH.GetArea)
			auth.POST("/system/areas", siteH.CreateArea)
			auth.PUT("/system/areas/:id", siteH.UpdateArea)
			auth.DELETE("/system/areas/:id", siteH.DeleteArea)
			auth.GET("/system/parameters", siteH.ListSystemParameters)
			auth.PUT("/system/parameters", siteH.UpdateSystemParameter)
			auth.GET("/system/operator-logs", miscH.ListOperatorLogs)

			// MR (Measurement Report)
			auth.GET("/mr-data", miscH.ListMRData)

			// CPE 诊断
			auth.POST("/diagnostics/result", diagnosticsH.ListDiagnosticsResult)
			auth.POST("/diagnostics/status", diagnosticsH.ListDiagnosticsStatus)
			auth.POST("/diagnostics/ping", diagnosticsH.DiagnosticsPing)
			auth.POST("/diagnostics/trace-route", diagnosticsH.DiagnosticsTraceRoute)
			auth.POST("/diagnostics/download", diagnosticsH.DiagnosticsDownload)
			auth.POST("/diagnostics/upload", diagnosticsH.DiagnosticsUpload)

			// 重启恢复
			auth.POST("/reset-tasks", resetH.AddResetTask)
			auth.GET("/reset-tasks", resetH.ListResetTasks)
			auth.DELETE("/reset-tasks/:id", resetH.DeleteResetTask)
			auth.POST("/reset-tasks/:id/start", resetH.StartResetTask)
			auth.POST("/reset-tasks/:id/cancel", resetH.CancelResetTask)
			auth.GET("/reset-tasks/:id/results", resetH.ListResetTaskResults)

			// 设备黑名单
			auth.POST("/blacklist/list", blacklistH.ListDeviceBlackList)
			auth.POST("/blacklist/add", blacklistH.AddDeviceToBlackList)
			auth.POST("/blacklist/delete", blacklistH.DeleteDeviceFromBlackList)
			auth.POST("/blacklist/batch-delete", blacklistH.BatchDeleteDeviceFromBlackList)
			auth.POST("/blacklist/operation-logs", blacklistH.ListBlackListOperationLog)

			// 北向接口
			auth.GET("/north-reports", miscH.ListNorthReports)
			auth.POST("/north-reports", miscH.CreateNorthReport)
			auth.PUT("/north-reports/:id", miscH.UpdateNorthReport)
			auth.DELETE("/north-reports/:id", miscH.DeleteNorthReport)

			// RADIUS
			auth.GET("/radius", miscH.ListRadius)
			auth.POST("/radius", miscH.SaveRadius)
			auth.DELETE("/radius/:id", miscH.DeleteRadius)

			// 文件上传
			auth.GET("/upload-files", miscH.ListUploadFiles)
			auth.POST("/upload-files", miscH.CreateUploadFile)
			auth.DELETE("/upload-files/:id", miscH.DeleteUploadFile)

			// NTP
			auth.POST("/listNTPConfig", ntpH.ListNTPConfig)
			auth.POST("/updateNTPConfig", ntpH.UpdateNTPConfig)
			auth.POST("/getNTPStatus", ntpH.GetNTPStatus)

			// SSH Label
			auth.POST("/addSSHLabel", sshH.AddSSHLabel)
			auth.POST("/deleteSSHLabel", sshH.DeleteSSHLabel)
			auth.POST("/listSSHLabels", sshH.ListSSHLabels)
			auth.POST("/updateSSHLabel", sshH.UpdateSSHLabel)

			// SSH Access Timer
			auth.POST("/sshAccessTimer", sshH.SetSSHAccessTimer)
			auth.POST("/listSSHAccessTimer", sshH.ListSSHAccessTimer)

			// Mail
			auth.POST("/listMailConfig", mailH.ListMailConfig)
			auth.POST("/updateMailConfig", mailH.UpdateMailConfig)
			auth.POST("/testMail", mailH.TestMail)
			auth.POST("/getEmailCode", mailH.GetEmailCode)
			auth.POST("/checkEmailCode", mailH.CheckEmailCode)
			auth.POST("/isEnabledEmailAuthentication", mailH.IsEnabledEmailAuthentication)

			// Security Rules
			auth.POST("/getSecurityRule", securityH.GetSecurityRule)
			auth.POST("/updateSecurityRule", securityH.UpdateSecurityRule)
			auth.GET("/getPasswordStrategy", securityH.GetPasswordStrategy)

			// 关机管理 (Shutdown Management)
			auth.POST("/shutdown-tasks", shutdownH.AddShutdownTask)
			auth.GET("/shutdown-tasks", shutdownH.ListShutdownTasks)
			auth.GET("/shutdown-tasks/:id", shutdownH.ViewShutdownTask)
			auth.DELETE("/shutdown-tasks/:id", shutdownH.DeleteShutdownTask)
			auth.GET("/shutdown-tasks/:id/results", shutdownH.ListShutdownResults)

			// 系统设置 (System Settings)
			auth.GET("/settings/device", systemsettingsH.ListDeviceSettings)
			auth.PUT("/settings/device", systemsettingsH.UpdateDeviceSettings)
			auth.GET("/settings/acs", systemsettingsH.ListACSSettings)
			auth.PUT("/settings/acs", systemsettingsH.UpdateACSSettings)
			auth.GET("/settings/log", systemsettingsH.ListLogSettings)
			auth.PUT("/settings/log", systemsettingsH.UpdateLogSettings)

			// 参数监控 (Parameter Monitor)
			auth.POST("/param-monitor/configs", parammonitorH.AddMonitorConfig)
			auth.POST("/param-monitor/configs/delete", parammonitorH.DeleteMonitorConfig)
			auth.POST("/param-monitor/configs/view", parammonitorH.ViewMonitorConfig)
			auth.PUT("/param-monitor/configs", parammonitorH.UpdateMonitorConfig)
			auth.GET("/param-monitor/configs", parammonitorH.ListMonitorConfigs)
			auth.POST("/param-monitor/configs/toggle", parammonitorH.ToggleMonitorConfig)
			auth.POST("/param-monitor/realtime", parammonitorH.GetRealtimeMonitorData)
			auth.POST("/param-monitor/reload", parammonitorH.ReloadMonitorParameters)
			auth.POST("/param-monitor/batch-query", parammonitorH.BatchQueryDeviceParameters)
			auth.POST("/param-monitor/batch-query-live", parammonitorH.BatchQueryDeviceParametersLive)
				// Parameter Compare
				auth.POST("/param-compare/compare", paramcompareH.Compare)
				auth.POST("/param-compare/batch", paramcompareH.BatchCompare)
				auth.GET("/param-compare/templates", paramcompareH.ListTemplates)

				// 参数监控阈值告警 (Parameter Monitor Threshold Alerts)
				auth.POST("/param-monitor/threshold", parammonitorH.CreateThresholdRule)
				auth.PUT("/param-monitor/threshold/:id", parammonitorH.UpdateThresholdRule)
				auth.DELETE("/param-monitor/threshold/:id", parammonitorH.DeleteThresholdRule)
				auth.GET("/param-monitor/threshold", parammonitorH.ListThresholdRules)
				auth.GET("/param-monitor/threshold/:id", parammonitorH.GetThresholdRule)
				auth.POST("/param-monitor/threshold/test", parammonitorH.TestThresholdRule)

			// 设备日志采集 (Device Log Collection)
			auth.POST("/device-log/collection", devicelogH.AddLogCollectionTask)
			auth.POST("/device-log/collection/results", devicelogH.ListLogCollectionResults)
			auth.POST("/device-log/delete-all", devicelogH.DeleteAllLogFile)
			auth.POST("/device-log/delete", devicelogH.DeleteLogFile)
			auth.POST("/device-log/download", devicelogH.DownloadLogFile)
			auth.POST("/device-log/files", devicelogH.ListLogFiles)
			auth.POST("/device-log/periodic/enable", devicelogH.EnablePeriodicUpload)
			auth.POST("/device-log/periodic/disable", devicelogH.DisablePeriodicUpload)

			// 基站配置备份与恢复 (Base Station Backup & Restore)
			auth.GET("/bs-backup/info", miscH.ListBaseStationBackupInfo)
			auth.POST("/bs-backup/import-config", miscH.ImportConfigFile)
			auth.POST("/bs-backup/export-config", miscH.ExportConfigFile)
			auth.POST("/bs-backup/backup-tasks", miscH.AddBSBackupTask)
			auth.POST("/bs-backup/backup-tasks/cancel", miscH.CancelBackupTask)
			auth.POST("/bs-backup/restore-tasks/cancel", miscH.CancelRestoreTask)
			auth.POST("/bs-backup/restore-tasks", miscH.AddBSRestoreTask)
			auth.POST("/bs-backup/tasks/start", miscH.StartBackupOrRestoreTask)
			auth.POST("/bs-backup/tasks", miscH.ListBSBackupTasks)
			auth.POST("/bs-backup/tasks/results", miscH.ListDeviceBackupResult)
			auth.POST("/bs-backup/download-config", miscH.DownloadConfigFile)

			// NMS备份与回退 (NMS Backup & Revert)
			auth.POST("/nms-backup/tasks", nmsbackupH.AddNMSBackupTask)
			auth.POST("/nms-backup/tasks/list", nmsbackupH.ListNMSBackupTask)
			auth.POST("/nms-backup/tasks/modify", nmsbackupH.ModifyNMSBackupTask)
			auth.POST("/nms-backup/tasks/run", nmsbackupH.RunNMSBackupTask)
			auth.POST("/nms-backup/tasks/delete", nmsbackupH.DeleteNMSBackupTask)
			auth.POST("/nms-backup/tasks/revert", nmsbackupH.RevertNMSBackupTask)
			auth.GET("/nms-backup/config", nmsbackupH.GetBackupAndRestoreConfig)
			auth.PUT("/nms-backup/config", nmsbackupH.UpdateBackupAndRestoreConfig)
			auth.POST("/nms-backup/logs", nmsbackupH.ListNMSBackupLogs)
			auth.POST("/nms-backup/logs/detail", nmsbackupH.GetNMSBackupLogDetail)

			// ========== Health Module ==========
			router.GET("/healthCheck", healthH.HealthCheck)
			auth.GET("/getMysqlInfo", healthH.GetMysqlInfo)
			auth.GET("/getRedisInfo", healthH.GetRedisInfo)
			auth.GET("/getQueueInfo", healthH.GetQueueInfo)
			auth.POST("/reportHAStatus", healthH.ReportHAStatus)
			router.HEAD("/reportHAStatus", healthH.ReportHAStatusHead)

			// ========== Resources Module ==========
			auth.POST("/cpu-mem-usage", resourcesH.GetCpuAndMemUsage)
			auth.GET("/table-status", resourcesH.GetTableStatus)
			auth.GET("/disk-usage", resourcesH.GetDiskUsage)
			auth.POST("/cpu-mem-threshold", resourcesH.SetCPUAndMemThreshold)
			auth.POST("/list-cpu-mem-threshold", resourcesH.ListCPUAndMemThreshold)

			// ========== Platform Settings Module ==========
			auth.GET("/getDate", platformH.GetDate)
			auth.GET("/getSupportedZone", platformH.GetSupportedZone)
			auth.GET("/getLogo", platformH.GetLogo)
			auth.POST("/listLogConfig", platformH.ListLogConfig)
			auth.POST("/updateLogConfig", platformH.UpdateLogConfig)
			auth.POST("/getFTPTransferLogConfig", platformH.GetFTPTransferLogConfig)
			auth.POST("/updateFTPTransferLogConfig", platformH.UpdateFTPTransferLogConfig)
			auth.POST("/getHECConfig", platformH.GetHECConfig)
			auth.POST("/updateHECConfig", platformH.UpdateHECConfig)
			auth.POST("/listNMSSecret", platformH.ListNMSSecret)
			auth.POST("/updateNMSSecret", platformH.UpdateNMSSecret)
			auth.GET("/downloadPasswordRSAPublicKey", platformH.DownloadPasswordRSAPublicKey)
			auth.GET("/downloadPlatformLogs", platformH.DownloadPlatformLogs)
			auth.GET("/downloadNMSManualDocument", platformH.DownloadNMSManualDocument)

			// ========== Tenancy Management Module ==========
			auth.POST("/addTenancy", tenancyH.AddTenancy)
			auth.POST("/updateTenancy", tenancyH.UpdateTenancy)
			auth.POST("/listTenancy", tenancyH.ListTenancy)
			auth.POST("/deleteTenancy", tenancyH.DeleteTenancy)
			auth.POST("/viewTenancy", tenancyH.ViewTenancy)

			// ========== CA/Certificate Module ==========
			auth.POST("/caFile/list", cacertH.ListCaFiles)
			auth.POST("/caFile/delete", cacertH.DeleteCaFile)
			auth.POST("/caFile/queryCaList", cacertH.QueryCaList)
			auth.POST("/caFile/upload", cacertH.UploadCaFile)
			router.GET("/acs-file-server/ca/downloadFile/:fileId", cacertH.DownloadCaFile)
			auth.POST("/catask/save", cacertH.SaveCaTask)
			auth.POST("/catask/list", cacertH.ListCaTasks)
			auth.POST("/catask/detail", cacertH.GetCaTaskDetail)
			auth.POST("/catask/delete", cacertH.DeleteCaTask)
			auth.POST("/catask/queryDeviceSendCaLog", cacertH.QueryDeviceSendCaLog)

			// ========== Dashboard Management Module ==========
			auth.POST("/listCpeOnlineStatistics", dashboardH.ListCpeOnlineStatistics)
			auth.POST("/listGNBOnlineStatistics", dashboardH.ListGNBOnlineStatistics)
			auth.POST("/listProductTypeAndDeviceCount", dashboardH.ListProductTypeAndDeviceCount)
			auth.POST("/listBaseStationStatistics", dashboardH.ListBaseStationStatistics)
			auth.POST("/listPDCPTrafficStatistic", dashboardH.ListPDCPTrafficStatistic)
			auth.POST("/listDeviceOnlineInfo", dashboardH.ListDeviceOnlineInfo)
			auth.POST("/statisticKPIForDevicelop", dashboardH.StatisticKPIForDevicelop)

			// ========== Heartbeat (SAS/CBSD) ==========
			auth.POST("/heartbeat/process", heartbeatH.ProcessHeartbeat)
			auth.GET("/heartbeat/status", heartbeatH.ListHeartbeatStatus)
			auth.POST("/heartbeat/send/:sn", heartbeatH.SendHeartbeat)
		}
	}

	// ========== REST API (Northbound) — offset-based pagination ==========
	rest := router.Group("/api/rest/v1")
	rest.Use(middleware.AuthMiddleware())
	{
		// Devices
		rest.GET("/devices", restapiH.ListDevices)
		rest.GET("/devices/:id", restapiH.GetDevice)
		rest.POST("/devices", restapiH.AddDevice)
		rest.PUT("/devices/:id", restapiH.ModifyDeviceById)
		rest.PUT("/devices/sn/:sn", restapiH.ModifyDeviceBySN)
		rest.DELETE("/devices/:id", restapiH.DeleteDevice)

		// Device Parameters
		rest.GET("/devices/:id/parameters", restapiH.GetDeviceParams)
		rest.PUT("/devices/:id/parameters", restapiH.SetDeviceParams)
		rest.POST("/devices/:id/parameters/preset", restapiH.PresetDeviceParams)
		rest.GET("/request-status/:requestId", restapiH.GetRequestStatus)

		// Alarms
		rest.GET("/alarms", restapiH.ListAlarms)
		rest.POST("/alarms/sync", restapiH.SyncAlarm)
		rest.POST("/alarms/clear", restapiH.ClearAlarm)

		// Upgrade Files & Tasks
		rest.POST("/upgrade-files", restapiH.UploadUpgradeFile)
		rest.GET("/upgrade-files", restapiH.ListUpgradeFiles)
		rest.DELETE("/upgrade-files/:id", restapiH.DeleteUpgradeFile)
		rest.POST("/upgrade-tasks", restapiH.CreateUpgradeTask)
		rest.GET("/upgrade-tasks", restapiH.ListUpgradeTasks)

		// TBG (Third-party Base Station Gateway)
		rest.POST("/tbg", restapiH.AddTBGs)
		rest.PUT("/tbg", restapiH.ModifyTBGs)
		rest.DELETE("/tbg", restapiH.DeleteTBGs)
		rest.GET("/tbg", restapiH.ListTBGs)
		rest.GET("/tbg/sn/:sn", restapiH.GetTBGBySN)
		rest.GET("/tbg/wan-mac/:wanMac", restapiH.GetTBGByWanMac)

		// Device Online Status (Task 6.2)
		rest.GET("/device/online-status", restapiH.ListDeviceOnlineStatus)
		rest.GET("/device/:elementId/online-status", restapiH.GetDeviceOnlineStatus)

		// ACS Settings (Task 6.3)
		rest.GET("/settings/acs", restapiH.GetACSSettings)
		rest.PUT("/settings/acs", restapiH.UpdateACSSettings)

		// SNMP Operations (Task 6.4)
		rest.POST("/snmp/get", restapiH.SnmpGet)
		rest.POST("/snmp/set", restapiH.SnmpSet)
		rest.GET("/snmp/operation-logs", restapiH.ListSnmpOperationLogs)
	}

	// TR069 ACS endpoints (CWMP) — optional HTTP Basic auth via middleware
	acsAuth := tr069ACS.ACSAuthMiddleware()
	router.POST("/tr069/acs", acsAuth, tr069ACS.HandleACS)
	router.POST("/tr069/cpeAcs", acsAuth, tr069ACS.HandleCpeACS)
	router.POST("/tr069/enbAcs", acsAuth, tr069ACS.HandleEnbACS)
	router.POST("/tr069/gnbAcs", acsAuth, tr069ACS.HandleGnbACS)

	// SNMP trap receiver (UDP listener)
	trapReceiver := snmp.NewTrapReceiver(database.DB, cfg.SNMP.TrapListenPort)
	if err := trapReceiver.Start(); err != nil {
		logger.Errorf("snmp trap receiver start failed: %v", err)
	}

	// SNMP queue worker (polls Redis for outbound traps)
	snmpWorker := snmp.NewWorker(database.DB)
	snmpWorker.Start()

	// SNMP periodic poller (polls SNMP devices at configured intervals)
	snmpPoller := snmp.NewSNMPPoller(database.DB)
	snmpPoller.Start()

	// HA VIP monitor (detects VIP changes and notifies SNMP subsystem)
	var vipMonitor *ha.VIPMonitor
	var vipSubscriber *snmp.VIPSubscriber
	if cfg.HA.Enabled {
		vipMonitor = ha.NewVIPMonitor(database.DB, redis.RDB, cfg)
		vipMonitor.Start()

		vipSubscriber = snmp.NewVIPSubscriber(database.DB)
		vipSubscriber.Start()
	}

	// Offline detection worker (periodically checks device online status)
	offlineWorker := tr069.NewOfflineWorker(db)
	offlineWorker.Start()

	// ZTP provisioning worker (consumes queue:ztp and sends SPV to devices)
	ztpWorker := tr069.NewZTPWorker(db, tr069MsgMgr)
	ztpWorker.Start()

	// Upgrade worker (consumes queue:upgrade and dispatches TR-069 Download/Reboot)
	tr069OpSender := tr069.NewOperationSender(db, tr069MsgMgr)
	upgradeWorker := upgrade.NewUpgradeWorker(db, tr069OpSender)
	upgradeWorker.Start()

	// MML command worker (consumes queue:mml and dispatches MML commands via TR-069)
	mmlWorker := mml.NewMMLWorker(db, tr069MsgMgr)
	mmlWorker.Start()

	// Parameter collection scheduler (periodically sends GPV to configured devices)
	paramScheduler := parameter.NewScheduler(db)
	paramScheduler.Start()

	// System resource collector (samples CPU/memory every 30s, caches to Redis)
	resourceCollector := resources.NewCollector()
	resourceCollector.Start()

	// Start unified cron scheduler (NMS backup jobs, etc.)
	mainScheduler.Start()

	// Alarm email notifier (subscribes to channel:alarm:notify)
	alarmNotifier.Start()

	// STUN server (stores device NAT addresses in Redis for connection requests)
	if cfg.STUN.Enabled {
		stunServer := tr069.NewSTUNServer(cfg.STUN.Port)
		stunServer.Start()
	}

	// 6. 启动HTTP服务
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	go func() {
		logger.Infof("NMS server starting on port %d...", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server failed: %v", err)
		}
	}()

	// 7. 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down server...")

	// Stop background workers
	wsBridge.Stop()
	snmpWorker.Stop()
	snmpPoller.Stop()
	if vipSubscriber != nil {
		vipSubscriber.Stop()
	}
	if vipMonitor != nil {
		vipMonitor.Stop()
	}
	offlineWorker.Stop()
	ztpWorker.Stop()
	mmlWorker.Stop()
	upgradeWorker.Stop()
	paramScheduler.Stop()
	resourceCollector.Stop()
	alarmNotifier.Stop()
	parammonitorH.StopThresholdChecker()
	mainScheduler.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Errorf("server forced to shutdown: %v", err)
	}

	logger.Info("server exited")
}

// placeholder 生成临时handler，用于尚未实现的端点
func placeholder(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		utils.OK(c, map[string]string{"status": "not_implemented", "endpoint": name})
	}
}
