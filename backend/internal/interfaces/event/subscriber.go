// Package event provides event subscription and streaming.
package event

import (
	"context"
	"sync"

	"github.com/rs/zerolog"

	domainEvent "github.com/dacrypt/cortex/backend/internal/domain/event"
)

// Subscriber manages event subscriptions.
type Subscriber struct {
	publisher      domainEvent.Publisher
	publisherSubID domainEvent.SubscriptionID
	subscribers    map[string]*Subscription
	logger         zerolog.Logger
	mu             sync.RWMutex
}

// Subscription represents an active subscription.
type Subscription struct {
	ID         string
	EventTypes []domainEvent.EventType
	Channel    chan domainEvent.Event
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewSubscriber creates a new event subscriber.
func NewSubscriber(publisher domainEvent.Publisher, logger zerolog.Logger) *Subscriber {
	s := &Subscriber{
		publisher:   publisher,
		subscribers: make(map[string]*Subscription),
		logger:      logger.With().Str("component", "event_subscriber").Logger(),
	}

	// Subscribe to all events from publisher and forward to subscribers.
	if publisher != nil {
		s.publisherSubID = publisher.SubscribeAll(func(ctx context.Context, evt *domainEvent.Event) error {
			s.Dispatch(*evt)
			return nil
		})
	}

	return s
}

// Subscribe creates a new subscription.
func (s *Subscriber) Subscribe(ctx context.Context, eventTypes []domainEvent.EventType) *Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()

	subCtx, cancel := context.WithCancel(ctx)
	sub := &Subscription{
		ID:         generateSubID(),
		EventTypes: eventTypes,
		Channel:    make(chan domainEvent.Event, 100),
		ctx:        subCtx,
		cancel:     cancel,
	}

	s.subscribers[sub.ID] = sub

	s.logger.Debug().
		Str("subscription_id", sub.ID).
		Int("event_types", len(eventTypes)).
		Msg("New subscription created")

	// Handle context cancellation
	go func() {
		<-subCtx.Done()
		s.Unsubscribe(sub.ID)
	}()

	return sub
}

// Unsubscribe removes a subscription.
func (s *Subscriber) Unsubscribe(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sub, exists := s.subscribers[id]; exists {
		sub.cancel()
		close(sub.Channel)
		delete(s.subscribers, id)

		s.logger.Debug().
			Str("subscription_id", id).
			Msg("Subscription removed")
	}
}

// Dispatch sends an event to all matching subscribers.
func (s *Subscriber) Dispatch(evt domainEvent.Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, sub := range s.subscribers {
		if s.matchesSubscription(evt, sub) {
			select {
			case sub.Channel <- evt:
			default:
				s.logger.Warn().
					Str("subscription_id", sub.ID).
					Str("event_type", string(evt.Type)).
					Msg("Subscription channel full, dropping event")
			}
		}
	}
}

// matchesSubscription checks if an event matches a subscription.
func (s *Subscriber) matchesSubscription(evt domainEvent.Event, sub *Subscription) bool {
	if len(sub.EventTypes) == 0 {
		return true // Subscribe to all events
	}

	for _, t := range sub.EventTypes {
		if t == evt.Type {
			return true
		}
	}
	return false
}

// SubscribeToFiles creates a subscription for file events.
func (s *Subscriber) SubscribeToFiles(ctx context.Context) *Subscription {
	return s.Subscribe(ctx, []domainEvent.EventType{
		domainEvent.EventFileCreated,
		domainEvent.EventFileModified,
		domainEvent.EventFileDeleted,
	})
}

// SubscribeToMetadata creates a subscription for metadata events.
func (s *Subscriber) SubscribeToMetadata(ctx context.Context) *Subscription {
	return s.Subscribe(ctx, []domainEvent.EventType{
		domainEvent.EventTagAdded,
		domainEvent.EventTagRemoved,
		domainEvent.EventContextAdded,
		domainEvent.EventContextRemoved,
	})
}

// SubscribeToTasks creates a subscription for task events.
func (s *Subscriber) SubscribeToTasks(ctx context.Context) *Subscription {
	return s.Subscribe(ctx, []domainEvent.EventType{
		domainEvent.EventTaskCreated,
		domainEvent.EventTaskStarted,
		domainEvent.EventTaskCompleted,
		domainEvent.EventTaskFailed,
	})
}

// SubscribeToScans creates a subscription for scan events.
func (s *Subscriber) SubscribeToScans(ctx context.Context) *Subscription {
	return s.Subscribe(ctx, []domainEvent.EventType{
		domainEvent.EventScanStarted,
		domainEvent.EventScanCompleted,
	})
}

// SubscribeToPipeline creates a subscription for pipeline events.
func (s *Subscriber) SubscribeToPipeline(ctx context.Context) *Subscription {
	return s.Subscribe(ctx, []domainEvent.EventType{
		domainEvent.EventPipelineStarted,
		domainEvent.EventPipelineProgress,
		domainEvent.EventPipelineCompleted,
		domainEvent.EventPipelineFailed,
	})
}

// Close closes all subscriptions.
func (s *Subscriber) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.publisher != nil && s.publisherSubID != "" {
		s.publisher.Unsubscribe(s.publisherSubID)
		s.publisherSubID = ""
	}

	for id, sub := range s.subscribers {
		sub.cancel()
		close(sub.Channel)
		delete(s.subscribers, id)
	}

	s.logger.Info().Msg("All subscriptions closed")
}

var subCounter int
var subMu sync.Mutex

func generateSubID() string {
	subMu.Lock()
	defer subMu.Unlock()
	subCounter++
	return string(rune('A'+subCounter%26)) + string(rune('0'+subCounter/26))
}
