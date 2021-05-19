package main

import (
	"context"

	"github.com/patrickmn/go-cache"
)

type ProcessMarker interface {
	// Acquire 処理権を得られればtrue
	Acquire(ctx context.Context, msgID string) (bool, error)
}

var _ ProcessMarker = (*LocalMarker)(nil)

type LocalMarker struct {
	cache *cache.Cache
}

func (c *LocalMarker) Acquire(ctx context.Context, msgID string) (bool, error) {
	err := c.cache.Add(msgID, struct{}{}, 0)
	return err == nil, nil
}
