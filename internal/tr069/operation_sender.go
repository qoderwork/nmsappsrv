package tr069

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"sync"
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
	o.saveTrackData(operationId, headerId, sn, "GET_PARAMETER_VALUES", "")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// BatchGPVTarget defines a single device target for batch GPV operations.
type BatchGPVTarget struct {
	SN         string
	ParamNames []string
}

// BatchGetParameterValuesResult holds the result of a batch GPV operation for a single device.
type BatchGetParameterValuesResult struct {
	SN      string
	Success bool
	Error   string
}

// BatchGetParameterValues sends GetParameterValues to multiple devices concurrently.
// maxConcurrency limits the number of simultaneous requests (0 = no limit).
func (o *OperationSender) BatchGetParameterValues(targets []BatchGPVTarget, maxConcurrency int, operationIdPrefix string) []BatchGetParameterValuesResult {
	if len(targets) == 0 {
		return nil
	}

	results := make([]BatchGetParameterValuesResult, len(targets))

	// Semaphore: buffered channel to limit concurrency
	var sem chan struct{}
	if maxConcurrency > 0 {
		sem = make(chan struct{}, maxConcurrency)
	}

	var wg sync.WaitGroup
	wg.Add(len(targets))

	for i, target := range targets {
		// Acquire semaphore slot if concurrency is limited
		if sem != nil {
			sem <- struct{}{}
		}

		go func(idx int, t BatchGPVTarget) {
			defer wg.Done()
			if sem != nil {
				defer func() { <-sem }()
			}

			operationId := fmt.Sprintf("%s_%s", operationIdPrefix, t.SN)
			err := o.SendGetParameterValues(t.SN, t.ParamNames, operationId)

			results[idx] = BatchGetParameterValuesResult{
				SN:      t.SN,
				Success: err == nil,
			}
			if err != nil {
				results[idx].Error = err.Error()
				logger.Errorf("batch GPV failed for device %s: %v", t.SN, err)
			}
		}(i, target)
	}

	wg.Wait()
	return results
}

// SendSetParameterValues sends a SetParameterValues request to the device.
func (o *OperationSender) SendSetParameterValues(sn string, params []soap.ParameterValueStruct, paramKey string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildSetParameterValues(headerId, params, paramKey)

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "SET_PARAMETER_VALUES", "")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendDownload sends a Download request to the device.
func (o *OperationSender) SendDownload(sn string, dl *soap.Download, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildDownload(headerId, dl)

	// Save tracking data with CommandKey for TransferComplete correlation
	o.saveTrackData(operationId, headerId, sn, "DOWNLOAD", dl.CommandKey)

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

	// Save tracking data with CommandKey for TransferComplete correlation
	o.saveTrackData(operationId, headerId, sn, "REBOOT", commandKey)

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendUpload sends an Upload request to the device.
func (o *OperationSender) SendUpload(sn string, upload *soap.Upload, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildUpload(headerId, upload)

	// Save tracking data with CommandKey for TransferComplete correlation
	o.saveTrackData(operationId, headerId, sn, "UPLOAD", upload.CommandKey)

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendFactoryReset sends a FactoryReset request to the device.
func (o *OperationSender) SendFactoryReset(sn string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildFactoryReset(headerId)

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "FACTORY_RESET", "")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendGetParameterNames sends a GetParameterNames request to the device.
func (o *OperationSender) SendGetParameterNames(sn string, paramPath string, nextLevel bool, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetParameterNames(headerId, paramPath, nextLevel)

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "GET_PARAMETER_NAMES", "")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendAddObject sends an AddObject request to the device.
func (o *OperationSender) SendAddObject(sn string, objectName string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildAddObject(headerId, objectName, "")

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "ADD_OBJECT", "")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendDeleteObject sends a DeleteObject request to the device.
func (o *OperationSender) SendDeleteObject(sn string, objectName string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildDeleteObject(headerId, objectName, "")

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "DELETE_OBJECT", "")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendConnectionRequest sends an HTTP connection request to wake up CPE.
// TR-069 connection requests are HTTP over TCP (not UDP).
// If the device has connection_request_username configured, HTTP Basic auth is included.
func (o *OperationSender) SendConnectionRequest(sn string) error {
	ctx := context.Background()

	// Read STUN info from Redis cache
	stunKey := fmt.Sprintf("device:stun:%s", sn)
	stunData := redis.HGetAll(ctx, stunKey)
	if len(stunData) == 0 {
		logger.Warnf("no STUN info found for device %s in Redis", sn)
		return fmt.Errorf("no STUN info for device %s", sn)
	}

	// Build TCP connection request URL
	ip := stunData["ip"]
	port := stunData["port"]

	if ip == "" || port == "" {
		return fmt.Errorf("incomplete STUN info for device %s: ip=%s, port=%s", sn, ip, port)
	}

	addr := fmt.Sprintf("%s:%s", ip, port)
	connReqURL := fmt.Sprintf("http://%s/", addr)

	logger.Infof("sending connection request to device %s at %s", sn, connReqURL)

	// Look up device credentials for HTTP Basic auth
	authHeader := ""
	type deviceCreds struct {
		ConnectionRequestUsername *string `gorm:"column:connection_request_username"`
		ConnectionRequestPassword *string `gorm:"column:connection_request_password"`
	}
	var creds deviceCreds
	if err := o.db.Table("cpe_element").
		Select("connection_request_username, connection_request_password").
		Where("serial_number = ? AND deleted = ?", sn, false).First(&creds).Error; err == nil {
		if creds.ConnectionRequestUsername != nil && *creds.ConnectionRequestUsername != "" {
			username := *creds.ConnectionRequestUsername
			password := ""
			if creds.ConnectionRequestPassword != nil {
				password = *creds.ConnectionRequestPassword
			}
			encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
			authHeader = "Basic " + encoded
		}
	}

	// Send HTTP GET via TCP with timeout
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		logger.Errorf("failed to dial TCP for device %s: %v", sn, err)
		return fmt.Errorf("failed to dial TCP: %w", err)
	}
	defer conn.Close()

	// Set read/write deadline
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Build HTTP GET request with optional auth header
	httpReq := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n", addr)
	if authHeader != "" {
		httpReq += fmt.Sprintf("Authorization: %s\r\n", authHeader)
	}
	httpReq += "\r\n"

	_, err = conn.Write([]byte(httpReq))
	if err != nil {
		logger.Errorf("failed to send connection request to device %s: %v", sn, err)
		return fmt.Errorf("failed to send connection request: %w", err)
	}

	logger.Infof("connection request sent to device %s", sn)
	return nil
}

// saveTrackData saves command tracking data to the event_log table.
// commandKey is the TR-069 CommandKey used to correlate TransferComplete back to the originating operation.
func (o *OperationSender) saveTrackData(operationId string, headerId string, sn string, operationType string, commandKey string) {
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

	// Marshal tracking data to JSON (includes command_key for TransferComplete correlation)
	trackData := map[string]interface{}{
		"operation_id":   operationId,
		"header_id":      headerId,
		"serial_number":  sn,
		"operation_type": operationType,
		"issue_time":     now.Format(time.RFC3339),
	}
	if commandKey != "" {
		trackData["command_key"] = commandKey
	}

	if jsonData, err := json.Marshal(trackData); err == nil {
		eventLog.CommandTrackData = stringPtr(string(jsonData))
	}

	if err := o.db.Create(&eventLog).Error; err != nil {
		logger.Errorf("failed to save track data for operation %s: %v", operationId, err)
	}

	// Also store in Redis for quick lookup during response processing
	trackKey := fmt.Sprintf("tr069:track:%s", headerId)
	redisTrackData := map[string]interface{}{
		"operation_id":   operationId,
		"header_id":      headerId,
		"sn":             sn,
		"operation_type": operationType,
		"event_log_id":   eventLog.Id,
	}
	if commandKey != "" {
		redisTrackData["command_key"] = commandKey
	}
	if trackJson, err := json.Marshal(redisTrackData); err == nil {
		if err := redis.Set(ctx, trackKey, string(trackJson), 24*time.Hour); err != nil {
			logger.Warnf("failed to cache track data in Redis: %v", err)
		}
	}
}
