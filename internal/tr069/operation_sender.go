package tr069

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"nmsappsrv/internal/eventlog"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// OperationSender builds SOAP XML from API commands and pushes to Redis per-device queues.
type OperationSender struct {
	db         *gorm.DB
	msgManager *MessageManager
}

// NewOperationSender creates a new OperationSender.
func NewOperationSender(db *gorm.DB, msgManager *MessageManager) *OperationSender {
	return &OperationSender{
		db:         db,
		msgManager: msgManager,
	}
}

// SendGetParameterValues sends a GetParameterValues request to the device.
func (o *OperationSender) SendGetParameterValues(sn string, paramNames []string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetParameterValues(headerId, paramNames)

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "GET_PARAMETER_VALUES")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendSetParameterValues sends a SetParameterValues request to the device.
func (o *OperationSender) SendSetParameterValues(sn string, params []soap.ParameterValueStruct, paramKey string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildSetParameterValues(headerId, params, paramKey)

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "SET_PARAMETER_VALUES")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendDownload sends a Download request to the device.
func (o *OperationSender) SendDownload(sn string, dl *soap.Download, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildDownload(headerId, dl)

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "DOWNLOAD")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendReboot sends a Reboot request to the device.
func (o *OperationSender) SendReboot(sn string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	commandKey := fmt.Sprintf("reboot_%d", time.Now().Unix())
	soapXml := soap.BuildReboot(headerId, commandKey)

	// Set Redis rebooting flag
	ctx := context.Background()
	rebootKey := fmt.Sprintf("device:rebooting:%s", sn)
	if err := redis.Set(ctx, rebootKey, "1", 10*time.Minute); err != nil {
		logger.Warnf("failed to set rebooting flag for %s: %v", sn, err)
	}

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "REBOOT")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendUpload sends an Upload request to the device.
func (o *OperationSender) SendUpload(sn string, upload *soap.Upload, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildUpload(headerId, upload)

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "UPLOAD")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendFactoryReset sends a FactoryReset request to the device.
func (o *OperationSender) SendFactoryReset(sn string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildFactoryReset(headerId)

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "FACTORY_RESET")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendGetParameterNames sends a GetParameterNames request to the device.
func (o *OperationSender) SendGetParameterNames(sn string, paramPath string, nextLevel bool, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetParameterNames(headerId, paramPath, nextLevel)

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "GET_PARAMETER_NAMES")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendAddObject sends an AddObject request to the device.
func (o *OperationSender) SendAddObject(sn string, objectName string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildAddObject(headerId, objectName, "")

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "ADD_OBJECT")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendDeleteObject sends a DeleteObject request to the device.
func (o *OperationSender) SendDeleteObject(sn string, objectName string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildDeleteObject(headerId, objectName, "")

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "DELETE_OBJECT")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendConnectionRequest sends a UDP connection request to wake up CPE.
func (o *OperationSender) SendConnectionRequest(sn string) error {
	ctx := context.Background()

	// Read STUN info from Redis cache
	stunKey := fmt.Sprintf("device:stun:%s", sn)
	stunData := redis.HGetAll(ctx, stunKey)
	if len(stunData) == 0 {
		logger.Warnf("no STUN info found for device %s in Redis", sn)
		return fmt.Errorf("no STUN info for device %s", sn)
	}

	// Build UDP connection request URL
	// Expected STUN data format: {"ip": "...", "port": "..."}
	ip := stunData["ip"]
	port := stunData["port"]

	if ip == "" || port == "" {
		return fmt.Errorf("incomplete STUN info for device %s: ip=%s, port=%s", sn, ip, port)
	}

	addr := fmt.Sprintf("%s:%s", ip, port)
	connReqURL := fmt.Sprintf("http://%s/", addr)

	logger.Infof("sending connection request to device %s at %s", sn, connReqURL)

	// Send raw HTTP GET via UDP DatagramPacket
	// Note: TR-069 connection requests are typically HTTP over TCP, not UDP
	// But if the requirement specifically says UDP, we use UDP here
	conn, err := net.Dial("udp", addr)
	if err != nil {
		logger.Errorf("failed to dial UDP for device %s: %v", sn, err)
		return fmt.Errorf("failed to dial UDP: %w", err)
	}
	defer conn.Close()

	// Build HTTP GET request
	httpReq := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", addr)

	_, err = conn.Write([]byte(httpReq))
	if err != nil {
		logger.Errorf("failed to send connection request to device %s: %v", sn, err)
		return fmt.Errorf("failed to send connection request: %w", err)
	}

	logger.Infof("connection request sent to device %s", sn)
	return nil
}

// saveTrackData saves command tracking data to the event_log table.
func (o *OperationSender) saveTrackData(operationId string, headerId string, sn string, operationType string) {
	ctx := context.Background()
	now := time.Now()

	// Find device by serial number to get element_id
	var elementId int64
	type DeviceLookup struct {
		NeNeid int64 `gorm:"column:ne_neid"`
	}
	var device DeviceLookup
	err := o.db.Table("cpe_element").Select("ne_neid").Where("serial_number = ? AND deleted = ?", sn, false).First(&device).Error
	if err == nil {
		elementId = device.NeNeid
	} else {
		logger.Warnf("device %s not found when saving track data: %v", sn, err)
	}

	// Create event log entry
	eventLog := eventlog.EventLog{
		EventType:        stringPtr(operationType),
		OperationTime:    &now,
		CommandIssueTime: &now,
		ElementId:        &elementId,
		Status:           intPtr(1), // pending
	}

	// Marshal tracking data to JSON
	trackData := map[string]interface{}{
		"operation_id":   operationId,
		"header_id":      headerId,
		"serial_number":  sn,
		"operation_type": operationType,
		"issue_time":     now.Format(time.RFC3339),
	}

	if jsonData, err := json.Marshal(trackData); err == nil {
		eventLog.CommandTrackData = stringPtr(string(jsonData))
	}

	if err := o.db.Create(&eventLog).Error; err != nil {
		logger.Errorf("failed to save track data for operation %s: %v", operationId, err)
	}

	// Also store in Redis for quick lookup during response processing
	trackKey := fmt.Sprintf("tr069:track:%s", headerId)
	if trackJson, err := json.Marshal(map[string]interface{}{
		"operation_id":   operationId,
		"header_id":      headerId,
		"sn":             sn,
		"operation_type": operationType,
		"event_log_id":   eventLog.Id,
	}); err == nil {
		if err := redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour); err != nil {
			logger.Warnf("failed to cache track data in Redis: %v", err)
		}
	}
}
