package main

import (
	"context"

	"github.com/redis/go-redis/v9"
)

var _ Counter = (*RedisCounter)(nil)

type RedisCounter struct {
	key    string
	client redis.UniversalClient
}

func (c *RedisCounter) Get(ctx context.Context) (int64, error) {
	return c.client.Get(ctx, c.key).Int64()
}

func (c *RedisCounter) Up(ctx context.Context) (int64, error) {
	return c.client.IncrBy(ctx, c.key, 1).Result()
}
