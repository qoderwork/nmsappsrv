package scheduledtask

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// CMBackupTask CM配置备份（每天00:00）
// 按租户导出 enb 设备配置（Common + AMF）为 CSV 文件
// CSV 路径：{basePath}/{tenancyId}/CM/{date}/
// 文件名：CM_{timestamp}_{sn}.csv
type CMBackupTask struct {
	db         *gorm.DB
	exportPath string
}

// cmParamPath CM 配置参数路径定义
// 镜像 Java CMBackupTask 中常见的关键参数
var cmParamPaths = []string{
	// Common 配置
	"Device.FAP.Service.Status",
	"Device.FAP.Service.OperationalStatus",
	"Device.FAP.Service.DownstreamFrequency",
	"Device.FAP.Service.UpstreamFrequency",
	"Device.FAP.Service.DownstreamPower",
	"Device.FAP.Service.UpstreamPower",
	"Device.Services.FAPService.1.FAPControl.NR.CellIdentity",
	"Device.Services.FAPService.1.FAPControl.NR.PCI",
	"Device.Services.FAPService.1.FAPControl.NR.TAC",
	"Device.Services.FAPService.1.FAPControl.NR.ARFCN",
	"Device.Services.FAPService.1.FAPControl.NR.Band",
	"Device.Services.FAPService.1.FAPControl.NR.Bandwidth",
	// AMF 配置
	"Device.Services.FAPService.1.FAPControl.NR.AMFPoolConfigParam.1.GUAMI.1.PLMNID",
	"Device.Services.FAPService.1.FAPControl.NR.AMFPoolConfigParam.1.GUAMI.1.AMFID",
	"Device.Services.FAPService.1.FAPControl.NR.AMFPoolConfigParam.1.GUAMI.1.AMFPointer",
	"Device.Services.FAPService.1.FAPControl.NR.AMFPoolConfigParam.1.SupportPLMNList.1.PLMNID",
	"Device.Services.FAPService.1.CellConfig.1.NR.NGC.Slice.SliceList.1.SNSSAIList.1.SST",
	"Device.Services.FAPService.1.CellConfig.1.NR.NGC.Slice.SliceList.1.SNSSAIList.1.SD",
}

// NewCMBackupTask 创建 CMBackupTask 实例
func NewCMBackupTask(db *gorm.DB, exportPath string) *CMBackupTask {
	return &CMBackupTask{
		db:         db,
		exportPath: exportPath,
	}
}

// ExportCM 执行 CM 配置导出
// 1. 查询所有租户
// 2. 对每个租户查询 enb 设备
// 3. 获取设备的 Common + AMF 配置参数
// 4. 导出为 CSV 文件
func (t *CMBackupTask) ExportCM() {
	// 查询所有租户
	var tenancies []struct {
		Id int `gorm:"column:id"`
	}
	if err := t.db.Table("license").Select("id").Find(&tenancies).Error; err != nil {
		logger.Errorf("CMBackupTask: failed to query tenancies: %v", err)
		return
	}

	for _, tenancy := range tenancies {
		t.exportForTenancy(tenancy.Id)
	}
}

// exportForTenancy 导出指定租户的 CM 配置
func (t *CMBackupTask) exportForTenancy(licenseId int) {
	// 查询该租户下的所有 enb 设备
	var devices []struct {
		ElementId    int64  `gorm:"column:ne_neid"`
		SerialNumber string `gorm:"column:serial_number"`
		DeviceName   string `gorm:"column:device_name"`
	}
	if err := t.db.Table("cpe_element").
		Select("ne_neid, serial_number, device_name").
		Where("license_id = ? AND deleted = ?", licenseId, false).
		Where("device_type = ?", "enb").
		Where("serial_number IS NOT NULL AND serial_number != ''").
		Find(&devices).Error; err != nil {
		logger.Errorf("CMBackupTask: failed to query devices for license %d: %v", licenseId, err)
		return
	}

	if len(devices) == 0 {
		return
	}

	// 创建导出目录: {exportPath}/{licenseId}/CM/{date}/
	date := time.Now().Format("20060102")
	exportDir := filepath.Join(t.exportPath, fmt.Sprintf("%d", licenseId), "CM", date)
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		logger.Errorf("CMBackupTask: failed to create export directory %s: %v", exportDir, err)
		return
	}

	for _, device := range devices {
		t.exportForDevice(licenseId, device.ElementId, device.SerialNumber, device.DeviceName, exportDir)
	}

	logger.Infof("CMBackupTask: exported CM config for %d devices in license %d", len(devices), licenseId)
}

// exportForDevice 导出单个设备的 CM 配置
func (t *CMBackupTask) exportForDevice(licenseId int, elementId int64, serialNumber, deviceName, exportDir string) {
	// 查询设备的参数值
	paramValues := t.getParamValues(elementId)

	// 生成文件名: CM_{timestamp}_{sn}.csv
	timestamp := time.Now().Format("20060102150405")
	fileName := fmt.Sprintf("CM_%s_%s.csv", timestamp, serialNumber)
	filePath := filepath.Join(exportDir, fileName)

	// 写入 CSV 文件
	if err := t.writeCMCSV(filePath, elementId, serialNumber, deviceName, paramValues); err != nil {
		logger.Errorf("CMBackupTask: failed to write CSV %s: %v", filePath, err)
		return
	}

	logger.Infof("CMBackupTask: exported CM config for device %s to %s", serialNumber, filePath)
}

// getParamValues 获取设备的参数值
func (t *CMBackupTask) getParamValues(elementId int64) map[string]string {
	values := make(map[string]string)

	for _, path := range cmParamPaths {
		var paramValue string
		t.db.Table("element_basic_info_parameter").
			Select("param_value").
			Where("element_id = ? AND param_name = ?", elementId, path).
			Scan(&paramValue)
		values[path] = paramValue
	}

	return values
}

// writeCMCSV 写入 CM 配置到 CSV 文件
func (t *CMBackupTask) writeCMCSV(filePath string, elementId int64, serialNumber, deviceName string, paramValues map[string]string) error {
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
	header := []string{"ElementId", "SerialNumber", "DeviceName", "ParameterPath", "ParameterValue"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// 写入数据行
	for _, path := range cmParamPaths {
		row := []string{
			fmt.Sprintf("%d", elementId),
			serialNumber,
			deviceName,
			path,
			paramValues[path],
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row for path %s: %w", path, err)
		}
	}

	return nil
}