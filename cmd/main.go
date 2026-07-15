package main

import (
	"context"
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
	"nmsappsrv/internal/ntp"
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
	"nmsappsrv/internal/upgrade"
	"nmsappsrv/internal/user"
	"nmsappsrv/internal/websocket"
	"nmsappsrv/internal/webssh"
	"nmsappsrv/pkg/database"
	"nmsappsrv/pkg/logger"
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
	// Topology management module
	topologyH := topology.NewHandler(db)

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

	// WebSSH WebSocket route (SSH auth handled server-side via system_config)
	webssh.RegisterRoutes(router.Group(""), db)

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
	}

	// ========== REST API (Northbound) — offset-based pagination ==========
	rest := router.Group("/api/rest/v1")
	rest.Use(middleware.AuthMiddleware())
	rest.Use(middleware.LicenseMiddleware(licenseEnf))
	{
		restapi.RegisterRoutes(rest, restapiH)
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

	// Upgrade worker (consumes queue:upgrade and dispatches TR-069 Download/Reboot)
	tr069OpSender := tr069.NewOperationSender(db, tr069MsgMgr)
	upgradeWorker := upgrade.NewUpgradeWorker(db, tr069OpSender)
	mgr.Add(workerTask("upgrade-worker", upgradeWorker.Start, upgradeWorker.Stop))

	// MML command worker (consumes queue:mml and dispatches MML commands via TR-069)
	mmlWorker := mml.NewMMLWorker(db, tr069MsgMgr)
	mgr.Add(workerTask("mml-worker", mmlWorker.Start, mmlWorker.Stop))

	// Parameter collection scheduler (periodically sends GPV to configured devices)
	paramScheduler := parameter.NewScheduler(db)
	mgr.Add(workerTask("param-scheduler", paramScheduler.Start, paramScheduler.Stop))

	// Monitor data ingestion (5-min cron): actively polls devices for each enabled
	// monitor task and persists samples into monitor_data via the tr069 GPV response
	// hook (mirrors Java MonitorValueTask + GetCpeStatisticMessageProcessor).
	tr069.DefaultSender = tr069OpSender
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
		if err := db.Where("login_time < ?", cutoff).Delete("login_log").Error; err != nil {
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
