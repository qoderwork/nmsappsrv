package websocket

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// Maximum message size allowed from peer.
	maxMessageSize = 1024
	// Send buffer size.
	sendBufferSize = 256
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSCommand represents a command received from the client.
type WSCommand struct {
	Action string `json:"action"` // "subscribe" or "unsubscribe"
	Topic  string `json:"topic"`
}

// WSHandler handles WebSocket connections.
type WSHandler struct {
	hub *Hub
}

// NewWSHandler creates a new WSHandler.
func NewWSHandler(hub *Hub) *WSHandler {
	return &WSHandler{hub: hub}
}

// ServeWS upgrades the HTTP connection to WebSocket and registers the client.
func (h *WSHandler) ServeWS(c *gin.Context) {
	// Bind the connection to the authenticated user when a valid JWT is
	// supplied. Browsers cannot set custom headers on the WebSocket handshake,
	// so the token is passed as a query parameter (?token=...); the
	// Authorization header is also accepted. Anonymous connections (no valid
	// token) are still allowed and receive broadcast topics only — they simply
	// cannot be targeted by SendToUser.
	username := extractWSUsername(c)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Errorf("websocket: upgrade failed: %v", err)
		return
	}

	clientID := uuid.New().String()
	client := &Client{
		hub:      h.hub,
		conn:     conn,
		send:     make(chan []byte, sendBufferSize),
		id:       clientID,
		username: username,
		topics:   make(map[string]bool),
	}

	// Register client with hub
	h.hub.register <- client

	// Start read and write pumps
	utils.SafeGo("ws-read-pump-"+clientID, func() {
		h.readPump(client)
	})
	utils.SafeGo("ws-write-pump-"+clientID, func() {
		h.writePump(client)
	})
}

// extractWSUsername extracts and validates the JWT from the WebSocket upgrade
// request and returns the authenticated username, or "" if no valid token is
// present (the connection then only receives broadcast topics).
func extractWSUsername(c *gin.Context) string {
	token := c.Query("token")
	if token == "" {
		if ah := c.GetHeader("Authorization"); ah != "" {
			if parts := strings.SplitN(ah, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				token = strings.TrimSpace(parts[1])
			}
		}
	}
	if token == "" {
		return ""
	}
	claims, err := middleware.ValidateToken(token)
	if err != nil {
		logger.Warnf("websocket: invalid token, connecting anonymously: %v", err)
		return ""
	}
	return claims.Username
}

// readPump reads messages from the WebSocket connection.
// Handles subscribe/unsubscribe commands from the client.
func (h *WSHandler) readPump(client *Client) {
	defer func() {
		h.hub.unregister <- client
		client.conn.Close()
	}()

	client.conn.SetReadLimit(maxMessageSize)
	client.conn.SetReadDeadline(time.Now().Add(pongWait))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logger.Errorf("websocket: read error from client %s: %v", client.id, err)
			}
			break
		}

		// Handle application-layer heartbeat (mirrors Java ResultWebSocket.onMessage
		// "heartbeat" branch). The client sends a plain-text "heartbeat" message
		// periodically; the server records the timestamp to track online status.
		msgStr := strings.TrimSpace(string(message))
		if msgStr == "heartbeat" || strings.HasPrefix(msgStr, "heartbeat") {
			if client.username != "" {
				h.hub.UpdateHeartbeat(client.username)
			}
			continue
		}

		// Handle message confirmation (mirrors Java ResultWebSocket.onMessage
		// "confirm {messageId}" branch). The client confirms receipt of a
		// cached message; the server removes it from the MessageCache.
		if strings.HasPrefix(msgStr, "confirm ") {
			messageId := strings.TrimSpace(strings.TrimPrefix(msgStr, "confirm "))
			if messageId != "" && h.hub.messageCache != nil {
				h.hub.messageCache.Remove(messageId)
			}
			continue
		}

		var cmd WSCommand
		if err := json.Unmarshal(message, &cmd); err != nil {
			logger.Errorf("websocket: invalid command from client %s: %v", client.id, err)
			continue
		}

		switch cmd.Action {
		case "subscribe":
			// The "user:" prefix is reserved for server-side per-user delivery
			// (SendToUser); clients must not subscribe to another user's stream.
			if cmd.Topic != "" && !strings.HasPrefix(cmd.Topic, "user:") {
				client.SubscribeTopic(cmd.Topic)
			}
		case "unsubscribe":
			if cmd.Topic != "" && !strings.HasPrefix(cmd.Topic, "user:") {
				client.UnsubscribeTopic(cmd.Topic)
			}
		default:
			logger.Errorf("websocket: unknown action %q from client %s", cmd.Action, client.id)
		}
	}
}

// writePump writes messages from the send channel to the WebSocket connection.
func (h *WSHandler) writePump(client *Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := client.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Drain queued messages into the same write
			n := len(client.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-client.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
