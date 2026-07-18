package websocket

import (
	"encoding/json"
	"time"

	"nmsappsrv/pkg/logger"
)

// Dispatcher polls the MessageCache every second and re-sends unconfirmed
// messages to the target user via the Hub. It also periodically cleans up
// messages older than 24 hours.
// Mirrors Java's WebSocketMessageDispatcher.
type Dispatcher struct {
	hub    *Hub
	cache  *MessageCache
	stopCh chan struct{}
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher(hub *Hub, cache *MessageCache) *Dispatcher {
	return &Dispatcher{
		hub:    hub,
		cache:  cache,
		stopCh: make(chan struct{}),
	}
}

// Start launches the dispatcher loop. Should be called in a goroutine.
func (d *Dispatcher) Start() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			logger.Info("websocket: dispatcher stopped")
			return
		case <-ticker.C:
			d.dispatchPending()
			d.cleanupExpired()
		}
	}
}

// Stop signals the dispatcher to stop.
func (d *Dispatcher) Stop() {
	close(d.stopCh)
}

// dispatchPending re-sends all messages due for retry.
func (d *Dispatcher) dispatchPending() {
	pending := d.cache.GetPending()
	if len(pending) == 0 {
		return
	}

	now := time.Now().UnixMilli()
	for _, msg := range pending {
		// Skip if user is offline (no active connection).
		if !d.hub.HasUserConnection(msg.Username) {
			continue
		}

		// Serialize the CachedMessage as WebSocketDTO JSON (mirrors Java).
		data, err := json.Marshal(msg)
		if err != nil {
			logger.Errorf("websocket: failed to marshal cached message %s: %v", msg.MessageId, err)
			continue
		}

		d.hub.SendToUserRaw(msg.Username, data)
		msg.LastSendTime = now
		msg.RetriedTimes++
	}
}

// cleanupExpired removes messages older than 24 hours.
func (d *Dispatcher) cleanupExpired() {
	expired := d.cache.GetExpired()
	if len(expired) == 0 {
		return
	}
	for _, msg := range expired {
		logger.Warnf("websocket: removing expired unconfirmed message %s for user %s", msg.MessageId, msg.Username)
		d.cache.Remove(msg.MessageId)
	}
}
