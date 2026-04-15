package tool

import (
	"context"
	"encoding/json"
)

// Cacheable is an optional interface for tools that support result caching.
// The tool owns key derivation — only the tool knows what state it depends on.
// Returns ok=false to indicate this call should not be cached.
//
//	if c, ok := t.(Cacheable); ok {
//	    key, ok := c.CacheKey(ctx, input)
//	}
type Cacheable interface {
	CacheKey(ctx context.Context, input json.RawMessage) (key string, ok bool)
}
