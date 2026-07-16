package webssh

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// WebSocket constants
// ---------------------------------------------------------------------------

const (
	wsWriteWait        = 10 * time.Second
	wsPongWait         = 60 * time.Second
	wsPingPeriod       = (wsPongWait * 9) / 10
	wsMaxMsgSize       = 4096
	sshReadBufSize     = 4096
	wsConnectTimeout   = 30 * time.Second // client must send "connect" within this window
)

// ---------------------------------------------------------------------------
// WebSocket message protocol (aligned with Java WebSSHData)
// ---------------------------------------------------------------------------

// webSSHData mirrors Java's WebSSHData pojo: the client sends JSON with an
// "operate" field and the relevant payload.
type webSSHData struct {
	Operate  string `json:"operate"`            // "connect" | "command" | "resize"
	Host     string `json:"host,omitempty"`     // connect: SSH host
	Port     int    `json:"port,omitempty"`     // connect: SSH port (default 22)
	Username string `json:"username,omitempty"` // connect: SSH username
	Password string `json:"password,omitempty"` // connect: SSH password
	Command  string `json:"command,omitempty"`  // command: keystrokes to send
	Cols     int    `json:"cols,omitempty"`     // resize: terminal columns
	Rows     int    `json:"rows,omitempty"`     // resize: terminal rows
}

const (
	operateConnect = "connect"
	operateCommand = "command"
	operateResize  = "resize" // Go extension (Java has no resize)
)

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
// HandleWebSSH is the main WebSocket endpoint: GET /webssh
//
// Flow (aligned with Java WebSSHServiceImpl):
//  1. Upgrade HTTP to WebSocket.
//  2. Wait for the client's "connect" message containing host/port/username/password.
//  3. Dial SSH using those client-provided credentials.
//  4. Bridge I/O: SSH stdout/stderr -> WebSocket, WebSocket "command" -> SSH stdin.
// ---------------------------------------------------------------------------

func HandleWebSSH(c *gin.Context, svc Service) {
	// 1. Upgrade HTTP to WebSocket
	conn, err := websshUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Errorf("webssh: upgrade failed: %v", err)
		return
	}

	// 2. Wait for "connect" message from client (with timeout)
	conn.SetReadLimit(wsMaxMsgSize)
	conn.SetReadDeadline(time.Now().Add(wsConnectTimeout))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		logger.Errorf("webssh: read connect message: %v", err)
		conn.Close()
		return
	}

	var connectMsg webSSHData
	if err := json.Unmarshal(raw, &connectMsg); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Invalid message format\r\n"))
		conn.Close()
		return
	}

	if connectMsg.Operate != operateConnect {
		conn.WriteMessage(websocket.TextMessage, []byte("Expected 'connect' operate first\r\n"))
		conn.Close()
		return
	}

	if connectMsg.Host == "" || connectMsg.Username == "" {
		conn.WriteMessage(websocket.TextMessage, []byte("host and username are required\r\n"))
		conn.Close()
		return
	}

	// 3. Build DeviceSSHInfo from client-provided credentials
	info := &DeviceSSHInfo{
		Host:     connectMsg.Host,
		Port:     connectMsg.Port,
		Username: connectMsg.Username,
		Password: connectMsg.Password,
	}

	// 4. Dial SSH
	client, session, err := svc.DialSSH(info)
	if err != nil {
		logger.Errorf("webssh: ssh dial %s@%s: %v", info.Username, info.Host, err)
		conn.WriteMessage(websocket.TextMessage, []byte("SSH connection failed: "+err.Error()+"\r\n"))
		conn.Close()
		return
	}

	// 5. Create interactive SSH session with PTY
	cols := connectMsg.Cols
	if cols <= 0 {
		cols = 80
	}
	rows := connectMsg.Rows
	if rows <= 0 {
		rows = 24
	}

	sshSess, err := NewSSHSession(client, session, cols, rows)
	if err != nil {
		logger.Errorf("webssh: create ssh session for %s: %v", info.Host, err)
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to create SSH session: "+err.Error()+"\r\n"))
		conn.Close()
		client.Close()
		return
	}

	// Send initial carriage return (aligned with Java transToSSH(channel, "\r"))
	sshSess.Write([]byte("\r"))

	logger.Infof("webssh: connected to %s@%s:%d", info.Username, info.Host, info.Port)

	// 6. Bridge WebSocket <-> SSH
	bridgeWebSocket(conn, sshSess, info.Host)
}

// ---------------------------------------------------------------------------
// Bridge: bidirectional I/O between WebSocket and SSH session
// ---------------------------------------------------------------------------

func bridgeWebSocket(conn *websocket.Conn, sshSess *SSHSession, host string) {
	var once sync.Once
	done := make(chan struct{})

	cleanup := func() {
		once.Do(func() {
			close(done)
			sshSess.Close()
			conn.Close()
			logger.Infof("webssh: session ended for %s", host)
		})
	}
	defer cleanup()

	// --- Goroutine A: SSH stdout -> WebSocket ---
	utils.SafeGo("webssh-stdout-"+host, func() {
		defer cleanup()
		buf := make([]byte, sshReadBufSize)
		for {
			n, err := sshSess.Stdout.Read(buf)
			if n > 0 {
				conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if writeErr := conn.WriteMessage(websocket.TextMessage, buf[:n]); writeErr != nil {
					logger.Debugf("webssh: ws write error %s: %v", host, writeErr)
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					logger.Debugf("webssh: ssh stdout read %s: %v", host, err)
				}
				return
			}
		}
	})

	// --- Goroutine B: SSH stderr -> WebSocket ---
	utils.SafeGo("webssh-stderr-"+host, func() {
		defer cleanup()
		buf := make([]byte, sshReadBufSize)
		for {
			n, err := sshSess.Stderr.Read(buf)
			if n > 0 {
				conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if writeErr := conn.WriteMessage(websocket.TextMessage, buf[:n]); writeErr != nil {
					logger.Debugf("webssh: ws write error %s: %v", host, writeErr)
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					logger.Debugf("webssh: ssh stderr read %s: %v", host, err)
				}
				return
			}
		}
	})

	// --- Goroutine C: SSH session wait -> cleanup when shell exits ---
	utils.SafeGo("webssh-wait-"+host, func() {
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

	pingTicker := time.NewTicker(wsPingPeriod)
	defer pingTicker.Stop()

	wsMsgCh := make(chan webSSHData, 16)
	wsErrCh := make(chan error, 1)

	utils.SafeGo("webssh-wsread-"+host, func() {
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				wsErrCh <- err
				return
			}

			var msg webSSHData
			if err := json.Unmarshal(raw, &msg); err != nil {
				// Treat as raw command input (for simple terminals that send raw data)
				wsMsgCh <- webSSHData{Operate: operateCommand, Command: string(raw)}
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
				logger.Debugf("webssh: ws read error %s: %v", host, err)
			}
			return

		case msg := <-wsMsgCh:
			conn.SetReadDeadline(time.Now().Add(wsPongWait))
			switch msg.Operate {
			case operateCommand:
				if msg.Command != "" {
					if err := sshSess.Write([]byte(msg.Command)); err != nil {
						logger.Debugf("webssh: ssh write error %s: %v", host, err)
						return
					}
				}
			case operateResize:
				if msg.Cols > 0 && msg.Rows > 0 {
					if err := sshSess.Resize(msg.Cols, msg.Rows); err != nil {
						logger.Debugf("webssh: resize error %s: %v", host, err)
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
