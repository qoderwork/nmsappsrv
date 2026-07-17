package scheduledtask

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// CertificateAlarmTask HTTPS证书告警
// 镜像 Java NMSCertificateAlarmTriggerTask
// 每小时执行，检查证书有效期，
// 距过期 <= 20 天时插入告警到 alarm 表
type CertificateAlarmTask struct {
	db       *gorm.DB
	certPath string
}

// 证书告警阈值：距过期 <= certAlarmThresholdDays 天触发告警
const certAlarmThresholdDays = 20

// NewCertificateAlarmTask 创建 CertificateAlarmTask 实例
func NewCertificateAlarmTask(db *gorm.DB, certPath string) *CertificateAlarmTask {
	return &CertificateAlarmTask{
		db:       db,
		certPath: certPath,
	}
}

// CheckCertificate 检查证书有效期并触发告警
// 1. 读取证书文件，解析有效期
// 2. 如果距过期 <= 20 天，插入告警到 alarm 表
// 3. 日志记录
func (t *CertificateAlarmTask) CheckCertificate() {
	// 1. 读取并解析证书
	notAfter, err := t.parseCertificateExpiry()
	if err != nil {
		logger.Errorf("CertificateAlarmTask: failed to parse certificate %s: %v", t.certPath, err)
		return
	}

	now := time.Now()
	daysUntilExpiry := int(notAfter.Sub(now).Hours() / 24)

	logger.Infof("CertificateAlarmTask: certificate expires at %s, %d days remaining", notAfter.Format("2006-01-02 15:04:05"), daysUntilExpiry)

	// 2. 距过期 <= 20 天时插入告警
	if daysUntilExpiry <= certAlarmThresholdDays {
		t.raiseAlarm(daysUntilExpiry, notAfter)
	}
}

// parseCertificateExpiry 读取证书文件并解析出过期时间
func (t *CertificateAlarmTask) parseCertificateExpiry() (time.Time, error) {
	data, err := os.ReadFile(t.certPath)
	if err != nil {
		return time.Time{}, err
	}

	// 尝试 PEM 解码
	block, _ := pem.Decode(data)
	var certBytes []byte
	if block != nil {
		certBytes = block.Bytes
	} else {
		// 非 PEM 格式，尝试直接 DER 解析
		certBytes = data
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return time.Time{}, err
	}

	return cert.NotAfter, nil
}

// raiseAlarm 插入证书过期告警到 alarm 表
func (t *CertificateAlarmTask) raiseAlarm(daysUntilExpiry int, notAfter time.Time) {
	severity := "Major"
	alarmIdentifier := "CertificateExpiring"
	probableCause := "Certificate about to expire"
	description := "HTTPS certificate will expire in %d days (expiry: %s)"
	now := time.Now()
	alarmType := 1 // AlarmTypeActive
	alarmStatus := 1 // AlarmStatusActiveUnconfirmed

	// 检查是否已存在相同的活跃告警，避免重复插入
	var existingCount int64
	t.db.Table("alarm").
		Where("alarm_identifier = ? AND alarm_status IN (?)", alarmIdentifier, []int{1, 3}).
		Count(&existingCount)
	if existingCount > 0 {
		logger.Infof("CertificateAlarmTask: active certificate alarm already exists, skipping")
		return
	}

	desc := fmt.Sprintf(description, daysUntilExpiry, notAfter.Format("2006-01-02 15:04:05"))
	result := t.db.Table("alarm").Create(map[string]interface{}{
		"alarm_type":       alarmType,
		"severity":         severity,
		"alarm_identifier": alarmIdentifier,
		"probable_cause":   probableCause,
		"event_time":       now,
		"alarm_status":     alarmStatus,
		"description":      desc,
		"create_time":      now,
	})
	if result.Error != nil {
		logger.Errorf("CertificateAlarmTask: failed to insert alarm: %v", result.Error)
		return
	}

	logger.Warnf("CertificateAlarmTask: certificate expiring in %d days (expiry: %s), alarm raised", daysUntilExpiry, notAfter.Format("2006-01-02 15:04:05"))
}
