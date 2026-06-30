package tr069

import (
	"encoding/base64"
	"io"
	"net/http"
	"strings"

	"nmsappsrv/internal/device"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ACSHandler is the core HTTP handler that processes CWMP SOAP requests from CPE devices.
// It replaces the Java ACSController + AcsServiceImpl.
type ACSHandler struct {
	db             *gorm.DB
	msgManager     *MessageManager
	eventProcessor *EventProcessor
}

// NewACSHandler creates a new ACSHandler with the given dependencies.
func NewACSHandler(db *gorm.DB, msgManager *MessageManager, eventProcessor *EventProcessor) *ACSHandler {
	return &ACSHandler{
		db:             db,
		msgManager:     msgManager,
		eventProcessor: eventProcessor,
	}
}

// HandleACS is the main Gin handler for POST /tr069/acs (generic device type).
func (h *ACSHandler) HandleACS(c *gin.Context) {
	h.handleACSWithType(c, "", "")
}

// ACSAuthMiddleware returns a Gin middleware that performs optional HTTP Basic authentication
// for TR-069 CPE connections. If the device has connection_request_username configured in DB,
// the middleware validates the incoming Basic auth credentials.
// If no credentials are configured for the device, the request is allowed through (backward compatible).
func (h *ACSHandler) ACSAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract SN from cookie first (for established sessions)
		sn := ""
		if cookie, err := c.Request.Cookie("SN"); err == nil {
			sn = cookie.Value
		}

		// If no SN cookie, try to peek at the SOAP body to extract SN from Inform
		if sn == "" {
			// Allow Inform to pass through without auth (device not yet identified)
			c.Next()
			return
		}

		// Look up device credentials
		var cpe device.CpeElement
		err := h.db.Select("connection_request_username, connection_request_password").
			Where("serial_number = ? AND deleted = ?", sn, false).First(&cpe).Error
		if err != nil {
			// Device not found or no credentials — allow through
			c.Next()
			return
		}

		// If device has no username configured, skip auth
		if cpe.ConnectionRequestUsername == nil || *cpe.ConnectionRequestUsername == "" {
			c.Next()
			return
		}

		// Validate HTTP Basic auth
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Header("WWW-Authenticate", `Basic realm="TR-069 ACS"`)
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Basic ") {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(authHeader[6:])
		if err != nil {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) != 2 {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		expectedUser := *cpe.ConnectionRequestUsername
		expectedPass := ""
		if cpe.ConnectionRequestPassword != nil {
			expectedPass = *cpe.ConnectionRequestPassword
		}

		if parts[0] != expectedUser || parts[1] != expectedPass {
			logger.Warnf("ACS auth failed for device %s: invalid credentials", sn)
			c.Header("WWW-Authenticate", `Basic realm="TR-069 ACS"`)
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		c.Next()
	}
}

// HandleEnbACS handles TR069 requests from eNodeB devices.
func (h *ACSHandler) HandleEnbACS(c *gin.Context) {
	h.handleACSWithType(c, "enb", "")
}

// HandleGnbACS handles TR069 requests from gNodeB (5G NR) devices.
func (h *ACSHandler) HandleGnbACS(c *gin.Context) {
	h.handleACSWithType(c, "enb", "NR")
}

// HandleCpeACS handles TR069 requests from CPE devices.
func (h *ACSHandler) HandleCpeACS(c *gin.Context) {
	h.handleACSWithType(c, "cpe", "")
}

// handleACSWithType is the shared implementation for all device-type-specific handlers.
func (h *ACSHandler) handleACSWithType(c *gin.Context, deviceType string, generation string) {
	// Read SOAP XML from request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Errorf("failed to read request body: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}
	defer c.Request.Body.Close()

	soapXml := strings.TrimSpace(string(body))

	// Get device SN from cookie (if exists)
	sn := ""
	if cookie, err := c.Request.Cookie("SN"); err == nil {
		sn = cookie.Value
	}

	// Delegate to the core processing logic
	h.processSoap(c, soapXml, sn, deviceType, generation)
}

// processSoap is the core logic that dispatches based on SOAP content and message type.
func (h *ACSHandler) processSoap(c *gin.Context, soapXml string, sn string, deviceType string, generation string) {
	// Case 1: Empty SOAP - CPE is polling for commands
	if soapXml == "" {
		if sn == "" {
			logger.Warn("empty SOAP request with no SN cookie, returning empty response")
			h.sendEmptyResponse(c)
			return
		}
		logger.Infof("device %s polling for pending commands", sn)
		h.pollForCommand(c, sn)
		return
	}

	// Case 2: SOAP present - detect message type
	msgType := soap.DetectMessageType(soapXml)
	logger.Infof("received SOAP from SN=%s, deviceType=%s, generation=%s, msgType=%d", sn, deviceType, generation, msgType)

	switch msgType {
	case soap.MsgInform:
		h.handleInform(c, soapXml, sn, deviceType, generation)

	case soap.MsgTransferComplete,
		soap.MsgAutonomousTransferComplete,
		soap.MsgFragmentTransferComplete:
		h.handleTransferComplete(c, soapXml, sn, deviceType, generation)

	default:
		// Any other response (GetParameterValuesResponse, SetParameterValuesResponse, etc.)
		h.handleGenericResponse(c, soapXml, sn, deviceType, generation)
	}
}

// handleInform processes an Inform message from CPE.
func (h *ACSHandler) handleInform(c *gin.Context, soapXml string, sn string, deviceType string, generation string) {
	inform, err := soap.ParseInform(soapXml)
	if err != nil {
		logger.Errorf("failed to parse Inform from SN=%s: %v", sn, err)
		h.sendEmptyResponse(c)
		return
	}

	// Extract SN from DeviceId.SerialNumber
	sn = inform.DeviceId.SerialNumber
	if sn == "" {
		logger.Error("Inform message has empty SerialNumber")
		h.sendEmptyResponse(c)
		return
	}

	// Set SN cookie on response
	c.SetCookie("SN", sn, 0, "/", "", false, false)
	logger.Infof("Inform from device SN=%s, manufacturer=%s, productClass=%s",
		sn, inform.DeviceId.Manufacturer, inform.DeviceId.ProductClass)

	// Enqueue Inform to event queue
	h.eventProcessor.ProcessInform(inform, sn, deviceType, generation)

	// Send InformResponse
	headerId := inform.Header.ID
	if headerId == "" {
		headerId = soap.GenerateHeaderID()
	}
	responseXml := soap.BuildInformResponse(headerId)
	h.sendSoapToResponse(c, responseXml)
}

// handleTransferComplete processes TransferComplete, AutonomousTransferComplete,
// and FragmentTransferComplete messages from CPE.
func (h *ACSHandler) handleTransferComplete(c *gin.Context, soapXml string, sn string, deviceType string, generation string) {
	// Enqueue to event result queue
	h.eventProcessor.ProcessResult(soapXml, sn, deviceType, generation)

	// Check msgManager for pending commands, send next if available
	if sn != "" {
		h.pollForCommand(c, sn)
		return
	}

	// No SN available, close session
	h.sendEmptyResponse(c)
}

// handleGenericResponse processes any other CWMP response from CPE
// (GetParameterValuesResponse, SetParameterValuesResponse, DownloadResponse, etc.)
func (h *ACSHandler) handleGenericResponse(c *gin.Context, soapXml string, sn string, deviceType string, generation string) {
	// Enqueue to event result queue
	h.eventProcessor.ProcessResult(soapXml, sn, deviceType, generation)

	// Check msgManager for pending commands, send next if available
	if sn != "" {
		h.pollForCommand(c, sn)
		return
	}

	// No SN available, close session
	h.sendEmptyResponse(c)
}

// pollForCommand polls the MessageManager for a pending command for the given device SN.
// If a command is found, it is written to the HTTP response. Otherwise, an empty response
// is sent to close the session.
func (h *ACSHandler) pollForCommand(c *gin.Context, sn string) {
	nextMsg := h.msgManager.GetMessage(sn)
	if nextMsg != "" {
		logger.Infof("sending pending command to device SN=%s", sn)
		h.sendSoapToResponse(c, nextMsg)
		return
	}

	// No pending commands - close session
	logger.Infof("no pending commands for device SN=%s, closing session", sn)
	h.sendEmptyResponse(c)
}

// sendSoapToResponse writes a SOAP XML message to the HTTP response with the correct Content-Type.
func (h *ACSHandler) sendSoapToResponse(c *gin.Context, soapXml string) {
	c.Header("Content-Type", "text/xml; charset=utf-8")
	c.String(http.StatusOK, soapXml)
}

// sendEmptyResponse sends an empty 200 response to signal session close.
func (h *ACSHandler) sendEmptyResponse(c *gin.Context) {
	c.Status(http.StatusOK)
}
