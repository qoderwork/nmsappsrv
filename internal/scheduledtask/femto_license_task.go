package scheduledtask

import (
	"context"
	"fmt"
	"time"

	"nmsappsrv/internal/license"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// FemtoLicenseTask 对在线设备下发 License 文件
// 镜像 Java FemtoLicenseTask.licenseCheck()
type FemtoLicenseTask struct {
	db              *gorm.DB
	opSender        *tr069.OperationSender
	licenseFilePath string
}

// supportedModels 是支持 License 下发的设备型号列表（Java StartNotifyConsumer.models）
var supportedModels = map[string]bool{
	"EU":      true,
	"RU":      true,
	"CPE-DK":  true,
}

// NewFemtoLicenseTask 创建 FemtoLicenseTask 实例
func NewFemtoLicenseTask(db *gorm.DB, opSender *tr069.OperationSender, licenseFilePath string) *FemtoLicenseTask {
	return &FemtoLicenseTask{
		db:              db,
		opSender:        opSender,
		licenseFilePath: licenseFilePath,
	}
}

// CheckLicense 执行 License 检查并下发
// 1. 查询所有未删除的 cpe_element
// 2. 对在线、不在黑名单、model在支持列表中的设备
// 3. 查 base_station_license 表（by_element_id）
// 4. 查 ztp_log 表确认 progress==6
// 5. 查 element_basic_info_parameter 获取 Device.DeviceInfo.License.Status
// 6. 如果 License.Status == "0" 或为空，则通过 SendDownload 下发 License 文件
func (t *FemtoLicenseTask) CheckLicense() {
	ctx := context.Background()

	// 1. 查询所有未删除的 cpe_element
	type cpeElementRow struct {
		NeNeid       int64  `gorm:"column:ne_neid"`
		SerialNumber string `gorm:"column:serial_number"`
		ModelName    string `gorm:"column:model_name"`
	}
	var elements []cpeElementRow
	if err := t.db.Table("cpe_element").
		Select("ne_neid, serial_number, model_name").
		Where("deleted = ? AND serial_number IS NOT NULL AND serial_number != ''", false).
		Find(&elements).Error; err != nil {
		logger.Errorf("FemtoLicenseTask: query cpe_element failed: %v", err)
		return
	}

	for _, elem := range elements {
		sn := elem.SerialNumber
		elementId := elem.NeNeid

		// 2a. 检查在线状态：Redis key "online_{sn}" == "yes"
		// Java 使用 online_{neId}，但 Redis key 模式与 Java 一致，使用 elementId
		onlineKey := fmt.Sprintf("online_%d", elementId)
		onlineVal, err := redis.Get(ctx, onlineKey)
		if err != nil || onlineVal != "yes" {
			continue
		}

		// 2b. 检查黑名单
		var blackListCount int64
		t.db.Table("element_black_list").
			Where("sn = ?", sn).
			Count(&blackListCount)
		if blackListCount > 0 {
			continue
		}

		// 2c. 检查 model 是否在支持列表中
		modelName := elem.ModelName
		if !supportedModels[modelName] {
			continue
		}

		// 3. 查 base_station_license 表（by_element_id）
		var bsLicense license.BaseStationLicense
		if err := t.db.Where("element_id = ?", elementId).First(&bsLicense).Error; err != nil {
			// 没有找到 license 记录，跳过
			logger.Debugf("FemtoLicenseTask: no base_station_license for element %d: %v", elementId, err)
			continue
		}

		// 4. 查 ztp_log 表确认 progress==6（ZTP 完成）
		var ztpLog misc.ZTPLog
		if err := t.db.Where("element_id = ?", elementId).First(&ztpLog).Error; err != nil {
			// 没有 ZTP 记录，跳过
			continue
		}
		if ztpLog.Progress == nil || *ztpLog.Progress != 6 {
			continue
		}

		// 5. 查 element_basic_info_parameter 获取 Device.DeviceInfo.License.Status
		const licenseStatusParam = "Device.DeviceInfo.License.Status"
		var param struct {
			ParamValue *string `gorm:"column:param_value"`
		}
		if err := t.db.Table("element_basic_info_parameter").
			Select("param_value").
			Where("element_id = ? AND param_name = ?", elementId, licenseStatusParam).
			Scan(&param).Error; err != nil {
			logger.Warnf("FemtoLicenseTask: query License.Status for element %d failed: %v", elementId, err)
			continue
		}

		// 6. 如果 License.Status == "0" 或为空，则下发 License 文件
		licenseStatus := ""
		if param.ParamValue != nil {
			licenseStatus = *param.ParamValue
		}
		if licenseStatus != "0" && licenseStatus != "" {
			continue
		}

		// 构造 License 文件名
		fileName := ""
		if bsLicense.FileName != nil {
			fileName = *bsLicense.FileName
		}
		if fileName == "" {
			logger.Warnf("FemtoLicenseTask: base_station_license for element %d has no file_name", elementId)
			continue
		}

		// 构造下载 URL
		downloadURL := fmt.Sprintf("%s/%s", t.licenseFilePath, fileName)

		// 构建 soap.Download 对象
		dl := &soap.Download{
			CommandKey:     fmt.Sprintf("license_%d_%d", elementId, time.Now().Unix()),
			FileType:       "1 Vendor Configuration File",
			URL:            downloadURL,
			TargetFileName: fileName,
		}

		// 生成 operationId
		operationId := fmt.Sprintf("license_download_%d", elementId)

		// 下发 License 文件
		if err := t.opSender.SendDownload(sn, dl, operationId); err != nil {
			logger.Errorf("FemtoLicenseTask: failed to send license download to %s (element %d): %v", sn, elementId, err)
			continue
		}

		logger.Infof("FemtoLicenseTask: license download sent to %s (element %d, file=%s)", sn, elementId, fileName)
	}
}
