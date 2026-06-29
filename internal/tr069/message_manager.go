package tr069

import (
	"context"
	"fmt"
	"time"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// MessageManager manages pending SOAP messages for CPE devices.
// It queues outbound ACS commands and delivers them when CPE polls.
type MessageManager struct{}

// NewMessageManager creates a new MessageManager.
func NewMessageManager() *MessageManager {
	return &MessageManager{}
}

// GetMessage retrieves the next pending SOAP message for the given device SN.
// Returns an empty string if no messages are pending.
func (m *MessageManager) GetMessage(sn string) string {
	ctx := context.Background()
	queueKey := fmt.Sprintf("tr069:queue:%s", sn)

	// Pop from the right side of the list (FIFO)
	msg, err := redis.RPop(ctx, queueKey)
	if err != nil {
		// No messages or error
		return ""
	}

	logger.Infof("retrieved pending message for device %s from queue", sn)
	return msg
}

// PutMessage enqueues a SOAP message to be sent to the device identified by SN.
func (m *MessageManager) PutMessage(sn string, soapXml string) error {
	ctx := context.Background()
	queueKey := fmt.Sprintf("tr069:queue:%s", sn)

	// Push to the left side of the list
	if err := redis.LPush(ctx, queueKey, soapXml); err != nil {
		logger.Errorf("failed to enqueue message for device %s: %v", sn, err)
		return err
	}

	// Set expiry on the queue key to prevent accumulation (24 hours)
	redis.Expire(ctx, queueKey, 24*time.Hour)

	logger.Infof("enqueued message for device %s, queue length: %d", sn, redis.LLen(ctx, queueKey))
	return nil
}
