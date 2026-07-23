package database

import (
	"nmsappsrv/internal/alarm"
	"nmsappsrv/internal/cacert"
	"nmsappsrv/internal/cbsd"
	"nmsappsrv/internal/corenet"
	"nmsappsrv/internal/device"
	"nmsappsrv/internal/devicelog"
	"nmsappsrv/internal/diagnostics"
	"nmsappsrv/internal/eventlog"
	"nmsappsrv/internal/heartbeat"
	"nmsappsrv/internal/license"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/mml"
	"nmsappsrv/internal/monitor"
	"nmsappsrv/internal/mnormalfile"
	"nmsappsrv/internal/nmsbackup"
	"nmsappsrv/internal/parameter"
	"nmsappsrv/internal/paramcompare"
	"nmsappsrv/internal/parammonitor"
	"nmsappsrv/internal/pm"
	"nmsappsrv/internal/pmfile"
	"nmsappsrv/internal/restapi"
	"nmsappsrv/internal/reset"
	"nmsappsrv/internal/site"
	sshmod "nmsappsrv/internal/ssh"
	"nmsappsrv/internal/snmp"
	"nmsappsrv/internal/systemsettings"
	"nmsappsrv/internal/upgrade"
	"nmsappsrv/internal/user"
	"nmsappsrv/pkg/logger"
	"gorm.io/gorm"
)

// AutoMigrateAll 自动创建/更新所有表结构 (替代Flyway)
func AutoMigrateAll(db *gorm.DB) error {
	models := []interface{}{
		// device (8)
		&device.CpeElement{},
		&device.DeviceGroup{},
		&device.GroupHasElement{},
		&device.ElementBasicInfoParameter{},
		&device.ElementBlackList{},
		&device.DeviceStatistic{},
		&device.CpeStatisticRecord{},
		&device.DeviceUeNumberRecord{},

		// alarm (7)
		&alarm.Alarm{},
		&alarm.AlarmFilter{},
		&alarm.AlarmFilterHasAlarm{},
		&alarm.AlarmFilterHasDevice{},
		&alarm.AlarmFilterHasDeviceGroup{},
		&alarm.AlarmLibrary{},
		&alarm.AlarmTemplate{},

		// user (6)
		&user.SysUser{},
		&user.Role{},
		&user.RoleHasPermission{},
		&user.UserHasRole{},
		&user.LoginLog{},
		&user.PasswordHistory{},

		// corenet (5)
		&corenet.CoreNetwork{},
		&corenet.CoreNetworkKpi{},
		&corenet.CoreNetworkOperationLog{},
		&corenet.CoreNetworkStatisticData{},
		&corenet.CoreNetworkData{},

		// parameter (12)
		&parameter.Parameter{},
		&parameter.ParameterAttributes{},
		&parameter.ParameterLog{},
		&parameter.ParameterBackupLog{},
		&parameter.ParameterSet{},
		&parameter.ParameterSetHasParameter{},
		&parameter.ParameterTemplate{},
		&parameter.ParameterTemplateHasParameter{},
		&parameter.ParameterDeploymentTemplate{},
		&parameter.ParameterDeploymentTemplateHasElement{},
		&parameter.TR069Parameter{},
		&parameter.ParameterDeploymentLog{},

		// upgrade (12)
		&upgrade.UpgradeTask{},
		&upgrade.UpgradeFile{},
		&upgrade.UpgradeLog{},
		&upgrade.RollbackTask{},
		&upgrade.RebootTask{},
		&upgrade.UpgradeTaskHasElement{},
		&upgrade.RollbackTaskHasElement{},
		&upgrade.ShutdownMyTask{},
		&upgrade.ShutdownLog{},
		&upgrade.EUAndRUBatchUpgradeLog{},
		&upgrade.AutoUpgradeTask{},
		&upgrade.ManualUpgradeLog{},

		// eventlog (2)
		&eventlog.EventLog{},
		&eventlog.TaskToEventLog{},

		// license (4)
		&license.License{},
		&license.BaseStationLicense{},
		&license.SASConfig{},
		&license.EntraEndpoint{},

		// pm (12)
		&pm.PerformanceKpi{},
		&pm.PerformanceKpiSet{},
		&pm.PerformanceKpiTemplate{},
		&pm.PerformanceKpiTemplateHasElement{},
		&pm.PerformanceKpiTemplateHasKpi{},
		&pm.PMFileLog{},
		&pm.KpiAlarmTemplate{},
		&pm.KpiAlarmTemplateHasElement{},
		&pm.KpiAlarmTemplateHasGroup{},
		&pm.KpiAlarmTemplateHasKpiThreshold{},
		&pm.DashboardPmStatisticData{},
		&pm.PDCPTraffic{},

		// monitor (4)
		&monitor.MonitorTask{},
		&monitor.MonitorData{},
		&monitor.MonitorElements{},
		&monitor.MonitorParameters{},

		// site (4)
		&site.SiteInfo{},
		&site.SysArea{},
		&site.SystemConfig{},
		&site.SystemParameter{},

		// mml (5)
		&mml.MmlCommand{},
		&mml.MmlCommandParam{},
		&mml.MmlSet{},
		&mml.MmlExecuteResult{},
		&mml.MmlExecuteResultFileLog{},

		// cbsd (5)
		&cbsd.CbsdInfo{},
		&cbsd.CBSDCertFileSendTask{},
		&cbsd.SendCBSDCertFileLog{},
		&cbsd.CbrsLog{},
		&cbsd.SasConfig{},
		&cbsd.CbsdCertificate{},

		// misc (40)
		&misc.BatchAddObjectTask{},
		&misc.BatchAddObjectTaskLog{},
		&misc.BatchConfigurationLog{},
		&misc.BatchConfigurationDeviceLog{},
		&misc.BatchProcessFile{},
		&misc.BatchProcessFileSendLog{},
		&misc.BackupOrRestoreTask{},
		&misc.RestoreAndBackUpDeviceLog{},
		&misc.MRData{},
		&misc.MRFileLog{},
		&misc.MRUploadTask{},
		&misc.MRUploadTaskHasElement{},
		&misc.ZTPLog{},
		&misc.ZTPRetryLog{},
		&misc.ZTPFileSendLog{},
		&misc.ZTPGnbIdUsed{},
		&misc.ZTPTACUsed{},
		&misc.NorthReport{},
		&misc.NorthInterfaceLog{},
		&misc.Radius{},
		&misc.UploadFile{},
		&misc.EmailNoticeResult{},
		&misc.SSHLabel{},
		&misc.SystemOperatorLog{},
		&misc.ConfigUploadLog{},
		&misc.PresetParametersTask{},
		&misc.ErrorInfo{},
		&misc.RemoteUpload{},
		&misc.CallTraceFileLog{},
		&misc.MACPMFileLog{},
		&misc.RPCMethod{},
		&misc.PSAPID{},
		&misc.PSAPIDSyncLog{},
		&misc.TBG{},
		&misc.SpatialFileMarket{},
		&misc.DeviceFileDownloadLog{},
		&misc.NMSUpgradeAndRollbackLog{},
		&misc.CaptureLog{},
		&misc.CaptureFileLog{},

		// mnormalfile (4)
		&mnormalfile.MNormalFile{},
		&mnormalfile.MNormalFileChunk{},
		&mnormalfile.MNormalFileDownloadLog{},
		&mnormalfile.DeviceMNormalFile{},

		// ssh (1)
		&sshmod.SSHAccessTimerTask{},

		// parammonitor (2)
		&parammonitor.ParameterMonitorConfig{},
		&parammonitor.MonitorConfigHasParameter{},
		&parammonitor.ThresholdRule{},

		// paramcompare (1)
		&paramcompare.TemplateValue{},

		// devicelog (1)
		&devicelog.NeLog{},

		// nmsbackup (3)
		&nmsbackup.NMSBackupAndRevertTask{},
		&nmsbackup.NMSBackupAndRevert{},
		&nmsbackup.NMSBackupAndRevertLog{},

		// restapi (1)
		&restapi.TBGDevice{},

		// cacert (3)
		&cacert.CaFile{},
		&cacert.CaTask{},
		&cacert.DeviceSendCaLog{},

		// systemsettings (1)
		&systemsettings.SysParameter{},

		// snmp (2)
		&snmp.SnmpTrapLog{},
		&snmp.SnmpOperationLog{},

		// heartbeat (1)
		&heartbeat.HeartbeatRecord{},

		// diagnostics (1) — parameter_value shared with TR-069 engine
		&diagnostics.ParameterValue{},

		// pmfile (2)
		&pmfile.PMFile{},
		&pmfile.PMKPIMeasurement{},

		// reset (1) — ResetTask table for scheduled factory-reset jobs
		// (mirrors Java ResetTask entity). TaskToEventLog is shared with
		// eventlog and already registered there.
		&reset.ResetTask{},

		// pm replenish task (1) — mirrors Java PMReplenishTask. The
		// replenish worker reads pending tasks by status IN (1,2).
		&pm.PMReplenishTask{},
	}

	logger.Infof("auto migrating %d model tables...", len(models))
	if err := db.AutoMigrate(models...); err != nil {
		return err
	}
	logger.Info("all tables migrated successfully")
	return nil
}
