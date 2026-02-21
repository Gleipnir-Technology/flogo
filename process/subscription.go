package process

import (
	"sync"
)

type Subscription[T any] struct {
	C       chan T
	closer  sync.Once
	id      int
	manager *SubscriptionManager[T]
}

func (s *Subscription[T]) Close() {
	s.closer.Do(func() {
		s.manager.CloseSubscription(s)
		close(s.C)
	})
}
