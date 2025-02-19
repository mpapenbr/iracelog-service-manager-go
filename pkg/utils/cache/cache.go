package cache

import (
	"context"
	"errors"
)

// based on github.com/kittpat1413/go-common/framework/cache/cache.go

var ErrCacheMiss = errors.New("cache miss")

type Cache[K comparable, V any] interface {
	Get(ctx context.Context, key K) (*V, error)
	// Set(ctx context.Context, key K, value *V)
	Invalidate(ctx context.Context, key K)
	// InvalidateAll(ctx context.Context) error
}
