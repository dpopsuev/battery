package bus

import "sync"

type typed interface {
	eventType() string
}

func (e MotorEvent) eventType() string  { return e.Type }
func (e SenseEvent) eventType() string  { return e.Type }
func (e SignalEvent) eventType() string { return e.Type }

type subscriber[E any] struct {
	id int
	fn func(E)
}

// InProcessBus is a thread-safe synchronous event bus with type-routed dispatch.
type InProcessBus[E typed] struct {
	mu         sync.RWMutex
	handlers   map[string][]subscriber[E]
	wildcard   []subscriber[E]
	deadLetter func(E)
	nextID     int
}

// NewInProcessBus creates a bus.
func NewInProcessBus[E typed]() *InProcessBus[E] {
	return &InProcessBus[E]{
		handlers: make(map[string][]subscriber[E]),
	}
}

// Subscribe registers a handler for the given event type. Use "*" for wildcard.
func (b *InProcessBus[E]) Subscribe(eventType string, handler func(E)) func() {
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	s := subscriber[E]{id: id, fn: handler}
	if eventType == "*" {
		b.wildcard = append(b.wildcard, s)
	} else {
		b.handlers[eventType] = append(b.handlers[eventType], s)
	}
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if eventType == "*" {
			b.wildcard = removeByID(b.wildcard, id)
		} else {
			b.handlers[eventType] = removeByID(b.handlers[eventType], id)
		}
	}
}

// Publish dispatches an event to matching subscribers synchronously.
func (b *InProcessBus[E]) Publish(event E) {
	b.mu.RLock()
	t := event.eventType()
	handlers := b.handlers[t]
	wildcard := b.wildcard
	dead := b.deadLetter
	b.mu.RUnlock()

	for _, h := range handlers {
		h.fn(event)
	}
	for _, h := range wildcard {
		h.fn(event)
	}

	if len(handlers) == 0 && len(wildcard) == 0 && dead != nil {
		dead(event)
	}
}

// SetDeadLetter sets a handler for events with no subscribers.
func (b *InProcessBus[E]) SetDeadLetter(fn func(E)) {
	b.mu.Lock()
	b.deadLetter = fn
	b.mu.Unlock()
}

func removeByID[E any](subs []subscriber[E], id int) []subscriber[E] {
	for i, s := range subs {
		if s.id == id {
			return append(subs[:i], subs[i+1:]...)
		}
	}
	return subs
}
