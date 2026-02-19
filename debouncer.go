package main

import (
	"context"
	"time"
)

type debouncer struct {
}
type debouncedFunc func()
type debouncerFunc func(debouncedFunc)

func newDebounce(ctx context.Context, d time.Duration) debouncerFunc {
	var timer *time.Timer
	return func(f debouncedFunc) {
		if timer == nil {
			// If we don't have a timer, create one and start waiting for more signal
			timer = time.NewTimer(d)
		} else {
			// Otherwise we are already waiting, so this is a 'bounce'
			return
		}
		go func() {
			select {
			case <-timer.C:
				timer = nil
				go f()
			case <-ctx.Done():
				return
			}
		}()
	}
}
