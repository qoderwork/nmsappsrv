package tr069

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
)

// STUN attribute types (RFC 5389)
const (
	stunAttrMappedAddress    uint16 = 0x0001
	stunAttrUsername         uint16 = 0x0006
	stunAttrXORMappedAddress uint16 = 0x0020
)

// STUN message types (RFC 5389)
// Message type encoding: bits encode method (STUN_METHOD_BINDING=1) and class.
// Binding Request  = 0x0001
// Binding Response = 0x0101
const (
	stunMsgBindingRequest  uint16 = 0x0001
	stunMsgBindingResponse uint16 = 0x0101
)

// STUN magic cookie value (RFC 5389)
const stunMagicCookie uint32 = 0x2112A442

// stunTTL is the Redis key TTL for STUN address entries.
const stunTTL = 10 * time.Minute

// STUNServer listens for STUN Binding Requests from CPE devices on UDP.
// When a device sends a STUN binding request, the server reads the source IP:port
// from the UDP packet and stores it in Redis at device:stun:{sn} so that
// SendConnectionRequest can use it to reach the device behind NAT.
type STUNServer struct {
	mu      sync.Mutex
	running bool
	port    int
	conn    *net.UDPConn
	done    chan struct{}
}

// NewSTUNServer creates a new STUNServer.
func NewSTUNServer(port int) *STUNServer {
	if port <= 0 {
		port = 3478
	}
	return &STUNServer{
		port: port,
	}
}

// Start begins listening for STUN Binding Requests in a background goroutine.
func (s *STUNServer) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.done = make(chan struct{})
	s.mu.Unlock()

	logger.Infof("STUN server starting on UDP port %d", s.port)

	utils.SafeGo("stun-server", func() {
		s.listenLoop()
	})
}

// Stop stops the STUN server.
func (s *STUNServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	if s.conn != nil {
		s.conn.Close()
	}
	close(s.done)
	logger.Info("STUN server stopped")
}

// IsRunning returns whether the server is currently running.
func (s *STUNServer) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// listenLoop opens the UDP socket and reads packets until stopped.
func (s *STUNServer) listenLoop() {
	addr := &net.UDPAddr{Port: s.port, IP: net.IPv4zero}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		logger.Errorf("STUN server: failed to listen on UDP port %d: %v", s.port, err)
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()

	logger.Infof("STUN server listening on UDP :%d", s.port)

	buf := make([]byte, 1500)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			// Check if we're shutting down
			if !s.IsRunning() {
				return
			}
			logger.Errorf("STUN server: read error: %v", err)
			continue
		}
		if n < 20 {
			// Too short to be a valid STUN message (header is 20 bytes)
			continue
		}
		// Process in a goroutine to avoid blocking the read loop
		msg := make([]byte, n)
		copy(msg, buf[:n])
		utils.SafeGo("stun-handler", func() {
			s.handleMessage(msg, remoteAddr)
		})
	}
}

// handleMessage processes a single STUN message.
// It parses the STUN Binding Request, extracts the device SN from the USERNAME
// attribute, and stores the source IP:port in Redis.
func (s *STUNServer) handleMessage(data []byte, remoteAddr *net.UDPAddr) {
	// Parse STUN header (20 bytes):
	//   [0:2]   message type (uint16, big-endian)
	//   [2:4]   message length (uint16, big-endian) — excludes 20-byte header
	//   [4:8]   magic cookie (uint32, big-endian) = 0x2112A442
	//   [8:20]  transaction ID (96 bits)
	msgType := binary.BigEndian.Uint16(data[0:2])
	msgLen := binary.BigEndian.Uint16(data[2:4])
	cookie := binary.BigEndian.Uint32(data[4:8])

	// Validate: must be a Binding Request
	if msgType != stunMsgBindingRequest {
		logger.Debugf("STUN server: ignoring non-Binding-Request (type=0x%04X) from %s", msgType, remoteAddr)
		return
	}

	// Validate magic cookie
	if cookie != stunMagicCookie {
		logger.Debugf("STUN server: bad magic cookie 0x%08X from %s", cookie, remoteAddr)
		return
	}

	// Validate message length doesn't exceed received data
	if int(msgLen)+20 > len(data) {
		logger.Debugf("STUN server: message length %d exceeds data size %d from %s", msgLen, len(data), remoteAddr)
		return
	}

	// Parse attributes to find USERNAME (device SN)
	sn := s.extractUsername(data[20 : 20+msgLen])

	if sn == "" {
		// No USERNAME attribute — use source IP as fallback key
		sn = remoteAddr.IP.String()
		logger.Debugf("STUN server: no USERNAME in request from %s, using source IP as key", remoteAddr)
	}

	// Store source address in Redis: device:stun:{sn} = hash{ip, port}
	sourceIP := remoteAddr.IP.String()
	sourcePort := strconv.Itoa(remoteAddr.Port)

	ctx := context.Background()
	stunKey := fmt.Sprintf("device:stun:%s", sn)

	if err := redis.HSet(ctx, stunKey, "ip", sourceIP, "port", sourcePort); err != nil {
		logger.Errorf("STUN server: failed to HSet for %s: %v", sn, err)
		return
	}
	if err := redis.Expire(ctx, stunKey, stunTTL); err != nil {
		logger.Errorf("STUN server: failed to set TTL for %s: %v", sn, err)
	}

	logger.Infof("STUN server: updated device %s -> %s:%s", sn, sourceIP, sourcePort)

	// Send STUN Binding Response back to the device
	s.sendBindingResponse(data, remoteAddr)
}

// extractUsername parses STUN attributes and returns the USERNAME value.
// The USERNAME may be just the device SN, or "sn:secret" format.
func (s *STUNServer) extractUsername(attrs []byte) string {
	offset := 0
	for offset+4 <= len(attrs) {
		attrType := binary.BigEndian.Uint16(attrs[offset : offset+2])
		attrLen := int(binary.BigEndian.Uint16(attrs[offset+2 : offset+4]))
		offset += 4

		if offset+attrLen > len(attrs) {
			break
		}

		if attrType == stunAttrUsername {
			username := string(attrs[offset : offset+attrLen])
			// USERNAME format may be "sn:secret" — extract the SN part
			if idx := strings.Index(username, ":"); idx > 0 {
				return username[:idx]
			}
			return username
		}

		// Attributes are padded to 4-byte boundaries
		padded := (attrLen + 3) &^ 3
		offset += padded
	}
	return ""
}

// sendBindingResponse sends a minimal STUN Binding Success Response back to the device.
// The response includes an XOR-MAPPED-ADDRESS attribute with the client's reflexive address.
func (s *STUNServer) sendBindingResponse(reqData []byte, remoteAddr *net.UDPAddr) {
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn == nil {
		return
	}

	// Build response
	// Header: type=0x0101, length=TBD, cookie, transaction ID (copied from request)
	resp := make([]byte, 20)
	binary.BigEndian.PutUint16(resp[0:2], stunMsgBindingResponse)
	binary.BigEndian.PutUint32(resp[4:8], stunMagicCookie)
	// Copy transaction ID from request (bytes 8..20)
	copy(resp[8:20], reqData[8:20])

	// Build XOR-MAPPED-ADDRESS attribute (12 bytes):
	//   type (2) + length (2) + reserved (1) + family (1) + XOR-port (2) + XOR-addr (4)
	xorPort := uint16(remoteAddr.Port) ^ uint16(stunMagicCookie>>16)
	ip4 := remoteAddr.IP.To4()
	var xorAddr uint32
	if ip4 != nil {
		xorAddr = binary.BigEndian.Uint32(ip4) ^ stunMagicCookie
	}

	attr := make([]byte, 12)
	binary.BigEndian.PutUint16(attr[0:2], stunAttrXORMappedAddress)
	binary.BigEndian.PutUint16(attr[2:4], 8) // value length: 8 bytes
	attr[4] = 0                              // reserved
	attr[5] = 0x01                           // family: IPv4
	binary.BigEndian.PutUint16(attr[6:8], xorPort)
	binary.BigEndian.PutUint32(attr[8:12], xorAddr)

	// Assemble: header + attribute
	resp = append(resp, attr...)
	// Update message length in header (excludes the 20-byte header)
	binary.BigEndian.PutUint16(resp[2:4], uint16(len(resp)-20))

	_, err := conn.WriteToUDP(resp, remoteAddr)
	if err != nil {
		logger.Errorf("STUN server: failed to send response to %s: %v", remoteAddr, err)
	}
}
