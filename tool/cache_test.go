package tool_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dpopsuev/battery/tool"
)

// CacheableContract validates any Cacheable implementation.
func CacheableContract(t *testing.T, newCacheable func(keyFn func(json.RawMessage) (string, bool)) tool.Cacheable) {
	t.Helper()

	t.Run("ReturnsCacheKey", func(t *testing.T) {
		c := newCacheable(func(_ json.RawMessage) (string, bool) {
			return "test-key", true
		})
		key, ok := c.CacheKey(context.Background(), json.RawMessage(`{}`))
		if !ok {
			t.Fatal("expected ok=true")
		}
		if key != "test-key" {
			t.Errorf("key = %q, want test-key", key)
		}
	})

	t.Run("ReturnsNotCacheable", func(t *testing.T) {
		c := newCacheable(func(_ json.RawMessage) (string, bool) {
			return "", false
		})
		_, ok := c.CacheKey(context.Background(), json.RawMessage(`{}`))
		if ok {
			t.Error("expected ok=false for uncacheable call")
		}
	})
}

func TestCacheable_OptionalOnTool(t *testing.T) {
	var plainTool tool.Tool = stubPlainTool{}
	if _, ok := plainTool.(tool.Cacheable); ok {
		t.Error("plain Tool should not implement Cacheable")
	}
}
