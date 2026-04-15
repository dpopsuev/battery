package testkit

import (
	"context"
	"sync"
	"time"

	"github.com/dpopsuev/battery/cache"
)

// StubCache implements cache.Cache with in-memory storage and call counters.
type StubCache struct {
	mu   sync.Mutex
	data map[string]map[string][]byte // namespace → key → value

	Gets   int
	Sets   int
	Dels   int
	Evicts int
	Err    error // configurable error for all operations
}

var _ cache.Cache = (*StubCache)(nil)

// NewStubCache creates an empty StubCache.
func NewStubCache() *StubCache {
	return &StubCache{data: make(map[string]map[string][]byte)}
}

func (c *StubCache) Get(_ context.Context, namespace, key string) (val []byte, ok bool, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Gets++
	if c.Err != nil {
		return nil, false, c.Err
	}
	ns, ok := c.data[namespace]
	if !ok {
		return nil, false, nil
	}
	v, ok := ns[key]
	if !ok {
		return nil, false, nil
	}
	cp := make([]byte, len(v))
	copy(cp, v)
	return cp, true, nil
}

func (c *StubCache) Set(_ context.Context, namespace, key string, value []byte, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Sets++
	if c.Err != nil {
		return c.Err
	}
	ns, ok := c.data[namespace]
	if !ok {
		ns = make(map[string][]byte)
		c.data[namespace] = ns
	}
	cp := make([]byte, len(value))
	copy(cp, value)
	ns[key] = cp
	return nil
}

func (c *StubCache) Delete(_ context.Context, namespace, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Dels++
	if c.Err != nil {
		return c.Err
	}
	if ns, ok := c.data[namespace]; ok {
		delete(ns, key)
	}
	return nil
}

func (c *StubCache) EvictNamespace(_ context.Context, namespace string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Evicts++
	if c.Err != nil {
		return c.Err
	}
	delete(c.data, namespace)
	return nil
}
