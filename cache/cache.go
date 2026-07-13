package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrNotFound 表示 key 不存在，上層不需要 import redis 就能判斷。
var ErrNotFound = errors.New("cache: key not found")

// Cache 抽象快取操作，方便 mock 測試或更換實作。
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	// Incr 將 key 的整數值 +1 並回傳新值，第一次建立時設定 ttl。
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	// Ping 檢查連線是否正常，供啟動檢查與 readiness 探針使用。
	Ping(ctx context.Context) error
}

// RedisCache 為 Cache 的 Redis 實作。
type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(addr string) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &RedisCache{client: client}
}

// Ping 檢查 Redis 連線是否正常。
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	return val, err
}

func (c *RedisCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *RedisCache) Del(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

func (c *RedisCache) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	val, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if val == 1 {
		if err := c.client.Expire(ctx, key, ttl).Err(); err != nil {
			return val, err
		}
	}
	return val, nil
}
