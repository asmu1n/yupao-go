package cache

import (
	"context"
	"time"
)

func TryFetch[T any](ctx context.Context, c Cache, key string, ttl time.Duration, do func() (T, error)) (T, error) {
	if c == nil {
		return do()
	}

	var dst T

	err := c.Once(ctx, key, ttl, &dst, func() (any, error) {
		return do()
	})

	return dst, err
}
