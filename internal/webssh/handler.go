package webssh

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// WebSocket constants (aligned with existing websocket/handler.go patterns)
// ---------------------------------------------------------------------------

const (
	wsWriteWait    = 10 * time.Second
	wsPongWait     = 60 * time.Second
	wsPingPeriod   = (wsPongWait * 9) / 10
	wsMaxMsgSize   = 4096 // terminal input should be small
	sshReadBufSize = 4096
)

// ---------------------------------------------------------------------------
// WebSocket message types from the client
// ---------------------------------------------------------------------------

// wsMessageType enumerates the types of messages the client can send.
type wsMessageType string

const (
	msgTypeInput  wsMessageType = "input"
	msgTypeResize wsMessageType = "resize"
)

// wsMessage is the JSON envelope sent by the browser terminal.
type wsMessage struct {
	Type wsMessageType `json:"type"`
	Data string        `json:"data,omitempty"` // for "input": raw keystrokes
	Cols int           `json:"cols,omitempty"` // for "resize"
	Rows int           `json:"rows,omitempty"` // for "resize"
}

// ---------------------------------------------------------------------------
// WebSocket upgrader
// ---------------------------------------------------------------------------

var websshUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ---------------------------------------------------------------------------
// HandleWebSSH is the main WebSocket endpoint: GET /webssh/:deviceId
// ---------------------------------------------------------------------------

// HandleWebSSH upgrades the HTTP connection to WebSocket, dials SSH to the
// target device, and bridges I/O between them.
func HandleWebSSH(c *gin.Context, svc Service) {
	// 1. Parse deviceId from URL
	deviceIDStr := c.Param("deviceId")
	deviceID, err := strconv.ParseInt(deviceIDStr, 10, 64)
	if err != nil || deviceID <= 0 {
		utils.Error(c, http.StatusBadRequest, "invalid deviceId")
		return
	}

	// 2. Resolve device SSH info from DB
	info, err := svc.ResolveDeviceInfo(deviceID)
	if err != nil {
		logger.Errorf("webssh: resolve device %d: %v", deviceID, err)
		utils.Error(c, http.StatusNotFound, "device not found or has no SSH info")
		return
	}

	// 3. Upgrade HTTP to WebSocket
	conn, err := websshUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Errorf("webssh: upgrade failed for device %d: %v", deviceID, err)
		return
	}

	// 4. Dial SSH
	client, session, err := svc.DialSSH(info)
	if err != nil {
		logger.Errorf("webssh: ssh dial device %d (%s): %v", deviceID, info.Host, err)
		conn.WriteMessage(websocket.TextMessage, []byte("SSH connection failed: "+err.Error()+"\r\n"))
		conn.Close()
		return
	}

	// 5. Create interactive SSH session with PTY
	// Parse initial terminal size from query params (optional)
	cols, _ := strconv.Atoi(c.DefaultQuery("cols", "80"))
	rows, _ := strconv.Atoi(c.DefaultQuery("rows", "24"))

	sshSess, err := NewSSHSession(client, session, cols, rows)
	if err != nil {
		logger.Errorf("webssh: create ssh session for device %d: %v", deviceID, err)
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to create SSH session: "+err.Error()+"\r\n"))
		conn.Close()
		client.Close()
		return
	}

	logger.Infof("webssh: connected to device %d (%s) user=%s", deviceID, info.DeviceName, info.Username)

	// 6. Bridge WebSocket <-> SSH
	bridgeWebSocket(conn, sshSess, deviceID)
}

// ---------------------------------------------------------------------------
// Bridge: bidirectional I/O between WebSocket and SSH session
// ---------------------------------------------------------------------------

func bridgeWebSocket(conn *websocket.Conn, sshSess *SSHSession, deviceID int64) {
	// done channel signals all goroutines to stop
	var once sync.Once
	done := make(chan struct{})

	cleanup := func() {
		once.Do(func() {
			close(done)
			sshSess.Close()
			conn.Close()
			logger.Infof("webssh: session ended for device %d", deviceID)
		})
	}
	defer cleanup()

	// --- Goroutine A: SSH stdout -> WebSocket ---
	utils.SafeGo("webssh-stdout-"+strconv.FormatInt(deviceID, 10), func() {
		defer cleanup()
		buf := make([]byte, sshReadBufSize)
		for {
			n, err := sshSess.Stdout.Read(buf)
			if n > 0 {
				conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					logger.Debugf("webssh: ws write error device %d: %v", deviceID, writeErr)
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					logger.Debugf("webssh: ssh stdout read device %d: %v", deviceID, err)
				}
				return
			}
		}
	})

	// --- Goroutine B: SSH stderr -> WebSocket ---
	utils.SafeGo("webssh-stderr-"+strconv.FormatInt(deviceID, 10), func() {
		defer cleanup()
		buf := make([]byte, sshReadBufSize)
		for {
			n, err := sshSess.Stderr.Read(buf)
			if n > 0 {
				conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					logger.Debugf("webssh: ws write error device %d: %v", deviceID, writeErr)
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					logger.Debugf("webssh: ssh stderr read device %d: %v", deviceID, err)
				}
				return
			}
		}
	})

	// --- Goroutine C: SSH session wait -> cleanup when shell exits ---
	utils.SafeGo("webssh-wait-"+strconv.FormatInt(deviceID, 10), func() {
		defer cleanup()
		sshSess.Wait()
	})

	// --- Main goroutine: WebSocket -> SSH stdin (with ping/pong heartbeat) ---
	conn.SetReadLimit(wsMaxMsgSize)
	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	// Ping ticker to keep WebSocket alive
	pingTicker := time.NewTicker(wsPingPeriod)
	defer pingTicker.Stop()

	// Read from WebSocket in a goroutine, send pings in the select loop
	wsMsgCh := make(chan wsMessage, 16)
	wsErrCh := make(chan error, 1)

	utils.SafeGo("webssh-wsread-"+strconv.FormatInt(deviceID, 10), func() {
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				wsErrCh <- err
				return
			}

			var msg wsMessage
			if err := json.Unmarshal(raw, &msg); err != nil {
				// Treat as raw input (for simple terminals that send raw data)
				wsMsgCh <- wsMessage{Type: msgTypeInput, Data: string(raw)}
				continue
			}
			wsMsgCh <- msg
		}
	})

	for {
		select {
		case <-done:
			return

		case err := <-wsErrCh:
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logger.Debugf("webssh: ws read error device %d: %v", deviceID, err)
			}
			return

		case msg := <-wsMsgCh:
			conn.SetReadDeadline(time.Now().Add(wsPongWait))
			switch msg.Type {
			case msgTypeInput:
				if msg.Data != "" {
					if err := sshSess.Write([]byte(msg.Data)); err != nil {
						logger.Debugf("webssh: ssh write error device %d: %v", deviceID, err)
						return
					}
				}
			case msgTypeResize:
				if msg.Cols > 0 && msg.Rows > 0 {
					if err := sshSess.Resize(msg.Cols, msg.Rows); err != nil {
						logger.Debugf("webssh: resize error device %d: %v", deviceID, err)
					}
				}
			}

		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
