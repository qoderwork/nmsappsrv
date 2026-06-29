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
	"nmsappsrv/internal/cbsd"
	"nmsappsrv/internal/config"
	"nmsappsrv/internal/corenet"
	"nmsappsrv/internal/device"
	"nmsappsrv/internal/eventlog"
	"nmsappsrv/internal/license"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/mml"
	"nmsappsrv/internal/monitor"
	"nmsappsrv/internal/parameter"
	"nmsappsrv/internal/pm"
	"nmsappsrv/internal/site"
	"nmsappsrv/internal/snmp"
	"nmsappsrv/internal/upgrade"
	"nmsappsrv/internal/tr069"
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

	// TR069 ACS
	tr069MsgMgr := tr069.NewMessageManager()
	tr069EventProc := tr069.NewEventProcessor(db)
	tr069ACS := tr069.NewACSHandler(db, tr069MsgMgr, tr069EventProc)

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
			auth.GET("/parameter-backup-logs", parameterH.ListBackupLogs)
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
			auth.POST("/reboot-tasks", upgradeH.CreateRebootTask)
			auth.GET("/reboot-tasks", upgradeH.ListRebootTasks)
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
			auth.POST("/ztp/provision", placeholder("ztpProvision")) // 后台引擎，待实现
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
		}
	}

	// TR069 ACS endpoints (CWMP) — 不需要认证，CPE直接访问
	router.POST("/tr069/acs", tr069ACS.HandleACS)
	router.POST("/tr069/cpeAcs", tr069ACS.HandleCpeACS)
	router.POST("/tr069/enbAcs", tr069ACS.HandleEnbACS)
	router.POST("/tr069/gnbAcs", tr069ACS.HandleGnbACS)

	// SNMP trap receiver (UDP listener)
	trapReceiver := snmp.NewTrapReceiver(database.DB, cfg.SNMP.TrapListenPort)
	if err := trapReceiver.Start(); err != nil {
		logger.Errorf("snmp trap receiver start failed: %v", err)
	}

	// SNMP queue worker (polls Redis for outbound traps)
	snmpWorker := snmp.NewWorker()
	snmpWorker.Start()

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
