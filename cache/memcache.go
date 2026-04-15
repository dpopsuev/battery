package cache

import (
	"context"
	"sync"
	"time"
)

// MemCacheConfig configures a MemCache instance.
type MemCacheConfig struct {
	MaxEntries int           // max total entries across all namespaces; 0 = unlimited
	DefaultTTL time.Duration // default TTL when Set is called with ttl=0; 0 = no expiry
}

// entry is one cached value with TTL metadata and LRU pointers.
type entry struct {
	namespace string
	key       string
	value     []byte
	expiresAt time.Time // zero value means no expiry

	// Doubly-linked list for LRU eviction.
	prev, next *entry
}

func (e *entry) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && now.After(e.expiresAt)
}

// MemCache is an in-memory Cache with LRU eviction and TTL support.
type MemCache struct {
	mu         sync.Mutex
	maxEntries int
	defaultTTL time.Duration
	data       map[string]map[string]*entry // namespace → key → entry
	count      int

	// LRU doubly-linked list. head is most recently used, tail is least.
	head, tail *entry
}

var _ Cache = (*MemCache)(nil)

// NewMemCache creates a new MemCache with the given configuration.
func NewMemCache(cfg MemCacheConfig) *MemCache {
	return &MemCache{
		maxEntries: cfg.MaxEntries,
		defaultTTL: cfg.DefaultTTL,
		data:       make(map[string]map[string]*entry),
	}
}

func (c *MemCache) Get(_ context.Context, namespace, key string) (val []byte, ok bool, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ns, exists := c.data[namespace]
	if !exists {
		return nil, false, nil
	}
	e, exists := ns[key]
	if !exists {
		return nil, false, nil
	}
	if e.expired(time.Now()) {
		c.removeEntry(e)
		return nil, false, nil
	}
	c.moveToFront(e)
	cp := make([]byte, len(e.value))
	copy(cp, e.value)
	return cp, true, nil
}

func (c *MemCache) Set(_ context.Context, namespace, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl == 0 {
		ttl = c.defaultTTL
	}

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	cp := make([]byte, len(value))
	copy(cp, value)

	// Update existing entry.
	if ns, ok := c.data[namespace]; ok {
		if e, ok := ns[key]; ok {
			e.value = cp
			e.expiresAt = expiresAt
			c.moveToFront(e)
			return nil
		}
	}

	// Evict LRU if at capacity.
	if c.maxEntries > 0 && c.count >= c.maxEntries {
		c.evictLRU()
	}

	e := &entry{
		namespace: namespace,
		key:       key,
		value:     cp,
		expiresAt: expiresAt,
	}

	ns, ok := c.data[namespace]
	if !ok {
		ns = make(map[string]*entry)
		c.data[namespace] = ns
	}
	ns[key] = e
	c.count++
	c.pushFront(e)
	return nil
}

func (c *MemCache) Delete(_ context.Context, namespace, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ns, ok := c.data[namespace]
	if !ok {
		return nil
	}
	e, ok := ns[key]
	if !ok {
		return nil
	}
	c.removeEntry(e)
	return nil
}

func (c *MemCache) EvictNamespace(_ context.Context, namespace string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ns, ok := c.data[namespace]
	if !ok {
		return nil
	}
	for _, e := range ns {
		c.unlinkLRU(e)
		c.count--
	}
	delete(c.data, namespace)
	return nil
}

// removeEntry removes an entry from both the namespace map and the LRU list.
func (c *MemCache) removeEntry(e *entry) {
	if ns, ok := c.data[e.namespace]; ok {
		delete(ns, e.key)
		if len(ns) == 0 {
			delete(c.data, e.namespace)
		}
	}
	c.unlinkLRU(e)
	c.count--
}

// evictLRU removes the least recently used entry (tail of the list).
func (c *MemCache) evictLRU() {
	if c.tail == nil {
		return
	}
	c.removeEntry(c.tail)
}

// pushFront adds an entry to the front of the LRU list.
func (c *MemCache) pushFront(e *entry) {
	e.prev = nil
	e.next = c.head
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
	if c.tail == nil {
		c.tail = e
	}
}

// moveToFront moves an existing entry to the front of the LRU list.
func (c *MemCache) moveToFront(e *entry) {
	if c.head == e {
		return
	}
	c.unlinkLRU(e)
	c.pushFront(e)
}

// unlinkLRU removes an entry from the LRU list without touching the data map.
func (c *MemCache) unlinkLRU(e *entry) {
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		c.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		c.tail = e.prev
	}
	e.prev = nil
	e.next = nil
}
