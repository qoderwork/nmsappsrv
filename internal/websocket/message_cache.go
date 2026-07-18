package websocket

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// CachedMessage represents a WebSocket message pending client confirmation.
// Mirrors Java's WebSocketDTO.
type CachedMessage struct {
	Data          string    `json:"data"`
	MessageId     string    `json:"messageId"`
	Username      string    `json:"username"`
	LastSendTime  int64     `json:"lastSendTime"`
	RetriedTimes  int       `json:"retriedTimes"`
	GeneratedTime int64     `json:"generatedTime"`
}

// MessageCache stores WebSocket messages pending client confirmation.
// Mirrors Java's MessageCache — in-memory ConcurrentHashMap, no Redis.
// Messages are retried until the client confirms receipt or they expire.
type MessageCache struct {
	mu       sync.RWMutex
	messages map[string]*CachedMessage
}

// NewMessageCache creates a new MessageCache.
func NewMessageCache() *MessageCache {
	return &MessageCache{
		messages: make(map[string]*CachedMessage),
	}
}

// Add stores a message in the cache. The message will be retried until the
// client confirms receipt via "confirm {messageId}" or it expires.
func (c *MessageCache) Add(msg *CachedMessage) {
	if msg.MessageId == "" {
		msg.MessageId = uuid.New().String()
	}
	if msg.GeneratedTime == 0 {
		msg.GeneratedTime = time.Now().UnixMilli()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages[msg.MessageId] = msg
}

// Remove deletes a message from the cache (called on client confirmation).
func (c *MessageCache) Remove(messageId string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.messages, messageId)
}

// GetPending returns messages that are due for retry. A message is due if:
//   - retriedTimes < 3 and lastSendTime > 10s ago, OR
//   - retriedTimes >= 3 and lastSendTime > retriedTimes*60s ago
// Mirrors Java's MessageCache.getMessages() logic.
func (c *MessageCache) GetPending() []*CachedMessage {
	now := time.Now().UnixMilli()
	c.mu.RLock()
	defer c.mu.RUnlock()

	var pending []*CachedMessage
	for _, msg := range c.messages {
		if msg.RetriedTimes < 3 {
			if now-msg.LastSendTime > 10*1000 {
				pending = append(pending, msg)
			}
		} else {
			if now-msg.LastSendTime > int64(msg.RetriedTimes)*60*1000 {
				pending = append(pending, msg)
			}
		}
	}
	return pending
}

// GetExpired returns messages older than 24 hours (generatedTime).
// Mirrors Java's MessageCache.getExpiredMessage().
func (c *MessageCache) GetExpired() []*CachedMessage {
	now := time.Now().UnixMilli()
	c.mu.RLock()
	defer c.mu.RUnlock()

	var expired []*CachedMessage
	for _, msg := range c.messages {
		if msg.GeneratedTime == 0 || now-msg.GeneratedTime > 24*60*60*1000 {
			expired = append(expired, msg)
		}
	}
	return expired
}

// Size returns the number of cached messages.
func (c *MessageCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.messages)
}
