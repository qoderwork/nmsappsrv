package mq

import (
	"context"
	"encoding/json"
	"time"

	"nmsappsrv/pkg/logger"
	redisclient "nmsappsrv/pkg/redis"
)

// Queue name constants (replacing RabbitMQ queues)
const (
	// OperationQueue is the unified device-operation queue. Aligns with
	// Java nms-serv's `operation_queue` RabbitMQ queue consumed by
	// `Receiver.operationQueue` → `apiCommandProcessor.processCommand`.
	// A single dispatcher in `internal/operation` BRPops this list, applies
	// a 200 ops/s rate limiter, and routes by `Operation` string to the
	// matching `tr069.OperationSender.Send*` primitive.
	OperationQueue    = "operation_queue"
	InformQueue       = "queue:inform"        // TR069 Inform messages
	EventResultQueue  = "queue:event_result"  // event/command results
	AlarmQueue        = "queue:alarm"         // alarm processing
	SNMPQueue         = "queue:snmp"          // SNMP operations
	WebCallbackQueue  = "queue:web_callback"  // WebSocket push to frontend
	PMQueue           = "queue:pm"            // performance monitoring data
	ParameterQueue    = "queue:parameter"     // parameter set/get
	UpgradeQueue      = "queue:upgrade"       // firmware upgrade
	BatchConfigQueue  = "queue:batch_config"  // batch configuration
	ZTPQueue          = "queue:ztp"           // zero-touch provisioning
	MMLQueue          = "queue:mml"           // MML command execution
	CoreNetQueue      = "queue:corenet"       // core network operations
	CBSDQueue         = "queue:cbsd"          // CBSD/SAS operations
	NorthQueue        = "queue:north"         // north interface
)

// Pub/Sub channel constants
const (
	ChannelDeviceOnline  = "channel:device:online"
	ChannelDeviceOffline = "channel:device:offline"
	ChannelAlarmNotify   = "channel:alarm:notify"
	ChannelTaskProgress  = "channel:task:progress"
	ChannelVIPChange     = "channel:ha:vip:change"
)

// Enqueue JSON marshals data and pushes to queue
func Enqueue(ctx context.Context, queue string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.Errorf("mq: failed to marshal data for queue %s: %v", queue, err)
		return err
	}

	if err := redisclient.LPush(ctx, queue, jsonData); err != nil {
		logger.Errorf("mq: failed to enqueue to %s: %v", queue, err)
		return err
	}

	logger.Debugf("mq: enqueued message to %s", queue)
	return nil
}

// Dequeue pops a message from the queue
func Dequeue(ctx context.Context, queue string) (string, error) {
	msg, err := redisclient.RPop(ctx, queue)
	if err != nil {
		return "", err
	}

	logger.Debugf("mq: dequeued message from %s", queue)
	return msg, nil
}

// DequeueBlocking blocks until a message is available or timeout
func DequeueBlocking(ctx context.Context, timeout time.Duration, queues ...string) ([]string, error) {
	result, err := redisclient.BRPop(ctx, timeout, queues...)
	if err != nil {
		return nil, err
	}

	logger.Debugf("mq: dequeued message from %s", result[0])
	return result, nil
}

// PublishEvent JSON marshals data and publishes to channel
func PublishEvent(ctx context.Context, channel string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.Errorf("mq: failed to marshal data for channel %s: %v", channel, err)
		return err
	}

	if err := redisclient.Publish(ctx, channel, jsonData); err != nil {
		logger.Errorf("mq: failed to publish to %s: %v", channel, err)
		return err
	}

	logger.Debugf("mq: published event to %s", channel)
	return nil
}

// QueueLength returns the current length of the queue
func QueueLength(ctx context.Context, queue string) int64 {
	return redisclient.LLen(ctx, queue)
}
