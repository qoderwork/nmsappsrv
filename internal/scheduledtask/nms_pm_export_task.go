package scheduledtask

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"gorm.io/gorm"
	"nmsappsrv/pkg/logger"
)

// NMSPMExportTask NMS资源PM导出定时任务
// 每1分钟执行，追加写入CPU/RAM使用率到CSV文件
type NMSPMExportTask struct {
	db         *gorm.DB
	exportPath string
}

// NewNMSPMExportTask 创建 NMSPMExportTask 实例
func NewNMSPMExportTask(db *gorm.DB, exportPath string) *NMSPMExportTask {
	return &NMSPMExportTask{
		db:         db,
		exportPath: exportPath,
	}
}

// Export 执行PM数据导出
// 1. 查询所有租户
// 2. 获取当前CPU/RAM使用率
// 3. 追加写入到各租户的CSV文件
func (t *NMSPMExportTask) Export() {
	// 获取CPU使用率
	cpuPercent, err := t.getCPUUsage()
	if err != nil {
		logger.Warnf("NMSPMExportTask: failed to get CPU usage: %v, using runtime stats", err)
		// 如果gopsutil失败，使用runtime作为备选
		cpuPercent = t.getCPUUsageFromRuntime()
	}

	// 获取RAM使用率
	ramPercent, err := t.getRAMUsage()
	if err != nil {
		logger.Warnf("NMSPMExportTask: failed to get RAM usage: %v, using runtime stats", err)
		// 如果gopsutil失败，使用runtime作为备选
		ramPercent = t.getRAMUsageFromRuntime()
	}

	// 查询所有租户
	var tenancies []struct {
		Id int `gorm:"column:id"`
	}
	if err := t.db.Table("tenant").Select("id").Find(&tenancies).Error; err != nil {
		logger.Errorf("NMSPMExportTask: failed to query tenancies: %v", err)
		return
	}

	// 当前时间
	now := time.Now()
	timeStr := now.Format("2006-01-02 15:04:05")

	// 对每个租户写入PM数据
	for _, tenancy := range tenancies {
		t.writePMData(tenancy.Id, timeStr, cpuPercent, ramPercent)
	}
}

// getCPUUsage 获取CPU使用率百分比
func (t *NMSPMExportTask) getCPUUsage() (float64, error) {
	// 使用gopsutil获取CPU使用率（1秒采样）
	percent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return 0, err
	}
	if len(percent) == 0 {
		return 0, fmt.Errorf("no CPU data returned")
	}
	return percent[0], nil
}

// getCPUUsageFromRuntime 使用runtime获取CPU使用率估算（备选方案）
func (t *NMSPMExportTask) getCPUUsageFromRuntime() float64 {
	// runtime不直接提供CPU使用率，这里返回0表示无法获取
	// 在实际场景中，可以考虑其他方案如周期性采样
	return 0
}

// getRAMUsage 获取RAM使用率百分比
func (t *NMSPMExportTask) getRAMUsage() (float64, error) {
	// 使用gopsutil获取内存使用率
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return vmStat.UsedPercent, nil
}

// getRAMUsageFromRuntime 使用runtime获取内存使用率（备选方案）
func (t *NMSPMExportTask) getRAMUsageFromRuntime() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 计算已分配内存占系统内存的百分比
	// 注意：这只是Go进程的内存使用，不是系统全局内存
	// 如果需要系统内存，还是应该使用gopsutil
	// 这里我们用 Alloc / Sys 作为估算
	if m.Sys > 0 {
		return float64(m.Alloc) / float64(m.Sys) * 100
	}
	return 0
}

// writePMData 写入PM数据到CSV文件
func (t *NMSPMExportTask) writePMData(tenantId int, timeStr string, cpuPercent, ramPercent float64) {
	// 创建目录: {exportPath}/{tenantId}/PM/{date}/
	dateStr := time.Now().Format("2006-01-02")
	pmDir := filepath.Join(t.exportPath, fmt.Sprintf("%d", tenantId), "PM", dateStr)
	if err := os.MkdirAll(pmDir, 0755); err != nil {
		logger.Errorf("NMSPMExportTask: failed to create PM directory %s: %v", pmDir, err)
		return
	}

	// 文件路径: NMS_Resource.csv
	filePath := filepath.Join(pmDir, "NMS_Resource.csv")

	// 检查文件是否存在，决定是否写入表头
	fileExists := false
	if _, err := os.Stat(filePath); err == nil {
		fileExists = true
	}

	// 打开文件（追加模式）
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Errorf("NMSPMExportTask: failed to open file %s: %v", filePath, err)
		return
	}
	defer file.Close()

	// 如果是新文件，写入UTF-8 BOM
	if !fileExists {
		file.WriteString("\xEF\xBB\xBF")
	}

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 如果是新文件，写入表头
	if !fileExists {
		header := []string{"Time", "CPU", "RAM"}
		if err := writer.Write(header); err != nil {
			logger.Errorf("NMSPMExportTask: failed to write header: %v", err)
			return
		}
	}

	// 写入数据行
	row := []string{
		timeStr,
		fmt.Sprintf("%.2f", cpuPercent),
		fmt.Sprintf("%.2f", ramPercent),
	}
	if err := writer.Write(row); err != nil {
		logger.Errorf("NMSPMExportTask: failed to write data row: %v", err)
		return
	}

	logger.Debugf("NMSPMExportTask: wrote PM data for tenancy %d: CPU=%.2f%%, RAM=%.2f%%",
		tenantId, cpuPercent, ramPercent)
}