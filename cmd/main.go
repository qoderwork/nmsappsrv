package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"nmsappsrv/internal/alarm"
	"nmsappsrv/internal/authz"
	"nmsappsrv/internal/blacklist"
	"nmsappsrv/internal/cacert"
	"nmsappsrv/internal/captcha"
	"nmsappsrv/internal/cbsd"
	"nmsappsrv/internal/config"
	"nmsappsrv/internal/corenet"
	"nmsappsrv/internal/dashboard"
	"nmsappsrv/internal/device"
	"nmsappsrv/internal/deviceauth"
	"nmsappsrv/internal/devicelog"
	"nmsappsrv/internal/diagnostics"
	"nmsappsrv/internal/eventlog"
	"nmsappsrv/internal/filebase"
	"nmsappsrv/internal/filepiecemeal"
	"nmsappsrv/internal/ha"
	"nmsappsrv/internal/health"
	"nmsappsrv/internal/heartbeat"
	"nmsappsrv/internal/initserver"
	"nmsappsrv/internal/license"
	"nmsappsrv/internal/mail"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/mml"
	"nmsappsrv/internal/monitor"
	"nmsappsrv/internal/mr"
	"nmsappsrv/internal/nmsbackup"
	"nmsappsrv/internal/northinterfacelog"
	"nmsappsrv/internal/ntp"
	"nmsappsrv/internal/operation"
	"nmsappsrv/internal/paramcompare"
	"nmsappsrv/internal/parameter"
	"nmsappsrv/internal/parammonitor"
	"nmsappsrv/internal/platform"
	"nmsappsrv/internal/pm"
	"nmsappsrv/internal/pmfile"
	"nmsappsrv/internal/reboot"
	"nmsappsrv/internal/reset"
	"nmsappsrv/internal/resources"
	"nmsappsrv/internal/restapi"
	"nmsappsrv/internal/scheduler"
	"nmsappsrv/internal/security"
	"nmsappsrv/internal/site"
	"nmsappsrv/internal/snmp"
	sshmod "nmsappsrv/internal/ssh"
	"nmsappsrv/internal/systemsettings"
	"nmsappsrv/internal/tcpdump"
	"nmsappsrv/internal/tenancy"
	"nmsappsrv/internal/topology"
	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/internal/upgrade"
	"nmsappsrv/internal/user"
	"nmsappsrv/internal/websocket"
	"nmsappsrv/internal/webssh"
	"nmsappsrv/internal/ztp"
	"nmsappsrv/internal/ztp/sftp"
	"path/filepath"
	"nmsappsrv/pkg/database"
	"nmsappsrv/pkg/logger"
	"gorm.io/gorm"
	"nmsappsrv/pkg/metrics"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/qoderwork/go-infra/lifecycle"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "nmsappsrv/docs" // swagger generated docs
)

// @title NMS Application Server API
// @version 1.0
// @description Network Management System for small cell base stations (gNB/eNB/CPE).
// @description Provides device management, alarm monitoring, parameter configuration, firmware upgrade, performance metrics, and TR-069 ACS.

// @contact.name API Support
// @contact.url https://github.com/qoderwork/nmsappsrv

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer JWT token. Format: "Bearer {token}"

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
	dbCfg := database.Config{
		Host:         cfg.DB.Host,
		Port:         cfg.DB.Port,
		User:         cfg.DB.User,
		Password:     cfg.DB.Password,
		DBName:       cfg.DB.DBName,
		Charset:      cfg.DB.Charset,
		MaxIdleConns: cfg.DB.MaxIdleConns,
		MaxOpenConns: cfg.DB.MaxOpenConns,
		LogLevel:     cfg.DB.LogLevel,
	}

	// 2a. 自动创建数据库（如果不存在）
	if err := database.EnsureDatabase(dbCfg); err != nil {
		fmt.Fprintf(os.Stderr, "ensure database failed: %v\n", err)
		os.Exit(1)
	}

	// 2b. 连接数据库
	db, err := database.Init(dbCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "database init failed: %v\n", err)
		os.Exit(1)
	}

	// 3. AutoMigrate 所有表
	if err := database.AutoMigrateAll(db); err != nil {
		logger.Fatalf("auto migrate failed: %v", err)
	}

	// 3a. 初始化默认数据（租户、admin用户、admin角色）
	if err := database.SeedInitialData(db); err != nil {
		logger.Warnf("seed initial data failed: %v", err)
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
	// 4a. Init JWT secret from config (must be >=32 bytes)
	middleware.SetJWTSecret(cfg.JWT.Secret)

	// 5. 设置Gin
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// 5a. 初始化 License Enforcer（L-2 发照/校验主流程）
	licenseRepo := license.NewRepository(db)
	licenseEnf, err := license.NewEnforcer(cfg.License, licenseRepo)
	if err != nil {
		logger.Fatalf("license enforcer init failed: %v", err)
	}
	if err := licenseEnf.LoadPersisted(); err != nil {
		logger.Warnf("license: failed to load persisted license: %v", err)
	}

	// 全局中间件
	router.Use(gin.Recovery())
	router.Use(metrics.Middleware())
	router.Use(middleware.CORSMiddleware(cfg.Server.CORSAllowedOrigins))
	router.Use(middleware.TraceID())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.TenancyMiddleware())

	// 健康检查 (liveness — process alive, no dependency checks)
	router.GET("/health", func(c *gin.Context) {
		utils.OK(c, "ok")
	})

	// 就绪检查 (readiness — dependencies healthy)
	router.GET("/ready", func(c *gin.Context) {
		status := map[string]string{}
		allOK := true

		// Check MySQL
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			status["mysql"] = "down"
			allOK = false
		} else {
			status["mysql"] = "up"
		}

		// Check Redis
		if err := redis.RDB.Ping(c.Request.Context()).Err(); err != nil {
			status["redis"] = "down"
			allOK = false
		} else {
			status["redis"] = "up"
		}

		if allOK {
			utils.OK(c, status)
		} else {
			c.JSON(503, utils.Response{Code: 503, Message: "service not ready", Data: status})
		}
	})

	// Prometheus metrics
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Swagger API documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// ========== 初始化所有模块Handler ==========
	deviceH := device.NewHandler(db)
	alarmH := alarm.NewHandler(db)
	// Captcha + adaptive login risk-control (Redis is initialized above).
	captchaMgr := captcha.NewManager(redis.RDB, cfg.Captcha.Length)
	userH := user.NewHandler(db, captchaMgr)
	licenseH := license.NewHandler(db, licenseEnf)
	parameterH := parameter.NewHandler(db)
	upgradeH := upgrade.NewHandler(db)
	eventlogH := eventlog.NewHandler(db)
	pmH := pm.NewHandler(db)
	// PM subsystem workers (collector + replenish) are registered later,
	// after mgr is created (see below). NMS_PM_WORKERS_DISABLED=1 skips
	// both for tests.
	pmSvc := pm.NewService(db)
	pmRepo := pm.NewRepository(db)
	monitorH := monitor.NewHandler(db)
	siteH := site.NewHandler(db)
	mmlH := mml.NewHandler(db)
	cbsdH := cbsd.NewHandler(db)
	corenetH := corenet.NewHandler(db)
	miscH := misc.NewHandler(db, cfg)
	miscH.EnqueueZTPFunc = tr069.EnqueueZTPProvision
	diagnosticsH := diagnostics.NewHandler(db)
	rebootH := reboot.NewHandler(db)
	resetH := reset.NewHandler(db)
	blacklistH := blacklist.NewHandler(db)
	ntpH := ntp.NewHandler(db)
	initserverH := initserver.NewHandler(db)
	deviceauthH := deviceauth.NewHandler(db)
	sshH := sshmod.NewHandler(db)
	mailH := mail.NewHandler(db, cfg.Mail.AESKey)
	mailSvc := mail.NewService(db, cfg.Mail.AESKey)
	alarmNotifier := alarm.NewAlarmNotifier(db, mailSvc)
	securityH := security.NewHandler(db)
	shutdownH := upgrade.NewShutdownHandler(db)
	systemsettingsH := systemsettings.NewSystemSettingsHandler(db, cfg.Mail.AESKey)
	// Device-facing file server + chunked upload + MR pipeline (ABSENT backfill).
	filebaseSvc := filebase.NewService(cfg.FileServer, db)
	filepiecemealSvc := filepiecemeal.NewService(cfg.FileServer.PiecemealTempDir)
	mrSvc := mr.NewService(db, cfg.FileServer)
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

	// Northbound interface audit log (ABSENT backfill #4). The capture middleware
	// is installed on the northbound group below; this handler serves the query.
	northLogRepo := northinterfacelog.NewRepository(db)
	northLogSvc := northinterfacelog.NewService(northLogRepo)
	northLogH := northinterfacelog.NewHandler(northLogSvc)

	// Second batch modules
	healthH := health.NewHandler(db)
	heartbeatH := heartbeat.NewHandler(db, cfg)
	resourcesH := resources.NewHandler(db)
	platformH := platform.NewHandler(db, cfg.Mail.AESKey, cfg.PlatformFiles)
	tenancyH := tenancy.NewHandler(db)
	cacertH := cacert.NewHandler(db)
	dashboardH := dashboard.NewHandler(db)
	// tcpdump and PM file modules
	tcpdumpH := tcpdump.NewHandler(db)
	pmfileH := pmfile.NewHandler(db)
	// Topology management module (initialized later after opSender is ready)
	var topologyH *topology.Handler

	// ========== Lifecycle Manager ==========
	// workerTask adapts the project's Start/Stop worker convention to lifecycle.Task.
	workerTask := func(name string, start, stop func()) lifecycle.Task {
		return lifecycle.NewFuncTask(name,
			func(ctx context.Context) error {
				utils.SafeGo(name, start)
				return nil
			},
			func(ctx context.Context) error {
				stop()
				return nil
			},
		)
	}
	mgr := lifecycle.New(
		lifecycle.WithTimeout(30 * time.Second),
	)

	// OnStop hooks: infrastructure cleanup (LIFO — runs after all tasks stop)
	mgr.OnStop(func(ctx context.Context) error {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
		return nil
	})
	mgr.OnStop(func(ctx context.Context) error {
		redis.RDB.Close()
		return nil
	})
	mgr.OnStop(func(ctx context.Context) error {
		logger.Cleanup()
		return nil
	})
	// ========== WebSocket ==========
	wsHub := websocket.NewHub()
	utils.SafeGo("ws-hub", func() { wsHub.Run() })
	wsH := websocket.NewWSHandler(wsHub)
	wsBridge := websocket.NewBridge(wsHub, db)
	mgr.Add(workerTask("ws-bridge", wsBridge.Start, wsBridge.Stop))

	// WebSocket route (no auth required)
	router.GET("/ws", wsH.ServeWS)

	// WebSSH WebSocket route (credentials provided by client via WebSocket "connect" message)
	webssh.RegisterRoutes(router.Group(""))

	// 启动SSH Access Timer后台过期检查
	sshH.StartExpiredChecker()

	// TR069 ACS
	tr069MsgMgr := tr069.NewMessageManager()
	tr069EventProc := tr069.NewEventProcessor(db)
	tr069ACS := tr069.NewACSHandler(db, tr069MsgMgr, tr069EventProc, cfg.TR069)

	// ========== RBAC 授权层 (casbin) ==========
	if err := authz.InitEnforcer(db); err != nil {
		logger.Fatalf("failed to init RBAC enforcer: %v", err)
	}
	if err := authz.SeedBuiltinRoles(db); err != nil {
		logger.Errorf("failed to seed built-in roles: %v", err)
	}

	// ========== API路由组 ==========
	api := router.Group("/api/v1")
	{
		// ===== 认证（公开） =====
		user.RegisterPublicRoutes(api, userH)
		// License upload/info must stay public so an admin can activate a license
		// before any gating is in effect.
		license.RegisterPublicRoutes(api, licenseH)

		// ===== 需要认证的路由 =====
		auth := api.Group("")
		auth.Use(middleware.AuthMiddleware())
		auth.Use(middleware.LicenseMiddleware(licenseEnf))
		{
			device.RegisterRoutes(auth, deviceH)
			site.RegisterRoutes(auth, siteH)
			alarm.RegisterRoutes(auth, alarmH)
			parameter.RegisterRoutes(auth, parameterH)
			upgrade.RegisterRoutes(auth, upgradeH)
			upgrade.RegisterShutdownRoutes(auth, shutdownH)
			reboot.RegisterRoutes(auth, rebootH)
			pm.RegisterRoutes(auth, pmH)
			monitor.RegisterRoutes(auth, monitorH)
			mml.RegisterRoutes(auth, mmlH)
			corenet.RegisterRoutes(auth, corenetH)
			cbsd.RegisterRoutes(auth, cbsdH)
			misc.RegisterRoutes(auth, miscH)
			user.RegisterRoutes(auth, userH)
			license.RegisterRoutes(auth, licenseH)
			eventlog.RegisterRoutes(auth, eventlogH)
			diagnostics.RegisterRoutes(auth, diagnosticsH)
			reset.RegisterRoutes(auth, resetH)
			blacklist.RegisterRoutes(auth, blacklistH)
			ntp.RegisterRoutes(auth, ntpH)
			initserver.RegisterRoutes(auth, initserverH)
			deviceauth.RegisterRoutes(auth, deviceauthH)
			sshmod.RegisterRoutes(auth, sshH)
			mail.RegisterRoutes(auth, mailH)
			security.RegisterRoutes(auth, securityH)
			systemsettings.RegisterRoutes(auth, systemsettingsH)
			parammonitor.RegisterRoutes(auth, parammonitorH)
			filepiecemeal.RegisterRoutes(auth, filepiecemeal.NewHandler(filepiecemealSvc))
			mr.RegisterRoutes(auth, mr.NewHandler(mrSvc))
			paramcompare.RegisterRoutes(auth, paramcompareH)
			devicelog.RegisterRoutes(auth, devicelogH)
			nmsbackup.RegisterRoutes(auth, nmsbackupH)
			health.RegisterRoutes(auth, healthH)
			resources.RegisterRoutes(auth, resourcesH)
			platform.RegisterRoutes(auth, platformH)
			tenancy.RegisterRoutes(auth, tenancyH)
			cacert.RegisterRoutes(auth, cacertH)
			dashboard.RegisterRoutes(auth, dashboardH)
			heartbeat.RegisterRoutes(auth, heartbeatH)
			tcpdump.RegisterRoutes(auth, tcpdumpH)
			pmfile.RegisterRoutes(auth, pmfileH)
			topology.RegisterRoutes(auth, topologyH)
		}
	}

	// Public routes (no auth)
	health.RegisterPublicRoutes(router, healthH)
	cacert.RegisterPublicRoutes(router, cacertH)
	upgrade.RegisterPublicRoutes(router, upgradeH)

	// ========== Device-facing file server (/acs-file-server/**, Basic auth) ==========
	// Registered at the root (no /api/v1) to mirror Java's gateway-stripped path.
	filebase.RegisterRoutes(router, systemsettingsH.Service(), filebaseSvc, mrSvc)

	// ========== Java-compatible permission endpoints (v2 prefix) ==========
	// Mirrors nms-serv RoleManagementServiceImpl.getPermissionIdsForUser /
	// getPermission so the frontend permission tree keeps working.
	v2 := router.Group("/api/v2")
	v2.Use(middleware.AuthMiddleware())
	v2.Use(middleware.LicenseMiddleware(licenseEnf))
	{
		v2.GET("/getPermissionIdsForUser", authz.GetPermissionIdsForUser)
		v2.GET("/getPermission", authz.GetPermission)
		// NorthBoundConfig (ABSENT backfill #3) — exact Java URLs for drop-in compat.
		v2.GET("/getNorthBoundConfig", systemsettingsH.GetNorthBoundConfig)
		v2.POST("/updateNorthBoundConfig", systemsettingsH.UpdateNorthBoundConfig)
		// NorthInterfaceLog query (ABSENT backfill #4) — exact Java URL.
		v2.POST("/listNorthInterfaceLog", northLogH.ListLogs)
	}

	// ========== REST API (Northbound) — offset-based pagination ==========
	rest := router.Group("/api/rest/v1")
	rest.Use(middleware.AuthMiddleware())
	rest.Use(middleware.LicenseMiddleware(licenseEnf))
	rest.Use(northinterfacelog.AuditMiddleware(northLogSvc))
	{
		restapi.RegisterRoutes(rest, restapiH)
	}

	// ========== REST API (Northbound) — /v1 alias ==========
	// Java's northbound base path is "/v1/...". When clients connect
	// directly to the NMS (no gateway doing URL rewrite), they expect the
	// Java-equivalent base. This alias group serves the SAME handlers under
	// "/v1" without touching the existing "/api/rest/v1" mount, so either
	// path works during the coding phase (no live devices/clients yet).
	// NOTE: only the base prefix is aliased; sub-path names remain the Go
	// naming (e.g. /v1/tbg, not Java's /v1/femtos). A full sub-path
	// rename to match Java exactly is a separate, larger task if ever needed.
	v1 := router.Group("/v1")
	v1.Use(middleware.AuthMiddleware())
	v1.Use(middleware.LicenseMiddleware(licenseEnf))
	v1.Use(northinterfacelog.AuditMiddleware(northLogSvc))
	{
		restapi.RegisterRoutes(v1, restapiH)
	}

	// TR069 ACS endpoints (CWMP) — optional HTTP Basic auth via middleware
	acsAuth := tr069ACS.ACSAuthMiddleware()
	router.POST("/tr069/acs", acsAuth, tr069ACS.HandleACS)
	router.POST("/tr069/cpeAcs", acsAuth, tr069ACS.HandleCpeACS)
	router.POST("/tr069/enbAcs", acsAuth, tr069ACS.HandleEnbACS)
	router.POST("/tr069/gnbAcs", acsAuth, tr069ACS.HandleGnbACS)
	router.POST("/tr069/gnbInitialAcs", acsAuth, tr069ACS.HandleGnbInitialACS)

	// Init Server endpoint (ZTP) — device-facing, no web UI auth
	router.POST("/init-server", initserverH.HandleInitServer)

	// SNMP trap receiver (UDP listener) — starts immediately, error checked inline
	trapReceiver := snmp.NewTrapReceiver(db, cfg.SNMP.TrapListenPort)
	if err := trapReceiver.Start(); err != nil {
		logger.Errorf("snmp trap receiver start failed: %v", err)
	}

	// ========== Register background workers as lifecycle tasks ==========
	// Startup order = registration order; shutdown is automatic LIFO.

	// SNMP queue worker (polls Redis for outbound traps)
	snmpWorker := snmp.NewWorker(db)
	mgr.Add(workerTask("snmp-worker", snmpWorker.Start, snmpWorker.Stop))

	// SNMP periodic poller (polls SNMP devices at configured intervals)
	snmpPoller := snmp.NewSNMPPoller(db)
	mgr.Add(workerTask("snmp-poller", snmpPoller.Start, snmpPoller.Stop))

	// HA VIP monitor (detects VIP changes and notifies SNMP subsystem)
	if cfg.HA.Enabled {
		vipMonitor := ha.NewVIPMonitor(db, redis.RDB, cfg)
		vipSubscriber := snmp.NewVIPSubscriber(db)
		mgr.Add(workerTask("vip-monitor", vipMonitor.Start, vipMonitor.Stop))
		mgr.Add(workerTask("vip-subscriber", vipSubscriber.Start, vipSubscriber.Stop))
	}

	// Offline detection worker (periodically checks device online status)
	offlineWorker := tr069.NewOfflineWorker(db)
	mgr.Add(workerTask("offline-worker", offlineWorker.Start, offlineWorker.Stop))

	// ZTP provisioning worker (consumes queue:ztp and sends SPV to devices)
	ztpWorker := tr069.NewZTPWorker(db, tr069MsgMgr)
	mgr.Add(workerTask("ztp-worker", ztpWorker.Start, ztpWorker.Stop))

	// Bridge the misc AOS-file generator into the tr069 worker so explicit
	// provisioning also produces the pull-path AOS artifact (served by
	// /acs-file-server/ztpFile). Kept as a func var to avoid an import cycle.
	tr069.GenerateAOSFunc = miscH.GenerateAOSFile

	// ZTP embedded SFTP server (Java ZTPSftpServer, port 10022). Opt-in via
	// ztp.sftp_enabled. Serves the AOS XML root (file_server.ztp_dir) to
	// ZTP-capable devices; auth is delegated to miscH.CheckSFTPCredentials
	// which validates against the persisted ZTPSetting. Mirrors Java's
	// behaviour: SSH transport + password auth (creds read from
	// system_config.ztp_config at each login).
	if cfg.ZTP.SFTPEnabled {
		ztpDir := cfg.FileServer.ZtpDir
		if ztpDir == "" {
			ztpDir = filepath.Join(cfg.FileServer.Root, "ztp")
		}
		if ztpDir == "" {
			ztpDir = "./data/acs-file-server/ztp"
		}
		host := cfg.ZTP.SFTPHost
		if host == "" {
			host = ":10022"
		}
		sftpSrv := sftp.NewServer(host, cfg.ZTP.SFTPHostKey, ztpDir, miscH.CheckSFTPCredentials)
		mgr.Add(workerTask("ztp-sftp",
			func() { _ = sftpSrv.Start() },
			func() { _ = sftpSrv.Stop() },
		))
		logger.Infof("ztp sftp: scheduled (opt-in: %v, host: %s, dir: %s)", cfg.ZTP.SFTPEnabled, host, ztpDir)
	}

	// Upgrade worker (consumes queue:upgrade and dispatches TR-069 Download/Reboot)
	tr069OpSender := tr069.NewOperationSender(db, tr069MsgMgr)
	upgradeWorker := upgrade.NewUpgradeWorker(db, tr069OpSender)
	mgr.Add(workerTask("upgrade-worker", upgradeWorker.Start, upgradeWorker.Stop))

	// MML command worker (consumes queue:mml and dispatches MML commands via TR-069)
	mmlWorker := mml.NewMMLWorker(db, tr069MsgMgr)
	mgr.Add(workerTask("mml-worker", mmlWorker.Start, mmlWorker.Stop))

	// Unified device-operation dispatcher (consumes mq.OperationQueue and routes
	// by Operation string to the matching tr069.OperationSender.Send* primitive).
	// Mirrors Java Receiver.operationQueue + apiCommandProcessor.processCommand
	// (single thread, 200 ops/s global rate limit, switch on EventType string).
	operationDispatcher := operation.NewDispatcher(db, tr069OpSender)
	operationWorker := operation.NewWorker(operationDispatcher)
	mgr.Add(workerTask("operation-worker", operationWorker.Start, operationWorker.Stop))

	// Parameter collection scheduler (periodically sends GPV to configured devices)
	paramScheduler := parameter.NewScheduler(db)
	mgr.Add(workerTask("param-scheduler", paramScheduler.Start, paramScheduler.Stop))

	// Monitor data ingestion (5-min cron): actively polls devices for each enabled
	// monitor task and persists samples into monitor_data via the tr069 GPV response
	// hook (mirrors Java MonitorValueTask + GetCpeStatisticMessageProcessor).
	tr069.DefaultSender = tr069OpSender
	topologyH = topology.NewHandler(db, tr069OpSender)
	monitorCollector := monitor.NewCollector(db)
	tr069.MonitorGPVCallback = monitorCollector.HandleGPVResponse

	// MML command result delivery: when the device ACKs (or faults on) the SPV that
	// carried an MML command, mark the execution result delivered (status=3) and
	// record success/failure via has_fault (对齐 Java MmlMessageProcessor).
	tr069.MMLResponseCallback = func(resultId int, success bool, faultString string) {
		mml.UpdateResultStatusOnAck(db, resultId, success, faultString)
	}
	if err := mainScheduler.AddJob("monitor-collect", "0 */5 * * * *", monitorCollector.RunOnce); err != nil {
		logger.Errorf("failed to register monitor-collect cron job: %v", err)
	}
	// Retention: prune monitor_data older than 90 days (daily at 03:10).
	if err := mainScheduler.AddJob("monitor-data-retention", "0 10 3 * * *", func() {
		if n, err := monitorCollector.Cleanup(time.Now().AddDate(0, 0, -90)); err != nil {
			logger.Errorf("monitor data retention failed: %v", err)
		} else if n > 0 {
			logger.Infof("monitor data retention pruned %d samples", n)
		}
	}); err != nil {
		logger.Errorf("failed to register monitor-data-retention cron job: %v", err)
	}

	// Device statistics fill (hourly cron): snapshot each device's online/active
	// state into device_statistic so the dashboard's online-statistics endpoints
	// have data to aggregate. One row per device per hour (idempotent per bucket).
	// Closes P1 cluster 5 (dashboard data source) + the missing dashboard-fill job
	// in P1 cluster 9.
	if err := mainScheduler.AddJob("device-statistic-fill", "0 0 * * * *", func() {
		ctx := context.Background()
		bucket := time.Now().Truncate(time.Hour)
		if n, err := device.FillDeviceStatistic(ctx, db, bucket); err != nil {
			logger.Errorf("device-statistic-fill failed for %s: %v", bucket, err)
		} else {
			logger.Infof("device-statistic-fill: wrote %d device snapshots for %s", n, bucket)
		}
	}); err != nil {
		logger.Errorf("failed to register device-statistic-fill cron job: %v", err)
	}
	// Bootstrap: seed the last 24 hourly buckets on startup so the dashboard is not
	// empty on first run. Past buckets reflect current liveness (a seed, not real
	// history) and are overwritten by the hourly job going forward.
	go func() {
		ctx := context.Background()
		now := time.Now()
		for h := 0; h < 24; h++ {
			bucket := now.Add(-time.Duration(h) * time.Hour).Truncate(time.Hour)
			if _, err := device.FillDeviceStatistic(ctx, db, bucket); err != nil {
				logger.Warnf("device-statistic backfill for %s failed: %v", bucket, err)
			}
		}
		logger.Infof("device-statistic backfill complete (last 24h seeded)")
	}()

	// Login-log retention (daily at 04:10): prune login_log older than 90 days.
	// Closes the "log cleanup" part of P1 cluster 9 (Java FileAndMysqlLogDeleteTask).
	if err := mainScheduler.AddJob("login-log-retention", "0 10 4 * * *", func() {
		cutoff := time.Now().AddDate(0, 0, -90)
		if err := db.Where("operation_time < ?", cutoff).Delete("login_log").Error; err != nil {
			logger.Errorf("login-log retention failed: %v", err)
		}
	}); err != nil {
		logger.Errorf("failed to register login-log retention cron job: %v", err)
	}

	// CBSD SAS operation-state maintainer (every 60s; Java runs every 90s).
	// Drives GRANTED/AUTHORIZED -> REGISTERED/GRANTED on grant/transmit expiry,
	// re-issuing grant/heartbeat. Closes P1 cluster 6 (heartbeat-driven SAS state machine).
	if err := mainScheduler.AddJob("cbsd-opstate-maintain", "0 * * * * *", func() {
		if n, err := cbsdH.MaintainOperationStates(context.Background()); err != nil {
			logger.Errorf("cbsd-opstate-maintain failed: %v", err)
		} else if n > 0 {
			logger.Infof("cbsd-opstate-maintain: transitioned %d CBSD(s)", n)
		}
	}); err != nil {
		logger.Errorf("failed to register cbsd-opstate-maintain cron job: %v", err)
	}

	// CBSD result task (every 2 minutes) — mirrors Java CBSDResultTask.noticeDevice.
	// Scans online devices with CBSD records and pushes UpdateCBSDStatus SOAP to
	// notify each CPE of its CBSDs' current SAS state (AUTHORIZED/GRANTED/REGISTERED...).
	if err := mainScheduler.AddJob("cbsd-notice-device", "0 */2 * * *", func() {
		notifyCbsdStatusToDevices(db, tr069OpSender)
	}); err != nil {
		logger.Errorf("failed to register cbsd-notice-device cron job: %v", err)
	}

	// CoreNetwork UE number statistic task (every 10 minutes).
	// Mirrors Java CoreNetworkUeNumberStatisticTask.statistic().
	// Reads IMS/SMF UE counts from CoreNetworkData and persists to core_network_statistic_data.
	if err := mainScheduler.AddJob("corenet-ue-statistic", "0 */10 * * *", func() {
		corenetRepo := corenet.NewRepository(db)
		dataList, err := corenetRepo.FindAllCoreNetworkData()
		if err != nil {
			logger.Errorf("corenet-ue-statistic: failed to get core network data: %v", err)
			return
		}
		if len(dataList) == 0 {
			return
		}
		now := time.Now()
		var stats []corenet.CoreNetworkStatisticData
		for _, data := range dataList {
			hasData := false
			var imsNum, smfNum *int
			if data.ImsUeNumber != nil && *data.ImsUeNumber != "" {
				var ueNum struct{ Data struct{ UeNum int } `json:"data"` }
				if json.Unmarshal([]byte(*data.ImsUeNumber), &ueNum) == nil {
					imsNum = &ueNum.Data.UeNum
					hasData = true
				}
			}
			if data.SmfUeNumber != nil && *data.SmfUeNumber != "" {
				var ueNum struct{ Data struct{ UeNum int } `json:"data"` }
				if json.Unmarshal([]byte(*data.SmfUeNumber), &ueNum) == nil {
					smfNum = &ueNum.Data.UeNum
					hasData = true
				}
			}
			if hasData {
				stats = append(stats, corenet.CoreNetworkStatisticData{
					CoreNetworkId: data.CoreNetworkId,
					ImsUeNumber:   imsNum,
					SmfUeNumber:   smfNum,
					StatisticTime: &now,
				})
			}
		}
		if len(stats) > 0 {
			if err := corenetRepo.BatchSaveCoreNetworkStatisticData(stats); err != nil {
				logger.Errorf("corenet-ue-statistic: failed to save statistic data: %v", err)
			} else {
				logger.Infof("corenet-ue-statistic: saved %d statistic records", len(stats))
			}
		}
	}); err != nil {
		logger.Errorf("failed to register corenet-ue-statistic cron job: %v", err)
	}

	// ZTP AOS-file generation scan (Java ZTPTask, every 10s). Picks up devices
	// flagged read_to_ztp=true with no AOS file yet and generates the pull-path
	// AOS XML (served by /acs-file-server/ztpFile). Runs every 30s.
	if err := mainScheduler.AddJob("ztp-aos-gen", "*/30 * * * * *", func() {
		if n, err := miscH.ScanAndGenerateAOSFiles(); err != nil {
			logger.Errorf("ztp-aos-gen failed: %v", err)
		} else if n > 0 {
			logger.Infof("ztp-aos-gen: generated %d AOS file(s)", n)
		}
	}); err != nil {
		logger.Errorf("failed to register ztp-aos-gen cron job: %v", err)
	}

	// ZTP external-system registration orchestrator (Java GenerateZTPFileThread).
	// Picks up devices that have an AOS file but have not yet been registered
	// against the E911 systems (MSAG / BMC old+new / LMF 1–4 / GMLC) and runs
	// the full allocation + geofence + registration + rollback flow. The wire
	// transport is the real mTLS HTTPTransport; a nil TransportConfig falls
	// back to the Java-default cert paths read from the environment. Runs
	// every 30s.
	ztpSvc := misc.NewService(db, cfg)
	ztpThread := ztp.NewThread(db, ztpSvc, nil, alarm.NewService(db))
	if err := mainScheduler.AddJob("ztp-external-gen", "*/30 * * * * *", func() {
		if n, err := ztpThread.ScanAndProcess(context.Background()); err != nil {
			logger.Errorf("ztp-external-gen failed: %v", err)
		} else if n > 0 {
			logger.Infof("ztp-external-gen: processed %d device(s)", n)
		}
	}); err != nil {
		logger.Errorf("failed to register ztp-external-gen cron job: %v", err)
	}

	// Scheduled reboot/reset tasks (Java Quartz RebootTaskJob/ResetTaskJob,
	// ExecuteMode==3). Scans for Waiting tasks whose trigger time has passed
	// and dispatches them. Runs every 30s.
	rebootSvc := reboot.NewService(db)
	resetSvc := reset.NewService(db)
	if err := mainScheduler.AddJob("reboot-timed", "*/30 * * * * *", func() {
		if n, err := rebootSvc.TriggerDueTimedTasks(context.Background()); err != nil {
			logger.Errorf("reboot-timed job failed: %v", err)
		} else if n > 0 {
			logger.Infof("reboot-timed: dispatched %d scheduled task(s)", n)
		}
	}); err != nil {
		logger.Errorf("failed to register reboot-timed cron job: %v", err)
	}
	if err := mainScheduler.AddJob("reset-timed", "*/30 * * * * *", func() {
		if n, err := resetSvc.TriggerDueTimedTasks(context.Background()); err != nil {
			logger.Errorf("reset-timed job failed: %v", err)
		} else if n > 0 {
			logger.Infof("reset-timed: dispatched %d scheduled task(s)", n)
		}
	}); err != nil {
		logger.Errorf("failed to register reset-timed cron job: %v", err)
	}

	// System resource collector (samples CPU/memory every 30s, caches to Redis)
	resourceCollector := resources.NewCollector()
	mgr.Add(workerTask("resource-collector", resourceCollector.Start, resourceCollector.Stop))

	// Unified cron scheduler (NMS backup jobs, etc.) — already created above for backup job registration
	mgr.Add(workerTask("main-scheduler", mainScheduler.Start, mainScheduler.Stop))

	// Alarm email notifier (subscribes to channel:alarm:notify)
	mgr.Add(workerTask("alarm-notifier", alarmNotifier.Start, alarmNotifier.Stop))
	// PM file processor (consumes queue:pm and parses uploaded PM files)
	pmfileProcessor := pmfile.NewProcessor(db)
	mgr.Add(workerTask("pmfile-processor", pmfileProcessor.Start, pmfileProcessor.Stop))

	// Parameter monitor threshold checker
	mgr.Add(workerTask("param-threshold",
		parammonitorH.StartThresholdChecker,
		parammonitorH.StopThresholdChecker,
	))

	// STUN server (stores device NAT addresses in Redis for connection requests)
	if cfg.STUN.Enabled {
		stunServer := tr069.NewSTUNServer(cfg.STUN.Port)
		mgr.Add(workerTask("stun-server", stunServer.Start, stunServer.Stop))
	}

	// PM subsystem workers. Collector periodically writes placeholder
	// PM files under file_server.pm_dir + inserts pm_file_log rows so
	// DownloadPMFile returns real data. ReplenishWorker flips
	// Waiting->Executing->Executed on pm_replenish_task rows and marks
	// every device Done=true so listDeviceReplenish returns a useful
	// response. Both are opt-out via NMS_PM_WORKERS_DISABLED=1.
	if os.Getenv("NMS_PM_WORKERS_DISABLED") != "1" {
		pmCollector := pm.NewCollector(db, pmRepo, 5*time.Minute, true)
		pmReplenishWorker := pm.NewReplenishWorker(pmSvc, pmRepo, 30*time.Second)
		mgr.Add(workerTask("pm-collector", pmCollector.Start, pmCollector.Stop))
		mgr.Add(workerTask("pm-replenish-worker", pmReplenishWorker.Start, pmReplenishWorker.Stop))
	}

	// HTTP server as lifecycle task
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}
	mgr.Add(lifecycle.NewFuncTask("http-server",
		func(ctx context.Context) error {
			logger.Infof("NMS server starting on port %d...", cfg.Server.Port)
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Fatalf("server failed: %v", err)
				}
			}()
			return nil
		},
		func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	))

	// Run: start all tasks → wait for signal → stop all tasks (LIFO) → cleanup
	if err := mgr.Run(); err != nil {
		logger.Errorf("lifecycle error: %v", err)
	}
}

// cbsdInfoForNotice is a slim view of cbsd_info used by the notice-device
// cron job. Mirrors the fields Java CBSDResultTask reads per CBSD.
type cbsdInfoForNotice struct {
	CbsdSerialNumber   string  `gorm:"column:cbsd_serial_number"`
	OperationState     string  `gorm:"column:operation_state"`
	LowFrequency       *int64  `gorm:"column:low_frequency"`
	HighFrequency      *int64  `gorm:"column:high_frequency"`
	TransmitExpireTime string  `gorm:"column:transmit_expire_time"`
	MaxEirp            float64 `gorm:"column:max_eirp"`
	PathLoss           int     `gorm:"column:path_loss"`
	AntennaGain        int     `gorm:"column:antenna_gain"`
}

// notifyCbsdStatusToDevices scans all CBSD records, groups them by device SN,
// builds UpdateCBSDStatus SOAP messages, and enqueues them. It mirrors Java
// CBSDResultTask.noticeDevice (every 2 min).
//
// For each CBSD in AUTHORIZED or GRANTED state, transmit power is computed as
//   floor(maxEirp + 10 + pathLoss - antennaGain)
// mirroring Java's expression.
func notifyCbsdStatusToDevices(db *gorm.DB, opSender *tr069.OperationSender) {
	ctx := context.Background()

	var rows []struct {
		SerialNumber string `gorm:"column:serial_number"`
		LicenseId    int    `gorm:"column:license_id"`
		cbsdInfoForNotice
	}
	if err := db.Table("cbsd_info").
		Where("serial_number IS NOT NULL AND serial_number <> '' AND deleted = ?", false).
		Select("serial_number, license_id, cbsd_serial_number, operation_state, low_frequency, high_frequency, transmit_expire_time, max_eirp, path_loss, antenna_gain").
		Scan(&rows).Error; err != nil {
		logger.Errorf("cbsd-notice-device: failed to scan cbsd_info: %v", err)
		return
	}

	type devKey struct {
		sn   string
		lic  int
	}
	byDevice := make(map[devKey][]cbsdInfoForNotice)
	for _, r := range rows {
		k := devKey{sn: r.SerialNumber, lic: r.LicenseId}
		byDevice[k] = append(byDevice[k], r.cbsdInfoForNotice)
	}

	sent := 0
	for k, infos := range byDevice {
		onlineVal, _ := redis.Get(ctx, "online_"+k.sn)
		if onlineVal != "yes" {
			continue
		}

		cbsdInfos := make([]soap.CBSDInfo, 0, len(infos))
		for _, info := range infos {
			entry := soap.CBSDInfo{
				CBSDSerialNumber:   info.CbsdSerialNumber,
				State:              info.OperationState,
				LowFrequency:       info.LowFrequency,
				HighFrequency:      info.HighFrequency,
				TransmitExpireTime: info.TransmitExpireTime,
			}
			if info.OperationState == "AUTHORIZED" || info.OperationState == "GRANTED" {
				power := int(info.MaxEirp + 10 + float64(info.PathLoss) - float64(info.AntennaGain))
				entry.TxPower = &power
			}
			cbsdInfos = append(cbsdInfos, entry)
		}

		if len(cbsdInfos) == 0 {
			continue
		}

		ucs := &soap.UpdateCBSDStatus{CBSDInfos: cbsdInfos}
		opId := fmt.Sprintf("cbsd_notice_%s_%d", k.sn, time.Now().Unix())
		if err := opSender.SendUpdateCBSDStatus(k.sn, ucs, opId); err != nil {
			logger.Warnf("cbsd-notice-device: failed to enqueue for SN=%s: %v", k.sn, err)
			continue
		}
		sent++
	}

	if sent > 0 {
		logger.Infof("cbsd-notice-device: sent UpdateCBSDStatus to %d device(s)", sent)
	}
}
