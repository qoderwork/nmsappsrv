package websocket

import (
	"sync"
	"time"

	"nmsappsrv/pkg/logger"

	"github.com/gorilla/websocket"
)

// heartbeatTimeout is the threshold (in seconds) used to determine if a user is
// considered online based on the last heartbeat time. Mirrors Java's 120s
// threshold used in SystemUserManagementServiceImpl.
const heartbeatTimeout = 120 * time.Second

// Hub manages all connected WebSocket clients and message broadcasting.
type Hub struct {
	// All connected clients
	clients map[*Client]bool
	// Broadcast channel for messages sent to all clients
	broadcast chan []byte
	// Register channel for new client connections
	register chan *Client
	// Unregister channel for client disconnections
	unregister chan *Client
	// Mutex for thread-safe access to clients map
	mu sync.RWMutex
	// lastHeartbeatTime records the last heartbeat timestamp per username,
	// mirroring Java's AbstractWebSocket.lastHeartbeatTime. Used to determine
	// whether a user is online (for user list display and targeted delivery).
	heartbeatMu       sync.RWMutex
	lastHeartbeatTime map[string]time.Time
	// messageCache stores per-user messages pending client confirmation.
	// When non-nil, SendToUser enqueues into the cache instead of sending
	// directly; a Dispatcher goroutine handles actual delivery + retry.
	// Mirrors Java's MessageCache + WebSocketMessageDispatcher.
	messageCache *MessageCache
}

// Client represents a single WebSocket connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	id   string
	// username is the authenticated user this connection belongs to, bound from
	// the JWT supplied on the WebSocket handshake. Empty for anonymous
	// connections, which only receive broadcast topics. Used by SendToUser to
	// mirror Java's per-user directed delivery (/websocket/{username}).
	username string
	// Topics this client is subscribed to
	topics map[string]bool
	// Mutex for thread-safe access to topics
	topicsMu sync.RWMutex
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		clients:           make(map[*Client]bool),
		broadcast:         make(chan []byte, 256),
		register:          make(chan *Client),
		unregister:        make(chan *Client),
		lastHeartbeatTime: make(map[string]time.Time),
	}
}

// SetMessageCache attaches a MessageCache to the Hub. Once set, SendToUser
// enqueues messages into the cache for confirmed delivery (with retry) instead
// of sending directly. Mirrors Java's MessageCache being used by
// WebSocketMessageDispatcher.
func (h *Hub) SetMessageCache(cache *MessageCache) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messageCache = cache
}

// Run starts the hub's main event loop. Should be called in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			// Initialize heartbeat on connect (mirrors Java onOpen)
			if client.username != "" {
				h.UpdateHeartbeat(client.username)
			}
			logger.Infof("websocket: client %s connected, total clients: %d", client.id, len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			// Clear heartbeat if no other connection for this user remains
			if client.username != "" {
				h.maybeClearHeartbeat(client.username)
			}
			logger.Infof("websocket: client %s disconnected, total clients: %d", client.id, len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client's send buffer is full, disconnect it
					h.mu.RUnlock()
					h.mu.Lock()
					if _, ok := h.clients[client]; ok {
						delete(h.clients, client)
						close(client.send)
					}
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastMessage sends a message to all connected clients.
func (h *Hub) BroadcastMessage(message []byte) {
	select {
	case h.broadcast <- message:
	default:
		logger.Errorf("websocket: broadcast channel full, dropping message")
	}
}

// BroadcastToTopic sends a message only to clients subscribed to the given topic.
func (h *Hub) BroadcastToTopic(topic string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		client.topicsMu.RLock()
		subscribed := client.topics[topic]
		client.topicsMu.RUnlock()

		if subscribed {
			select {
			case client.send <- message:
			default:
				logger.Errorf("websocket: client %s send buffer full, skipping", client.id)
			}
		}
	}
}

// SendToUser delivers a message only to WebSocket clients authenticated as the
// given username. This mirrors Java's per-user directed delivery
// (/websocket/{username}) — as opposed to BroadcastToTopic's fan-out to every
// subscriber. If no client for that user is connected, the message is dropped.
// An empty username is a no-op (anonymous connections cannot be targeted).
// SendToUser sends a message to all connections of a specific user.
// If a MessageCache is attached, the message is enqueued into the cache for
// confirmed delivery (with retry) instead of being sent directly — mirrors
// Java's sendMessage flow. The caller passes the raw payload; the cache wraps
// it in a CachedMessage (WebSocketDTO) before delivery.
func (h *Hub) SendToUser(username string, message []byte) {
	if username == "" {
		return
	}
	// If message cache is configured, enqueue for confirmed delivery.
	if h.messageCache != nil {
		msg := &CachedMessage{
			Data:         string(message),
			Username:     username,
			LastSendTime: 0, // 0 => dispatcher will pick it up immediately
		}
		h.messageCache.Add(msg)
		return
	}
	// Fallback: direct delivery when no cache (preserves old behavior).
	h.SendToUserRaw(username, message)
}

// SendToUserRaw sends a message directly to all connections of a user without
// going through the cache. Used by the Dispatcher for actual delivery.
func (h *Hub) SendToUserRaw(username string, message []byte) {
	if username == "" {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.username != username {
			continue
		}
		select {
		case client.send <- message:
		default:
			logger.Errorf("websocket: client %s (user %s) send buffer full, dropping per-user message", client.id, username)
		}
	}
}

// HasUserConnection returns true if the user has at least one active WebSocket
// connection. Used by the Dispatcher to skip offline users.
func (h *Hub) HasUserConnection(username string) bool {
	if username == "" {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.username == username {
			return true
		}
	}
	return false
}

// SubscribeTopic subscribes a client to a topic.
func (c *Client) SubscribeTopic(topic string) {
	c.topicsMu.Lock()
	defer c.topicsMu.Unlock()
	c.topics[topic] = true
	logger.Infof("websocket: client %s subscribed to topic %s", c.id, topic)
}

// UnsubscribeTopic unsubscribes a client from a topic.
func (c *Client) UnsubscribeTopic(topic string) {
	c.topicsMu.Lock()
	defer c.topicsMu.Unlock()
	delete(c.topics, topic)
	logger.Infof("websocket: client %s unsubscribed from topic %s", c.id, topic)
}

// UpdateHeartbeat records the current time as the last heartbeat for the given
// username. Called on connect and whenever a "heartbeat" message is received.
// Mirrors Java's lastHeartbeatTime.put(username, System.currentTimeMillis()).
func (h *Hub) UpdateHeartbeat(username string) {
	if username == "" {
		return
	}
	h.heartbeatMu.Lock()
	defer h.heartbeatMu.Unlock()
	h.lastHeartbeatTime[username] = time.Now()
}

// GetLastHeartbeat returns the last heartbeat time for the given username and
// whether it exists.
func (h *Hub) GetLastHeartbeat(username string) (time.Time, bool) {
	h.heartbeatMu.RLock()
	defer h.heartbeatMu.RUnlock()
	t, ok := h.lastHeartbeatTime[username]
	return t, ok
}

// IsUserOnline returns true if the user has sent a heartbeat within
// heartbeatTimeout. Mirrors Java's check:
//   System.currentTimeMillis() - l < 120000
func (h *Hub) IsUserOnline(username string) bool {
	if username == "" {
		return false
	}
	h.heartbeatMu.RLock()
	defer h.heartbeatMu.RUnlock()
	t, ok := h.lastHeartbeatTime[username]
	if !ok {
		return false
	}
	return time.Since(t) < heartbeatTimeout
}

// ClearHeartbeat removes the heartbeat record for a username.
func (h *Hub) ClearHeartbeat(username string) {
	h.heartbeatMu.Lock()
	defer h.heartbeatMu.Unlock()
	delete(h.lastHeartbeatTime, username)
}

// maybeClearHeartbeat clears the heartbeat for a username only if no other
// active connection for that user remains. This handles the case where a user
// has multiple browser tabs open.
func (h *Hub) maybeClearHeartbeat(username string) {
	h.mu.RLock()
	for c := range h.clients {
		if c.username == username {
			h.mu.RUnlock()
			return
		}
	}
	h.mu.RUnlock()
	h.ClearHeartbeat(username)
}
