package cache

import (
	"context"
	"math/rand"
	"time"

	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
)

type Cache interface {
	Once(ctx context.Context, key string, ttl time.Duration, dst any, do func() (any, error)) error
	Delete(ctx context.Context, keys ...string) error
}

type RedisCache struct {
	cache *cache.Cache
}

func New(client redis.UniversalClient) *RedisCache {
	return &RedisCache{
		cache: cache.New(&cache.Options{
			Redis: client,
			// 在 redis 的基础上再加入内存缓存，允许 1000 个数据，过期时间十分钟
			LocalCache: cache.NewTinyLFU(1000, 10*time.Minute),
		}),
	}
}

// 如果缓存存在则直接写入dst，否则执行do函数并缓存结果然后写入
func (c *RedisCache) Once(ctx context.Context, key string, ttl time.Duration, dst any, do func() (any, error)) error {
	return c.cache.Once(&cache.Item{
		Ctx:   ctx,
		Key:   key,
		Value: dst,
		TTL:   withJitter(ttl),
		Do: func(*cache.Item) (any, error) {
			return do()
		},
	})
}

// Delete 删除缓存
func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if err := c.cache.Delete(ctx, key); err != nil && err != cache.ErrCacheMiss {
			return err
		}
	}
	return nil
}

// 带随机抖动的 TTL，避免大量缓存同时过期
func withJitter(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return ttl
	}
	return ttl - time.Duration(rand.Int63n(int64(ttl/5)))
}
