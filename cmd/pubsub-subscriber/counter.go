package main

import (
	"context"
	"sync/atomic"
)

type Counter interface {
	Up(ctx context.Context) (int64, error)
	Get(ctx context.Context) (int64, error)
}

var _ Counter = (*LocalCounter)(nil)

type LocalCounter struct {
	count int64
}

func (c *LocalCounter) Get(ctx context.Context) (int64, error) {
	return atomic.AddInt64(&c.count, 0), nil
}

func (c *LocalCounter) Up(ctx context.Context) (int64, error) {
	return atomic.AddInt64(&c.count, 1), nil
}
