package process

import (
	"sync"

	"github.com/rs/zerolog/log"
)

type SubscriptionManager[T any] struct {
	mu          sync.Mutex
	subscribers map[*Subscription[T]]struct{}
}

func NewSubscriptionManager[T any]() *SubscriptionManager[T] {
	return &SubscriptionManager[T]{
		mu:          sync.Mutex{},
		subscribers: make(map[*Subscription[T]]struct{}, 0),
	}
}
func (m *SubscriptionManager[T]) CloseSubscription(s *Subscription[T]) {
	m.mu.Lock()
	delete(m.subscribers, s)
	m.mu.Unlock()
}
func (m *SubscriptionManager[T]) Publish(t T) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for sub := range m.subscribers {
		select {
		case sub.C <- t:
		default:
			log.Warn().Msg("failed to publish to subscription")
			// Channel full, drop event or handle differently
		}
	}
}
func (m *SubscriptionManager[T]) Subscribe() *Subscription[T] {
	sub := &Subscription[T]{
		C:       make(chan T, 10), // buffered so we can start the process immediately and then handle events
		manager: m,
	}
	m.mu.Lock()
	m.subscribers[sub] = struct{}{}
	m.mu.Unlock()
	return sub
}
