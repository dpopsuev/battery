package cache_test

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/dpopsuev/battery/cache"
)

// CacheContract validates any Cache implementation.
func CacheContract(t *testing.T, newCache func() cache.Cache) {
	t.Helper()

	t.Run("SetGetRoundTrip", func(t *testing.T) {
		c := newCache()
		ctx := context.Background()

		if err := c.Set(ctx, "ns", "key1", []byte("value1"), 0); err != nil {
			t.Fatalf("Set: %v", err)
		}
		got, ok, err := c.Get(ctx, "ns", "key1")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !ok {
			t.Fatal("expected cache hit")
		}
		if !bytes.Equal(got, []byte("value1")) {
			t.Errorf("got %q, want %q", got, "value1")
		}
	})

	t.Run("GetMiss", func(t *testing.T) {
		c := newCache()
		ctx := context.Background()

		_, ok, err := c.Get(ctx, "ns", "missing")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if ok {
			t.Error("expected cache miss")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		c := newCache()
		ctx := context.Background()

		if err := c.Set(ctx, "ns", "key1", []byte("val"), 0); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if err := c.Delete(ctx, "ns", "key1"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, ok, err := c.Get(ctx, "ns", "key1")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if ok {
			t.Error("expected miss after delete")
		}
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		c := newCache()
		ctx := context.Background()

		if err := c.Delete(ctx, "ns", "nope"); err != nil {
			t.Fatalf("Delete non-existent: %v", err)
		}
	})

	t.Run("EvictNamespace", func(t *testing.T) {
		c := newCache()
		ctx := context.Background()

		_ = c.Set(ctx, "ns1", "a", []byte("1"), 0)
		_ = c.Set(ctx, "ns1", "b", []byte("2"), 0)
		_ = c.Set(ctx, "ns2", "a", []byte("3"), 0)

		if err := c.EvictNamespace(ctx, "ns1"); err != nil {
			t.Fatalf("EvictNamespace: %v", err)
		}

		// ns1 entries gone.
		_, ok, _ := c.Get(ctx, "ns1", "a")
		if ok {
			t.Error("ns1.a should be evicted")
		}
		_, ok, _ = c.Get(ctx, "ns1", "b")
		if ok {
			t.Error("ns1.b should be evicted")
		}

		// ns2 untouched.
		_, ok, _ = c.Get(ctx, "ns2", "a")
		if !ok {
			t.Error("ns2.a should still exist")
		}
	})

	t.Run("Overwrite", func(t *testing.T) {
		c := newCache()
		ctx := context.Background()

		_ = c.Set(ctx, "ns", "key", []byte("v1"), 0)
		_ = c.Set(ctx, "ns", "key", []byte("v2"), 0)

		got, ok, _ := c.Get(ctx, "ns", "key")
		if !ok {
			t.Fatal("expected hit")
		}
		if !bytes.Equal(got, []byte("v2")) {
			t.Errorf("got %q, want %q", got, "v2")
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		c := newCache()
		ctx := context.Background()
		var wg sync.WaitGroup

		for i := range 20 {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				key := string(rune('a' + i%10))
				_ = c.Set(ctx, "ns", key, []byte("val"), 0)
				_, _, _ = c.Get(ctx, "ns", key)
			}(i)
		}
		wg.Wait()
	})
}
