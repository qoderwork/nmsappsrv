package captcha

import (
	"context"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

// redisStore implements base64Captcha.Store over Redis so captcha answers
// survive restarts and are shared across instances (matching the Java scheme
// where the answer lives in Redis under captcha_code_<key>).
type redisStore struct {
	rdb *goredis.Client
	ttl time.Duration
}

func (s *redisStore) Set(id string, value string) error {
	return s.rdb.Set(context.Background(), captchaKeyPrefix+id, value, s.ttl).Err()
}

func (s *redisStore) Get(id string, clear bool) string {
	key := captchaKeyPrefix + id
	val, err := s.rdb.Get(context.Background(), key).Result()
	if err != nil {
		return ""
	}
	if clear {
		_ = s.rdb.Del(context.Background(), key).Err()
	}
	return val
}

func (s *redisStore) Verify(id, answer string, clear bool) bool {
	if id == "" || answer == "" {
		return false
	}
	return s.Get(id, clear) == answer
}
