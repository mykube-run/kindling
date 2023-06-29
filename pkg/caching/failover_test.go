package caching

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"testing"
	"time"
)

var (
	key             = "default"
	value           = "OK"
	refreshed int64 = 0
	fn1             = func(key string) (interface{}, error) {
		refreshed = time.Now().Unix()
		log.Info().Msg("refresh function called")
		time.Sleep(time.Second)
		return value, nil
	}
	fn2 = func(key string) (interface{}, error) {
		refreshed = time.Now().Unix()
		log.Info().Msg("refresh function called")
		time.Sleep(time.Second)
		return nil, fmt.Errorf("designed error")
	}
)

func TestFailOverCache_Get(t *testing.T) {
	cache := NewFailOverCache(time.Second*6, DefaultLevel2CacheExpiration)
	fmt.Println("refreshed:", refreshed)

	// 1. First call, key was not cached
	{
		v, err := cache.Get(key, fn1)
		if err != nil {
			t.Fatalf("expecting nil error, got %v", err)
		}
		s, ok := v.(string)
		if !ok {
			t.Fatalf("expecting string value")
		}
		if s != value {
			t.Fatalf("expecting value == %v, got %v", value, s)
		}

		if refreshed == 0 {
			t.Fatalf("refresh function should be called")
		}
		_, hit := cache.l1.Get(key)
		if !hit {
			t.Fatalf("expecting value was cached in level 1 cache")
		}
		_, hit = cache.l2.Get(key)
		if !hit {
			t.Fatalf("expecting value was cached in level 2 cache")
		}
	}

	// 2. Second call, key should be cached
	{
		lastRefresh := refreshed

		v, err := cache.Get(key, fn1)
		if err != nil {
			t.Fatalf("expecting nil error, got %v", err)
		}
		s, ok := v.(string)
		if !ok {
			t.Fatalf("expecting string value")
		}
		if s != value {
			t.Fatalf("expecting value == %v, got %v", value, s)
		}

		if lastRefresh != refreshed {
			t.Fatalf("refresh function should not be called")
		}
		_, hit := cache.l1.Get(key)
		if !hit {
			t.Fatalf("expecting value was cached in level 1 cache")
		}
		_, hit = cache.l2.Get(key)
		if !hit {
			t.Fatalf("expecting value was cached in level 2 cache")
		}
	}

	// 3. Third call, key was cached and refresh function should be called
	time.Sleep(time.Second * 3)
	{
		lastRefresh := refreshed

		v, err := cache.Get(key, fn1)
		if err != nil {
			t.Fatalf("expecting nil error, got %v", err)
		}
		s, ok := v.(string)
		if !ok {
			t.Fatalf("expecting string value")
		}
		if s != value {
			t.Fatalf("expecting value == %v, got %v", value, s)
		}

		time.Sleep(time.Second * 2) // Wait for the goroutine finish
		if lastRefresh == refreshed {
			t.Fatalf("refresh function should be called again")
		}
		_, hit := cache.l1.Get(key)
		if !hit {
			t.Fatalf("expecting value was cached in level 1 cache")
		}
		_, hit = cache.l2.Get(key)
		if !hit {
			t.Fatalf("expecting value was cached in level 2 cache")
		}
	}

	// 4. Fourth call, key was expired and refresh function should be called
	time.Sleep(time.Second * 5)
	{
		lastRefresh := refreshed

		_, hit := cache.l1.Get(key)
		if hit {
			t.Fatalf("expecting value in level 1 cache was expired")
		}
		_, hit = cache.l2.Get(key)
		if !hit {
			t.Fatalf("expecting value was cached in level 2 cache")
		}

		v, err := cache.Get(key, fn1)
		if err != nil {
			t.Fatalf("expecting nil error, got %v", err)
		}
		s, ok := v.(string)
		if !ok {
			t.Fatalf("expecting string value")
		}
		if s != value {
			t.Fatalf("expecting value == %v, got %v", value, s)
		}

		if lastRefresh == refreshed {
			t.Fatalf("refresh function should be called")
		}
		_, hit = cache.l1.Get(key)
		if !hit {
			t.Fatalf("expecting value was cached in level 1 cache")
		}
		_, hit = cache.l2.Get(key)
		if !hit {
			t.Fatalf("expecting value was cached in level 2 cache")
		}
	}

	// 5. Fifth call, key was expired and refresh function should be called
	// This time fn2 is provided, thus level 2 cache should be used
	time.Sleep(time.Second * 10)
	{
		lastRefresh := refreshed

		_, hit := cache.l1.Get(key)
		if hit {
			t.Fatalf("expecting value in level 1 cache was expired")
		}
		_, hit = cache.l2.Get(key)
		if !hit {
			t.Fatalf("expecting value was cached in level 2 cache")
		}

		v, err := cache.Get(key, fn2)
		if err != nil {
			t.Fatalf("expecting nil error, got %v", err)
		}
		s, ok := v.(string)
		if !ok {
			t.Fatalf("expecting string value")
		}
		if s != value {
			t.Fatalf("expecting value == %v, got %v", value, s)
		}

		if lastRefresh == refreshed {
			t.Fatalf("refresh function should be called")
		}
		_, hit = cache.l1.Get(key)
		if hit {
			t.Fatalf("expecting value in level 1 cache was expired")
		}
		_, hit = cache.l2.Get(key)
		if !hit {
			t.Fatalf("expecting value was cached in level 2 cache")
		}
	}
}
