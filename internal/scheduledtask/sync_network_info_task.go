package scheduledtask

import (
	"context"
	"fmt"

	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// SyncNetworkInfoTask 定时同步核心网设备信息
// 镜像 Java SyncNetworkInfoTask.syncInfo()
type SyncNetworkInfoTask struct {
	db       *gorm.DB
	opSender *tr069.OperationSender
}

// NewSyncNetworkInfoTask 创建 SyncNetworkInfoTask 实例
func NewSyncNetworkInfoTask(db *gorm.DB, opSender *tr069.OperationSender) *SyncNetworkInfoTask {
	return &SyncNetworkInfoTask{
		db:       db,
		opSender: opSender,
	}
}

// SyncInfo 执行核心网信息同步
// 1. 查询所有未删除的 core_network
// 2. 构建一组 HTTP 请求（elementInfo、elementConfig、ueInfoAndUeNumber、elementAlarm、imsi）
// 3. 对在线的核心网设备，通过 HttpRequestProxy 下发同步请求
func (t *SyncNetworkInfoTask) SyncInfo() {
	ctx := context.Background()

	// 1. 查询所有未删除的 core_network
	type coreNetworkRow struct {
		Id        int   `gorm:"primaryKey;column:id"`
		ElementId int64 `gorm:"column:element_id"`
	}
	var coreNetworks []coreNetworkRow
	if err := t.db.Table("core_network").
		Where("deleted = ? OR deleted IS NULL", false).
		Find(&coreNetworks).Error; err != nil {
		logger.Errorf("SyncNetworkInfoTask: query core_network failed: %v", err)
		return
	}

	if len(coreNetworks) == 0 {
		return
	}

	// 2. 构建一组 HTTP 请求（与 Java SyncNetworkInfoTask 一致）
	requests := buildSyncRequests()

	// 3. 对在线的核心网设备，通过 CPE HttpRequestProxy 下发
	for _, cn := range coreNetworks {
		if cn.ElementId == 0 {
			continue
		}

		// 检查在线状态
		onlineKey := fmt.Sprintf("online_%d", cn.ElementId)
		onlineVal, err := redis.Get(ctx, onlineKey)
		if err != nil || onlineVal != "yes" {
			continue
		}

		// 查 CPE 设备 SN
		var sn string
		if err := t.db.Table("cpe_element").
			Select("serial_number").
			Where("ne_neid = ? AND deleted = ?", cn.ElementId, false).
			Scan(&sn).Error; err != nil || sn == "" {
			continue
		}

		// 通过 CPE 的 HttpRequestProxy 代理 HTTP 请求
		proxy := &soap.HttpRequestProxy{Requests: requests}
		operationId := fmt.Sprintf("corenet_sync_%d_%d", cn.Id, cn.ElementId)

		if err := t.opSender.SendHttpRequestProxy(sn, proxy, operationId); err != nil {
			logger.Errorf("SyncNetworkInfoTask: failed to send HttpRequestProxy to %s for core network %d: %v", sn, cn.Id, err)
			continue
		}

		logger.Infof("SyncNetworkInfoTask: sync request sent to %s for core network %d (%d requests)", sn, cn.Id, len(requests))
	}
}

// buildSyncRequests 构建核心网信息同步的 HTTP 请求列表
// 镜像 Java SyncNetworkInfoTask 中 elementInfo + elementConfig + ueInfoAndUeNumber + elementAlarm + imsi
func buildSyncRequests() []soap.HttpRequest {
	var requests []soap.HttpRequest

	// elementInfo: 各元素系统状态
	requests = append(requests,
		soap.HttpRequest{URL: "http://127.0.0.110:33030/api/rest/systemManagement/v1/elementType/ims/objectType/systemState", HttpMethod: "GET", RequestId: "query:IMS_INFO"},
		soap.HttpRequest{URL: "http://127.0.0.120:33030/api/rest/systemManagement/v1/elementType/amf/objectType/systemState", HttpMethod: "GET", RequestId: "query:AMF_INFO"},
		soap.HttpRequest{URL: "http://127.0.0.130:33030/api/rest/systemManagement/v1/elementType/ausf/objectType/systemState", HttpMethod: "GET", RequestId: "query:AUSF_INFO"},
		soap.HttpRequest{URL: "http://127.0.0.140:33030/api/rest/systemManagement/v1/elementType/udm/objectType/systemState", HttpMethod: "GET", RequestId: "query:UDM_INFO"},
		soap.HttpRequest{URL: "http://127.0.0.150:33030/api/rest/systemManagement/v1/elementType/smf/objectType/systemState", HttpMethod: "GET", RequestId: "query:SMF_INFO"},
		soap.HttpRequest{URL: "http://127.0.0.160:33030/api/rest/systemManagement/v1/elementType/pcf/objectType/systemState", HttpMethod: "GET", RequestId: "query:PCF_INFO"},
		soap.HttpRequest{URL: "http://127.0.0.190:33030/api/rest/systemManagement/v1/elementType/upf/objectType/systemState", HttpMethod: "GET", RequestId: "query:UPF_INFO"},
	)

	// ueInfoAndUeNumber: UE 信息和数量
	requests = append(requests,
		soap.HttpRequest{URL: "http://127.0.0.150:33030/api/rest/ueManagement/v1/elementType/smf/objectType/ueInfo", HttpMethod: "GET", RequestId: "query:SMF_UE_INFO"},
		soap.HttpRequest{URL: "http://127.0.0.150:33030/api/rest/ueManagement/v1/elementType/smf/objectType/ueNum", HttpMethod: "GET", RequestId: "query:SMF_UE_NUMBER"},
		soap.HttpRequest{URL: "http://127.0.0.110:33030/api/rest/ueManagement/v1/elementType/ims/objectType/ueInfo", HttpMethod: "GET", RequestId: "query:IMS_UE_INFO"},
		soap.HttpRequest{URL: "http://127.0.0.110:33030/api/rest/ueManagement/v1/elementType/ims/objectType/ueNum", HttpMethod: "GET", RequestId: "query:IMS_UE_NUMBER"},
		soap.HttpRequest{URL: "http://127.0.0.160:33030/api/rest/ueManagement/v1/elementType/pcf/objectType/ueInfo", HttpMethod: "GET", RequestId: "query:PCF_UE_INFO"},
	)

	// elementAlarm: SMF 和 UDM 告警
	requests = append(requests,
		soap.HttpRequest{URL: "http://127.0.0.150:33030/api/rest/faultManagement/v1/elementType/smf/objectType/alarms", HttpMethod: "GET", RequestId: "query:SMF_ALARM"},
		soap.HttpRequest{URL: "http://127.0.0.140:33030/api/rest/faultManagement/v1/elementType/udm/objectType/alarms", HttpMethod: "GET", RequestId: "query:UDM_ALARM"},
	)

	// imsi: 获取所有 IMSI
	requests = append(requests,
		soap.HttpRequest{URL: "http://127.0.0.140:8080/ue-manage/v1/get-all-imsi", HttpMethod: "POST", Body: "{}", RequestId: "get_all_imsi"},
	)

	return requests
}
