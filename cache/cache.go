// Package cache defines the caching interface for Battery.
// Namespace is a first-class concept — MCP tools are namespaced (server.tool)
// and cache entries need server-scoped eviction.
package cache

import (
	"context"
	"errors"
	"time"
)

// ErrCacheFull indicates the cache is at capacity and cannot accept new entries.
var ErrCacheFull = errors.New("battery: cache full")

// Cache is the interface for namespaced key-value caching with TTL support.
type Cache interface {
	// Get retrieves a value. Returns (nil, false, nil) on cache miss.
	Get(ctx context.Context, namespace, key string) ([]byte, bool, error)

	// Set stores a value with a TTL. Zero TTL means use the implementation default
	// or no expiration.
	Set(ctx context.Context, namespace, key string, value []byte, ttl time.Duration) error

	// Delete removes a single entry. No error if the key does not exist.
	Delete(ctx context.Context, namespace, key string) error

	// EvictNamespace removes all entries in the given namespace.
	EvictNamespace(ctx context.Context, namespace string) error
}
