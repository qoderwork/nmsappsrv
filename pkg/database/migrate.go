package database

import (
	"nmsappsrv/internal/alarm"
	"nmsappsrv/internal/cacert"
	"nmsappsrv/internal/cbsd"
	"nmsappsrv/internal/corenet"
	"nmsappsrv/internal/device"
	"nmsappsrv/internal/devicelog"
	"nmsappsrv/internal/eventlog"
	"nmsappsrv/internal/heartbeat"
	"nmsappsrv/internal/license"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/mml"
	"nmsappsrv/internal/monitor"
	"nmsappsrv/internal/nmsbackup"
	"nmsappsrv/internal/parameter"
	"nmsappsrv/internal/paramcompare"
	"nmsappsrv/internal/parammonitor"
	"nmsappsrv/internal/pm"
	"nmsappsrv/internal/pmfile"
	"nmsappsrv/internal/restapi"
	"nmsappsrv/internal/site"
	"nmsappsrv/internal/tcpdump"
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

		// parameter (8)
		&parameter.Parameter{},
		&parameter.ParameterAttributes{},
		&parameter.ParameterLog{},
		&parameter.ParameterBackupLog{},
		&parameter.ParameterSet{},
		&parameter.ParameterSetHasParameter{},
		&parameter.ParameterTemplate{},
		&parameter.ParameterTemplateHasParameter{},

		// upgrade (8)
		&upgrade.UpgradeTask{},
		&upgrade.UpgradeFile{},
		&upgrade.UpgradeLog{},
		&upgrade.RollbackTask{},
		&upgrade.RebootTask{},
		&upgrade.ShutdownMyTask{},
		&upgrade.ShutdownLog{},
		&upgrade.EUAndRUBatchUpgradeLog{},

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

		// mml (4)
		&mml.MmlCommand{},
		&mml.MmlCommandParam{},
		&mml.MmlSet{},
		&mml.MmlExecuteResult{},

		// cbsd (4)
		&cbsd.CbsdInfo{},
		&cbsd.CBSDCertFileSendTask{},
		&cbsd.SendCBSDCertFileLog{},
		&cbsd.CbrsLog{},

		// misc (31)
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
		// tcpdump (1)
		&tcpdump.TcpdumpTask{},

		// pmfile (2)
		&pmfile.PMFile{},
		&pmfile.PMKPIMeasurement{},
	}

	logger.Infof("auto migrating %d model tables...", len(models))
	if err := db.AutoMigrate(models...); err != nil {
		return err
	}
	logger.Info("all tables migrated successfully")
	return nil
}
