package scheduledtask

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// ZTPFailedTask ZTP失败任务（每1分钟）
// 对齐 Java ZTPTask.deleteFailedTask + ZTPFailedConsumer：
//   Phase 1 (publishFailedOne): 检测→写重试日志→LPUSH Redis ztp_failed_notify→删除 ztp_log→CSV导出
//   Phase 2 (consumeFailedNotifications): RPOP Redis→清除 e911_data→清除 aos_file→删除 geo 缓存
// Redis 队列替代 Java RabbitMQ ztpFailedNotify。
type ZTPFailedTask struct {
	db           *gorm.DB
	exportPath   string // 北向导出路径
	timeoutMins  int    // 超时分钟数，默认30分钟
}

// ztpDeviceInfo 用于存储设备信息
type ztpDeviceInfo struct {
	SerialNumber string
	DeviceName   string
	TenantId    int
}

// ztpLogRow 对应 ztp_log 表的行
type ztpLogRow struct {
	Id        int64      `gorm:"column:id"`
	ElementId *int64     `gorm:"column:element_id"`
	Progress  *int       `gorm:"column:progress"`
	Done      *bool      `gorm:"column:done"`
	Info      *string    `gorm:"column:info;type:longtext"`
	StartTime *time.Time `gorm:"column:start_time"`
	EndTime   *time.Time `gorm:"column:end_time"`
	HasFault  *bool      `gorm:"column:has_fault"`
}

// ztpFailedNotifyMsg is the JSON payload pushed to the Redis ztp_failed_notify
// queue, replacing Java's RabbitMQ ztpFailedNotify.
type ztpFailedNotifyMsg struct {
	ElementId    int64  `json:"elementId"`
	SerialNumber string `json:"serialNumber"`
	DeviceName   string `json:"deviceName"`
	Info         string `json:"info"`
	TenantId     int    `json:"tenantId"`
}

const ztpFailedNotifyQueue = "ztp_failed_notify"

// NewZTPFailedTask 创建 ZTPFailedTask 实例
func NewZTPFailedTask(db *gorm.DB, exportPath string, timeoutMins int) *ZTPFailedTask {
	if timeoutMins <= 0 {
		timeoutMins = 30
	}
	return &ZTPFailedTask{
		db:          db,
		exportPath:  exportPath,
		timeoutMins: timeoutMins,
	}
}

// CleanupFailed 执行失败 ZTP 记录检测与通知（对齐 Java ZTPTask.deleteFailedTask）
// 1. 查询 has_fault=true 或超时的 ztp_log
// 2. 对每条记录：写重试日志 → LPUSH Redis ztp_failed_notify（替代 Java RabbitMQ）
//    → 删除 ztp_log → CSV 北向导出
// 3. 消费 Redis 队列，对每条通知执行设备清理（对齐 Java ZTPFailedConsumer）
func (t *ZTPFailedTask) CleanupFailed() {
	ctx := context.Background()
	timeout := t.timeoutMins
	if to := t.loadZTPTimeoutMinutes(); to > 0 {
		timeout = to
	}
	cutoff := time.Now().Add(-time.Duration(timeout) * time.Minute)

	var logs []ztpLogRow
	if err := t.db.Table("ztp_log").
		Where("has_fault = ? OR (end_time IS NULL AND start_time < ?)", true, cutoff).
		Order("id ASC").
		Find(&logs).Error; err != nil {
		logger.Errorf("ZTPFailedTask: query ztp_log failed: %v", err)
		return
	}

	if len(logs) == 0 {
		return
	}

	// Phase 1: detection + notification (Java ZTPTask.deleteFailedTask).
	for _, log := range logs {
		t.publishFailedOne(ctx, log)
	}

	// Phase 2: consume + cleanup (Java ZTPFailedConsumer).
	t.consumeFailedNotifications(ctx)
}

// publishFailedOne 检测到失败设备后，写重试日志、推送 Redis 通知、删除 ztp_log、
// 导出 CSV（替代 RabbitMQ 的北向通知）。设备侧清理（e911/geo/aos）交由消费端处理。
func (t *ZTPFailedTask) publishFailedOne(ctx context.Context, log ztpLogRow) {
	elementId := int64Val2(log.ElementId)
	if elementId == 0 {
		return
	}

	// 1. 记录重试日志（Java: ZTPTask.deleteFailedTask 写 ZTPRetryLog）。
	now := time.Now()
	info := nullString2(log.Info)
	if info == "" {
		info = "ZTP failed"
	}
	t.db.Table("ztp_retry_log").Create(map[string]interface{}{
		"element_id": elementId,
		"retry_time": now,
		"info":       info,
	})

	// 2. 推送 Redis 通知（替代 Java RabbitMQ ztpFailedNotify）。
	deviceInfo := t.lookupDeviceInfo(elementId)
	msg := ztpFailedNotifyMsg{
		ElementId:    elementId,
		SerialNumber: deviceInfo.SerialNumber,
		DeviceName:   deviceInfo.DeviceName,
		Info:         info,
		TenantId:     deviceInfo.TenantId,
	}
	msgJSON, _ := json.Marshal(msg)
	if err := redis.LPush(ctx, ztpFailedNotifyQueue, string(msgJSON)); err != nil {
		logger.Errorf("ZTPFailedTask: LPUSH failed for element %d: %v", elementId, err)
	}

	// 3. 删除 ztp_log（Java: ZTPTask 删除日志后发 RabbitMQ）。
	if err := t.db.Table("ztp_log").Where("id = ?", log.Id).Delete(nil).Error; err != nil {
		logger.Errorf("ZTPFailedTask: failed to delete ztp_log %d: %v", log.Id, err)
	}

	// 4. 导出 CSV 北向通知（Go 补充的离线记录，保留）。
	t.exportFailedNotification(log, elementId)

	logger.Infof("ZTPFailedTask: published failed notification for element %d to Redis", elementId)
}

// consumeFailedNotifications 逐条消费 Redis ztp_failed_notify 队列，执行设备清理
//（对齐 Java ZTPFailedConsumer：清除 E911 引用 / aos_file_name / geo 缓存）。
func (t *ZTPFailedTask) consumeFailedNotifications(ctx context.Context) {
	for {
		raw, err := redis.RPop(ctx, ztpFailedNotifyQueue)
		if err != nil || raw == "" {
			return
		}
		var msg ztpFailedNotifyMsg
		if err := json.Unmarshal([]byte(raw), &msg); err != nil {
			logger.Warnf("ZTPFailedTask: bad notify msg: %v", err)
			continue
		}
		t.applyFailedCleanup(ctx, msg)
	}
}

// applyFailedCleanup 对单个失败设备执行清理：清除本地 e911_data、删除 geo 缓存、
// 清除 aos_file_name 并停止重新下发（对齐 Java ZTPFailedConsumer）。
func (t *ZTPFailedTask) applyFailedCleanup(ctx context.Context, msg ztpFailedNotifyMsg) {
	elementId := msg.ElementId

	// 1. 清除本地 e911_data 引用（Java 消费端同时远程注销 E911 系统，暂注：
	//    remote de-reg 通过 ztp/external 做，此处因包依赖限制未引入）。
	t.db.Table("cpe_element").
		Where("ne_neid = ?", elementId).
		Update("e911_data", nil)

	// 2. 清除 AOS 文件引用并停止重新下发（Java: updateAOSFileNameAndReadyToZTP(id, null, FALSE)）。
	t.db.Table("cpe_element").
		Where("ne_neid = ?", elementId).
		Updates(map[string]interface{}{
			"aos_file_name": nil,
			"read_to_ztp":   false,
		})

	// 3. 删除 Redis geo 缓存（Java key: device_geo_<id>）。
	geoKey := fmt.Sprintf("device_geo_%d", elementId)
	redis.Del(ctx, geoKey)

	logger.Infof("ZTPFailedNotify consumer: cleaned up device %d (%s)", elementId, msg.SerialNumber)
}

// lookupDeviceInfo queries cpe_element for serial_number, device_name, tenant_id.
func (t *ZTPFailedTask) lookupDeviceInfo(elementId int64) ztpDeviceInfo {
	var row struct {
		SerialNumber string `gorm:"column:serial_number"`
		DeviceName   string `gorm:"column:device_name"`
		TenantId     int    `gorm:"column:tenant_id"`
	}
	t.db.Table("cpe_element").
		Select("serial_number, device_name, tenant_id").
		Where("ne_neid = ?", elementId).
		Scan(&row)
	return ztpDeviceInfo{
		SerialNumber: row.SerialNumber,
		DeviceName:   row.DeviceName,
		TenantId:     row.TenantId,
	}
}

// loadZTPTimeoutMinutes reads ztpTimeoutTime (minutes) from system_config,
// mirroring Java's ZTPSetting.ztpTimeoutTime (default 15 min). Returns 0 when
// unset so the constructor's default (15) applies.
func (t *ZTPFailedTask) loadZTPTimeoutMinutes() int {
	var cfg string
	if err := t.db.Table("system_config").Select("config").Where("id = ?", "ztp_config").Scan(&cfg).Error; err != nil || cfg == "" {
		return 0
	}
	var s struct {
		ZTPTimeoutTime *int `json:"ztpTimeoutTime"`
	}
	if err := json.Unmarshal([]byte(cfg), &s); err != nil {
		return 0
	}
	if s.ZTPTimeoutTime != nil {
		return *s.ZTPTimeoutTime
	}
	return 0
}

// exportFailedNotification 导出 ZTP 失败通知到北向 CSV
func (t *ZTPFailedTask) exportFailedNotification(log ztpLogRow, elementId int64) {
	deviceInfo := t.lookupDeviceInfo(elementId)

	// 创建导出目录: {exportPath}/{tenantId}/ZTP/
	exportDir := filepath.Join(t.exportPath, fmt.Sprintf("%d", deviceInfo.TenantId), "ZTP")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		logger.Errorf("ZTPFailedTask: failed to create export directory %s: %v", exportDir, err)
		return
	}

	// 生成文件名: ZTP_failed_{timestamp}_{sn}.csv
	timestamp := time.Now().Format("20060102150405")
	fileName := fmt.Sprintf("ZTP_failed_%s_%s.csv", timestamp, deviceInfo.SerialNumber)
	filePath := filepath.Join(exportDir, fileName)

	// 写入 CSV 文件
	if err := t.writeFailedCSV(filePath, log, elementId, deviceInfo); err != nil {
		logger.Errorf("ZTPFailedTask: failed to write CSV %s: %v", filePath, err)
		return
	}

	logger.Infof("ZTPFailedTask: exported ZTP failed notification to %s", filePath)
}

// writeFailedCSV 写入 ZTP 失败通知到 CSV 文件
func (t *ZTPFailedTask) writeFailedCSV(filePath string, log ztpLogRow, elementId int64, deviceInfo ztpDeviceInfo) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// 写入 UTF-8 BOM
	file.WriteString("\xEF\xBB\xBF")

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	header := []string{
		"ElementId", "SerialNumber", "DeviceName", "Progress",
		"Info", "StartTime", "EndTime", "HasFault",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// 写入数据行
	row := []string{
		fmt.Sprintf("%d", elementId),
		deviceInfo.SerialNumber,
		deviceInfo.DeviceName,
		nullIntToString2(log.Progress),
		nullString2(log.Info),
		nullTimeToString2(log.StartTime, "2006-01-02 15:04:05"),
		nullTimeToString2(log.EndTime, "2006-01-02 15:04:05"),
		nullBoolToString(log.HasFault),
	}
	if err := writer.Write(row); err != nil {
		return fmt.Errorf("failed to write row: %w", err)
	}

	return nil
}

// 辅助函数（避免与 preset_parameters_task.go 中的同名函数冲突）
func int64Val2(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func nullString2(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nullIntToString2(i *int) string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%d", *i)
}

func nullTimeToString2(t *time.Time, format string) string {
	if t == nil {
		return ""
	}
	return t.Format(format)
}

func nullBoolToString(b *bool) string {
	if b == nil {
		return ""
	}
	if *b {
		return "true"
	}
	return "false"
}