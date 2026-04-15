package cache_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/dpopsuev/battery/cache"
)

func TestMemCache_Contract(t *testing.T) {
	CacheContract(t, func() cache.Cache {
		return cache.NewMemCache(cache.MemCacheConfig{MaxEntries: 1000})
	})
}

func TestMemCache_LRUEviction(t *testing.T) {
	c := cache.NewMemCache(cache.MemCacheConfig{MaxEntries: 3})
	ctx := context.Background()

	_ = c.Set(ctx, "ns", "a", []byte("1"), 0)
	_ = c.Set(ctx, "ns", "b", []byte("2"), 0)
	_ = c.Set(ctx, "ns", "c", []byte("3"), 0)

	// "a" is LRU. Adding "d" should evict "a".
	_ = c.Set(ctx, "ns", "d", []byte("4"), 0)

	_, ok, _ := c.Get(ctx, "ns", "a")
	if ok {
		t.Error("expected 'a' to be evicted")
	}

	// b, c, d should still exist.
	for _, k := range []string{"b", "c", "d"} {
		_, ok, _ := c.Get(ctx, "ns", k)
		if !ok {
			t.Errorf("expected %q to exist", k)
		}
	}
}

func TestMemCache_LRUAccessPromotes(t *testing.T) {
	c := cache.NewMemCache(cache.MemCacheConfig{MaxEntries: 3})
	ctx := context.Background()

	_ = c.Set(ctx, "ns", "a", []byte("1"), 0)
	_ = c.Set(ctx, "ns", "b", []byte("2"), 0)
	_ = c.Set(ctx, "ns", "c", []byte("3"), 0)

	// Access "a" to promote it. Now "b" is LRU.
	_, _, _ = c.Get(ctx, "ns", "a")

	_ = c.Set(ctx, "ns", "d", []byte("4"), 0)

	_, ok, _ := c.Get(ctx, "ns", "b")
	if ok {
		t.Error("expected 'b' to be evicted (was LRU after 'a' was accessed)")
	}
	_, ok, _ = c.Get(ctx, "ns", "a")
	if !ok {
		t.Error("expected 'a' to survive (was promoted by Get)")
	}
}

func TestMemCache_TTLExpiry(t *testing.T) {
	c := cache.NewMemCache(cache.MemCacheConfig{MaxEntries: 100})
	ctx := context.Background()

	_ = c.Set(ctx, "ns", "short", []byte("expires"), 1*time.Millisecond)
	_ = c.Set(ctx, "ns", "long", []byte("stays"), 1*time.Hour)

	time.Sleep(5 * time.Millisecond)

	_, ok, _ := c.Get(ctx, "ns", "short")
	if ok {
		t.Error("expected 'short' to expire")
	}
	_, ok, _ = c.Get(ctx, "ns", "long")
	if !ok {
		t.Error("expected 'long' to still exist")
	}
}

func TestMemCache_DefaultTTL(t *testing.T) {
	c := cache.NewMemCache(cache.MemCacheConfig{MaxEntries: 100, DefaultTTL: 1 * time.Millisecond})
	ctx := context.Background()

	_ = c.Set(ctx, "ns", "k", []byte("v"), 0) // uses default TTL

	time.Sleep(5 * time.Millisecond)

	_, ok, _ := c.Get(ctx, "ns", "k")
	if ok {
		t.Error("expected entry to expire with default TTL")
	}
}

func TestMemCache_Capacity(t *testing.T) {
	c := cache.NewMemCache(cache.MemCacheConfig{MaxEntries: 5})
	ctx := context.Background()

	for i := range 10 {
		_ = c.Set(ctx, "ns", strconv.Itoa(i), []byte("v"), 0)
	}

	// Only the last 5 should survive.
	count := 0
	for i := range 10 {
		_, ok, _ := c.Get(ctx, "ns", strconv.Itoa(i))
		if ok {
			count++
		}
	}
	if count != 5 {
		t.Errorf("expected 5 entries, got %d", count)
	}
}

func TestMemCache_UnlimitedCapacity(t *testing.T) {
	c := cache.NewMemCache(cache.MemCacheConfig{}) // MaxEntries=0 means unlimited
	ctx := context.Background()

	for i := range 100 {
		_ = c.Set(ctx, "ns", strconv.Itoa(i), []byte("v"), 0)
	}

	for i := range 100 {
		_, ok, _ := c.Get(ctx, "ns", strconv.Itoa(i))
		if !ok {
			t.Errorf("expected entry %d to exist with unlimited capacity", i)
		}
	}
}

func TestMemCache_OverwriteDoesNotChangeCount(t *testing.T) {
	c := cache.NewMemCache(cache.MemCacheConfig{MaxEntries: 2})
	ctx := context.Background()

	_ = c.Set(ctx, "ns", "a", []byte("v1"), 0)
	_ = c.Set(ctx, "ns", "b", []byte("v2"), 0)
	_ = c.Set(ctx, "ns", "a", []byte("v3"), 0) // overwrite, not a new entry

	// Should still have room — no eviction of "b".
	_, ok, _ := c.Get(ctx, "ns", "b")
	if !ok {
		t.Error("expected 'b' to exist — overwrite should not increase count")
	}
}
