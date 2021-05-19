package main

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

var _ ProcessMarker = (*RedisMarker)(nil)

type RedisMarker struct {
	client redis.UniversalClient
}

func (c *RedisMarker) Acquire(ctx context.Context, msgID string) (bool, error) {
	return c.client.SetNX(ctx, "subscriber-processed-check:"+msgID, "v", time.Second*60).Result()
}
