package caching

import (
	"fmt"
	"github.com/patrickmn/go-cache"
	"github.com/rs/zerolog/log"
	"sync/atomic"
	"time"
)

const (
	locked   = 1
	unlocked = 0
)

var (
	DefaultLevel2CacheExpiration  = time.Hour * 6
	DefaultCachePreUpdateDuration = time.Second * 5
)

// RefreshFunc cache refresh function to retrieve the newest value, accepts a key as input which is also the cache key
// NOTE:
//		- Handle function timeout carefully (better finish in no more than 2 seconds)
//		- Avoid returning a nil value while error is nil too
type RefreshFunc func(key string) (interface{}, error)

// FailOverCache implements fail-over caching strategy via two-level cache
type FailOverCache struct {
	l1 *cache.Cache // Level 1 cache
	l2 *cache.Cache // Level 2 cache, might be holding a older copy of cache

	// Enabled when level 1 cache expiration is longer than DefaultCachePreUpdateDuration.
	// When enabled and cache TTL is less than DefaultCachePreUpdateDuration, cache will be refreshed
	enablePreRefresh bool
	lock             int64 // Cache pre-refresh atomic lock
}

// NewFailOverCache instantiates a fail-over cache
// NOTE:
//		- exp1: level 1 cache expiration, e.g. 5 minutes
//		- exp2: level 2 cache expiration, DefaultLevel2CacheExpiration is recommended
func NewFailOverCache(exp1, exp2 time.Duration) *FailOverCache {
	c := &FailOverCache{
		l1:               cache.New(exp1, time.Minute),
		l2:               cache.New(exp2, time.Minute),
		enablePreRefresh: false,
		lock:             unlocked,
	}
	if exp1.Seconds() > float64(DefaultCachePreUpdateDuration/time.Second) {
		c.enablePreRefresh = true
	}
	return c
}

// Get tries to get cached key, calls fn when cached needs to be updated.
// NOTE:
//		- key: cache key
//		- fn: business code to get the newest value of key, e.g. issuing an API call
func (c *FailOverCache) Get(key string, fn RefreshFunc) (v interface{}, err error) {
	// 1. Try level 1 cache
	cached, exp, hit := c.l1.GetWithExpiration(key)
	if hit {
		// 1.1 If hit, check whether it is possible to update the cache before expiration
		update := c.enablePreRefresh && exp.After(time.Now()) &&
			exp.Sub(time.Now()) < DefaultCachePreUpdateDuration
		if update && c.tryLock() {
			go func() {
				defer c.unlock()
				if err = c.refreshCache(key, fn); err == nil {
					log.Trace().Str("key", key).Msg("updated cache before expiration")
				}
			}()
		}

		// 1.2 Return the cached value
		return cached, nil
	}

	// 2. Cache miss, refresh the cache by calling fn
	if err = c.refreshCache(key, fn); err != nil {
		// 2.1 Refreshing cache failed, return level 2 cache as a fallback
		log.Warn().Str("key", key).Err(err).Msg("error refreshing cache, using fail over cache")
		cached, hit = c.l2.Get(key)
	} else {
		// 2.2 Successfully refreshed the cache
		cached, hit = c.l1.Get(key)
	}

	if hit {
		return cached, nil
	} else {
		return nil, fmt.Errorf("error refreshing cache: %w", err)
	}
}

// Remove removes both level 1 & level 2 cache
func (c *FailOverCache) Remove(key string) {
	c.l1.Delete(key)
	c.l2.Delete(key)
}

// refreshCache calls fn to acquire the newest value of key, cache it in level 1 & 2 cache
func (c *FailOverCache) refreshCache(key string, fn RefreshFunc) error {
	v, err := fn(key)
	if err != nil {
		log.Err(err).Str("key", key).Msg("failed to update cache")
		return err
	}

	c.l1.Set(key, v, 0)
	c.l2.Set(key, v, 0)
	return nil
}

// tryLock tries to acquire atomic lock, returns true if locked, otherwise false
func (c *FailOverCache) tryLock() (ok bool) {
	return atomic.CompareAndSwapInt64(&c.lock, unlocked, locked)
}

// unlock unlocks atomic lock
func (c *FailOverCache) unlock() {
	atomic.StoreInt64(&c.lock, unlocked)
}
