package redis

import (
	"context"
	"fmt"
	"time"

	"nmsappsrv/pkg/logger"
	goredis "github.com/go-redis/redis/v8"
)

var RDB *goredis.Client

type Config struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}

func Init(cfg Config) error {
	RDB = goredis.NewClient(&goredis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := RDB.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect redis: %w", err)
	}

	logger.Info("redis connected successfully")
	return nil
}

// --- Queue operations (replacing RabbitMQ) ---

// LPush 左推入队列
func LPush(ctx context.Context, queue string, values ...interface{}) error {
	return RDB.LPush(ctx, queue, values...).Err()
}

// RPop 右弹出队列
func RPop(ctx context.Context, queue string) (string, error) {
	return RDB.RPop(ctx, queue).Result()
}

// BRPop 阻塞右弹出
func BRPop(ctx context.Context, timeout time.Duration, queues ...string) ([]string, error) {
	return RDB.BRPop(ctx, timeout, queues...).Result()
}

// LLen 队列长度
func LLen(ctx context.Context, queue string) int64 {
	return RDB.LLen(ctx, queue).Val()
}

// --- Key operations ---

func Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return RDB.Set(ctx, key, value, expiration).Err()
}

func Get(ctx context.Context, key string) (string, error) {
	return RDB.Get(ctx, key).Result()
}

func Del(ctx context.Context, keys ...string) error {
	return RDB.Del(ctx, keys...).Err()
}

func Exists(ctx context.Context, key string) bool {
	return RDB.Exists(ctx, key).Val() > 0
}

func Expire(ctx context.Context, key string, expiration time.Duration) error {
	return RDB.Expire(ctx, key, expiration).Err()
}

// --- Hash operations ---

func HSet(ctx context.Context, key string, values ...interface{}) error {
	return RDB.HSet(ctx, key, values...).Err()
}

func HGet(ctx context.Context, key, field string) (string, error) {
	return RDB.HGet(ctx, key, field).Result()
}

func HGetAll(ctx context.Context, key string) map[string]string {
	return RDB.HGetAll(ctx, key).Val()
}

func HDel(ctx context.Context, key string, fields ...string) error {
	return RDB.HDel(ctx, key, fields...).Err()
}

// --- Distributed lock ---

// Lock acquires a distributed lock using SET NX with expiration.
// Returns true if the lock was acquired, false if it was already held.
func Lock(ctx context.Context, key string, expiration time.Duration) bool {
	return RDB.SetNX(ctx, key, 1, expiration).Val()
}

// Unlock releases a distributed lock.
func Unlock(ctx context.Context, key string) {
	RDB.Del(ctx, key)
}

// --- Pub/Sub ---

func Publish(ctx context.Context, channel string, message interface{}) error {
	return RDB.Publish(ctx, channel, message).Err()
}

func Subscribe(ctx context.Context, channels ...string) *goredis.PubSub {
	return RDB.Subscribe(ctx, channels...)
}
