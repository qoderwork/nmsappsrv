package health

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"nmsappsrv/internal/mq"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"

	"gorm.io/gorm"
)

// Service provides health check operations
type Service struct {
	repo    *Repository
	haCache sync.Map
}

// NewService creates a new Service
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// HealthCheck returns basic health status
func (s *Service) HealthCheck() HealthStatus {
	return newHealthStatus()
}

// GetMysqlInfo returns MySQL health metrics
func (s *Service) GetMysqlInfo() (*MysqlInfo, error) {
	metrics, err := s.repo.GetMysqlGlobalStatus()
	if err != nil {
		return nil, err
	}

	return &MysqlInfo{
		Uptime:               metrics["Uptime"],
		ThreadsConnected:     metrics["Threads_connected"],
		AbortedConnects:      metrics["Aborted_connects"],
		SlowQueries:          metrics["Slow_queries"],
		CreatedTmpTables:     metrics["Created_tmp_tables"],
		CreatedTmpDiskTables: metrics["Created_tmp_disk_tables"],
		TableLocksWaited:     metrics["Table_locks_waited"],
		ComRollback:          metrics["Com_rollback"],
	}, nil
}

// GetRedisInfo returns Redis health metrics
func (s *Service) GetRedisInfo() (*RedisInfo, error) {
	ctx := context.Background()
	info, err := redis.RDB.Info(ctx, "all").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get redis info: %w", err)
	}

	metrics := make(map[string]string)
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if !strings.Contains(line, ":") || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			metrics[parts[0]] = parts[1]
		}
	}

	return &RedisInfo{
		ProcessId:                metrics["process_id"],
		RedisVersion:             metrics["redis_version"],
		GccVersion:               metrics["gcc_version"],
		UptimeInSeconds:          metrics["uptime_in_seconds"],
		UptimeInDays:             metrics["uptime_in_days"],
		ConnectedClients:         metrics["connected_clients"],
		TotalConnectionsReceived: metrics["total_connections_received"],
		TotalCommandsProcessed:   metrics["total_commands_processed"],
	}, nil
}

// GetQueueInfo returns Redis queue lengths
func (s *Service) GetQueueInfo() []QueueInfo {
	ctx := context.Background()
	queues := []string{
		mq.OperationQueue,
		mq.InformQueue,
		mq.EventResultQueue,
		mq.AlarmQueue,
		mq.SNMPQueue,
		mq.WebCallbackQueue,
		mq.PMQueue,
		mq.ParameterQueue,
		mq.UpgradeQueue,
		mq.BatchConfigQueue,
		mq.ZTPQueue,
		mq.MMLQueue,
		mq.CoreNetQueue,
		mq.CBSDQueue,
		mq.NorthQueue,
	}

	result := make([]QueueInfo, 0, len(queues))
	for _, name := range queues {
		length := redis.LLen(ctx, name)
		result = append(result, QueueInfo{
			QueueName: name,
			Length:    length,
		})
	}

	return result
}

// ReportHAStatus processes HA component status report
func (s *Service) ReportHAStatus(status HAComponentStatus) error {
	key := fmt.Sprintf("%s:%s", status.Hostname, status.ComponentName)

	// Store in memory cache
	s.haCache.Store(key, status)

	// Persist to file
	s.persistHAStatus()

	logger.Infof("HA status updated: %s = %s", key, status.Status)
	return nil
}

// persistHAStatus saves HA status to file
func (s *Service) persistHAStatus() {
	data := make(map[string]HAComponentStatus)
	s.haCache.Range(func(key, value interface{}) bool {
		if status, ok := value.(HAComponentStatus); ok {
			k := key.(string)
			data[k] = status
		}
		return true
	})

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		logger.Errorf("failed to marshal HA status: %v", err)
		return
	}

	if err := os.WriteFile("/home/ha_status.txt", jsonData, 0644); err != nil {
		logger.Errorf("failed to write HA status file: %v", err)
	}
}
