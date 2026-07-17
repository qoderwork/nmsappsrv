package scheduledtask

import (
	"context"
	"encoding/json"
	"fmt"

	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// StunConfigDetectionTask STUN配置检测
// 镜像 Java StunConfigDetectionTask
// 每小时执行，对比在线设备的 STUN 参数与系统配置，
// 有差异时通过 SPV 修正
type StunConfigDetectionTask struct {
	db       *gorm.DB
	opSender *tr069.OperationSender
}

// stunConfig 对应 system_config 表中 stun_config 的结构
type stunConfig struct {
	AcsURL           *string `json:"acsUrl"`
	StunServerAddr   *string `json:"stunServerAddress"`
	StunServerPort   *string `json:"stunServerPort"`
	StunUsername     *string `json:"stunUsername"`
}

// STUN 相关的 TR-069 参数路径
const (
	stunParamConnectionRequestURL = "Device.ManagementServer.ConnectionRequestURL"
	stunParamSTUNServerAddress    = "Device.ManagementServer.STUNServerAddress"
	stunParamSTUNServerPort       = "Device.ManagementServer.STUNServerPort"
	stunParamSTUNUsername         = "Device.ManagementServer.STUNUsername"
)

// NewStunConfigDetectionTask 创建 StunConfigDetectionTask 实例
func NewStunConfigDetectionTask(db *gorm.DB, opSender *tr069.OperationSender) *StunConfigDetectionTask {
	return &StunConfigDetectionTask{
		db:       db,
		opSender: opSender,
	}
}

// DetectAndFix 执行 STUN 配置检测和修正
// 1. 从 system_config 读取 STUN 配置
// 2. 查询所有在线设备
// 3. 对比设备的 STUN 参数与系统配置值
// 4. 有差异时通过 SPV 修正
func (t *StunConfigDetectionTask) DetectAndFix() {
	ctx := context.Background()

	// 1. 读取 STUN 配置
	config := t.loadStunConfig()
	if config == nil {
		logger.Warnf("StunConfigDetectionTask: no stun_config found, skipping")
		return
	}

	// 2. 查询所有在线设备
	type cpeElementRow struct {
		NeNeid       int64  `gorm:"column:ne_neid"`
		SerialNumber string `gorm:"column:serial_number"`
	}
	var elements []cpeElementRow
	if err := t.db.Table("cpe_element").
		Select("ne_neid, serial_number").
		Where("deleted = ? AND serial_number IS NOT NULL AND serial_number != ''", false).
		Find(&elements).Error; err != nil {
		logger.Errorf("StunConfigDetectionTask: query cpe_element failed: %v", err)
		return
	}

	if len(elements) == 0 {
		return
	}

	fixCount := 0

	for _, elem := range elements {
		elementId := elem.NeNeid
		sn := elem.SerialNumber

		// 检查在线状态
		onlineKey := fmt.Sprintf("online_%d", elementId)
		onlineVal, err := redis.Get(ctx, onlineKey)
		if err != nil || onlineVal != "yes" {
			continue
		}

		// 3. 对比 STUN 参数，收集有差异的参数
		diffParams := t.compareStunParams(elementId, config)
		if len(diffParams) == 0 {
			continue
		}

		// 4. 通过 SPV 修正
		operationId := fmt.Sprintf("stun_config_fix_%d", elementId)
		if err := t.opSender.SendSetParameterValues(sn, diffParams, "", operationId); err != nil {
			logger.Errorf("StunConfigDetectionTask: failed to send STUN SPV to %s (element %d): %v", sn, elementId, err)
			continue
		}

		fixCount++
		logger.Infof("StunConfigDetectionTask: fixed STUN config for %s (element %d), %d params updated", sn, elementId, len(diffParams))
	}

	if fixCount > 0 {
		logger.Infof("StunConfigDetectionTask: fixed STUN config for %d devices", fixCount)
	}
}

// loadStunConfig 从 system_config 表读取 STUN 配置
func (t *StunConfigDetectionTask) loadStunConfig() *stunConfig {
	var configStr *string
	if err := t.db.Table("system_config").
		Select("config").
		Where("id = ?", "stun_config").
		Scan(&configStr).Error; err != nil {
		logger.Errorf("StunConfigDetectionTask: failed to load stun_config: %v", err)
		return nil
	}

	if configStr == nil || *configStr == "" {
		return nil
	}

	var cfg stunConfig
	if err := json.Unmarshal([]byte(*configStr), &cfg); err != nil {
		logger.Errorf("StunConfigDetectionTask: failed to unmarshal stun_config: %v", err)
		return nil
	}

	return &cfg
}

// compareStunParams 比较设备的 STUN 参数与系统配置，返回有差异的参数列表
func (t *StunConfigDetectionTask) compareStunParams(elementId int64, config *stunConfig) []soap.ParameterValueStruct {
	var diffParams []soap.ParameterValueStruct

	// 对比 ConnectionRequestURL（ACS URL）
	if config.AcsURL != nil && *config.AcsURL != "" {
		currentVal := t.getParamValue(elementId, stunParamConnectionRequestURL)
		if currentVal != *config.AcsURL {
			diffParams = append(diffParams, soap.ParameterValueStruct{
				Name:  stunParamConnectionRequestURL,
				Value: *config.AcsURL,
				Type:  "xsd:string",
			})
		}
	}

	// 对比 STUNServerAddress
	if config.StunServerAddr != nil && *config.StunServerAddr != "" {
		currentVal := t.getParamValue(elementId, stunParamSTUNServerAddress)
		if currentVal != *config.StunServerAddr {
			diffParams = append(diffParams, soap.ParameterValueStruct{
				Name:  stunParamSTUNServerAddress,
				Value: *config.StunServerAddr,
				Type:  "xsd:string",
			})
		}
	}

	// 对比 STUNServerPort
	if config.StunServerPort != nil && *config.StunServerPort != "" {
		currentVal := t.getParamValue(elementId, stunParamSTUNServerPort)
		if currentVal != *config.StunServerPort {
			diffParams = append(diffParams, soap.ParameterValueStruct{
				Name:  stunParamSTUNServerPort,
				Value: *config.StunServerPort,
				Type:  "xsd:string",
			})
		}
	}

	// 对比 STUNUsername
	if config.StunUsername != nil && *config.StunUsername != "" {
		currentVal := t.getParamValue(elementId, stunParamSTUNUsername)
		if currentVal != *config.StunUsername {
			diffParams = append(diffParams, soap.ParameterValueStruct{
				Name:  stunParamSTUNUsername,
				Value: *config.StunUsername,
				Type:  "xsd:string",
			})
		}
	}

	return diffParams
}

// getParamValue 从 element_basic_info_parameter 表获取设备的参数当前值
func (t *StunConfigDetectionTask) getParamValue(elementId int64, paramName string) string {
	row := struct {
		ParamValue *string `gorm:"column:param_value"`
	}{}
	if err := t.db.Table("element_basic_info_parameter").
		Select("param_value").
		Where("element_id = ? AND param_name = ?", elementId, paramName).
		Scan(&row).Error; err != nil {
		return ""
	}
	if row.ParamValue != nil {
		return *row.ParamValue
	}
	return ""
}
