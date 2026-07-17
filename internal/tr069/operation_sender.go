package tr069

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strings"
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

const defaultPasswordSecret = "waveoss1waveoss1waveoss1password"

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
	o.saveTrackData(operationId, headerId, sn, "GET_PARAMETER_VALUES", "", "", 0)

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
	o.saveTrackData(operationId, headerId, sn, "SET_PARAMETER_VALUES", "", "", 0)

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendDownload sends a Download request to the device.
func (o *OperationSender) SendDownload(sn string, dl *soap.Download, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildDownload(headerId, dl)

	// Save tracking data with CommandKey for TransferComplete correlation.
	// Store file_type so TransferComplete can distinguish download types
	// (e.g. License download triggers auto SOFT_REBOOT in Java).
	o.saveTrackData(operationId, headerId, sn, "DOWNLOAD", dl.CommandKey, "", 0,
		map[string]interface{}{"file_type": dl.FileType})

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendReboot sends a Reboot request to the device.
//
// Mirrors Java `Tr069MessageBuilder.reboot` post-processing:
//   - delete `online_{neId}`
//   - set `rebootUser_{neId} = operationUser` (5 minutes, matches Java
//     `redisService.setKeyAndValue(..., 5, TimeUnit.MINUTES)`)
//   - set `rebooting_{neId} = "yes"` (5 minutes)
//
// `expiredAtMillis` is forwarded to `EventDto.expiredAt` (Java-equivalent
// stale-timeout). Pass 0 to use the default.
func (o *OperationSender) SendReboot(sn string, operationId string, operationUser string, expiredAtMillis int64) error {
	headerId := soap.GenerateHeaderID()
	commandKey := fmt.Sprintf("reboot_%d", time.Now().Unix())
	soapXml := soap.BuildReboot(headerId, commandKey)

	ctx := context.Background()
	var elementId int64
	type DeviceLookup struct {
		NeNeid int64 `gorm:"column:ne_neid"`
	}
	var dev DeviceLookup
	if err := o.db.Table("cpe_element").Select("ne_neid").Where("serial_number = ? AND deleted = ?", sn, false).First(&dev).Error; err == nil {
		elementId = dev.NeNeid

		// Java Tr069MessageBuilder.reboot post-processing (5 min TTL each).
		onlineKey := fmt.Sprintf("online_%d", elementId)
		rebootUserKey := fmt.Sprintf("rebootUser_%d", elementId)
		rebootingKey := fmt.Sprintf("rebooting_%d", elementId)
		redis.Del(ctx, onlineKey)
		redis.Set(ctx, rebootUserKey, operationUser, 5*time.Minute)
		redis.Set(ctx, rebootingKey, "yes", 5*time.Minute)
	}

	// Save tracking data with CommandKey for TransferComplete correlation
	o.saveTrackData(operationId, headerId, sn, "REBOOT", commandKey, operationUser, expiredAtMillis)

	// Push to device queue
	return o.msgManager.PutMessageWithPriority(sn, soapXml, QueueNormal, expiredAtMillis)
}

// SendUpload sends an Upload request to the device.
func (o *OperationSender) SendUpload(sn string, upload *soap.Upload, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildUpload(headerId, upload)

	// Save tracking data with CommandKey for TransferComplete correlation
	o.saveTrackData(operationId, headerId, sn, "UPLOAD", upload.CommandKey, "", 0)

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendFactoryReset sends a FactoryReset request to the device.
func (o *OperationSender) SendFactoryReset(sn string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildFactoryReset(headerId)

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "FACTORY_RESET", "", "", 0)

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendGetParameterNames sends a GetParameterNames request to the device.
func (o *OperationSender) SendGetParameterNames(sn string, paramPath string, nextLevel bool, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetParameterNames(headerId, paramPath, nextLevel)

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "GET_PARAMETER_NAMES", "", "", 0)

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendAddObject sends an AddObject request to the device.
func (o *OperationSender) SendAddObject(sn string, objectName string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildAddObject(headerId, objectName, "")

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "ADD_OBJECT", "", "", 0)

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendDeleteObject sends a DeleteObject request to the device.
func (o *OperationSender) SendDeleteObject(sn string, objectName string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildDeleteObject(headerId, objectName, "")

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "DELETE_OBJECT", "", "", 0)

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendGetRPCMethods sends a GetRPCMethods request to the device.
func (o *OperationSender) SendGetRPCMethods(sn string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetRPCMethods(headerId)

	// Save tracking data
	o.saveTrackData(operationId, headerId, sn, "GET_RPC_METHODS", "", "", 0)

	// Push to device queue
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendCapture sends a Capture request to the device (vendor extension).
func (o *OperationSender) SendCapture(sn string, cap *soap.Capture, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildCapture(headerId, cap)
	o.saveTrackData(operationId, headerId, sn, "CAPTURE", cap.CommandKey, "", 0)
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendSoftReboot sends a SoftReboot request to the device.
//
// Mirrors Java `Tr069MessageBuilder.softReboot` post-processing:
//   - delete `online_{neId}`
//   - set `rebootUser_{neId} = operationUser` (5 minutes)
//
// `expiredAtMillis` is forwarded to `EventDto.expiredAt`.
func (o *OperationSender) SendSoftReboot(sn string, operationId string, operationUser string, expiredAtMillis int64) error {
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
		rebootUserKey := fmt.Sprintf("rebootUser_%d", elementId)
		redis.Del(ctx, onlineKey)
		redis.Set(ctx, rebootUserKey, operationUser, 5*time.Minute)
	}

	o.saveTrackData(operationId, headerId, sn, "SOFT_REBOOT", commandKey, operationUser, expiredAtMillis)
	return o.msgManager.PutMessageWithPriority(sn, soapXml, QueueNormal, expiredAtMillis)
}

// SendBatchUpgrade sends a BatchUpgrade request to the device (vendor extension).
func (o *OperationSender) SendBatchUpgrade(sn string, batch *soap.BatchUpgrade, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildBatchUpgrade(headerId, batch)
	o.saveTrackData(operationId, headerId, sn, "BATCH_UPGRADE", batch.CommandKey, "", 0)
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendCancelFutureUpgrade sends a CancelFutureUpgrade request to the device (vendor extension).
func (o *OperationSender) SendCancelFutureUpgrade(sn string, commandKey string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildCancelFutureUpgrade(headerId, commandKey)
	o.saveTrackData(operationId, headerId, sn, "CANCEL_FUTURE_UPGRADE", commandKey, "", 0)
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendHttpRequestProxy sends an HttpRequestProxy request to the device (vendor extension).
func (o *OperationSender) SendHttpRequestProxy(sn string, proxy *soap.HttpRequestProxy, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildHttpRequestProxy(headerId, proxy)
	o.saveTrackData(operationId, headerId, sn, "HTTP_REQUEST_PROXY", "", "", 0)
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendSetParameterAttributes sends a SetParameterAttributes request to the device.
func (o *OperationSender) SendSetParameterAttributes(sn string, spa *soap.SetParameterAttributes, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildSetParameterAttributes(headerId, spa)
	o.saveTrackData(operationId, headerId, sn, "SET_PARAMETER_ATTRIBUTES", "", "", 0)
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendGetParameterAttributes sends a GetParameterAttributes request to the device.
func (o *OperationSender) SendGetParameterAttributes(sn string, names []string, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildGetParameterAttributes(headerId, names)
	o.saveTrackData(operationId, headerId, sn, "GET_PARAMETER_ATTRIBUTES", "", "", 0)
	return o.msgManager.PutMessage(sn, soapXml)
}

// SendUpdateCBSDStatus sends an UpdateCBSDStatus request to the device (vendor extension).
func (o *OperationSender) SendUpdateCBSDStatus(sn string, ucs *soap.UpdateCBSDStatus, operationId string) error {
	headerId := soap.GenerateHeaderID()
	soapXml := soap.BuildUpdateCBSDStatus(headerId, ucs)
	o.saveTrackData(operationId, headerId, sn, "UPDATE_CBSD_STATUS", "", "", 0)
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
	ts := fmt.Sprintf("%d", time.Now().Unix())
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
		decrypted, err := aesGCMDecrypt(*creds.ConnectionRequestPassword, defaultPasswordSecret)
		if err != nil {
			logger.Warnf("failed to decrypt connection request password for device %s: %v", sn, err)
		} else {
			password = decrypted
		}
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
		if creds.ConnectionRequestPassword != nil && *creds.ConnectionRequestPassword != "" {
			decrypted, err := aesGCMDecrypt(*creds.ConnectionRequestPassword, defaultPasswordSecret)
			if err != nil {
				logger.Warnf("failed to decrypt connection request password for device %s: %v", sn, err)
				password = *creds.ConnectionRequestPassword
			} else {
				password = decrypted
			}
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
	return strings.ToUpper(hex.EncodeToString(mac.Sum(nil)))
}

// aesGCMDecrypt decrypts AES-GCM encrypted data (Base64 encoded, IV prepended).
// Matches Java AESGCMUtil.decrypt(encryptedData, key).
func aesGCMDecrypt(encryptedData string, secret string) (string, error) {
	// Fix key to 32 bytes (AES-256), matching Java AESGCMUtil.fixTo32
	key := []byte(fixTo32(secret))

	decodedBytes, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	gcmIVLength := 12
	if len(decodedBytes) < gcmIVLength {
		return "", fmt.Errorf("encrypted data too short")
	}

	iv := decodedBytes[:gcmIVLength]
	ciphertext := decodedBytes[gcmIVLength:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("gcm open: %w", err)
	}

	return string(plaintext), nil
}

// fixTo32 pads or truncates a string to exactly 32 bytes.
// Matches Java AESGCMUtil.fixTo32.
func fixTo32(input string) string {
	if len(input) >= 32 {
		return input[:32]
	}
	var buf bytes.Buffer
	buf.WriteString(input)
	for buf.Len() < 32 {
		buf.WriteByte('0')
	}
	return buf.String()
}

// saveTrackData saves command tracking data to the event_log table.
// commandKey is the TR-069 CommandKey used to correlate TransferComplete back to the originating operation.
// operationUser is recorded on the event_log row; expiredAtMillis (when > 0)
// overrides the default operationTime+5min expiration for the SOAP message.
func (o *OperationSender) saveTrackData(operationId string, headerId string, sn string, operationType string, commandKey string, operationUser string, expiredAtMillis int64, extraFields ...map[string]interface{}) {
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
	if operationUser != "" {
		eventLog.User = stringPtr(operationUser)
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
	if operationUser != "" {
		trackData["operation_user"] = operationUser
	}
	if expiredAtMillis > 0 {
		trackData["expired_at"] = expiredAtMillis
	}
	for _, extra := range extraFields {
		for k, v := range extra {
			trackData[k] = v
		}
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
