package scheduledtask

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
	"nmsappsrv/internal/snmp"
	"nmsappsrv/pkg/logger"
)

// NMSHeartbeatTask NMS心跳SNMP trap定时任务
// 每2分钟执行，发送SNMP trap心跳到北向服务器
type NMSHeartbeatTask struct {
	db            *gorm.DB
	sequenceNumber int64 // 序号，递增
}

// NewNMSHeartbeatTask 创建 NMSHeartbeatTask 实例
func NewNMSHeartbeatTask(db *gorm.DB) *NMSHeartbeatTask {
	return &NMSHeartbeatTask{
		db:            db,
		sequenceNumber: 0,
	}
}

// SendHeartbeat 执行心跳发送
// 1. 查询所有License
// 2. 检查是否有alarm_trap类型的north_report
// 3. 构造并发送SNMP trap
func (t *NMSHeartbeatTask) SendHeartbeat() {
	// 获取企业OID
	enterpriseOID := t.getEnterpriseOID()

	// 获取NMS IP和Hostname
	nmsIP := t.getNMSIP()
	nmsHostname, _ := os.Hostname()

	// 查询所有License
	var licenses []struct {
		Id      int    `gorm:"column:id"`
		OmcName *string `gorm:"column:omc_name"`
	}
	if err := t.db.Table("tenant").Select("id, omc_name").Find(&licenses).Error; err != nil {
		logger.Errorf("NMSHeartbeatTask: failed to query licenses: %v", err)
		return
	}

	for _, license := range licenses {
		t.sendHeartbeatForLicense(license.Id, license.OmcName, enterpriseOID, nmsIP, nmsHostname)
	}
}

// sendHeartbeatForLicense 为指定License发送心跳
func (t *NMSHeartbeatTask) sendHeartbeatForLicense(tenantId int, omcName *string, enterpriseOID uint32, nmsIP, nmsHostname string) {
	// 查询该 license 下 task_type=alarm_trap 且 task_state=true 的 north_report
	// 记录。Java 的 NorthReport 实体没有 port/community 字段 — 完整连接信息
	// 编码在 server_url 字段里（snmp://... 格式），通过 ParseConnectionURL 解析。
	// 这里直接读取 north_report 行，再用 snmp.ParseConnectionURL 解析 server_url，
	// 对齐 Java NMSHeartbeatTask + AlarmGeneratedPostprocessor.getConnectionInfo。
	var reports []struct {
		Id        int     `gorm:"column:id"`
		ServerUrl *string `gorm:"column:server_url"`
	}
	if err := t.db.Table("north_report").
		Select("id, server_url").
		Where("tenant_id = ? AND task_type = ? AND task_state = ?", tenantId, "alarm_trap", true).
		Find(&reports).Error; err != nil {
		logger.Errorf("NMSHeartbeatTask: failed to query north_report for license %d: %v", tenantId, err)
		return
	}

	if len(reports) == 0 {
		return
	}

	// NMS名称
	nmsName := ""
	if omcName != nil {
		nmsName = *omcName
	}

	// 当前时间字符串 (ISO 8601 with timezone, matches Java NMSHeartbeatTask:
	// DateFormatUtils.format(new Date(), "yyyy-MM-dd'T'HH:mm:ssXXX"))
	eventTime := time.Now().Format("2006-01-02T15:04:05Z07:00")

	// 递增序号
	seqNum := atomic.AddInt64(&t.sequenceNumber, 1)

	// 构造OID前缀: 1.3.6.1.4.1.{oid}.17
	oidPrefix := fmt.Sprintf(".1.3.6.1.4.1.%d.17", enterpriseOID)
	trapOID := fmt.Sprintf("1.3.6.1.4.1.%d.17", enterpriseOID)

	// 构造SNMP参数
	params := []snmp.SnmpParameter{
		{OID: oidPrefix + ".1", Type: "string", Value: eventTime},                         // 当前时间
		{OID: oidPrefix + ".2", Type: "int32", Value: strconv.FormatInt(seqNum, 10)},      // 序号
		{OID: oidPrefix + ".3", Type: "string", Value: nmsName},                          // NMS名称
		{OID: oidPrefix + ".4", Type: "string", Value: nmsIP},                            // NMS IP
		{OID: oidPrefix + ".5", Type: "string", Value: nmsHostname},                      // NMS hostname
	}

	// 添加trap OID标记
	trapMarker := snmp.SnmpParameter{
		OID:   ".1.3.6.1.6.3.1.1.4.1.0",
		Type:  "oid",
		Value: trapOID,
	}
	params = append([]snmp.SnmpParameter{trapMarker}, params...)

	// 对每个 north_report 解析 server_url 后发送 trap
	for _, report := range reports {
		if report.ServerUrl == nil || *report.ServerUrl == "" {
			continue
		}
		connInfo, err := snmp.ParseConnectionURL(*report.ServerUrl)
		if err != nil {
			logger.Errorf("NMSHeartbeatTask: failed to parse server_url %q for license %d: %v",
				*report.ServerUrl, tenantId, err)
			continue
		}

		if err := snmp.SendTrap(*connInfo, params); err != nil {
			logger.Errorf("NMSHeartbeatTask: failed to send trap to %s:%d for license %d: %v",
				connInfo.IP, connInfo.Port, tenantId, err)
			continue
		}

		logger.Infof("NMSHeartbeatTask: sent heartbeat trap to %s:%d for license %d (seq=%d)",
			connInfo.IP, connInfo.Port, tenantId, seqNum)
	}
}

// getEnterpriseOID 从system_config获取企业OID
func (t *NMSHeartbeatTask) getEnterpriseOID() uint32 {
	var configStr *string
	if err := t.db.Table("system_config").
		Select("config").
		Where("id = ?", "enterprise_oid").
		Scan(&configStr).Error; err != nil {
		logger.Warnf("NMSHeartbeatTask: failed to get enterprise_oid from system_config: %v, using default", err)
		return snmp.DefaultEnterpriseOID
	}

	if configStr == nil || *configStr == "" {
		return snmp.DefaultEnterpriseOID
	}

	// 尝试解析OID
	oid, err := strconv.ParseUint(*configStr, 10, 32)
	if err != nil {
		logger.Warnf("NMSHeartbeatTask: invalid enterprise_oid value %q: %v, using default", *configStr, err)
		return snmp.DefaultEnterpriseOID
	}

	return uint32(oid)
}

// getNMSIP 获取NMS的IP地址
func (t *NMSHeartbeatTask) getNMSIP() string {
	// 获取本机首选非回环IP地址
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		logger.Warnf("NMSHeartbeatTask: failed to get interface addresses: %v", err)
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return "127.0.0.1"
}