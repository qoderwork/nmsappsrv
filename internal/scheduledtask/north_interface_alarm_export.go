package scheduledtask

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
	"nmsappsrv/pkg/logger"
)

// NorthInterfaceAlarmExport 北向告警导出定时任务
// 每30分钟执行，导出未导出的活跃告警到CSV文件
type NorthInterfaceAlarmExport struct {
	db         *gorm.DB
	exportPath string
}

// alarmExportRow 查询告警的行数据
type alarmExportRow struct {
	Id                    int64      `gorm:"column:id"`
	AdditionalInformation *string    `gorm:"column:additional_information"`
	AlarmIdentifier       *string    `gorm:"column:alarm_identifier"`
	AlarmSource           *string    `gorm:"column:alarm_source"`
	AlarmStatus           *int       `gorm:"column:alarm_status"`
	EventType             *string    `gorm:"column:event_type"`
	Severity              *string    `gorm:"column:severity"`
	NetworkElement        *string    `gorm:"column:network_element"`
	ProbableCause         *string    `gorm:"column:probable_cause"`
	SpecificProblem       *string    `gorm:"column:specific_problem"`
	AlarmType             *int       `gorm:"column:alarm_type"`
	EventTime             *time.Time `gorm:"column:event_time"`
	ElementId             *int64     `gorm:"column:element_id"`
	SerialNumber          *string    `gorm:"column:serial_number"`
}

// NewNorthInterfaceAlarmExport 创建 NorthInterfaceAlarmExport 实例
func NewNorthInterfaceAlarmExport(db *gorm.DB, exportPath string) *NorthInterfaceAlarmExport {
	return &NorthInterfaceAlarmExport{
		db:         db,
		exportPath: exportPath,
	}
}

// Export 执行告警导出
// 1. 查询所有租户
// 2. 对每个租户下的每个设备，查询未导出的活跃告警
// 3. 导出为CSV文件
// 4. 更新告警的 exported 标志
func (t *NorthInterfaceAlarmExport) Export() {
	// 查询所有租户
	var tenancies []struct {
		Id int `gorm:"column:id"`
	}
	if err := t.db.Table("tenant").Select("id").Find(&tenancies).Error; err != nil {
		logger.Errorf("NorthInterfaceAlarmExport: failed to query tenancies: %v", err)
		return
	}

	for _, tenancy := range tenancies {
		tenantId := tenancy.Id
		t.exportForTenancy(tenantId)
	}
}

// exportForTenancy 导出指定租户的未导出告警
func (t *NorthInterfaceAlarmExport) exportForTenancy(tenantId int) {
	// 查询该租户下的所有设备
	var devices []struct {
		ElementId    int64  `gorm:"column:ne_neid"`
		SerialNumber string `gorm:"column:serial_number"`
	}
	if err := t.db.Table("cpe_element").
		Select("ne_neid, serial_number").
		Where("tenant_id = ? AND deleted = ?", tenantId, false).
		Where("serial_number IS NOT NULL AND serial_number != ''").
		Find(&devices).Error; err != nil {
		logger.Errorf("NorthInterfaceAlarmExport: failed to query devices for license %d: %v", tenantId, err)
		return
	}

	for _, device := range devices {
		t.exportForDevice(tenantId, device.ElementId, device.SerialNumber)
	}
}

// exportForDevice 导出指定设备的未导出告警
func (t *NorthInterfaceAlarmExport) exportForDevice(tenantId int, elementId int64, serialNumber string) {
	// 查询未导出的活跃告警: alarm_type=1 (活跃), exported IS NULL
	var alarms []alarmExportRow
	if err := t.db.Table("alarm").
		Select("alarm.id, alarm.additional_information, alarm.alarm_identifier, alarm.alarm_source, "+
			"alarm.alarm_status, alarm.event_type, alarm.severity, alarm.network_element, "+
			"alarm.probable_cause, alarm.specific_problem, alarm.alarm_type, alarm.event_time, "+
			"alarm.element_id, cpe_element.serial_number").
		Joins("LEFT JOIN cpe_element ON alarm.element_id = cpe_element.ne_neid").
		Where("alarm.tenant_id = ? AND alarm.element_id = ?", tenantId, elementId).
		Where("alarm.alarm_type = ?", 1). // 活跃告警
		Where("alarm.exported IS NULL").
		Order("alarm.id ASC").
		Find(&alarms).Error; err != nil {
		logger.Errorf("NorthInterfaceAlarmExport: failed to query alarms for device %d: %v", elementId, err)
		return
	}

	if len(alarms) == 0 {
		return
	}

	// 创建导出目录: {exportPath}/{tenantId}/
	exportDir := filepath.Join(t.exportPath, fmt.Sprintf("%d", tenantId))
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		logger.Errorf("NorthInterfaceAlarmExport: failed to create export directory %s: %v", exportDir, err)
		return
	}

	// 生成文件名: FM_{timestamp}_{sn}.csv
	// Go 时间格式使用参考时间 2006-01-02 15:04:05
	timestamp := time.Now().Format("20060102150405")
	fileName := fmt.Sprintf("FM_%s_%s.csv", timestamp, serialNumber)
	filePath := filepath.Join(exportDir, fileName)

	// 写入CSV文件
	if err := t.writeCSV(filePath, alarms); err != nil {
		logger.Errorf("NorthInterfaceAlarmExport: failed to write CSV %s: %v", filePath, err)
		return
	}

	logger.Infof("NorthInterfaceAlarmExport: exported %d alarms to %s", len(alarms), filePath)

	// 更新告警的 exported 标志
	var alarmIds []int64
	for _, a := range alarms {
		alarmIds = append(alarmIds, a.Id)
	}
	exported := true
	if err := t.db.Table("alarm").
		Where("id IN ?", alarmIds).
		Update("exported", exported).Error; err != nil {
		logger.Errorf("NorthInterfaceAlarmExport: failed to update exported flag: %v", err)
	}
}

// writeCSV 写入告警到CSV文件
func (t *NorthInterfaceAlarmExport) writeCSV(filePath string, alarms []alarmExportRow) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// 写入UTF-8 BOM，确保Excel正确识别编码
	file.WriteString("\xEF\xBB\xBF")

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	header := []string{
		"Index", "AdditionalInformation", "AlarmIdentifier", "AlarmSource",
		"AlarmStatus", "EventType", "Severity", "NetworkElement",
		"ProbableCause", "SpecificProblem", "AlarmType", "EventTime",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// 写入数据行
	for i, alarm := range alarms {
		row := []string{
			fmt.Sprintf("%d", i+1),                                              // Index
			nullString(alarm.AdditionalInformation),                             // AdditionalInformation
			nullString(alarm.AlarmIdentifier),                                   // AlarmIdentifier
			nullString(alarm.AlarmSource),                                       // AlarmSource
			nullIntToString(alarm.AlarmStatus),                                  // AlarmStatus
			nullString(alarm.EventType),                                         // EventType
			nullString(alarm.Severity),                                          // Severity
			nullString(alarm.NetworkElement),                                    // NetworkElement
			nullString(alarm.ProbableCause),                                     // ProbableCause
			nullString(alarm.SpecificProblem),                                   // SpecificProblem
			nullIntToString(alarm.AlarmType),                                    // AlarmType
			nullTimeToString(alarm.EventTime, "2006-01-02 15:04:05"), // EventTime
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row %d: %w", i+1, err)
		}
	}

	return nil
}

// nullString 返回字符串指针的值，如果为nil则返回空字符串
func nullString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// nullIntToString 返回int指针的字符串表示，如果为nil则返回空字符串
func nullIntToString(i *int) string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%d", *i)
}

// nullTimeToString 返回时间指针的格式化字符串，如果为nil则返回空字符串
func nullTimeToString(t *time.Time, format string) string {
	if t == nil {
		return ""
	}
	return t.Format(format)
}