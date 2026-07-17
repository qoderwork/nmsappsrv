package tr069

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

const (
	QueuePrior  = 1
	QueueCBSD   = 2
	QueueNormal = 0
)

const (
	queuePriorPrefix  = "station_prior_event_queue_"
	queueCbsdPrefix   = "station_cbsd_event_queue_"
	queueNormalPrefix = "station_event_queue_"
)

type QueueMessage struct {
	SoapXml   string `json:"soapXml"`
	ExpiredAt int64  `json:"expiredAt"`
}

type MessageManager struct{}

func NewMessageManager() *MessageManager {
	return &MessageManager{}
}

func queueKey(priority int, sn string) string {
	switch priority {
	case QueuePrior:
		return queuePriorPrefix + sn
	case QueueCBSD:
		return queueCbsdPrefix + sn
	default:
		return queueNormalPrefix + sn
	}
}

func (m *MessageManager) GetMessage(sn string) string {
	ctx := context.Background()

	keys := []string{
		queueKey(QueuePrior, sn),
		queueKey(QueueCBSD, sn),
		queueKey(QueueNormal, sn),
	}

	now := time.Now().UnixMilli()

	for _, key := range keys {
		for {
			msgStr, err := redis.RPop(ctx, key)
			if err != nil || msgStr == "" {
				break
			}

			var qm QueueMessage
			if err := json.Unmarshal([]byte(msgStr), &qm); err != nil {
				logger.Warnf("failed to unmarshal queue message from key %s: %v", key, err)
				continue
			}

			if qm.ExpiredAt == -1 || now < qm.ExpiredAt {
				logger.Infof("retrieved pending message for device %s from queue %s", sn, key)
				return qm.SoapXml
			}
			logger.Debugf("dropped expired command for device %s from queue %s", sn, key)
		}
	}

	return ""
}

func (m *MessageManager) PutMessage(sn string, soapXml string) error {
	return m.PutMessageWithPriority(sn, soapXml, QueueNormal, 0)
}

func (m *MessageManager) PutMessageWithPriority(sn string, soapXml string, priority int, expiredAtMillis int64) error {
	ctx := context.Background()

	expiredAt := int64(-1)
	if expiredAtMillis > 0 {
		expiredAt = expiredAtMillis
	}

	qm := QueueMessage{
		SoapXml:   soapXml,
		ExpiredAt: expiredAt,
	}

	data, err := json.Marshal(qm)
	if err != nil {
		return fmt.Errorf("marshal queue message: %w", err)
	}

	key := queueKey(priority, sn)

	if err := redis.LPush(ctx, key, string(data)); err != nil {
		logger.Errorf("failed to enqueue message for device %s (priority=%d): %v", sn, priority, err)
		return err
	}

	redis.Expire(ctx, key, 24*time.Hour)

	logger.Infof("enqueued message for device %s, priority=%d, queue length: %d", sn, priority, redis.LLen(ctx, key))
	return nil
}
