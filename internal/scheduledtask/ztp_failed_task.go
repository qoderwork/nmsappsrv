package scheduledtask

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// ZTPFailedTask ZTP失败任务清理（每1分钟）
// 查询 ztp_log 表中 has_fault=true 且超过一定时间（如30分钟）的记录
// 删除相关的：
//   - AOS 文件（aos_file 表）
//   - geo 缓存（Redis key `geo_{elementId}`）
//   - ZTP 日志本身
// 发送 ztp_failed 通知到北向
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

// CleanupFailed 执行失败 ZTP 记录清理
// 1. 查询 has_fault=true 且 end_time < now-timeoutMins 的 ztp_log 记录
// 2. 对每条记录：
//    - 删除 aos_file 记录（如果存在）
//    - 删除 Redis geo 缓存
//    - 导出失败通知到北向
//    - 删除 ztp_log 记录
func (t *ZTPFailedTask) CleanupFailed() {
	ctx := context.Background()
	cutoff := time.Now().Add(-time.Duration(t.timeoutMins) * time.Minute)

	// 查询失败的 ZTP 日志：has_fault=true 且 end_time 早于 cutoff
	var logs []ztpLogRow
	if err := t.db.Table("ztp_log").
		Where("has_fault = ? AND end_time IS NOT NULL AND end_time < ?", true, cutoff).
		Order("id ASC").
		Find(&logs).Error; err != nil {
		logger.Errorf("ZTPFailedTask: query ztp_log failed: %v", err)
		return
	}

	if len(logs) == 0 {
		return
	}

	for _, log := range logs {
		t.cleanupOne(ctx, log)
	}
}

// cleanupOne 清理单条失败的 ZTP 记录
func (t *ZTPFailedTask) cleanupOne(ctx context.Context, log ztpLogRow) {
	elementId := int64Val2(log.ElementId)
	if elementId == 0 {
		return
	}

	// 1. 删除 aos_file 记录（如果存在）
	// AOS 文件路径存储在 cpe_element.aos_file_name 中
	var aosFileName string
	t.db.Table("cpe_element").
		Select("aos_file_name").
		Where("ne_neid = ?", elementId).
		Scan(&aosFileName)

	if aosFileName != "" {
		// 删除 AOS 文件记录（如果存在独立表）
		// 注意：当前系统中 AOS 文件名直接存储在 cpe_element.aos_file_name
		// 这里清理 cpe_element 中的 aos_file_name 字段
		t.db.Table("cpe_element").
			Where("ne_neid = ?", elementId).
			Updates(map[string]interface{}{
				"aos_file_name": nil,
				"read_to_ztp":   false,
			})
	}

	// 2. 删除 Redis geo 缓存
	geoKey := fmt.Sprintf("geo_%d", elementId)
	redis.Del(ctx, geoKey)

	// 3. 导出失败通知到北向（CSV 文件）
	t.exportFailedNotification(log, elementId)

	// 4. 删除 ztp_log 记录
	if err := t.db.Table("ztp_log").Where("id = ?", log.Id).Delete(nil).Error; err != nil {
		logger.Errorf("ZTPFailedTask: failed to delete ztp_log %d: %v", log.Id, err)
		return
	}

	logger.Infof("ZTPFailedTask: cleaned up failed ZTP log %d for element %d", log.Id, elementId)
}

// exportFailedNotification 导出 ZTP 失败通知到北向 CSV
func (t *ZTPFailedTask) exportFailedNotification(log ztpLogRow, elementId int64) {
	// 查询设备信息
	var deviceInfoRow struct {
		SerialNumber string `gorm:"column:serial_number"`
		DeviceName   string `gorm:"column:device_name"`
		TenantId    int    `gorm:"column:tenant_id"`
	}
	t.db.Table("cpe_element").
		Select("serial_number, device_name, tenant_id").
		Where("ne_neid = ?", elementId).
		Scan(&deviceInfoRow)

	deviceInfo := ztpDeviceInfo{
		SerialNumber: deviceInfoRow.SerialNumber,
		DeviceName:   deviceInfoRow.DeviceName,
		TenantId:    deviceInfoRow.TenantId,
	}

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