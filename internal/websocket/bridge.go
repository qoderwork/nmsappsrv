package websocket

import (
	"context"
	"encoding/json"
	"time"

	"nmsappsrv/internal/mq"
	"nmsappsrv/pkg/logger"
	redisclient "nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// WSMessage is the JSON format sent to WebSocket clients.
type WSMessage struct {
	Topic     string      `json:"topic"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp"`
}

// channelToTopic maps Redis pub/sub channels to WebSocket topics.
var channelToTopic = map[string]string{
	"channel:alarm:notify": "alarm",
	"device:status:change": "device_status",
	"parameter:change":     "parameter_change",
}

// Bridge bridges Redis pub/sub messages and queue messages to WebSocket clients.
type Bridge struct {
	hub    *Hub
	db     *gorm.DB
	cancel context.CancelFunc
}

// NewBridge creates a new Bridge.
func NewBridge(hub *Hub, db *gorm.DB) *Bridge {
	return &Bridge{
		hub: hub,
		db:  db,
	}
}

// Start begins consuming Redis pub/sub channels and the web callback queue.
func (b *Bridge) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel

	// Start Redis pub/sub subscriber
	utils.SafeGo("ws-bridge-pubsub", func() {
		b.subscribePubSub(ctx)
	})

	// Start web callback queue consumer
	utils.SafeGo("ws-bridge-queue", func() {
		b.consumeWebCallbackQueue(ctx)
	})

	logger.Info("websocket: bridge started")
}

// Stop gracefully shuts down the bridge.
func (b *Bridge) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
	logger.Info("websocket: bridge stopped")
}

// subscribePubSub subscribes to Redis pub/sub channels and bridges messages to WebSocket topics.
func (b *Bridge) subscribePubSub(ctx context.Context) {
	channels := make([]string, 0, len(channelToTopic))
	for ch := range channelToTopic {
		channels = append(channels, ch)
	}

	pubsub := redisclient.Subscribe(ctx, channels...)
	defer pubsub.Close()

	// Wait for subscription confirmation
	_, err := pubsub.Receive(ctx)
	if err != nil {
		logger.Errorf("websocket: failed to subscribe to pub/sub channels: %v", err)
		return
	}

	logger.Infof("websocket: subscribed to Redis channels: %v", channels)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			topic, exists := channelToTopic[msg.Channel]
			if !exists {
				continue
			}

			wsMsg := WSMessage{
				Topic:     topic,
				Data:      json.RawMessage(msg.Payload),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}

			data, err := json.Marshal(wsMsg)
			if err != nil {
				logger.Errorf("websocket: failed to marshal bridge message: %v", err)
				continue
			}

			b.hub.BroadcastToTopic(topic, data)
		}
	}
}

// consumeWebCallbackQueue consumes messages from the web_callback Redis queue
// using BRPop and broadcasts them to all WebSocket clients.
func (b *Bridge) consumeWebCallbackQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result, err := redisclient.BRPop(ctx, 5*time.Second, mq.WebCallbackQueue)
		if err != nil {
			// BRPop returns error on timeout, which is normal
			continue
		}

		if len(result) < 2 {
			continue
		}

		payload := result[1]

		wsMsg := WSMessage{
			Topic:     "web_callback",
			Data:      json.RawMessage(payload),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}

		data, err := json.Marshal(wsMsg)
		if err != nil {
			logger.Errorf("websocket: failed to marshal web callback message: %v", err)
			continue
		}

		b.hub.BroadcastMessage(data)
	}
}
