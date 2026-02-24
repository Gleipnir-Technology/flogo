package main

import (
	"context"
	"sync"
	"time"
)

type debouncedFunc func()
type debouncerFunc func(debouncedFunc)

func newDebounce(ctx context.Context, d time.Duration) debouncerFunc {
	var (
		mu    sync.Mutex
		timer *time.Timer
	)

	return func(f debouncedFunc) {
		mu.Lock()
		defer mu.Unlock()

		// Stop existing timer and drain channel if needed
		if timer != nil {
			timer.Stop()
			select {
			case <-timer.C:
			default:
			}
		}

		// Create/reset timer
		timer = time.AfterFunc(d, func() {
			select {
			case <-ctx.Done():
				return
			default:
				f()
			}
		})
	}
}
