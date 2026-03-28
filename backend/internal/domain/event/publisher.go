package event

import (
	"context"
	"sync"
)

// Handler is a function that handles an event.
type Handler func(ctx context.Context, event *Event) error

// Publisher defines the interface for event publishing.
type Publisher interface {
	// Publish publishes an event to all subscribers.
	Publish(ctx context.Context, event *Event) error

	// Subscribe subscribes to events of specific types.
	Subscribe(eventTypes []EventType, handler Handler) SubscriptionID

	// SubscribeAll subscribes to all events.
	SubscribeAll(handler Handler) SubscriptionID

	// Unsubscribe removes a subscription.
	Unsubscribe(id SubscriptionID)
}

// SubscriptionID is a unique identifier for a subscription.
type SubscriptionID string

// InMemoryPublisher is an in-memory implementation of Publisher.
type InMemoryPublisher struct {
	mu            sync.RWMutex
	subscriptions map[SubscriptionID]*subscription
	nextID        uint64
}

type subscription struct {
	id         SubscriptionID
	eventTypes []EventType // nil means all events
	handler    Handler
}

// NewInMemoryPublisher creates a new in-memory event publisher.
func NewInMemoryPublisher() *InMemoryPublisher {
	return &InMemoryPublisher{
		subscriptions: make(map[SubscriptionID]*subscription),
	}
}

// Publish publishes an event to all matching subscribers.
func (p *InMemoryPublisher) Publish(ctx context.Context, event *Event) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var wg sync.WaitGroup
	errors := make(chan error, len(p.subscriptions))

	for _, sub := range p.subscriptions {
		if sub.matches(event.Type) {
			wg.Add(1)
			go func(s *subscription) {
				defer wg.Done()
				if err := s.handler(ctx, event); err != nil {
					errors <- err
				}
			}(sub)
		}
	}

	wg.Wait()
	close(errors)

	// Return first error if any
	for err := range errors {
		return err
	}

	return nil
}

// Subscribe subscribes to events of specific types.
func (p *InMemoryPublisher) Subscribe(eventTypes []EventType, handler Handler) SubscriptionID {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.nextID++
	id := SubscriptionID(string(rune(p.nextID)))

	p.subscriptions[id] = &subscription{
		id:         id,
		eventTypes: eventTypes,
		handler:    handler,
	}

	return id
}

// SubscribeAll subscribes to all events.
func (p *InMemoryPublisher) SubscribeAll(handler Handler) SubscriptionID {
	return p.Subscribe(nil, handler)
}

// Unsubscribe removes a subscription.
func (p *InMemoryPublisher) Unsubscribe(id SubscriptionID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.subscriptions, id)
}

func (s *subscription) matches(eventType EventType) bool {
	if s.eventTypes == nil {
		return true // subscribed to all events
	}
	for _, t := range s.eventTypes {
		if t == eventType {
			return true
		}
	}
	return false
}

// BufferedPublisher wraps a Publisher with event buffering.
type BufferedPublisher struct {
	inner      Publisher
	buffer     chan *Event
	bufferSize int
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewBufferedPublisher creates a new buffered event publisher.
func NewBufferedPublisher(inner Publisher, bufferSize int) *BufferedPublisher {
	ctx, cancel := context.WithCancel(context.Background())
	p := &BufferedPublisher{
		inner:      inner,
		buffer:     make(chan *Event, bufferSize),
		bufferSize: bufferSize,
		ctx:        ctx,
		cancel:     cancel,
	}

	p.wg.Add(1)
	go p.processEvents()

	return p
}

// Publish adds an event to the buffer.
func (p *BufferedPublisher) Publish(ctx context.Context, event *Event) error {
	select {
	case p.buffer <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Subscribe delegates to the inner publisher.
func (p *BufferedPublisher) Subscribe(eventTypes []EventType, handler Handler) SubscriptionID {
	return p.inner.Subscribe(eventTypes, handler)
}

// SubscribeAll delegates to the inner publisher.
func (p *BufferedPublisher) SubscribeAll(handler Handler) SubscriptionID {
	return p.inner.SubscribeAll(handler)
}

// Unsubscribe delegates to the inner publisher.
func (p *BufferedPublisher) Unsubscribe(id SubscriptionID) {
	p.inner.Unsubscribe(id)
}

// Close stops the buffered publisher.
func (p *BufferedPublisher) Close() {
	p.cancel()
	p.wg.Wait()
}

func (p *BufferedPublisher) processEvents() {
	defer p.wg.Done()

	for {
		select {
		case event := <-p.buffer:
			_ = p.inner.Publish(p.ctx, event)
		case <-p.ctx.Done():
			// Drain remaining events
			for len(p.buffer) > 0 {
				event := <-p.buffer
				_ = p.inner.Publish(context.Background(), event)
			}
			return
		}
	}
}
