package util

import (
	"context"
	"sync"
	"time"
)

type DelayWorker[T any] struct {
	notifyC chan T
}

func (s *DelayWorker[T]) Notify(t T) {
	s.notifyC <- t
}

func NewDelayWorker[T any](wg *sync.WaitGroup, ctx context.Context, delayInterval time.Duration, fn func(value T, t time.Time)) *DelayWorker[T] {
	d := &DelayWorker[T]{}
	if wg != nil {
		wg.Add(1)
	}
	go d.delaying(wg, ctx, delayInterval, fn)

	return d
}

func (s *DelayWorker[T]) delaying(wg *sync.WaitGroup, ctx context.Context, interval time.Duration, fn func(value T, t time.Time)) {
	s.notifyC = make(chan T)
	idleTimeout := time.NewTimer(interval)
	defer idleTimeout.Stop()
	dirty := false

	if wg != nil {
		defer wg.Done()
	}

	var updateTime time.Time
	var value T

	for {
		idleTimeout.Reset(interval)

		select {
		case <-ctx.Done():
			return
		case value = <-s.notifyC:
			updateTime = time.Now()
			dirty = true
		case <-idleTimeout.C:
			if dirty {
				fn(value, updateTime)
				dirty = false
			}
		}
	}
}
