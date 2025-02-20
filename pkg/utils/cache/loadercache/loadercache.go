package loadercache

import (
	"context"
	"sync"
	"time"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/cache"
)

// based on github.com/kittpat1413/go-common/framework/cache/localcache/localcache.go

type (
	Option[K comparable, V any] func(*config[K, V])
	item[T any]                 struct {
		data    T
		expires *time.Time
	}
	loaderFunc[K comparable, V any] func(K) (*V, error)
	config[K comparable, V any]     struct {
		expiration time.Duration
		loader     loaderFunc[K, V]
		l          *log.Logger
	}
	loaderCache[K comparable, V any] struct {
		mutex  sync.Mutex
		items  map[K]item[*V]
		config *config[K, V]
	}
)

func WithExpiration[K comparable, V any](expiration time.Duration) Option[K, V] {
	return func(c *config[K, V]) {
		c.expiration = expiration
	}
}

func WithLoader[K comparable, V any](lf loaderFunc[K, V]) Option[K, V] {
	return func(c *config[K, V]) {
		c.loader = lf
	}
}

func WithLogger[K comparable, V any](arg *log.Logger) Option[K, V] {
	return func(c *config[K, V]) {
		c.l = arg
	}
}

func New[K comparable, V any](opts ...Option[K, V]) cache.Cache[K, V] {
	c := &config[K, V]{
		expiration: 5 * time.Minute,
		l:          log.Default().Named("cache"),
	}
	for _, opt := range opts {
		opt(c)
	}
	return &loaderCache[K, V]{
		mutex:  sync.Mutex{},
		items:  make(map[K]item[*V]),
		config: c,
	}
}

func (c *loaderCache[K, V]) Get(ctx context.Context, key K) (*V, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if cacheItem, ok := c.items[key]; ok {
		if cacheItem.expires != nil && cacheItem.expires.Before(time.Now()) {
			delete(c.items, key)
			return c.load(ctx, key)
		}
		return cacheItem.data, nil
	} else {
		return c.load(ctx, key)
	}
}

func (c *loaderCache[K, V]) load(ctx context.Context, key K) (*V, error) {
	if c.config.loader != nil {
		v, err := c.config.loader(key)
		c.config.l.Debug("loaderCache.load", log.Any("key", key))
		if err != nil {
			c.config.l.Error("error loading entry", log.ErrorField(err))
			return nil, err
		}
		expires := time.Now().Add(c.config.expiration)
		c.items[key] = item[*V]{data: v, expires: &expires}
		return v, nil
	}
	return nil, cache.ErrCacheMiss
}

func (c *loaderCache[K, V]) Invalidate(ctx context.Context, key K) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.config.l.Debug("Invalidate", log.Any("key", key))

	delete(c.items, key)
	c.config.l.Debug("Invalidate", log.Int("remain items", len(c.items)))
	for k, v := range c.items {
		c.config.l.Debug("Invalidate", log.Any("key", k), log.Any("value", v))
	}
}
