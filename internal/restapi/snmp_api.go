package restapi

import (
	"encoding/json"
	"time"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/internal/snmp"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"github.com/gin-gonic/gin"
)

// ============================
// SNMP Operations (Task 6.4)
// ============================

// SnmpGet queues an SNMP GET operation to the Redis SNMP queue
func (s *service) SnmpGet(c *gin.Context, req *SnmpGetRequest) error {
	tenantId := middleware.GetTenantId(c)
	username := middleware.GetUsername(c)

	// Verify device exists and get connection info
	dev, err := s.repo.GetDeviceById(req.ElementId, tenantId)
	if err != nil {
		return apperror.ErrDeviceNotFound
	}

	if dev.DeviceIp == nil || *dev.DeviceIp == "" {
		return apperror.ErrInvalidInput.WithMessage("device has no IP address configured")
	}

	// Build SNMP parameters from OIDs
	var payload []snmp.SnmpParameter
	for _, oid := range req.OIDs {
		payload = append(payload, snmp.SnmpParameter{
			OID:  oid,
			Type: "string",
		})
	}

	// Build SNMP message
	msg := snmp.SnmpMessage{
		OperationType: snmp.OperationGet,
		ConnectionInfo: snmp.SnmpConnectionInfo{
			IP:        *dev.DeviceIp,
			Port:      161,
			Version:   2,
			Community: "public",
		},
		Payload: payload,
	}

	// Marshal and push to Redis SNMP queue
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return apperror.Wrap(err, "MARSHAL_SNMP_MESSAGE_FAILED", 500, "failed to marshal SNMP message")
	}

	ctx := c.Request.Context()
	if err := redis.LPush(ctx, snmp.SnmpQueueName, string(msgJSON)); err != nil {
		logger.Errorf("Failed to push SNMP GET to queue: %v", err)
		return apperror.Wrap(err, "QUEUE_SNMP_GET_FAILED", 500, "failed to queue SNMP GET operation")
	}

	logger.Infof("SNMP GET queued for device %d (%d OIDs) by user %s", req.ElementId, len(req.OIDs), username)
	return nil
}

// SnmpSet queues an SNMP SET operation to the Redis SNMP queue
func (s *service) SnmpSet(c *gin.Context, req *SnmpSetRequest) error {
	tenantId := middleware.GetTenantId(c)
	username := middleware.GetUsername(c)

	// Verify device exists and get connection info
	dev, err := s.repo.GetDeviceById(req.ElementId, tenantId)
	if err != nil {
		return apperror.ErrDeviceNotFound
	}

	if dev.DeviceIp == nil || *dev.DeviceIp == "" {
		return apperror.ErrInvalidInput.WithMessage("device has no IP address configured")
	}

	// Build SNMP parameters
	var payload []snmp.SnmpParameter
	for _, p := range req.Parameters {
		payload = append(payload, snmp.SnmpParameter{
			OID:   p.OID,
			Type:  p.Type,
			Value: p.Value,
		})
	}

	// Build SNMP message
	msg := snmp.SnmpMessage{
		OperationType: snmp.OperationSet,
		ConnectionInfo: snmp.SnmpConnectionInfo{
			IP:        *dev.DeviceIp,
			Port:      161,
			Version:   2,
			Community: "public",
		},
		Payload: payload,
	}

	// Marshal and push to Redis SNMP queue
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return apperror.Wrap(err, "MARSHAL_SNMP_MESSAGE_FAILED", 500, "failed to marshal SNMP message")
	}

	ctx := c.Request.Context()
	if err := redis.LPush(ctx, snmp.SnmpQueueName, string(msgJSON)); err != nil {
		logger.Errorf("Failed to push SNMP SET to queue: %v", err)
		return apperror.Wrap(err, "QUEUE_SNMP_SET_FAILED", 500, "failed to queue SNMP SET operation")
	}

	logger.Infof("SNMP SET queued for device %d (%d params) by user %s", req.ElementId, len(req.Parameters), username)
	return nil
}

// ListSnmpOperationLogs returns SNMP operation logs with pagination
func (s *service) ListSnmpOperationLogs(c *gin.Context, offset, limit int) ([]SnmpOperationLogVo, int64, error) {
	logs, total, err := s.repo.ListSnmpOperationLogs(offset, limit)
	if err != nil {
		logger.Errorf("Failed to list SNMP operation logs: %v", err)
		return nil, 0, apperror.Wrap(err, "LIST_SNMP_LOGS_FAILED", 500, "failed to list SNMP operation logs")
	}

	var result []SnmpOperationLogVo
	for _, l := range logs {
		vo := SnmpOperationLogVo{}
		if v, ok := l["id"].(int64); ok {
			vo.Id = v
		}
		if v, ok := l["element_id"].(int64); ok {
			vo.ElementId = &v
		}
		if v, ok := l["operation"].(string); ok {
			vo.Operation = v
		}
		if v, ok := l["oid"].(string); ok {
			vo.OID = v
		}
		if v, ok := l["value"].(string); ok {
			vo.Value = v
		}
		if v, ok := l["status"].(string); ok {
			vo.Status = v
		}
		if v, ok := l["error_msg"].(string); ok {
			vo.ErrorMsg = v
		}
		if v, ok := l["operator"].(string); ok {
			vo.Operator = v
		}
		if v, ok := l["operate_time"].(time.Time); ok {
			vo.OperateTime = v.Format("2006-01-02T15:04:05Z07:00")
		}
		result = append(result, vo)
	}

	return result, total, nil
}
