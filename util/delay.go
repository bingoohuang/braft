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

func NewDelayWorker[T any](ctx context.Context, wg *sync.WaitGroup, delayInterval time.Duration, fn func(value T, t time.Time)) *DelayWorker[T] {
	d := &DelayWorker[T]{}
	wg.Add(1)
	go d.delaying(ctx, wg, delayInterval, fn)

	return d
}

func (s *DelayWorker[T]) delaying(ctx context.Context, wg *sync.WaitGroup, interval time.Duration, fn func(value T, t time.Time)) {
	s.notifyC = make(chan T)
	idleTimeout := time.NewTimer(interval)
	defer idleTimeout.Stop()
	dirty := false

	defer wg.Done()

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
