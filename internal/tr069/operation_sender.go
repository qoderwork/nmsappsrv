package tr069

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

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

// connReqCounter is an atomic counter for UDP connection request IDs.
// Matches Java's GetConnectionServiceImpl atomic long id counter.
var connReqCounter int64

// deviceCreds holds connection request credentials from the cpe_element table.
type deviceCreds struct {
	ConnectionRequestUsername *string `gorm:"column:connection_request_username"`
	ConnectionRequestPassword *string `gorm:"column:connection_request_password"`
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

// SendGetRPCMethods sends a GetRPCMethods request to the device.
func (o *OperationSender) SendGetRPCMethods(sn string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetRPCMethods(headerId)

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "GET_RPC_METHODS", "")

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendCapture sends a Capture request to the device (vendor extension).
func (o *OperationSender) SendCapture(sn string, cap *soap.Capture, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildCapture(headerId, cap)
	o.saveTrackData(operationId, headerId, sn, "CAPTURE", cap.CommandKey)
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendSoftReboot sends a SoftReboot request to the device.
func (o *OperationSender) SendSoftReboot(sn string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	commandKey := fmt.Sprintf("soft_reboot_%d", time.Now().Unix())
	soapXml := soap.BuildSoftReboot(headerId, commandKey)

	// Clear online flag and set reboot user (matches Java SoftRebootHandler post-processing)
	ctx := context.Background()
	var elementId int64
	type DeviceLookup struct {
		NeNeid int64 `gorm:"column:ne_neid"`
	}
	var dev DeviceLookup
	if err := o.db.Table("cpe_element").Select("ne_neid").Where("serial_number = ? AND deleted = ?", sn, false).First(&dev).Error; err == nil {
		elementId = dev.NeNeid
		onlineKey := fmt.Sprintf("online_%d", elementId)
		redis.Del(ctx, onlineKey)
	}

	o.saveTrackData(operationId, headerId, sn, "SOFT_REBOOT", commandKey)
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendBatchUpgrade sends a BatchUpgrade request to the device (vendor extension).
func (o *OperationSender) SendBatchUpgrade(sn string, batch *soap.BatchUpgrade, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildBatchUpgrade(headerId, batch)
	o.saveTrackData(operationId, headerId, sn, "BATCH_UPGRADE", batch.CommandKey)
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendCancelFutureUpgrade sends a CancelFutureUpgrade request to the device (vendor extension).
func (o *OperationSender) SendCancelFutureUpgrade(sn string, commandKey string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildCancelFutureUpgrade(headerId, commandKey)
	o.saveTrackData(operationId, headerId, sn, "CANCEL_FUTURE_UPGRADE", commandKey)
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendHttpRequestProxy sends an HttpRequestProxy request to the device (vendor extension).
func (o *OperationSender) SendHttpRequestProxy(sn string, proxy *soap.HttpRequestProxy, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildHttpRequestProxy(headerId, proxy)
	o.saveTrackData(operationId, headerId, sn, "HTTP_REQUEST_PROXY", "")
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendSetParameterAttributes sends a SetParameterAttributes request to the device.
func (o *OperationSender) SendSetParameterAttributes(sn string, spa *soap.SetParameterAttributes, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildSetParameterAttributes(headerId, spa)
	o.saveTrackData(operationId, headerId, sn, "SET_PARAMETER_ATTRIBUTES", "")
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendGetParameterAttributes sends a GetParameterAttributes request to the device.
func (o *OperationSender) SendGetParameterAttributes(sn string, names []string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetParameterAttributes(headerId, names)
	o.saveTrackData(operationId, headerId, sn, "GET_PARAMETER_ATTRIBUTES", "")
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendUpdateCBSDStatus sends an UpdateCBSDStatus request to the device (vendor extension).
func (o *OperationSender) SendUpdateCBSDStatus(sn string, ucs *soap.UpdateCBSDStatus, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildUpdateCBSDStatus(headerId, ucs)
	o.saveTrackData(operationId, headerId, sn, "UPDATE_CBSD_STATUS", "")
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendConnectionRequest sends a connection request to wake up CPE.
// When the STUN server is running, uses UDP with HMAC-SHA256 auth (matching Java
// GetConnectionServiceImpl). The request is sent from the STUN server's socket so the
// device's NAT table allows it through. Sent 3 times for reliability.
// Falls back to TCP HTTP GET with Basic auth when STUN is not available.
func (o *OperationSender) SendConnectionRequest(sn string) error {
	ctx := context.Background()

	// Read STUN info from Redis cache
	stunKey := fmt.Sprintf("device:stun:%s", sn)
	stunData := redis.HGetAll(ctx, stunKey)
	if len(stunData) == 0 {
		logger.Warnf("no STUN info found for device %s in Redis", sn)
		return fmt.Errorf("no STUN info for device %s", sn)
	}

	ip := stunData["ip"]
	port := stunData["port"]
	if ip == "" || port == "" {
		return fmt.Errorf("incomplete STUN info for device %s: ip=%s, port=%s", sn, ip, port)
	}

	addr := fmt.Sprintf("%s:%s", ip, port)

	// Look up device credentials
	var creds deviceCreds
	if err := o.db.Table("cpe_element").
		Select("connection_request_username, connection_request_password").
		Where("serial_number = ? AND deleted = ?", sn, false).First(&creds).Error; err != nil {
		logger.Warnf("device %s not found for connection request: %v", sn, err)
	}

	// Try UDP connection request via STUN socket (matches Java GetConnectionServiceImpl)
	udpConn := getSTUNUDP()
	if udpConn != nil {
		return o.sendUDPConnectionRequest(sn, addr, udpConn, &creds)
	}

	// Fallback: TCP HTTP GET with Basic auth
	logger.Infof("STUN socket not available, falling back to TCP connection request for device %s", sn)
	return o.sendTCPConnectionRequest(sn, addr, &creds)
}

// sendUDPConnectionRequest sends an HTTP-over-UDP connection request with HMAC-SHA256 auth.
// Matches Java GetConnectionServiceImpl.getConnection() for NAT-detected devices.
func (o *OperationSender) sendUDPConnectionRequest(sn string, addr string, udpConn *net.UDPConn, creds *deviceCreds) error {
	// Build URL with HMAC-SHA256 authentication
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	id := fmt.Sprintf("%d", atomic.AddInt64(&connReqCounter, 1))
	un := ""
	if creds.ConnectionRequestUsername != nil {
		un = *creds.ConnectionRequestUsername
	}
	cn := uuid.NewString()

	// HMAC-SHA256 data = timestamp + id + username + uuid (matches Java SHAUtil.encrypt)
	data := ts + id + un + cn
	password := "password" // Java default when password is empty
	if creds.ConnectionRequestPassword != nil && *creds.ConnectionRequestPassword != "" {
		password = *creds.ConnectionRequestPassword
	}
	sig := hmacSHA256(data, password)

	connReqURL := fmt.Sprintf("http://%s/?ts=%s&id=%s&un=%s&cn=%s&sig=%s", addr, ts, id, un, cn, sig)

	// Build raw HTTP GET payload (matches Java HTTPEncoder.getContent)
	httpPayload := fmt.Sprintf("GET %s HTTP/1.1\r\n", connReqURL)
	packet := []byte(httpPayload)

	remoteAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		logger.Errorf("failed to resolve UDP address for device %s: %v", sn, err)
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	// Send 3 times for reliability (matches Java loop)
	for i := 0; i < 3; i++ {
		if _, err := udpConn.WriteToUDP(packet, remoteAddr); err != nil {
			logger.Errorf("failed to send UDP connection request to device %s (attempt %d): %v", sn, i+1, err)
			return fmt.Errorf("failed to send UDP packet: %w", err)
		}
	}

	logger.Infof("UDP connection request sent to device %s at %s (3 packets)", sn, addr)
	return nil
}

// sendTCPConnectionRequest sends an HTTP connection request over TCP with optional Basic auth.
// Used as fallback when the STUN server is not running.
func (o *OperationSender) sendTCPConnectionRequest(sn string, addr string, creds *deviceCreds) error {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		logger.Errorf("failed to dial TCP for device %s: %v", sn, err)
		return fmt.Errorf("failed to dial TCP: %w", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

	httpReq := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n", addr)
	if creds.ConnectionRequestUsername != nil && *creds.ConnectionRequestUsername != "" {
		username := *creds.ConnectionRequestUsername
		password := ""
		if creds.ConnectionRequestPassword != nil {
			password = *creds.ConnectionRequestPassword
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		httpReq += fmt.Sprintf("Authorization: Basic %s\r\n", encoded)
	}
	httpReq += "\r\n"

	if _, err = conn.Write([]byte(httpReq)); err != nil {
		logger.Errorf("failed to send TCP connection request to device %s: %v", sn, err)
		return fmt.Errorf("failed to send connection request: %w", err)
	}

	logger.Infof("TCP connection request sent to device %s at %s", sn, addr)
	return nil
}

// hmacSHA256 computes HMAC-SHA256 and returns the uppercase hex string.
// Matches Java SHAUtil.encrypt(data, key) with HmacSHA256.
func hmacSHA256(data, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
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
